package askcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcpaugment "github.com/Airgap-Castaways/deck/internal/askaugment/mcp"
	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askhooks"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askreview"
	"github.com/Airgap-Castaways/deck/internal/askscaffold"
	"github.com/Airgap-Castaways/deck/internal/askstate"
)

func Execute(ctx context.Context, opts Options, client askprovider.Client) error {
	if client == nil {
		return fmt.Errorf("ask backend is not configured")
	}
	if opts.Create && opts.Edit {
		return fmt.Errorf("--create and --edit are mutually exclusive")
	}
	if opts.Review && (opts.Create || opts.Edit) {
		return fmt.Errorf("--review cannot be combined with --create or --edit")
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	hooks := askhooks.Default()
	progress := newAskProgress(opts.Stdout)
	resolvedRoot, err := filepath.Abs(strings.TrimSpace(opts.Root))
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}
	requestText, requestSource, err := loadRequestText(resolvedRoot, strings.TrimSpace(opts.Prompt), strings.TrimSpace(opts.FromPath))
	if err != nil {
		return err
	}
	var resumedPlan *askcontract.PlanResponse
	var resumedPlanJSON string
	if isPlanArtifactInput(strings.TrimSpace(opts.FromPath)) {
		loadedPlan, planJSONPath, loadErr := loadPlanArtifact(resolvedRoot, strings.TrimSpace(opts.FromPath))
		if loadErr != nil {
			return loadErr
		}
		if len(opts.Answers) > 0 {
			loadedPlan, loadErr = applyPlanAnswers(loadedPlan, opts.Answers)
			if loadErr != nil {
				return loadErr
			}
		}
		resumedPlan = &loadedPlan
		resumedPlanJSON = planJSONPath
	}
	if len(opts.Answers) > 0 && resumedPlan == nil {
		return fmt.Errorf("--answer requires --from pointing to a saved plan artifact")
	}
	requestText = strings.TrimSpace(hooks.PreClassify(requestText))
	if requestText == "" && !opts.Review {
		return fmt.Errorf("ask request is required")
	}
	state, err := askstate.Load(resolvedRoot)
	if err != nil {
		return err
	}
	workspace, err := askretrieve.InspectWorkspace(resolvedRoot)
	if err != nil {
		return err
	}
	heuristic := hooks.PostClassify(askintent.Classify(askintent.Input{
		Prompt:          requestText,
		CreateFlag:      opts.Create,
		EditFlag:        opts.Edit,
		ReviewFlag:      opts.Review,
		HasWorkflowTree: workspace.HasWorkflowTree,
		HasPrepare:      workspace.HasPrepare,
		HasApply:        workspace.HasApply,
	}))
	if resumedPlan != nil && !opts.PlanOnly {
		heuristic = resumedPlanDecision(*resumedPlan)
	}
	effective, err := askconfig.ResolveEffective(askconfig.Settings{Provider: opts.Provider, Model: opts.Model, Endpoint: opts.Endpoint})
	if err != nil {
		return err
	}
	if effective.OAuthTokenSource == "session" || effective.OAuthTokenSource == "session-expired" {
		session, source, status, err := askconfig.ResolveRuntimeSessionWithContext(ctx, effective.Provider)
		if err != nil {
			return err
		}
		if strings.TrimSpace(session.AccessToken) != "" {
			effective.OAuthToken = session.AccessToken
			effective.OAuthTokenSource = source
			effective.AuthStatus = status
			effective.AccountID = session.AccountID
		}
	}
	logger := newAskLogger(opts.Stderr, effective.LogLevel, resolvedRoot)
	logger.info("request_received", "route_candidate", heuristic.Route, "review", opts.Review)
	logger.info("config_resolved", "provider", effective.Provider, "model", effective.Model, "endpoint", effective.Endpoint, "api_key_source", effective.APIKeySource, "oauth_token_source", effective.OAuthTokenSource, "account_id", strings.TrimSpace(effective.AccountID) != "", "log_level", effective.LogLevel)
	logger.debug("command", "command", renderUserCommand(opts))
	if requestSource != "" {
		logger.debug("request_source", "type", requestSource, "from", strings.TrimSpace(opts.FromPath))
	}
	logger.trace("request", "content", strings.TrimSpace(requestText))

	decision := heuristic
	classifierLLM := false
	classifierSystem := classifierSystemPrompt()
	classifierUser := classifierUserPrompt(requestText, opts.Review, workspace)
	switch {
	case canUseLLM(effective) && resumedPlan == nil && !askintent.IsHardOverride(heuristic):
		progress.status("classifying request")
		logger.debug("phase_started", "phase", "classify", "provider", effective.Provider, "model", effective.Model)
		classified, classifyErr := classifyWithLLM(ctx, client, effective, classifierSystem, classifierUser, logger)
		if classifyErr == nil {
			decision = classified
			classifierLLM = true
			logger.info("phase_succeeded", "phase", "classify", "route", decision.Route, "confidence", decision.Confidence, "reason", decision.Reason)
		} else {
			var cErr classifierError
			if ok := errors.As(classifyErr, &cErr); ok && cErr.kind == classifierErrorSemantic {
				decision = askintent.Decision{Route: askintent.RouteClarify, Confidence: 0.0, Reason: "classifier could not determine a safe route", Target: heuristic.Target, AllowGeneration: false, AllowRetry: false, RequiresLint: false, LLMPolicy: askintent.LLMOptional}
				logger.debug("phase_failed", "phase", "classify", "result", "clarify", "error", classifyErr)
				break
			}
			return classifyErr
		}
	case canUseLLM(effective):
		logger.debug("phase_skipped", "phase", "classify", "reason", "hard-override-or-resumed-plan")
	default:
		if !askintent.IsHardOverride(heuristic) {
			return fmt.Errorf("ask classifier requires model access; use --create, --edit, or --review, or configure provider credentials")
		}
		logger.debug("phase_skipped", "phase", "classify", "reason", "no-llm-required-for-hard-override")
	}
	if decision.Route == askintent.RouteRefine && !workspace.HasWorkflowTree {
		return fmt.Errorf("cannot refine workflow files because this workspace has no workflow tree yet; run a draft generation first")
	}

	evidencePlan, evidenceEvents, err := buildEvidencePlan(ctx, client, effective, requestText, decision, workspace, logger)
	if err != nil {
		return err
	}
	mcpChunks := []askretrieve.Chunk{}
	mcpEvents := append([]string(nil), evidenceEvents...)
	forceAuthoringAugment := isAuthoringRoute(decision.Route) && askFeatureEnabled("DECK_ASK_ENABLE_AUGMENT")
	switch {
	case forceAuthoringAugment || strings.TrimSpace(evidencePlan.Decision) != "unnecessary":
		mcpChunks, mcpEvents = mcpaugment.Gather(ctx, effective.MCP, decision.Route, requestText)
		mcpEvents = append(evidenceEvents, mcpEvents...)
	case isAuthoringRoute(decision.Route):
		mcpEvents = append(mcpEvents, "mcp: disabled for default local pipeline")
	default:
		mcpEvents = append(mcpEvents, "mcp: skipped by evidence plan (unnecessary)")
	}
	externalChunks := append([]askretrieve.Chunk{}, mcpChunks...)
	mcpEvents = append(mcpEvents, externalEvidenceWarningEvents(mcpChunks)...)
	if failure := requiredExternalEvidenceFailure(evidencePlan, mcpChunks, mcpEvents); failure != "" {
		if isAuthoringRoute(decision.Route) {
			return fmt.Errorf("required external evidence could not be fetched for this request: %s; check `deck ask config health`", failure)
		}
		externalChunks = append(externalChunks, externalEvidenceFailureChunk(failure))
		mcpEvents = append(mcpEvents, "mcp: required external evidence unavailable")
	}
	if warning := weakExternalEvidenceMessage(evidencePlan, mcpChunks, mcpEvents); warning != "" {
		externalChunks = append(externalChunks, weakExternalEvidenceChunk(warning))
		mcpEvents = append(mcpEvents, "mcp: weak install evidence fallback enabled")
	}
	if !isAuthoringRoute(decision.Route) {
		externalChunks = append(externalChunks, projectContextChunk(resolvedRoot))
	}
	retrieval := askretrieve.Retrieve(decision.Route, requestText, decision.Target, workspace, state, externalChunks)
	requirements := askpolicy.BuildScenarioRequirements(requestText, retrieval, workspace, decision)
	authoringBrief := askpolicy.BriefFromRequirements(requirements, decision)
	bundle := askcontext.CurrentBundle()
	result := runResult{
		Route:         decision.Route,
		Target:        decision.Target,
		Confidence:    decision.Confidence,
		Reason:        decision.Reason,
		RetriesUsed:   0,
		Chunks:        retrieval.Chunks,
		DroppedChunks: retrieval.Dropped,
		ConfigSource:  effective,
		ClassifierLLM: classifierLLM,
		AugmentEvents: append([]string(nil), mcpEvents...),
		UserCommand:   renderUserCommand(opts),
	}
	if canUseLLM(effective) {
		result.PromptTraces = append(result.PromptTraces, promptTrace{Label: "classifier", SystemPrompt: classifierSystem, UserPrompt: classifierUser})
	}

	progress.status("loading workspace context")
	logger.debug("phase_started", "phase", "augment", "mcp", effective.MCP.Enabled)
	for _, event := range result.AugmentEvents {
		prefix := "augment"
		if strings.HasPrefix(event, "mcp:") {
			prefix = "mcp"
		}
		logger.debug("augment_event", "source", prefix, "detail", event)
	}
	logger.debug("retrieve_summary", "phase", "retrieve", "chunks", len(result.Chunks), "dropped", len(result.DroppedChunks))

	if decision.LLMPolicy == askintent.LLMRequired && !canUseLLM(effective) {
		return fmt.Errorf("missing ask credentials for provider %q; set %s, %s, or run `deck ask config set --api-key ...` / `deck ask config set --oauth-token ...`", effective.Provider, "DECK_ASK_API_KEY", "DECK_ASK_OAUTH_TOKEN")
	}
	if opts.PlanOnly && !isAuthoringRoute(decision.Route) {
		return fmt.Errorf("ask plan is intended for draft/refine authoring requests; got route %s. Try `deck ask %q` instead", decision.Route, strings.TrimSpace(requestText))
	}

	planNeeded := isAuthoringRoute(decision.Route) && resumedPlan == nil
	var plan askcontract.PlanResponse
	var planCritic askcontract.PlanCriticResponse
	if resumedPlan != nil && opts.PlanOnly {
		progress.status("resuming saved plan")
		plan = *resumedPlan
		result.Plan = &plan
		result.PlanJSON = resumedPlanJSON
		planMD := renderPlanMarkdown(plan, strings.TrimSuffix(resumedPlanJSON, ".json")+".md")
		planMDPath, planJSONPath, saveErr := savePlanArtifact(resolvedRoot, opts, plan, planMD)
		if saveErr != nil {
			return saveErr
		}
		result.PlanMarkdown = planMDPath
		result.PlanJSON = planJSONPath
		updatedPlan, aborted, clarifyErr := maybeClarifyPlanInteractively(resolvedRoot, opts, &result, requestText, plan, askcontract.PlanCriticResponse{})
		if clarifyErr != nil {
			return clarifyErr
		}
		plan = updatedPlan
		if aborted {
			return render(opts.Stdout, opts.Stderr, result)
		}
		result.Summary = "updated plan artifact"
		if hasFatalPlanReviewIssues(plan, askcontract.PlanCriticResponse{}) {
			result.Termination = "plan-awaiting-clarification"
			result.FallbackNote = "plan updated but still requires clarification before generation"
		} else {
			result.Termination = "plan-resumed"
			result.FallbackNote = "plan updated from saved artifact"
		}
		result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
		if err := askstate.Save(resolvedRoot, askstate.Context{LastMode: "plan", LastRoute: string(result.Route), LastPrompt: strings.TrimSpace(requestText), LastFiles: filePathsFromPlan(plan), LastLLMUsed: false, LastClassifierLLM: result.ClassifierLLM, LastTermination: result.Termination}, requestText, resultToMarkdown(result)); err != nil {
			return err
		}
		return render(opts.Stdout, opts.Stderr, result)
	}
	if planNeeded {
		if !canUseLLM(effective) {
			return fmt.Errorf("route %s requires model access; configure provider credentials first", decision.Route)
		}
		progress.status("planning authoring workflow")
		logger.info("phase_started", "phase", "plan", "route", decision.Route)
		cfg := askconfigSettings{provider: effective.Provider, model: effective.Model, apiKey: effective.APIKey, oauthToken: effective.OAuthToken, accountID: effective.AccountID, endpoint: effective.Endpoint}
		planned, reviewedCritic, usedFallback, planErr := buildPlanWithReview(ctx, client, cfg, decision, retrieval, requestText, workspace, requirements, logger)
		planCritic = reviewedCritic
		if planErr != nil {
			return planErr
		}
		if usedFallback {
			logger.debug("phase_fallback", "phase", "plan", "reason", "using defaults after planner failure")
		}
		plan = planned
		result.Plan = &plan
		if planCritic.Summary != "" || len(planCritic.Blocking) > 0 || len(planCritic.Advisory) > 0 || len(planCritic.MissingContracts) > 0 || len(planCritic.SuggestedFixes) > 0 {
			result.PlanCritic = &planCritic
		}
		logger.info("phase_succeeded", "phase", "plan", "files", len(plan.Files), "blockers", len(plan.Blockers))
		planMD := renderPlanMarkdown(plan, ".deck/plan/latest.md")
		planMDPath, planJSONPath, saveErr := savePlanArtifact(resolvedRoot, opts, plan, planMD)
		if saveErr != nil {
			return saveErr
		}
		logger.info("artifact_saved", "phase", "plan", "markdown", planMDPath, "json", planJSONPath)
		result.PlanMarkdown = planMDPath
		result.PlanJSON = planJSONPath
		planMarkdownFinal := renderPlanMarkdown(plan, planMDPath)
		if updateErr := os.WriteFile(filepath.Join(resolvedRoot, filepath.FromSlash(planMDPath)), []byte(planMarkdownFinal+"\n"), 0o600); updateErr == nil {
			_ = os.WriteFile(filepath.Join(filepath.Dir(filepath.Join(resolvedRoot, filepath.FromSlash(planMDPath))), "latest.md"), []byte(planMarkdownFinal+"\n"), 0o600)
		}
		if askpolicy.PlanNeedsClarification(plan) {
			progress.status("waiting for clarification")
		}
		updatedPlan, aborted, clarifyErr := maybeClarifyPlanInteractively(resolvedRoot, opts, &result, requestText, plan, planCritic)
		if clarifyErr != nil {
			return clarifyErr
		}
		plan = updatedPlan
		if aborted {
			return render(opts.Stdout, opts.Stderr, result)
		}
		if opts.PlanOnly {
			result.Summary = "generated plan artifact"
			result.Termination = "plan-only-requested"
			result.FallbackNote = "plan requested"
			result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
			result.ReviewLines = append(result.ReviewLines, renderPlanCriticNotes(planCritic)...)
			if err := askstate.Save(resolvedRoot, askstate.Context{
				LastMode:          "plan",
				LastRoute:         string(result.Route),
				LastPrompt:        strings.TrimSpace(requestText),
				LastFiles:         filePathsFromPlan(plan),
				LastLLMUsed:       true,
				LastClassifierLLM: result.ClassifierLLM,
				LastTermination:   result.Termination,
			}, requestText, resultToMarkdown(result)); err != nil {
				return err
			}
			return render(opts.Stdout, opts.Stderr, result)
		}
		if hasFatalPlanReviewIssues(plan, planCritic) {
			result.Summary = "plan generated with review blockers"
			result.Termination = "plan-only-review-blocked"
			result.FallbackNote = "generation stopped because plan review found blocking issues"
			result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
			result.ReviewLines = append(result.ReviewLines, renderPlanCriticNotes(planCritic)...)
			if err := askstate.Save(resolvedRoot, askstate.Context{
				LastMode:          "plan",
				LastRoute:         string(result.Route),
				LastPrompt:        strings.TrimSpace(requestText),
				LastFiles:         filePathsFromPlan(plan),
				LastLLMUsed:       true,
				LastClassifierLLM: result.ClassifierLLM,
				LastTermination:   result.Termination,
			}, requestText, resultToMarkdown(result)); err != nil {
				return err
			}
			return render(opts.Stdout, opts.Stderr, result)
		}
		secondPassExternal := append([]askretrieve.Chunk{}, externalChunks...)
		secondPassExternal = append(secondPassExternal, repoMapChunk(workspace), planChunk(plan))
		secondPassExternal = append(secondPassExternal, planWorkspaceChunks(plan, workspace)...)
		decision.Target = planTarget(plan, decision.Target)
		retrieval = askretrieve.Retrieve(decision.Route, requestText, decision.Target, workspace, state, secondPassExternal)
		requirements = askpolicy.MergeRequirementsWithPlan(askpolicy.BuildScenarioRequirements(requestText, retrieval, workspace, decision), plan)
		authoringBrief = plan.AuthoringBrief
		if strings.TrimSpace(authoringBrief.RouteIntent) == "" {
			authoringBrief = askpolicy.BriefFromRequirements(requirements, decision)
		}
		result.Chunks = retrieval.Chunks
		result.DroppedChunks = retrieval.Dropped
		logger.debug("retrieve_summary", "phase", "retrieve-second-pass", "chunks", len(result.Chunks), "dropped", len(result.DroppedChunks))
	} else if resumedPlan != nil {
		progress.status("resuming saved plan")
		plan = *resumedPlan
		result.Plan = &plan
		result.PlanJSON = resumedPlanJSON
		result.PlanMarkdown = strings.TrimSuffix(resumedPlanJSON, ".json") + ".md"
		if askpolicy.PlanNeedsClarification(plan) {
			progress.status("waiting for clarification")
		}
		updatedPlan, aborted, clarifyErr := maybeClarifyPlanInteractively(resolvedRoot, opts, &result, requestText, plan, askcontract.PlanCriticResponse{})
		if clarifyErr != nil {
			return clarifyErr
		}
		plan = updatedPlan
		if aborted {
			return render(opts.Stdout, opts.Stderr, result)
		}
		if hasFatalPlanReviewIssues(plan, askcontract.PlanCriticResponse{}) {
			result.Summary = "saved plan still requires clarification"
			result.Termination = "plan-awaiting-clarification"
			result.FallbackNote = "apply --answer to the saved plan artifact before generation"
			result.ReviewLines = append(result.ReviewLines, renderPlanNotes(plan)...)
			if err := askstate.Save(resolvedRoot, askstate.Context{LastMode: "plan", LastRoute: string(result.Route), LastPrompt: strings.TrimSpace(requestText), LastFiles: filePathsFromPlan(plan), LastLLMUsed: false, LastClassifierLLM: result.ClassifierLLM, LastTermination: result.Termination}, requestText, resultToMarkdown(result)); err != nil {
				return err
			}
			return render(opts.Stdout, opts.Stderr, result)
		}
		secondPassExternal := append([]askretrieve.Chunk{}, externalChunks...)
		secondPassExternal = append(secondPassExternal, repoMapChunk(workspace), planChunk(plan))
		secondPassExternal = append(secondPassExternal, planWorkspaceChunks(plan, workspace)...)
		decision.Target = planTarget(plan, decision.Target)
		retrieval = askretrieve.Retrieve(decision.Route, requestText, decision.Target, workspace, state, secondPassExternal)
		requirements = askpolicy.MergeRequirementsWithPlan(askpolicy.BuildScenarioRequirements(requestText, retrieval, workspace, decision), plan)
		authoringBrief = plan.AuthoringBrief
		result.Chunks = retrieval.Chunks
		result.DroppedChunks = retrieval.Dropped
	}

	switch decision.Route {
	case askintent.RouteDraft, askintent.RouteRefine:
		if !canUseLLM(effective) {
			return fmt.Errorf("route %s requires model access; configure provider credentials first", decision.Route)
		}
		attempts := generationAttempts(opts.MaxIterations, decision, requestText)
		scaffold := askscaffold.Build(requirements, workspace, decision, plan, bundle)
		executionModel := plan.ExecutionModel
		if len(executionModel.ArtifactContracts) == 0 && len(executionModel.SharedStateContracts) == 0 && strings.TrimSpace(executionModel.RoleExecution.RoleSelector) == "" && len(executionModel.ApplyAssumptions) == 0 {
			executionModel = askpolicy.ExecutionModelFromRequirements(requirements)
		}
		generationPrompt := generationUserPrompt(workspace, state, requestText, strings.TrimSpace(opts.FromPath), decision.Route, plan)
		generationPrompt = appendPlanAdvisoryPrompt(generationPrompt, plan, planCritic)
		generationKind := "generate-fast"
		if askFeatureEnabled("DECK_ASK_ENABLE_JUDGE") {
			generationKind = "generate"
		}
		generationRequest := askprovider.Request{
			Kind:               generationKind,
			Provider:           effective.Provider,
			Model:              effective.Model,
			APIKey:             effective.APIKey,
			OAuthToken:         effective.OAuthToken,
			AccountID:          effective.AccountID,
			Endpoint:           effective.Endpoint,
			SystemPrompt:       generationSystemPrompt(decision.Route, decision.Target, requestText, retrieval, requirements, plan, authoringBrief, executionModel, scaffold),
			Prompt:             generationPrompt,
			ResponseSchema:     askcontract.GenerationResponseSchema(),
			ResponseSchemaName: "deck_generation_response",
			MaxRetries:         providerRetryCount("generate"),
			Timeout:            askRequestTimeout("generate", attempts, generationPrompt, generationPrompt),
		}
		result.PromptTraces = append(result.PromptTraces, promptTrace{Label: "generation", SystemPrompt: generationRequest.SystemPrompt, UserPrompt: generationRequest.Prompt})
		progress.status("generating workflow output")
		logger.info("phase_started", "phase", "generation", "route", decision.Route, "max_attempts", attempts)
		gen, files, lintSummary, critic, judge, retriesUsed, genErr := generateWithValidation(ctx, client, generationRequest, resolvedRoot, attempts, logger, decision, plan, authoringBrief, retrieval, planCritic)
		if genErr != nil {
			return genErr
		}
		if askFeatureEnabled("DECK_ASK_ENABLE_POSTPROCESS") {
			postSummary, postErr := maybePostProcessGeneration(ctx, client, generationRequest, resolvedRoot, logger, decision, plan, authoringBrief, retrieval, gen, files, lintSummary, critic, judge, planCritic)
			switch {
			case postErr != nil:
				logger.debug("phase_skipped", "phase", "postprocess", "error", postErr)
			case postSummary.Applied:
				gen = postSummary.Generation
				files = postSummary.Files
				lintSummary = postSummary.LintSummary
				critic = postSummary.Critic
				judge = postSummary.Judge
				result.ReviewLines = append(result.ReviewLines, postSummary.Notes...)
			case len(postSummary.Notes) > 0:
				result.ReviewLines = append(result.ReviewLines, postSummary.Notes...)
			}
		}
		logger.info("phase_succeeded", "phase", "generation", "files", len(files), "lint", lintSummary)
		result.LLMUsed = true
		result.RetriesUsed = retriesUsed
		result.Files = files
		result.Summary = gen.Summary
		result.ReviewLines = append(result.ReviewLines, gen.Review...)
		result.ReviewLines = append(result.ReviewLines, renderPlanCriticNotes(planCritic)...)
		result.LintSummary = lintSummary
		result.LocalFindings = localFindings(result.Files)
		result.Critic = &critic
		if judge.Summary != "" || len(judge.Blocking) > 0 || len(judge.Advisory) > 0 || len(judge.MissingCapabilities) > 0 || len(judge.SuggestedFixes) > 0 {
			result.Judge = &judge
		}
		result.ReviewLines = append(result.ReviewLines, critic.Advisory...)
		if err := writeFiles(resolvedRoot, result.Files); err != nil {
			return err
		}
		result.WroteFiles = true
		if retriesUsed > 0 {
			result.Termination = "generated-after-repair"
		} else {
			result.Termination = "generated"
		}
	default:
		switch decision.Route {
		case askintent.RouteReview:
			result.LocalFindings = askreview.Workspace(resolvedRoot)
			result.ReviewLines = append(result.ReviewLines, findingsToLines(result.LocalFindings)...)
			if canUseLLM(effective) {
				systemPrompt, userPrompt := infoPrompts(decision.Route, decision.Target, retrieval, workspace, requestText)
				result.PromptTraces = append(result.PromptTraces, promptTrace{Label: string(decision.Route), SystemPrompt: systemPrompt, UserPrompt: userPrompt})
				progress.status("answering %s request", phaseLabel(string(decision.Route)))
				logger.info("phase_started", "phase", "answer", "route", decision.Route)
				info, infoErr := answerWithLLM(ctx, client, effective, decision, retrieval, requestText, logger)
				if infoErr == nil {
					result.LLMUsed = true
					result.Summary = info.Summary
					result.Answer = info.Answer
					result.ReviewLines = append(result.ReviewLines, info.Suggestions...)
					result.ReviewLines = append(result.ReviewLines, info.Findings...)
					result.ReviewLines = append(result.ReviewLines, info.SuggestedChange...)
					logger.info("phase_succeeded", "phase", "answer", "route", decision.Route)
				} else {
					result.ReviewLines = append(result.ReviewLines, "LLM response failed; using local fallback: "+infoErr.Error())
					logger.debug("phase_fallback", "phase", "answer", "error", infoErr)
				}
			}
		case askintent.RouteClarify:
			applyLocalFallback(&result, resolvedRoot, workspace, requestText)
		default:
			if canUseLLM(effective) {
				systemPrompt, userPrompt := infoPrompts(decision.Route, decision.Target, retrieval, workspace, requestText)
				result.PromptTraces = append(result.PromptTraces, promptTrace{Label: string(decision.Route), SystemPrompt: systemPrompt, UserPrompt: userPrompt})
				progress.status("answering %s request", phaseLabel(string(decision.Route)))
				logger.info("phase_started", "phase", "answer", "route", decision.Route)
				info, infoErr := answerWithLLM(ctx, client, effective, decision, retrieval, requestText, logger)
				if infoErr == nil {
					result.LLMUsed = true
					result.Summary = info.Summary
					result.Answer = info.Answer
					result.ReviewLines = append(result.ReviewLines, info.Suggestions...)
					result.ReviewLines = append(result.ReviewLines, info.Findings...)
					result.ReviewLines = append(result.ReviewLines, info.SuggestedChange...)
					logger.info("phase_succeeded", "phase", "answer", "route", decision.Route)
				} else {
					result.ReviewLines = append(result.ReviewLines, "LLM response failed; using local fallback: "+infoErr.Error())
					logger.debug("phase_fallback", "phase", "answer", "error", infoErr)
					applyLocalFallback(&result, resolvedRoot, workspace, requestText)
				}
			} else {
				applyLocalFallback(&result, resolvedRoot, workspace, requestText)
			}
		}
		if result.Termination == "" {
			result.Termination = "answered"
		}
	}

	if err := askstate.Save(resolvedRoot, askstate.Context{
		LastMode:            string(result.Route),
		LastRoute:           string(result.Route),
		LastConfidence:      result.Confidence,
		LastReason:          result.Reason,
		LastTargetKind:      result.Target.Kind,
		LastTargetPath:      result.Target.Path,
		LastTargetName:      result.Target.Name,
		LastPrompt:          strings.TrimSpace(requestText),
		LastFiles:           filePaths(result.Files),
		LastLint:            result.LintSummary,
		LastLLMUsed:         result.LLMUsed,
		LastClassifierLLM:   result.ClassifierLLM,
		LastChunkIDs:        chunkIDs(result.Chunks),
		LastDroppedChunkIDs: append([]string(nil), result.DroppedChunks...),
		LastAugmentEvents:   append([]string(nil), result.AugmentEvents...),
		LastMCPChunkIDs:     chunkIDsBySource(result.Chunks, "mcp"),
		LastRetries:         result.RetriesUsed,
		LastTermination:     result.Termination,
	}, requestText, resultToMarkdown(result)); err != nil {
		return err
	}

	return render(opts.Stdout, opts.Stderr, result)
}

func resumedPlanDecision(plan askcontract.PlanResponse) askintent.Decision {
	route := askintent.ParseRoute(plan.Intent)
	decision := routeDefaults(route)
	decision.Confidence = 1.0
	decision.Reason = "saved plan artifact"
	decision.Target = planTarget(plan, askintent.Target{Kind: plan.AuthoringBrief.TargetScope, Path: plan.EntryScenario})
	return decision
}

func maybeClarifyPlanInteractively(root string, opts Options, result *runResult, requestText string, plan askcontract.PlanResponse, planCritic askcontract.PlanCriticResponse) (askcontract.PlanResponse, bool, error) {
	if !askpolicy.PlanNeedsClarification(plan) {
		return plan, false, nil
	}
	if !interactiveSessionProbe(opts.Stdin, opts.Stdout) {
		return plan, false, nil
	}
	updatedPlan, aborted, err := runInteractiveClarifications(opts.Stdin, opts.Stdout, plan)
	if err != nil {
		return plan, false, err
	}
	planMD := renderPlanMarkdown(updatedPlan, result.PlanMarkdown)
	planMDPath, planJSONPath, saveErr := savePlanArtifact(root, opts, updatedPlan, planMD)
	if saveErr != nil {
		return updatedPlan, false, saveErr
	}
	result.Plan = &updatedPlan
	result.PlanMarkdown = planMDPath
	result.PlanJSON = planJSONPath
	result.ReviewLines = append(result.ReviewLines, renderPlanNotes(updatedPlan)...)
	result.ReviewLines = append(result.ReviewLines, renderPlanCriticNotes(planCritic)...)
	if aborted {
		result.Summary = "saved plan after interactive clarification exit"
		result.Termination = "plan-clarification-aborted"
		result.FallbackNote = "clarification stopped; resume later from the saved plan artifact"
		if err := askstate.Save(root, askstate.Context{LastMode: "plan", LastRoute: string(result.Route), LastPrompt: strings.TrimSpace(requestText), LastFiles: filePathsFromPlan(updatedPlan), LastLLMUsed: true, LastClassifierLLM: result.ClassifierLLM, LastTermination: result.Termination}, requestText, resultToMarkdown(*result)); err != nil {
			return updatedPlan, false, err
		}
	}
	return updatedPlan, aborted, nil
}
