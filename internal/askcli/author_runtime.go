package askcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

const (
	agentRuntimeMinTurns             = 4
	agentRuntimeTurnsPerVerification = 2
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

type agentSearchMatch struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet,omitempty"`
	Source  string `json:"source,omitempty"`
}

type agentReadResult struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Source  string `json:"source,omitempty"`
	Content string `json:"content,omitempty"`
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
	toolEvents          []askstate.AgentToolEvent
	lastLintSummary     string
	lastLintPassed      bool
	lastCritic          askcontract.CriticResponse
	lastDiagnostics     []askdiagnostic.Diagnostic
	verificationBudget  int
	verificationFailure int
	turnBudget          int
	turnsUsed           int
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
	session := newAuthoringAgentSession(workspace.Root, requestText, decision, plan, requirements, workspace, state, retrieval, effective, evidencePlan, logger, verificationBudget, turnBudget)
	request := askprovider.Request{
		Kind:               "generate-fast",
		Provider:           effective.Provider,
		Model:              effective.Model,
		APIKey:             effective.APIKey,
		OAuthToken:         effective.OAuthToken,
		AccountID:          effective.AccountID,
		Endpoint:           effective.Endpoint,
		ResponseSchema:     askcontract.AgentTurnResponseSchema(),
		ResponseSchemaName: "deck_agent_turn_response",
		MaxRetries:         providerRetryCount("generate-fast"),
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
	tools := []string{"file_search", "file_read", "file_write", "deck_lint"}
	if decision.Route == askintent.RouteDraft && !workspace.HasWorkflowTree {
		tools = append(tools, "deck_init")
	}
	allowMCPTool := effective.MCP.Enabled && strings.TrimSpace(evidencePlan.Decision) != "unnecessary"
	if allowMCPTool {
		tools = append(tools, "mcp_web_search")
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
		req.Timeout = askRequestTimeout(req.Kind, session.turnBudget, systemPrompt, userPrompt)
		session.logger.prompt("author-runtime", systemPrompt, userPrompt)
		resp, err := client.Generate(ctx, req)
		if err != nil {
			return "", nil, session.lastLintSummary, session.lastCritic, err
		}
		session.logger.response("author-runtime", resp.Content)
		turnResp, err := askcontract.ParseAgentTurn(resp.Content)
		if err != nil {
			return "", nil, session.lastLintSummary, session.lastCritic, err
		}
		if turnResp.Clarification != nil {
			return "", nil, session.lastLintSummary, session.lastCritic, authoringClarificationError{Question: turnResp.Clarification.Question, Reason: turnResp.Clarification.Reason}
		}
		if len(turnResp.ToolCalls) > 0 {
			for _, call := range turnResp.ToolCalls {
				session.executeTool(ctx, turn, call)
			}
			if session.verificationFailure > session.verificationBudget {
				return "", nil, session.lastLintSummary, session.lastCritic, fmt.Errorf("authoring exceeded verification failure budget after %d lint failures", session.verificationFailure)
			}
			continue
		}
		if turnResp.Finish != nil {
			if !session.lastLintPassed {
				session.appendSyntheticFailure(turn, "finish", "finish rejected until deck_lint succeeds in this session")
				continue
			}
			files := session.candidateFiles()
			if len(files) == 0 && session.decision.Route != askintent.RouteRefine {
				session.appendSyntheticFailure(turn, "finish", "finish rejected because no candidate files have been written")
				continue
			}
			summary := strings.TrimSpace(turnResp.Summary)
			if summary == "" {
				summary = "generated workflows"
			}
			return summary, files, session.lastLintSummary, session.lastCritic, nil
		}
	}
	return "", nil, session.lastLintSummary, session.lastCritic, fmt.Errorf("authoring tool loop exhausted after %d turns", session.turnBudget)
}

func (s *authoringAgentSession) executeTool(ctx context.Context, turn int, call askcontract.AgentToolCall) {
	startedAt := time.Now().UTC()
	var result agentToolResult
	switch call.Name {
	case "file_search":
		result = s.runFileSearch(call)
	case "file_read":
		result = s.runFileRead(call)
	case "file_write":
		result = s.runFileWrite(call)
	case "deck_init":
		result = s.runDeckInit()
	case "deck_lint":
		result = s.runDeckLint(ctx, call)
	case "mcp_web_search":
		result = s.runMCPWebSearch(ctx, call)
	default:
		result = agentToolResult{OK: false, Summary: fmt.Sprintf("unsupported tool %s", call.Name), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("unsupported tool %s", call.Name)}}
	}
	event := askstate.AgentToolEvent{
		Turn:      turn,
		Name:      call.Name,
		Path:      strings.TrimSpace(call.Path),
		Paths:     append([]string(nil), call.Paths...),
		Query:     strings.TrimSpace(call.Query),
		Include:   append([]string(nil), call.Include...),
		Intent:    strings.TrimSpace(call.Intent),
		OK:        result.OK,
		Summary:   strings.TrimSpace(result.Summary),
		Result:    renderAgentPayload(result.Payload),
		CreatedAt: startedAt,
	}
	s.toolEvents = append(s.toolEvents, event)
}

func (s *authoringAgentSession) runFileSearch(call askcontract.AgentToolCall) agentToolResult {
	query := strings.TrimSpace(call.Query)
	if query == "" {
		query = strings.TrimSpace(call.Path)
	}
	if query == "" && len(call.Paths) > 0 {
		query = strings.TrimSpace(call.Paths[0])
	}
	matches := []agentSearchMatch{}
	for _, item := range s.searchableFiles(call.Path, call.Include) {
		if snippet, ok := matchSnippet(item.content, item.path, query); ok {
			matches = append(matches, agentSearchMatch{Path: item.path, Snippet: snippet, Source: item.source})
		}
		if len(matches) >= 6 {
			break
		}
	}
	return agentToolResult{
		OK:      true,
		Summary: fmt.Sprintf("file_search found %d matches", len(matches)),
		Payload: map[string]any{"ok": true, "query": query, "matches": matches},
	}
}

func (s *authoringAgentSession) runFileRead(call askcontract.AgentToolCall) agentToolResult {
	paths := callPaths(call)
	results := make([]agentReadResult, 0, len(paths))
	for _, rawPath := range paths {
		path, err := s.normalizePath(rawPath)
		if err != nil {
			return agentToolResult{OK: false, Summary: "file_read rejected path", Payload: map[string]any{"ok": false, "error": err.Error()}}
		}
		if !s.readAllowed(path) {
			return agentToolResult{OK: false, Summary: fmt.Sprintf("file_read denied for %s", path), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("path %s is outside the approved read scope", path)}}
		}
		if file, ok := s.candidateByPath[path]; ok {
			results = append(results, agentReadResult{Path: path, Exists: true, Source: "candidate", Content: file.Content})
			continue
		}
		abs, err := fsutil.ResolveUnder(s.root, strings.Split(filepath.ToSlash(path), "/")...)
		if err != nil {
			return agentToolResult{OK: false, Summary: "file_read failed", Payload: map[string]any{"ok": false, "error": err.Error()}}
		}
		raw, err := os.ReadFile(abs) //nolint:gosec // Path stays under the workspace root.
		if err != nil {
			if os.IsNotExist(err) {
				results = append(results, agentReadResult{Path: path, Exists: false})
				continue
			}
			return agentToolResult{OK: false, Summary: "file_read failed", Payload: map[string]any{"ok": false, "error": fmt.Sprintf("read %s: %v", path, err)}}
		}
		results = append(results, agentReadResult{Path: path, Exists: true, Source: "workspace", Content: string(raw)})
	}
	return agentToolResult{OK: true, Summary: fmt.Sprintf("file_read returned %d files", len(results)), Payload: map[string]any{"ok": true, "files": results}}
}

func (s *authoringAgentSession) runFileWrite(call askcontract.AgentToolCall) agentToolResult {
	path, err := s.normalizePath(call.Path)
	if err != nil {
		return agentToolResult{OK: false, Summary: "file_write rejected path", Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	if !s.writeAllowed(path) {
		return agentToolResult{OK: false, Summary: fmt.Sprintf("file_write denied for %s", path), Payload: map[string]any{"ok": false, "error": fmt.Sprintf("path %s is outside the approved write scope", path)}}
	}
	file := askcontract.GeneratedFile{Path: path, Content: normalizeGeneratedContent(path, call.Content)}
	if err := validateGeneratedFile(s.root, file); err != nil {
		return agentToolResult{OK: false, Summary: fmt.Sprintf("file_write rejected for %s", path), Payload: map[string]any{"ok": false, "error": err.Error()}}
	}
	s.candidateByPath[path] = file
	return agentToolResult{OK: true, Summary: fmt.Sprintf("staged %s in candidate state", path), Payload: map[string]any{"ok": true, "path": path, "bytes": len(file.Content), "lines": countLines(file.Content)}}
}

func (s *authoringAgentSession) runDeckInit() agentToolResult {
	if s.workspace.HasWorkflowTree {
		return agentToolResult{OK: false, Summary: "deck_init is disabled", Payload: map[string]any{"ok": false, "error": "deck_init is disabled because the workspace already has a workflow tree"}}
	}
	s.scaffoldRequested = true
	return agentToolResult{OK: true, Summary: "prepared default workspace scaffold", Payload: map[string]any{"ok": true, "createdDirectories": []string{"workflows/scenarios", "workflows/components", "outputs/files", "outputs/images", "outputs/packages"}, "createdFiles": []string{".gitignore", ".deckignore"}}}
}

func (s *authoringAgentSession) runDeckLint(ctx context.Context, call askcontract.AgentToolCall) agentToolResult {
	files := s.lintFiles()
	if len(files) == 0 {
		s.lastLintPassed = false
		s.lastLintSummary = ""
		s.lastCritic = askcontract.CriticResponse{Blocking: []string{"no candidate or approved workflow files are available to lint"}}
		s.lastDiagnostics = []askdiagnostic.Diagnostic{{Code: "no_candidate_files", Severity: "blocking", Message: "no candidate or approved workflow files are available to lint"}}
		return agentToolResult{OK: false, Summary: "deck_lint could not run", Payload: map[string]any{"ok": false, "summary": "no candidate or approved workflow files are available to lint", "critic": s.lastCritic, "diagnostics": s.lastDiagnostics}}
	}
	gen := askcontract.GenerationResponse{Summary: "agent candidate", Files: append([]askcontract.GeneratedFile(nil), files...)}
	lintSummary, critic, err := validateGeneration(ctx, s.root, gen, files, s.decision, s.plan, normalizedAuthoringBrief(s.plan, s.plan.AuthoringBrief), s.retrieval)
	s.lastCritic = critic
	if err != nil {
		s.lastLintPassed = false
		s.lastLintSummary = strings.TrimSpace(err.Error())
		s.lastDiagnostics = askdiagnostic.FromValidationError(err, err.Error(), askcontext.CurrentBundle())
		s.verificationFailure++
		return agentToolResult{OK: false, Summary: "deck_lint found blocking issues", Payload: map[string]any{"ok": false, "summary": s.lastLintSummary, "critic": critic, "diagnostics": s.lastDiagnostics, "selectedPaths": callPaths(call)}}
	}
	s.lastLintPassed = true
	s.lastLintSummary = strings.TrimSpace(lintSummary)
	s.lastDiagnostics = nil
	return agentToolResult{OK: true, Summary: lintSummary, Payload: map[string]any{"ok": true, "summary": lintSummary, "critic": critic, "selectedPaths": callPaths(call)}}
}

func (s *authoringAgentSession) runMCPWebSearch(ctx context.Context, call askcontract.AgentToolCall) agentToolResult {
	if !s.allowMCPTool {
		return agentToolResult{OK: false, Summary: "mcp_web_search is unavailable", Payload: map[string]any{"ok": false, "error": "mcp_web_search is disabled by current evidence policy or config"}}
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
	return agentToolResult{OK: true, Summary: fmt.Sprintf("mcp_web_search returned %d evidence chunks", len(typed)), Payload: map[string]any{"ok": true, "query": strings.TrimSpace(call.Query), "events": events, "chunks": typed}}
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
		raw, err := os.ReadFile(abs) //nolint:gosec // Path stays under the workspace root.
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
	if last.Name == "deck_lint" && !last.OK {
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

func (s *authoringAgentSession) searchableFiles(scope string, include []string) []searchableFile {
	seen := map[string]bool{}
	out := make([]searchableFile, 0, len(s.workspace.Files)+len(s.candidateByPath))
	for _, file := range s.candidateFiles() {
		if scopeAllows(file.Path, scope) && matchesIncludes(file.Path, include) {
			seen[file.Path] = true
			out = append(out, searchableFile{path: file.Path, content: file.Content, source: "candidate"})
		}
	}
	for _, workspaceFile := range s.workspace.Files {
		if seen[workspaceFile.Path] || !scopeAllows(workspaceFile.Path, scope) || !matchesIncludes(workspaceFile.Path, include) {
			continue
		}
		seen[workspaceFile.Path] = true
		out = append(out, searchableFile{path: workspaceFile.Path, content: workspaceFile.Content, source: "workspace"})
	}
	for _, prefix := range []string{"workflows", "docs", "test/workflows", "internal/stepspec", "internal/stepmeta", "internal/askpolicy", "internal/askdraft", "cmd"} {
		base := filepath.Join(s.root, filepath.FromSlash(prefix))
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d == nil || d.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(s.root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if seen[rel] || !scopeAllows(rel, scope) || !matchesIncludes(rel, include) {
				return nil
			}
			lower := strings.ToLower(rel)
			if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") && !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".go") {
				return nil
			}
			raw, readErr := os.ReadFile(path) //nolint:gosec // Search stays under approved workspace-relative roots.
			if readErr != nil {
				return readErr
			}
			seen[rel] = true
			source := "workspace"
			if strings.HasPrefix(rel, "test/workflows/") || strings.HasPrefix(rel, "docs/") {
				source = "example"
			} else if strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/") {
				source = "code"
			}
			out = append(out, searchableFile{path: rel, content: string(raw), source: source})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
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
	b.WriteString("You are authoring deck workflow files through an explicit bounded tool loop.\n")
	b.WriteString("Operate like a constrained domain agent: inspect files, write candidate files, run deck_lint, then finish.\n")
	b.WriteString("Do not return workflow documents directly. Use tools instead.\n")
	b.WriteString("Ask for clarification only for true blockers that cannot be resolved safely from available files and tool results.\n")
	b.WriteString("Finish only after deck_lint succeeds in this session and the request is satisfied.\n")
	b.WriteString("Code owns path scope and verification; any tool rejection means you must stay inside the approved boundaries.\n")
	b.WriteString("file_write updates candidate state first. Nothing is committed to disk until finish succeeds.\n")
	b.WriteString("The current workspace root may be an empty temp directory. Internal repo paths mentioned in retrieved local facts may not exist under this workspace root; use the provided facts instead of searching for those paths.\n")
	b.WriteString("For create or edit requests, finish is invalid until at least one file_write happened and deck_lint succeeded in this session.\n")
	b.WriteString("Workflow YAML must use the real deck schema: top-level `version`, then either `steps` or `phases`; each step needs exact case-sensitive `id`, `kind`, and `spec` fields.\n")
	b.WriteString("Use exact step kinds such as `InstallPackage`, `ManageService`, `InitKubeadm`, `CheckKubernetesCluster`, and `Command` when they fit.\n")
	b.WriteString("Use only flat tool-call objects inside toolCalls. Do not use nested args/arguments. Do not use a tool key.\n")
	b.WriteString("Available tools in this run:\n")
	for _, tool := range session.availableTools {
		b.WriteString("- ")
		b.WriteString(tool)
		b.WriteString("\n")
	}
	b.WriteString("Approved write paths:\n")
	for _, path := range session.approvedPathList {
		b.WriteString("- ")
		b.WriteString(path)
		b.WriteString("\n")
	}
	if len(session.plan.ValidationChecklist) > 0 {
		b.WriteString("Validation checklist:\n")
		for _, item := range session.plan.ValidationChecklist {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	b.WriteString("Respond with JSON using exactly one mode per turn:\n")
	b.WriteString("- toolCalls: {\"summary\":\"inspect vars\",\"review\":[\"read before editing\"],\"toolCalls\":[{\"name\":\"file_read\",\"path\":\"" + workspacepaths.CanonicalVarsWorkflow + "\"}]}\n")
	b.WriteString("- write and lint: {\"summary\":\"update workflow\",\"review\":[\"verify after editing\"],\"toolCalls\":[{\"name\":\"file_write\",\"path\":\"" + workspacepaths.CanonicalApplyWorkflow + "\",\"content\":\"version: v1alpha1\\n...\"},{\"name\":\"deck_lint\"}]}\n")
	b.WriteString("- finish: {\"summary\":\"workflow ready\",\"review\":[\"deck_lint passed\"],\"finish\":{\"reason\":\"deck_lint passed and requested files are ready\"}}\n")
	b.WriteString("- refine no-op: {\"summary\":\"no justified extraction needed\",\"review\":[\"anchor remains stable\",\"deck_lint passed on current files\"],\"toolCalls\":[{\"name\":\"deck_lint\"}]} then next turn finish with no file_write if no changes are justified.\n")
	b.WriteString("- clarification: {\"summary\":\"need one answer\",\"review\":[\"blocked on missing topology detail\"],\"clarification\":{\"question\":\"Which node acts as control plane?\",\"reason\":\"required to complete the workflow\"}}\n")
	b.WriteString("After any file_write, call deck_lint before finish.\n")
	b.WriteString("Prefer short review bullets that explain why the next action is enough.\n")
	return strings.TrimSpace(b.String())
}

func authoringAgentUserPrompt(session *authoringAgentSession, turn int) string {
	b := &strings.Builder{}
	b.WriteString("User request:\n")
	b.WriteString(session.requestText)
	b.WriteString("\n\n")
	b.WriteString("Route: ")
	b.WriteString(string(session.decision.Route))
	b.WriteString("\n")
	b.WriteString("Turn budget remaining: ")
	_, _ = fmt.Fprintf(b, "%d\n", maxInt(session.turnBudget-turn+1, 0))
	b.WriteString("Verification failures: ")
	_, _ = fmt.Fprintf(b, "%d/%d\n", session.verificationFailure, session.verificationBudget)
	if session.workspace.HasWorkflowTree {
		b.WriteString("Workspace workflow files:\n")
		for _, file := range session.workspace.Files {
			b.WriteString("- ")
			b.WriteString(file.Path)
			b.WriteString("\n")
		}
	} else {
		b.WriteString("Workspace workflow files: none yet\n")
	}
	if len(session.retrieval.Chunks) > 0 {
		b.WriteString("\nRetrieved local context (most relevant excerpts only):\n")
		for _, rendered := range renderAuthoringChunks(session.retrieval.Chunks) {
			b.WriteString(rendered)
			b.WriteString("\n\n")
		}
	}
	if session.state.LastLint != "" {
		b.WriteString("Last saved lint summary from prior run: ")
		b.WriteString(session.state.LastLint)
		b.WriteString("\n")
	}
	if session.decision.Route == askintent.RouteDraft && !session.workspace.HasWorkflowTree && len(session.candidateByPath) == 0 {
		b.WriteString("Draft hint: this workspace is empty. For straightforward requests, stop searching and write the approved target file directly, then run deck_lint.\n")
	}
	if session.decision.Route == askintent.RouteRefine && len(session.candidateByPath) == 0 {
		b.WriteString("Refine hint: if no extraction is clearly justified, you may run deck_lint on the current approved files and then finish with no-op.\n")
	}
	if len(session.toolEvents) > 0 {
		b.WriteString("\nTool transcript so far:\n")
		for _, event := range session.toolEvents {
			_, _ = fmt.Fprintf(b, "Turn %d %s ok=%t\n", event.Turn, event.Name, event.OK)
			if strings.TrimSpace(event.Summary) != "" {
				b.WriteString("Summary: ")
				b.WriteString(strings.TrimSpace(event.Summary))
				b.WriteString("\n")
			}
			if strings.TrimSpace(event.Result) != "" {
				b.WriteString(truncateString(event.Result, 1200))
				b.WriteString("\n")
			}
		}
		if last := session.toolEvents[len(session.toolEvents)-1]; last.Name == "finish" && !last.OK {
			b.WriteString("Next turn hint: do not finish yet. Use toolCalls, usually file_read/file_write/deck_lint.\n")
		}
	}
	if len(session.candidateByPath) > 0 {
		b.WriteString("\nCurrent candidate files:\n")
		for _, file := range session.candidateFiles() {
			b.WriteString("--- ")
			b.WriteString(file.Path)
			b.WriteString("\n")
			b.WriteString(truncateString(file.Content, 1600))
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
	b.WriteString("\nImportant response rules:\n")
	b.WriteString("- Use toolCalls with flat fields like name/path/query/content.\n")
	b.WriteString("- Do not use tool, args, or arguments keys.\n")
	b.WriteString("- After any file_write, call deck_lint before finish.\n")
	b.WriteString("- If blocked on missing information, ask one clarification question.\n")
	b.WriteString("\nRespond with the next best tool action, a single clarification question, or finish if the current candidate state is ready.\n")
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
		out = append(out, fmt.Sprintf("[additional-context]\n%d more retrieval chunks were omitted from the prompt for brevity; use file_read or file_search only when the current workspace actually contains the target path.", len(chunks)-len(selected)))
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

func callPaths(call askcontract.AgentToolCall) []string {
	paths := []string{}
	if strings.TrimSpace(call.Path) != "" {
		paths = append(paths, strings.TrimSpace(call.Path))
	}
	for _, path := range call.Paths {
		clean := strings.TrimSpace(path)
		if clean != "" {
			paths = append(paths, clean)
		}
	}
	return dedupe(paths)
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

func matchSnippet(content string, path string, query string) (string, bool) {
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	lowerPath := strings.ToLower(path)
	lowerContent := strings.ToLower(content)
	if lowerQuery == "" {
		return "", false
	}
	if strings.Contains(lowerPath, lowerQuery) {
		return firstMeaningfulLine(content), true
	}
	if idx := strings.Index(lowerContent, lowerQuery); idx >= 0 {
		return snippetAround(content, idx, len(lowerQuery)), true
	}
	for _, token := range strings.Fields(lowerQuery) {
		if len(token) < 3 {
			continue
		}
		if strings.Contains(lowerPath, token) {
			return firstMeaningfulLine(content), true
		}
		if idx := strings.Index(lowerContent, token); idx >= 0 {
			return snippetAround(content, idx, len(token)), true
		}
	}
	return "", false
}

func firstMeaningfulLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateString(line, 240)
		}
	}
	return ""
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

func matchesIncludes(path string, includes []string) bool {
	if len(includes) == 0 {
		return true
	}
	for _, include := range includes {
		include = strings.TrimSpace(include)
		if include == "" {
			continue
		}
		matched, err := filepath.Match(include, path)
		if err == nil && matched {
			return true
		}
		if strings.Contains(path, include) {
			return true
		}
	}
	return false
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
