package askcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	mcpaugment "github.com/Airgap-Castaways/deck/internal/askaugment/mcp"
	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askrepair"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
)

const (
	agentRuntimeMinTurns             = 4
	agentRuntimeMaxTurns             = 30
	agentRuntimeTurnsPerVerification = 3
)

type authoringClarificationError struct {
	Question string
	Reason   string
}

func (e authoringClarificationError) Error() string {
	question := strings.TrimSpace(e.Question)
	if question == "" {
		question = "authoring requested clarification"
	}
	if strings.TrimSpace(e.Reason) == "" {
		return question
	}
	return question + ": " + strings.TrimSpace(e.Reason)
}

type agentToolResult struct {
	OK      bool
	Summary string
	Payload any
}

type authoringAgentSession struct {
	root                string
	requestText         string
	decision            askintent.Decision
	plan                askcontract.PlanResponse
	requirements        askpolicy.ScenarioRequirements
	workspace           askretrieve.WorkspaceSummary
	state               askstate.Context
	retrieval           askretrieve.RetrievalResult
	effective           askconfig.EffectiveSettings
	evidencePlan        askcontract.EvidencePlan
	logger              askLogger
	approvedPaths       map[string]bool
	approvedPathList    []string
	availableTools      []string
	candidateByPath     map[string]askcontract.GeneratedFile
	readState           map[string]authorReadSnapshot
	readCount           map[string]int
	schemaCache         map[string]agentToolResult
	toolEvents          []askstate.AgentToolEvent
	lastLintSummary     string
	lastLintPassed      bool
	lastCritic          askcontract.CriticResponse
	lastDiagnostics     []askdiagnostic.Diagnostic
	verificationBudget  int
	verificationFailure int
	turnBudget          int
	turnsUsed           int
	postValidateIdle    int
	scaffoldRequested   bool
	allowMCPTool        bool
}

func maybeExecuteAuthoringRuntime(ctx context.Context, opts Options, client askprovider.Client, effective askconfig.EffectiveSettings, logger askLogger, progress askProgress, requestText string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary, state askstate.Context, retrieval askretrieve.RetrievalResult, evidencePlan askcontract.EvidencePlan, result runResult, resumedPlan *askcontract.PlanResponse) (runResult, bool, error) {
	if !isAuthoringRoute(decision.Route) || opts.PlanOnly {
		return result, false, nil
	}
	if !canUseLLM(effective) {
		return result, true, fmt.Errorf("route %s requires model access; configure provider credentials first", decision.Route)
	}
	progress.status("running authoring preflight")
	plan, requirements := askpolicy.BuildAuthoringPreflight(requestText, retrieval, workspace, decision, resumedPlan)
	result.Plan = &plan
	if askpolicy.PlanNeedsClarification(plan) {
		progress.status("waiting for clarification")
		if interactiveSessionProbe(opts.Stdin, opts.Stdout) {
			updatedPlan, aborted, err := runInteractiveClarifications(opts.Stdin, opts.Stdout, plan)
			if err != nil {
				return result, true, err
			}
			plan = updatedPlan
			result.Plan = &plan
			if aborted {
				result.Summary = "authoring clarification stopped"
				result.Termination = "authoring-clarification-aborted"
				result.FallbackNote = "rerun the request and answer the clarification prompts to continue"
				result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
				return result, true, nil
			}
		}
		if askpolicy.PlanNeedsClarification(plan) {
			result.Summary = "authoring needs clarification"
			result.Termination = "authoring-awaiting-clarification"
			result.FallbackNote = "answer the clarification prompts interactively or make the request more specific"
			result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
			return result, true, nil
		}
	}
	verificationBudget := generationAttempts(opts.MaxIterations, decision, requestText)
	turnBudget := maxInt(agentRuntimeMinTurns, verificationBudget*agentRuntimeTurnsPerVerification+2)
	if turnBudget < agentRuntimeMaxTurns {
		turnBudget = agentRuntimeMaxTurns
	}
	session := newAuthoringAgentSession(workspace.Root, requestText, decision, plan, requirements, workspace, state, retrieval, effective, evidencePlan, logger, verificationBudget, turnBudget)
	request := askprovider.Request{
		Kind:                     "generate-fast",
		Provider:                 effective.Provider,
		Model:                    effective.Model,
		APIKey:                   effective.APIKey,
		OAuthToken:               effective.OAuthToken,
		AccountID:                effective.AccountID,
		Endpoint:                 effective.Endpoint,
		Tools:                    authoringToolDefinitions(session),
		ToolChoiceRequired:       true,
		DisableParallelToolCalls: true,
		MaxRetries:               providerRetryCount("generate-fast"),
	}
	progress.status("authoring workflow files")
	summary, files, lintSummary, critic, err := runAuthoringAgentRuntime(ctx, client, request, session)
	transcriptPath, saveErr := session.persist()
	if saveErr == nil {
		result.ToolTranscriptPath = transcriptPath
	}
	if err != nil {
		var clarifyErr authoringClarificationError
		if errors.As(err, &clarifyErr) {
			result.Summary = "authoring needs clarification"
			result.Termination = "authoring-awaiting-clarification"
			result.FallbackNote = strings.TrimSpace(clarifyErr.Reason)
			result.ReviewLines = append(result.ReviewLines, strings.TrimSpace(clarifyErr.Question))
			result.ApprovedPaths = append([]string(nil), session.approvedPathList...)
			result.ToolCalls = session.toolCallNames()
			result.CandidateFiles = filePaths(session.candidateFiles())
			return result, true, nil
		}
		if saveErr != nil {
			return result, true, saveErr
		}
		return result, true, err
	}
	if saveErr != nil {
		return result, true, saveErr
	}
	result.LLMUsed = true
	result.RetriesUsed = session.verificationFailure
	result.Files = files
	result.Summary = strings.TrimSpace(summary)
	result.ReviewLines = append(result.ReviewLines, "used bounded authoring agent runtime")
	result.ReviewLines = append(result.ReviewLines, fmt.Sprintf("tool loop turns: %d", session.turnsUsed))
	result.LintSummary = lintSummary
	result.LocalFindings = localFindings(result.Files)
	result.ApprovedPaths = append([]string(nil), session.approvedPathList...)
	result.ToolCalls = session.toolCallNames()
	result.CandidateFiles = filePaths(session.candidateFiles())
	if len(critic.Blocking) > 0 || len(critic.Advisory) > 0 || len(critic.RequiredFixes) > 0 {
		result.Critic = &critic
	}
	result.ReviewLines = append(result.ReviewLines, critic.Advisory...)
	if err := writeFiles(workspace.Root, result.Files); err != nil {
		return result, true, err
	}
	result.WroteFiles = len(result.Files) > 0
	if session.verificationFailure > 0 {
		result.Termination = "generated-after-repair"
	} else {
		result.Termination = "generated"
	}
	return result, true, nil
}

func newAuthoringAgentSession(root string, requestText string, decision askintent.Decision, plan askcontract.PlanResponse, requirements askpolicy.ScenarioRequirements, workspace askretrieve.WorkspaceSummary, state askstate.Context, retrieval askretrieve.RetrievalResult, effective askconfig.EffectiveSettings, evidencePlan askcontract.EvidencePlan, logger askLogger, verificationBudget int, turnBudget int) *authoringAgentSession {
	approved := allowedPlanPaths(plan)
	approvedList := make([]string, 0, len(approved))
	for path := range approved {
		approvedList = append(approvedList, path)
	}
	sort.Strings(approvedList)
	tools := []string{"read", "glob", "grep", "file_write", "file_edit", "validate", "schema"}
	if decision.Route == askintent.RouteDraft && !workspace.HasWorkflowTree {
		tools = append(tools, "init")
	}
	allowMCPTool := effective.MCP.Enabled && strings.TrimSpace(evidencePlan.Decision) != "unnecessary"
	if allowMCPTool {
		tools = append(tools, "web_search")
	}
	return &authoringAgentSession{
		root:               root,
		requestText:        strings.TrimSpace(requestText),
		decision:           decision,
		plan:               plan,
		requirements:       requirements,
		workspace:          workspace,
		state:              state,
		retrieval:          retrieval,
		effective:          effective,
		evidencePlan:       evidencePlan,
		logger:             logger,
		approvedPaths:      approved,
		approvedPathList:   approvedList,
		availableTools:     tools,
		candidateByPath:    map[string]askcontract.GeneratedFile{},
		readState:          map[string]authorReadSnapshot{},
		readCount:          map[string]int{},
		schemaCache:        map[string]agentToolResult{},
		verificationBudget: verificationBudget,
		turnBudget:         turnBudget,
		allowMCPTool:       allowMCPTool,
	}
}

func runAuthoringAgentRuntime(ctx context.Context, client askprovider.Client, base askprovider.Request, session *authoringAgentSession) (string, []askcontract.GeneratedFile, string, askcontract.CriticResponse, error) {
	for turn := 1; turn <= session.turnBudget; turn++ {
		session.turnsUsed = turn
		systemPrompt := authoringAgentSystemPrompt(session)
		userPrompt := authoringAgentUserPrompt(session, turn)
		req := base
		req.SystemPrompt = systemPrompt
		req.Prompt = userPrompt
		req.Tools = authoringToolDefinitions(session)
		req.Timeout = askRequestTimeout(req.Kind, session.turnBudget, systemPrompt, userPrompt)
		session.logger.prompt("author-runtime", systemPrompt, userPrompt)
		resp, err := client.Generate(ctx, req)
		if err != nil {
			return "", nil, session.lastLintSummary, session.lastCritic, err
		}
		session.logger.response("author-runtime", renderProviderResponse(resp))
		if len(resp.ToolCalls) == 0 {
			return "", nil, session.lastLintSummary, session.lastCritic, fmt.Errorf("authoring response did not include a native tool call")
		}
		finishedSummary, finished, err := session.handleToolCalls(ctx, turn, resp.ToolCalls)
		if err != nil {
			var clarifyErr authoringClarificationError
			if errors.As(err, &clarifyErr) {
				return "", nil, session.lastLintSummary, session.lastCritic, clarifyErr
			}
			return "", nil, session.lastLintSummary, session.lastCritic, err
		}
		if finished {
			files := session.candidateFiles()
			return finishedSummary, files, session.lastLintSummary, session.lastCritic, nil
		}
		if session.shouldAutoFinish() {
			files := session.candidateFiles()
			return "generated workflows", files, session.lastLintSummary, session.lastCritic, nil
		}
		if session.verificationFailure > session.verificationBudget {
			return "", nil, session.lastLintSummary, session.lastCritic, fmt.Errorf("authoring exceeded verification failure budget after %d lint failures", session.verificationFailure)
		}
	}
	return "", nil, session.lastLintSummary, session.lastCritic, fmt.Errorf("authoring tool loop exhausted after %d turns", session.turnBudget)
}

func (s *authoringAgentSession) handleToolCalls(ctx context.Context, turn int, calls []askprovider.ToolCall) (string, bool, error) {
	readOnlyTurn := len(calls) > 0
	for _, call := range calls {
		switch strings.TrimSpace(call.Name) {
		case authorToolClarification:
			if len(calls) != 1 {
				return "", false, fmt.Errorf("clarification tool must be the only call in a turn")
			}
			clarify, err := parseAuthorClarificationTool(call)
			if err != nil {
				return "", false, err
			}
			return "", false, authoringClarificationError(clarify)
		case authorToolFinish:
			if len(calls) != 1 {
				return "", false, fmt.Errorf("finish tool must be the only call in a turn")
			}
			finish, err := parseAuthorFinishTool(call)
			if err != nil {
				return "", false, err
			}
			if !s.lastLintPassed {
				s.appendSyntheticFailure(turn, authorToolFinish, "finish rejected until deck_lint succeeds in this session")
				return "", false, nil
			}
			files := s.candidateFiles()
			if len(files) == 0 && s.decision.Route != askintent.RouteRefine {
				s.appendSyntheticFailure(turn, authorToolFinish, "finish rejected because no candidate files have been written")
				return "", false, nil
			}
			summary := strings.TrimSpace(finish.Summary)
			if summary == "" {
				summary = strings.TrimSpace(finish.Reason)
			}
			if summary == "" {
				summary = "generated workflows"
			}
			return summary, true, nil
		default:
			toolCall, err := parseAuthorToolCall(call)
			if err != nil {
				s.appendSyntheticFailure(turn, strings.TrimSpace(call.Name), err.Error())
				continue
			}
			if !isReadOnlyAuthorTool(toolCall.Name) {
				readOnlyTurn = false
			}
			s.executeTool(ctx, turn, toolCall)
		}
	}
	if s.lastLintPassed && readOnlyTurn {
		s.postValidateIdle++
	} else if !readOnlyTurn {
		s.postValidateIdle = 0
	}
	return "", false, nil
}

func isReadOnlyAuthorTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "read", "glob", "grep", "schema", "web_search":
		return true
	default:
		return false
	}
}

func (s *authoringAgentSession) shouldAutoFinish() bool {
	return s.lastLintPassed && len(s.candidateByPath) > 0 && s.postValidateIdle >= 2
}

func (s *authoringAgentSession) executeTool(ctx context.Context, turn int, call authorToolCall) {
	startedAt := time.Now().UTC()
	var result agentToolResult
	switch call.Name {
	case "read":
		result = s.runRead(call)
	case "glob":
		result = s.runGlob(call)
	case "grep":
		result = s.runGrep(call)
	case "file_write":
		result = s.runFileWrite(call)
	case "file_edit":
		result = s.runFileEdit(call)
	case "init":
		result = s.runInit()
	case "validate":
		result = s.runValidate(ctx, call)
	case "schema":
		result = s.runSchema(call)
	case "web_search":
		result = s.runWebSearch(ctx, call)
	default:
		result = agentToolResult{OK: false, Summary: fmt.Sprintf("unsupported tool %s", call.Name), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("unsupported tool %s", call.Name)}}
	}
	event := askstate.AgentToolEvent{
		Turn:      turn,
		Name:      call.Name,
		Path:      strings.TrimSpace(call.Path),
		Paths:     append([]string(nil), call.Paths...),
		Query:     strings.TrimSpace(call.Query),
		Include:   []string{strings.TrimSpace(call.Glob)},
		Intent:    strings.TrimSpace(call.Intent),
		OK:        result.OK,
		Summary:   strings.TrimSpace(result.Summary),
		Result:    renderAgentPayload(result.Payload),
		CreatedAt: startedAt,
	}
	s.toolEvents = append(s.toolEvents, event)
}

func (s *authoringAgentSession) runRead(call authorToolCall) agentToolResult {
	paths := authorToolPaths(call)
	results := make([]authorReadResult, 0, len(paths))
	hints := []string{}
	for _, rawPath := range paths {
		path, err := s.normalizePath(rawPath)
		if err != nil {
			return agentToolResult{OK: false, Summary: "read rejected path", Payload: map[string]any{"ok": false, "error": err.Error()}}
		}
		if !s.readAllowed(path) {
			return agentToolResult{OK: false, Summary: fmt.Sprintf("read denied for %s", path), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("path %s is outside the approved read scope", path)}}
		}
		s.readCount[path]++
		if s.readCount[path] >= 3 {
			hints = append(hints, fmt.Sprintf("hint: %s already read %d times — use file_edit to modify", path, s.readCount[path]))
		}
		content, exists, source, err := s.currentFileContent(path)
		if err != nil {
			return agentToolResult{OK: false, Summary: "read failed", Payload: map[string]any{"ok": false, "error": err.Error()}}
		}
		result := authorReadResult{Path: path, Exists: exists, Source: source, WasCandidate: source == "candidate"}
		if exists {
			result.Content, result.StartLine, result.EndLine, result.TotalLines, result.Truncated = renderReadWindow(content, call.Offset, call.Limit)
			if call.Offset <= 0 && call.Limit <= 0 {
				s.readState[path] = authorReadSnapshot{Content: content, Exists: true, Candidate: source == "candidate", WasFullRead: true}
			}
		}
		results = append(results, result)
	}
	payload := map[string]any{"ok": true, "files": results}
	if len(hints) > 0 {
		payload["hints"] = hints
	}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("read returned %d files", len(results)), Payload: payload}
}

func (s *authoringAgentSession) runGlob(call authorToolCall) agentToolResult {
	pattern := strings.TrimSpace(call.Pattern)
	if pattern == "" {
		return agentToolResult{OK: false, Summary: "glob requires pattern", Payload: map[string]any{"ok": false, "error": "glob requires pattern"}}
	}
	base := strings.TrimSpace(call.Path)
	matches := []string{}
	seen := map[string]bool{}
	for _, file := range s.listWorkspaceFiles(base) {
		if seen[file.path] || !matchesGlob(file.path, pattern) {
			continue
		}
		seen[file.path] = true
		matches = append(matches, file.path)
	}
	sort.Strings(matches)
	truncated := false
	if len(matches) > 100 {
		matches = matches[:100]
		truncated = true
	}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("glob found %d files", len(matches)), Payload: map[string]any{"ok": true, "pattern": pattern, "path": base, "files": matches, "truncated": truncated}}
}

func (s *authoringAgentSession) runGrep(call authorToolCall) agentToolResult {
	pattern := strings.TrimSpace(call.Pattern)
	if pattern == "" {
		return agentToolResult{OK: false, Summary: "grep requires pattern", Payload: map[string]any{"ok": false, "error": "grep requires pattern"}}
	}
	re, err := compileAuthorRegex(pattern)
	if err != nil {
		return agentToolResult{OK: false, Summary: "grep failed", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	limit := call.Limit
	if limit <= 0 {
		limit = 20
	}
	matches := []authorGrepMatch{}
	for _, file := range s.listWorkspaceFiles(call.Path) {
		if call.Glob != "" && !matchesGlob(file.path, call.Glob) {
			continue
		}
		for idx, line := range strings.Split(file.content, "\n") {
			loc := re.FindStringIndex(line)
			if loc == nil {
				continue
			}
			matches = append(matches, authorGrepMatch{Path: file.path, Line: idx + 1, Match: truncateString(strings.TrimSpace(line), 240), Snippet: truncateString(snippetAround(line, loc[0], loc[1]-loc[0]), 240), Source: file.source, Candidate: file.source == "candidate"})
			if len(matches) >= limit {
				return agentToolResult{OK: true, Summary: fmt.Sprintf("grep found %d matches", len(matches)), Payload: map[string]any{"ok": true, "pattern": pattern, "matches": matches, "truncated": true}}
			}
		}
	}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("grep found %d matches", len(matches)), Payload: map[string]any{"ok": true, "pattern": pattern, "matches": matches}}
}

func (s *authoringAgentSession) runFileWrite(call authorToolCall) agentToolResult {
	s.postValidateIdle = 0
	path, err := s.normalizePath(call.Path)
	if err != nil {
		return agentToolResult{OK: false, Summary: "file_write rejected path", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	if !s.writeAllowed(path) {
		return agentToolResult{OK: false, Summary: fmt.Sprintf("file_write denied for %s", path), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("path %s is outside the approved write scope", path)}}
	}
	file := askcontract.GeneratedFile{Path: path, Content: normalizeGeneratedContent(path, call.Content)}
	s.candidateByPath[path] = file
	s.readState[path] = authorReadSnapshot{Content: file.Content, Exists: true, Candidate: true, WasFullRead: true}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("staged %s in candidate state", path), Payload: map[string]any{"ok": true, "path": path, "bytes": len(file.Content), "lines": countLines(file.Content)}}
}

func (s *authoringAgentSession) runFileEdit(call authorToolCall) agentToolResult {
	s.postValidateIdle = 0
	path, err := s.normalizePath(call.Path)
	if err != nil {
		return agentToolResult{OK: false, Summary: "file_edit rejected path", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	if !s.writeAllowed(path) {
		return agentToolResult{OK: false, Summary: fmt.Sprintf("file_edit denied for %s", path), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("path %s is outside the approved write scope", path)}}
	}
	snapshot, ok := s.readState[path]
	if !ok || !snapshot.WasFullRead {
		return agentToolResult{OK: false, Summary: "file_edit requires a prior full read", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("read %s fully before editing it", path)}}
	}
	current, exists, _, err := s.currentFileContent(path)
	if err != nil {
		return agentToolResult{OK: false, Summary: "file_edit failed", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	if exists != snapshot.Exists || current != snapshot.Content {
		return agentToolResult{OK: false, Summary: "file_edit detected stale content", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("%s changed since the last full read; read it again before editing", path)}}
	}
	if !exists {
		return agentToolResult{OK: false, Summary: "file_edit failed", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("%s does not exist; use file_write to create it", path)}}
	}
	if call.OldString == call.NewString {
		return agentToolResult{OK: false, Summary: "file_edit would not change the file", Payload: map[string]any{"ok": false, "error": "old_string and new_string are identical"}}
	}
	count := strings.Count(current, call.OldString)
	if count == 0 {
		return agentToolResult{OK: false, Summary: "file_edit target not found", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("old_string was not found in %s", path)}}
	}
	if count > 1 && !call.ReplaceAll {
		return agentToolResult{OK: false, Summary: "file_edit target is ambiguous", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("old_string matched %d times in %s; use replace_all or make the target more specific", count, path)}}
	}
	updated := strings.Replace(current, call.OldString, call.NewString, 1)
	if call.ReplaceAll {
		updated = strings.ReplaceAll(current, call.OldString, call.NewString)
	}
	updated = normalizeGeneratedContent(path, updated)
	s.candidateByPath[path] = askcontract.GeneratedFile{Path: path, Content: updated}
	s.readState[path] = authorReadSnapshot{Content: updated, Exists: true, Candidate: true, WasFullRead: true}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("edited %s in candidate state", path), Payload: map[string]any{"ok": true, "path": path, "replacements": countIf(call.ReplaceAll, count, 1), "bytes": len(updated)}}
}

func (s *authoringAgentSession) runInit() agentToolResult {
	s.postValidateIdle = 0
	if s.workspace.HasWorkflowTree {
		return agentToolResult{OK: false, Summary: "init is disabled", Payload: map[string]any{"ok": false, "error": "init is disabled because the workspace already has a workflow tree"}}
	}
	s.scaffoldRequested = true
	return agentToolResult{OK: true, Summary: "prepared default workspace scaffold", Payload: map[string]any{"ok": true, "createdDirectories": []string{"workflows/scenarios", "workflows/components", "outputs/files", "outputs/images", "outputs/packages"}, "createdFiles": []string{".gitignore", ".deckignore"}}}
}

func (s *authoringAgentSession) runValidate(ctx context.Context, call authorToolCall) agentToolResult {
	s.postValidateIdle = 0
	files := s.lintFiles()
	if len(files) == 0 {
		s.lastLintPassed = false
		s.lastLintSummary = ""
		s.lastCritic = askcontract.CriticResponse{Blocking: []string{"no candidate or approved workflow files are available to lint"}}
		s.lastDiagnostics = []askdiagnostic.Diagnostic{{Code: "no_candidate_files", Severity: "blocking", Message: "no candidate or approved workflow files are available to lint"}}
		return agentToolResult{OK: false, Summary: "validate could not run", Payload: map[string]any{"ok": false, "summary": "no candidate or approved workflow files are available to lint", "critic": s.lastCritic, "diagnostics": s.lastDiagnostics}}
	}
	lintSummary, critic, err := validateCandidateFiles(ctx, s.root, files, s.decision, s.plan, normalizedAuthoringBrief(s.plan, s.plan.AuthoringBrief), s.retrieval)
	s.lastCritic = critic
	if err != nil {
		s.lastLintPassed = false
		s.lastLintSummary = strings.TrimSpace(err.Error())
		s.lastDiagnostics = askdiagnostic.FromValidationError(err, err.Error(), askcontext.CurrentBundle())
		if repaired, notes, repairedSummary, repairedCritic, repairedErr, applied := s.tryAutoRepairAfterLintFailure(ctx, files, s.lastDiagnostics); applied {
			critic = repairedCritic
			if repairedErr == nil {
				return agentToolResult{OK: true, Summary: repairedSummary, Payload: map[string]any{"ok": true, "summary": repairedSummary, "critic": critic, "selectedPaths": authorToolPaths(call), "autoRepair": notes, "repairContext": buildLintRepairContext(s.lastDiagnostics), "repairedFiles": filePaths(repaired)}}
			}
			s.lastLintPassed = false
			s.lastLintSummary = strings.TrimSpace(repairedErr.Error())
			s.lastDiagnostics = askdiagnostic.FromValidationError(repairedErr, repairedErr.Error(), askcontext.CurrentBundle())
		}
		s.verificationFailure++
		return agentToolResult{OK: false, Summary: "validate found blocking issues", Payload: map[string]any{"ok": false, "summary": s.lastLintSummary, "critic": critic, "diagnostics": s.lastDiagnostics, "selectedPaths": authorToolPaths(call), "repairContext": buildLintRepairContext(s.lastDiagnostics)}}
	}
	s.lastLintPassed = true
	s.lastLintSummary = strings.TrimSpace(lintSummary)
	s.lastDiagnostics = nil
	return agentToolResult{OK: true, Summary: lintSummary, Payload: map[string]any{"ok": true, "summary": lintSummary, "critic": critic, "selectedPaths": authorToolPaths(call)}}
}

func (s *authoringAgentSession) runSchema(call authorToolCall) agentToolResult {
	cacheKey := strings.TrimSpace(call.Topic) + "::" + strings.TrimSpace(call.Kind)
	if cached, ok := s.schemaCache[cacheKey]; ok {
		cached.Summary = cached.Summary + " (cached — schema already retrieved, use file_write or file_edit to proceed)"
		return cached
	}
	payload, err := buildSchemaReadPayload(authorSchemaReadCall{Topic: strings.TrimSpace(call.Topic), Kind: strings.TrimSpace(call.Kind)})
	if err != nil {
		return agentToolResult{OK: false, Summary: "schema failed", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	summary := "returned workflow schema context"
	if strings.TrimSpace(call.Kind) != "" {
		summary = fmt.Sprintf("returned %s schema context", strings.TrimSpace(call.Kind))
	}
	result := agentToolResult{OK: true, Summary: summary, Payload: payload}
	s.schemaCache[cacheKey] = result
	return result
}

func (s *authoringAgentSession) tryAutoRepairAfterLintFailure(ctx context.Context, files []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic) ([]askcontract.GeneratedFile, []string, string, askcontract.CriticResponse, error, bool) {
	repairPaths := s.approvedPathList
	if len(repairPaths) == 0 {
		repairPaths = filePaths(s.candidateFiles())
	}
	repaired, notes, applied, err := askrepair.TryAutoRepairWithProgram(s.root, files, diags, repairPaths, s.plan.AuthoringProgram)
	if err != nil || !applied {
		return files, nil, "", askcontract.CriticResponse{}, err, false
	}
	for _, file := range repaired {
		if s.writeAllowed(file.Path) {
			s.candidateByPath[file.Path] = file
			s.readState[file.Path] = authorReadSnapshot{Content: file.Content, Exists: !file.Delete, Candidate: true, WasFullRead: !file.Delete}
		}
	}
	summary, critic, validateErr := validateCandidateFiles(ctx, s.root, repaired, s.decision, s.plan, normalizedAuthoringBrief(s.plan, s.plan.AuthoringBrief), s.retrieval)
	if validateErr == nil {
		s.lastLintPassed = true
		s.lastLintSummary = strings.TrimSpace(summary)
		s.lastDiagnostics = nil
		s.lastCritic = critic
	}
	return repaired, notes, summary, critic, validateErr, true
}

func (s *authoringAgentSession) runWebSearch(ctx context.Context, call authorToolCall) agentToolResult {
	if !s.allowMCPTool {
		return agentToolResult{OK: false, Summary: "web_search is unavailable", Payload: map[string]any{"ok": false, "error": "web_search is disabled by current evidence policy or config"}}
	}
	chunks, events := mcpaugment.Gather(ctx, s.effective.MCP, s.decision.Route, strings.TrimSpace(call.Query))
	typed := make([]map[string]any, 0, len(chunks))
	for _, chunk := range chunks {
		item := map[string]any{"id": chunk.ID, "label": chunk.Label, "content": strings.TrimSpace(chunk.Content)}
		if chunk.Evidence != nil {
			item["evidence"] = chunk.Evidence
		}
		typed = append(typed, item)
	}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("web_search returned %d evidence chunks", len(typed)), Payload: map[string]any{"ok": true, "query": strings.TrimSpace(call.Query), "events": events, "chunks": typed}}
}

func (s *authoringAgentSession) currentFileContent(path string) (string, bool, string, error) {
	if file, ok := s.candidateByPath[path]; ok {
		if file.Delete {
			return "", false, "candidate", nil
		}
		return file.Content, true, "candidate", nil
	}
	abs, err := fsutil.ResolveUnder(s.root, strings.Split(filepath.ToSlash(path), "/")...)
	if err != nil {
		return "", false, "", err
	}
	raw, err := os.ReadFile(abs) //nolint:gosec // Path stays under the workspace root.
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, "workspace", nil
		}
		return "", false, "", fmt.Errorf("read %s: %v", path, err)
	}
	return string(raw), true, "workspace", nil
}

func (s *authoringAgentSession) listWorkspaceFiles(scope string) []searchableFile {
	seen := map[string]bool{}
	out := make([]searchableFile, 0, len(s.workspace.Files)+len(s.candidateByPath))
	for _, file := range s.candidateFiles() {
		if scopeAllows(file.Path, scope) {
			seen[file.Path] = true
			out = append(out, searchableFile{path: file.Path, content: file.Content, source: "candidate"})
		}
	}
	for _, workspaceFile := range s.workspace.Files {
		if seen[workspaceFile.Path] || !scopeAllows(workspaceFile.Path, scope) {
			continue
		}
		seen[workspaceFile.Path] = true
		out = append(out, searchableFile{path: workspaceFile.Path, content: workspaceFile.Content, source: "workspace"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func renderReadWindow(content string, offset int, limit int) (string, int, int, int, bool) {
	lines := strings.Split(content, "\n")
	total := len(lines)
	if total == 1 && lines[0] == "" {
		return "", 0, 0, 0, false
	}
	start := 0
	if offset > 0 {
		start = offset - 1
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
	}
	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	selected := lines[start:end]
	numbered := make([]string, 0, len(selected))
	for i, line := range selected {
		numbered = append(numbered, fmt.Sprintf("%d: %s", start+i+1, line))
	}
	return strings.Join(numbered, "\n"), start + 1, end, total, end < len(lines)
}

func compileAuthorRegex(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re, nil
	}
	return nil, fmt.Errorf("compile regex %q: %w", pattern, err)
}

func countIf(replaceAll bool, all int, one int) int {
	if replaceAll {
		return all
	}
	return one
}

func (s *authoringAgentSession) appendSyntheticFailure(turn int, name string, message string) {
	s.toolEvents = append(s.toolEvents, askstate.AgentToolEvent{Turn: turn, Name: name, OK: false, Summary: strings.TrimSpace(message), Result: renderAgentPayload(map[string]any{"ok": false, "error": strings.TrimSpace(message)}), CreatedAt: time.Now().UTC()})
}

func (s *authoringAgentSession) candidateFiles() []askcontract.GeneratedFile {
	files := make([]askcontract.GeneratedFile, 0, len(s.candidateByPath))
	for _, file := range s.candidateByPath {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func (s *authoringAgentSession) lintFiles() []askcontract.GeneratedFile {
	files := append([]askcontract.GeneratedFile(nil), s.candidateFiles()...)
	seen := map[string]bool{}
	for _, file := range files {
		seen[file.Path] = true
	}
	for _, file := range s.baseFilesForLint() {
		if seen[file.Path] {
			continue
		}
		files = append(files, file)
	}
	return normalizeGeneratedFiles(files)
}

func (s *authoringAgentSession) baseFilesForLint() []askcontract.GeneratedFile {
	files := make([]askcontract.GeneratedFile, 0, len(s.approvedPathList))
	for _, path := range s.approvedPathList {
		abs, err := fsutil.ResolveUnder(s.root, strings.Split(filepath.ToSlash(path), "/")...)
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(abs) //nolint:gosec // Path stays under the workspace root. //nolint:gosec // Path stays under the workspace root.
		if err != nil {
			continue
		}
		files = append(files, askcontract.GeneratedFile{Path: path, Content: string(raw)})
	}
	return normalizeGeneratedFiles(files)
}

func (s *authoringAgentSession) toolCallNames() []string {
	if len(s.toolEvents) == 0 {
		return nil
	}
	names := make([]string, 0, len(s.toolEvents))
	for _, event := range s.toolEvents {
		names = append(names, event.Name)
	}
	return names
}

func (s *authoringAgentSession) persist() (string, error) {
	return askstate.SaveAgentSession(s.root, askstate.AgentSession{
		Prompt:               s.requestText,
		Route:                string(s.decision.Route),
		ApprovedPaths:        append([]string(nil), s.approvedPathList...),
		ToolEvents:           append([]askstate.AgentToolEvent(nil), s.toolEvents...),
		CandidateFiles:       append([]askcontract.GeneratedFile(nil), s.candidateFiles()...),
		VerifierSummary:      strings.TrimSpace(s.lastLintSummary),
		VerificationFailures: s.verificationFailure,
		Turns:                s.turnsUsed,
		TerminationReason:    strings.TrimSpace(s.lastTermination()),
	})
}

func (s *authoringAgentSession) lastTermination() string {
	if s.lastLintPassed {
		return "generated"
	}
	if len(s.toolEvents) == 0 {
		return "not-started"
	}
	last := s.toolEvents[len(s.toolEvents)-1]
	if last.Name == "finish" && !last.OK {
		return "finish-rejected"
	}
	if last.Name == "validate" && !last.OK {
		return "lint-blocked"
	}
	return "incomplete"
}

func (s *authoringAgentSession) normalizePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty")
	}
	candidate := trimmed
	if filepath.IsAbs(candidate) {
		rel, err := filepath.Rel(s.root, candidate)
		if err != nil {
			return "", fmt.Errorf("resolve path %s: %w", trimmed, err)
		}
		candidate = rel
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(candidate)))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "..") {
		return "", fmt.Errorf("path %s leaves the workspace", trimmed)
	}
	return clean, nil
}

func (s *authoringAgentSession) writeAllowed(path string) bool {
	return s.approvedPaths[path]
}

func (s *authoringAgentSession) readAllowed(path string) bool {
	if s.approvedPaths[path] {
		return true
	}
	for _, prefix := range []string{"workflows/", "docs/", "test/workflows/", "internal/", "cmd/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

type searchableFile struct {
	path    string
	content string
	source  string
}

func scopeAllows(path string, scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "." {
		return true
	}
	if strings.HasSuffix(scope, "/") {
		return strings.HasPrefix(path, scope)
	}
	return path == scope || strings.HasPrefix(path, scope+"/")
}

func authoringAgentSystemPrompt(session *authoringAgentSession) string {
	b := &strings.Builder{}
	b.WriteString("You are authoring deck workflow files as a constrained CLI agent.\n")
	b.WriteString("Use the provided native tools to inspect workspace state, write candidate files, validate with the validate tool, and converge on the requested result.\n")
	b.WriteString("Treat tool results as the source of truth for scope, validation, and current candidate state.\n")
	b.WriteString("Use ")
	b.WriteString(authorToolFinish)
	b.WriteString(" only when the current candidate state satisfies the request and verifier state is acceptable.\n")
	b.WriteString("Use ")
	b.WriteString(authorToolClarification)
	b.WriteString(" only for true blockers that cannot be safely inferred from available files and tool results.\n")
	b.WriteString("The workspace may be empty. Retrieval excerpts can mention repo paths that are not present under the current workspace root.\n")
	b.WriteString("Prefer tool-driven inspection over assumptions. Keep edits inside the approved paths and let validate drive repairs.\n")
	b.WriteString("Budget discipline: call schema at most once per topic; prefer file_write early over repeated schema lookups. Write candidate files first, then validate and repair.\n")
	return strings.TrimSpace(b.String())
}

func authoringAgentUserPrompt(session *authoringAgentSession, turn int) string {
	b := &strings.Builder{}
	b.WriteString("Request:\n")
	b.WriteString(session.requestText)
	b.WriteString("\n\n")
	b.WriteString("Route: ")
	b.WriteString(string(session.decision.Route))
	b.WriteString("\n")
	b.WriteString("Turn budget remaining: ")
	_, _ = fmt.Fprintf(b, "%d\n", maxInt(session.turnBudget-turn+1, 0))
	b.WriteString("Verification failures: ")
	_, _ = fmt.Fprintf(b, "%d/%d\n", session.verificationFailure, session.verificationBudget)
	b.WriteString("Available tools: ")
	b.WriteString(authoringToolCatalogSummary(session))
	b.WriteString("\n")
	b.WriteString("Approved write paths: ")
	b.WriteString(approvedPathsSummary(session.approvedPathList))
	b.WriteString("\n")
	b.WriteString("Workspace workflow files: ")
	b.WriteString(workspaceSummaryForPrompt(session))
	b.WriteString("\n")
	if len(session.plan.ValidationChecklist) > 0 {
		b.WriteString("Validation checklist:\n")
		for _, item := range session.plan.ValidationChecklist {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	if len(session.retrieval.Chunks) > 0 {
		b.WriteString("\nRetrieved context:\n")
		for _, rendered := range renderAuthoringChunks(session.retrieval.Chunks) {
			b.WriteString(rendered)
			b.WriteString("\n\n")
		}
	}
	if session.state.LastLint != "" {
		b.WriteString("Last saved lint summary: ")
		b.WriteString(session.state.LastLint)
		b.WriteString("\n")
	}
	if len(session.toolEvents) > 0 {
		events := session.toolEvents
		recentStart := 0
		if len(events) > 0 {
			lastTurn := events[len(events)-1].Turn
			cutoff := lastTurn - 4
			for i, event := range events {
				if event.Turn >= cutoff {
					recentStart = i
					break
				}
			}
		}
		if recentStart > 0 {
			_, _ = fmt.Fprintf(b, "\nTool transcript (showing last %d of %d events):\n", len(events)-recentStart, len(events))
		} else {
			b.WriteString("\nTool transcript:\n")
		}
		for i, event := range events {
			_, _ = fmt.Fprintf(b, "Turn %d %s ok=%t\n", event.Turn, event.Name, event.OK)
			if strings.TrimSpace(event.Summary) != "" {
				b.WriteString("Summary: ")
				b.WriteString(strings.TrimSpace(event.Summary))
				b.WriteString("\n")
			}
			if i < recentStart {
				continue
			}
			if strings.TrimSpace(event.Result) != "" {
				if event.Name == "read" && strings.TrimSpace(event.Path) != "" {
					if _, inCandidate := session.candidateByPath[strings.TrimSpace(event.Path)]; inCandidate {
						lines := strings.Count(strings.TrimSpace(event.Result), "\n") + 1
						_, _ = fmt.Fprintf(b, "read %s: %d lines, content in candidate state\n", strings.TrimSpace(event.Path), lines)
						continue
					}
				}
				b.WriteString(truncateString(event.Result, 4000))
				b.WriteString("\n")
			}
		}
	}
	if len(session.candidateByPath) > 0 {
		b.WriteString("\nCurrent candidate files:\n")
		for _, file := range session.candidateFiles() {
			b.WriteString("--- ")
			b.WriteString(file.Path)
			b.WriteString("\n")
			b.WriteString(truncateString(file.Content, 4000))
			if !strings.HasSuffix(file.Content, "\n") {
				b.WriteString("\n")
			}
		}
	}
	if strings.TrimSpace(session.lastLintSummary) != "" {
		b.WriteString("\nLatest verifier summary: ")
		b.WriteString(strings.TrimSpace(session.lastLintSummary))
		b.WriteString("\n")
	}
	if len(session.lastDiagnostics) > 0 {
		b.WriteString("Structured lint diagnostics:\n")
		b.WriteString(truncateString(renderAgentPayload(session.lastDiagnostics), 1600))
		b.WriteString("\n")
	}
	b.WriteString("\nUse tools to inspect, search, read, edit, write, validate, consult schema, finish, or request clarification.\n")
	return strings.TrimSpace(b.String())
}

func renderAuthoringChunks(chunks []askretrieve.Chunk) []string {
	const maxChunks = 5
	const maxChunkContent = 1000
	selected := append([]askretrieve.Chunk(nil), chunks...)
	sort.SliceStable(selected, func(i, j int) bool {
		pi := authoringChunkPriority(selected[i])
		pj := authoringChunkPriority(selected[j])
		if pi != pj {
			return pi < pj
		}
		if selected[i].Score != selected[j].Score {
			return selected[i].Score > selected[j].Score
		}
		return selected[i].Label < selected[j].Label
	})
	if len(selected) > maxChunks {
		selected = selected[:maxChunks]
	}
	out := make([]string, 0, len(selected)+1)
	for _, chunk := range selected {
		b := &strings.Builder{}
		label := strings.TrimSpace(chunk.Label)
		if label == "" {
			label = chunk.ID
		}
		b.WriteString("[")
		b.WriteString(label)
		b.WriteString("]\n")
		b.WriteString(truncateString(strings.TrimSpace(chunk.Content), maxChunkContent))
		out = append(out, strings.TrimSpace(b.String()))
	}
	if len(chunks) > len(selected) {
		out = append(out, fmt.Sprintf("[additional-context]\n%d more retrieval chunks were omitted from the prompt for brevity; use read, glob, or grep only when the current workspace actually contains the target path.", len(chunks)-len(selected)))
	}
	return out
}

func authoringChunkPriority(chunk askretrieve.Chunk) int {
	label := strings.ToLower(strings.TrimSpace(chunk.Label))
	source := strings.ToLower(strings.TrimSpace(chunk.Source))
	switch {
	case strings.HasPrefix(label, "workflows/"):
		return 0
	case label == "workflow-summary" || label == "workspace-topology" || label == "prepare-apply-guidance":
		return 1
	case source == "local-facts":
		return 2
	case source == "example":
		return 3
	default:
		return 4
	}
}

func renderAgentPayload(payload any) string {
	if payload == nil {
		return "{}"
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\n  \"error\": %q\n}", err.Error())
	}
	return string(raw)
}

func snippetAround(content string, idx int, matchLen int) string {
	start := maxInt(idx-80, 0)
	end := minInt(idx+matchLen+120, len(content))
	snippet := strings.TrimSpace(content[start:end])
	return truncateString(strings.Join(strings.Fields(snippet), " "), 240)
}

func truncateString(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
