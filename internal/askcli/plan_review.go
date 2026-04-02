package askcli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askauthoring"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func buildPlanWithReview(ctx context.Context, client askprovider.Client, cfg askconfigSettings, decision askintent.Decision, retrieval askretrieve.RetrievalResult, requestText string, workspace askretrieve.WorkspaceSummary, requirements askpolicy.ScenarioRequirements, logger askLogger) (askcontract.PlanResponse, askcontract.PlanCriticResponse, bool, error) {
	var planned askcontract.PlanResponse
	var critic askcontract.PlanCriticResponse
	currentPrompt := requestText
	for attempt := 1; attempt <= 2; attempt++ {
		current, planErr := planWithLLM(ctx, client, cfg, decision, retrieval, currentPrompt, workspace, logger)
		if planErr != nil {
			logger.debug("phase_fallback", "phase", "plan", "error", planErr)
			planned = askpolicy.BuildPlanDefaults(requirements, requestText, decision, workspace)
			return planned, askcontract.PlanCriticResponse{}, true, nil
		}
		planned = current
		criticResp, criticErr := critiquePlanWithLLM(ctx, client, cfg, planned, logger)
		if criticErr != nil {
			logger.debug("phase_skipped", "phase", "plan-critic", "error", criticErr)
			return planned, askcontract.PlanCriticResponse{}, false, nil
		}
		critic = criticResp
		critic = normalizePlanCritic(planned, critic)
		if len(critic.Blocking) == 0 && len(critic.MissingContracts) == 0 {
			return planned, critic, false, nil
		}
		logger.debug("phase_retry", "phase", "plan-critic", "attempt", attempt, "blocking", len(critic.Blocking), "missing", len(critic.MissingContracts))
		if attempt == 2 {
			return planned, critic, false, nil
		}
		currentPrompt = appendPlanCriticRetryPrompt(requestText, planned, critic)
	}
	return planned, critic, false, nil
}

func appendPlanCriticRetryPrompt(base string, plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) string {
	b := &strings.Builder{}
	b.WriteString(strings.TrimSpace(base))
	b.WriteString("\n\nPlan critic requested a stronger plan before generation. Address these issues in the next plan JSON:\n")
	for _, finding := range critic.Findings {
		message := strings.TrimSpace(finding.Message)
		if message == "" {
			continue
		}
		label := strings.TrimSpace(string(finding.Severity))
		if label == "" {
			label = string(workflowissues.SeverityAdvisory)
		}
		b.WriteString("- ")
		b.WriteString(label)
		if code := strings.TrimSpace(string(finding.Code)); code != "" {
			b.WriteString(" [")
			b.WriteString(code)
			b.WriteString("]")
		}
		if path := strings.TrimSpace(finding.Path); path != "" {
			b.WriteString(" @ ")
			b.WriteString(path)
		}
		b.WriteString(": ")
		b.WriteString(message)
		b.WriteString("\n")
	}
	for _, item := range critic.SuggestedFixes {
		if strings.TrimSpace(item) != "" {
			b.WriteString("- fix: ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	b.WriteString("Required plan updates before generation:\n")
	added := false
	if planNeedsArtifactContracts(plan) {
		b.WriteString("- Ensure executionModel.artifactContracts explicitly cover each staged producer/consumer handoff used by apply.\n")
		added = true
	}
	if planNeedsMultiRoleExecution(plan) {
		b.WriteString("- Ensure shared-state and role-execution contracts are explicit for every control-plane/worker handoff.\n")
		added = true
	}
	if planNeedsStagedVerification(plan) {
		b.WriteString("- Ensure executionModel.verification and apply assumptions reflect the intended execution order and verification stage.\n")
		added = true
	}
	if !added {
		b.WriteString("- Tighten executionModel and validationChecklist so the requested workflow shape is explicit and internally consistent.\n")
	}
	b.WriteString("- Prefer recoverable omissions to be fixed in the plan rather than adding new blockers or open questions.\n")
	return strings.TrimSpace(b.String())
}

func normalizePlanCritic(plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) askcontract.PlanCriticResponse {
	findings := normalizedPlanCriticFindings(plan, critic)
	blocking := make([]string, 0, len(findings))
	advisory := make([]string, 0, len(findings))
	missing := make([]string, 0, len(findings))
	for _, finding := range findings {
		text := strings.TrimSpace(finding.Message)
		if text == "" {
			continue
		}
		switch {
		case planCriticFindingIsFatal(finding):
			blocking = append(blocking, text)
		case finding.Severity == workflowissues.SeverityMissingContract && !planCriticFindingIsRecoverable(finding):
			missing = append(missing, text)
		default:
			advisory = append(advisory, text)
		}
	}
	critic.Findings = findings
	critic.Blocking = dedupe(blocking)
	critic.MissingContracts = dedupe(missing)
	critic.Advisory = dedupe(advisory)
	return critic
}

func hasFatalPlanReviewIssues(plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) bool {
	return len(fatalPlanReviewReasons(plan, critic)) > 0
}

func fatalPlanReviewReasons(plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) []string {
	reasons := []string{}
	for _, finding := range normalizedPlanCriticFindings(plan, critic) {
		if !planCriticFindingIsFatal(finding) && !planCriticFindingNeedsExecutionGate(plan, finding) {
			continue
		}
		if text := strings.TrimSpace(finding.Message); text != "" {
			reasons = append(reasons, text)
		}
	}
	for _, item := range fatalPlanBlockers(plan) {
		if strings.TrimSpace(item) != "" {
			reasons = append(reasons, item)
		}
	}
	reasons = append(reasons, fatalPlanScopeReasons(plan)...)
	reasons = append(reasons, fatalPlanProgramReasons(plan)...)
	reasons = append(reasons, fatalPlanGraphReasons(plan)...)
	return dedupe(reasons)
}

func fatalPlanScopeReasons(plan askcontract.PlanResponse) []string {
	if askintent.ParseRoute(plan.Intent) != askintent.RouteRefine {
		return nil
	}
	if len(plan.AuthoringBrief.AnchorPaths) == 0 {
		return []string{"refine generation requires at least one anchor path before execution can continue"}
	}
	return nil
}

func fatalPlanProgramReasons(plan askcontract.PlanResponse) []string {
	reasons := []string{}
	program := plan.AuthoringProgram
	if strings.TrimSpace(program.Platform.Family) == "" && strings.TrimSpace(program.Cluster.JoinFile) == "" && program.Verification.ExpectedNodeCount == 0 && strings.TrimSpace(program.Cluster.RoleSelector) == "" {
		return nil
	}
	if strings.TrimSpace(program.Platform.Family) == "" {
		missingPlatform := false
		for _, capability := range plan.AuthoringBrief.RequiredCapabilities {
			if strings.TrimSpace(capability) == "package-staging" || strings.TrimSpace(capability) == "prepare-artifacts" {
				missingPlatform = true
				break
			}
		}
		if missingPlatform {
			reasons = append(reasons, "authoring program is missing platform.family required for package authoring")
		}
	}
	if planNeedsMultiRoleExecution(plan) && strings.TrimSpace(program.Cluster.RoleSelector) == "" {
		reasons = append(reasons, "authoring program is missing cluster.roleSelector required for multi-role execution")
	}
	if hasPlanCapability(plan, "cluster-verification") && program.Verification.ExpectedNodeCount <= 0 {
		reasons = append(reasons, "authoring program is missing verification.expectedNodeCount required for cluster verification")
	}
	if hasPlanCapability(plan, "kubeadm-bootstrap") && strings.TrimSpace(program.Cluster.JoinFile) == "" {
		reasons = append(reasons, "authoring program is missing cluster.joinFile required for kubeadm bootstrap flows")
	}
	return dedupe(reasons)
}

func fatalPlanGraphReasons(plan askcontract.PlanResponse) []string {
	facts := askauthoring.InferFacts(plan.Request, plan.ArtifactKinds, plan.OfflineAssumption)
	graph := askauthoring.BuildContractGraph(facts, askauthoring.RequirementLike{
		Connectivity:   plan.OfflineAssumption,
		NeedsPrepare:   plan.NeedsPrepare,
		ArtifactKinds:  plan.ArtifactKinds,
		EntryScenario:  plan.EntryScenario,
		ScenarioIntent: plan.AuthoringBrief.RequiredCapabilities,
	}, askretrieve.WorkspaceSummary{})
	reasons := []string{}
	if strings.TrimSpace(plan.EntryScenario) == "" {
		reasons = append(reasons, "no viable entry scenario can be determined")
	}
	if strings.TrimSpace(plan.AuthoringBrief.ModeIntent) == "prepare+apply" && !hasPlannedPath(plan.Files, "workflows/prepare.yaml") {
		reasons = append(reasons, "required prepare/apply file structure is absent and cannot be defaulted")
	}
	if strings.TrimSpace(plan.AuthoringBrief.ModeIntent) == "prepare+apply" && !hasPlannedPath(plan.Files, filepath.ToSlash(strings.TrimSpace(plan.EntryScenario))) {
		reasons = append(reasons, "required prepare/apply entry scenario is absent and cannot be defaulted")
	}
	if len(graph.Artifacts) > 0 && len(plan.ExecutionModel.ArtifactContracts) == 0 {
		reasons = append(reasons, "artifact-dependent request has no viable artifact contract graph")
	}
	if len(graph.SharedState) > 0 && len(plan.ExecutionModel.SharedStateContracts) == 0 {
		reasons = append(reasons, "multi-role request has no viable shared-state contract graph")
	}
	if graph.RoleExecution.PerNodeInvocation && strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) == "" {
		reasons = append(reasons, "multi-role request has no viable role selector or branching model")
	}
	if graph.Verification.ExpectedNodeCount > 0 && plan.ExecutionModel.Verification.ExpectedNodeCount <= 0 {
		reasons = append(reasons, "verification contract is incomplete for the requested topology")
	}
	return reasons
}

func fatalPlanBlockers(plan askcontract.PlanResponse) []string {
	reasons := []string{}
	for _, item := range plan.Blockers {
		if text := strings.TrimSpace(item); text != "" {
			reasons = append(reasons, text)
		}
	}
	for _, item := range plan.Clarifications {
		if !item.BlocksGeneration || strings.TrimSpace(item.Answer) != "" {
			continue
		}
		if text := strings.TrimSpace(item.Question); text != "" {
			reasons = append(reasons, text)
		}
	}
	return dedupe(reasons)
}

func hasPlannedPath(files []askcontract.PlanFile, want string) bool {
	want = strings.TrimSpace(want)
	for _, file := range files {
		if filepath.ToSlash(strings.TrimSpace(file.Path)) == want {
			return true
		}
	}
	return false
}

func normalizedPlanCriticFindings(plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) []askcontract.PlanCriticFinding {
	findings := make([]askcontract.PlanCriticFinding, 0, len(critic.Findings)+len(critic.Blocking)+len(critic.Advisory)+len(critic.MissingContracts))
	findings = append(findings, critic.Findings...)
	seen := map[string]bool{}
	for _, finding := range critic.Findings {
		seen[planCriticFindingKey(finding)] = true
	}
	appendLegacy := func(items []string, severity workflowissues.Severity) {
		for _, item := range items {
			text := strings.TrimSpace(item)
			if text == "" {
				continue
			}
			finding := legacyPlanCriticFinding(text, severity)
			key := planCriticFindingKey(finding)
			if seen[key] {
				continue
			}
			seen[key] = true
			findings = append(findings, finding)
		}
	}
	appendLegacy(critic.Blocking, workflowissues.SeverityBlocking)
	appendLegacy(critic.Advisory, workflowissues.SeverityAdvisory)
	appendLegacy(critic.MissingContracts, workflowissues.SeverityMissingContract)
	return dedupePlanCriticFindings(findings)
}

func dedupePlanCriticFindings(findings []askcontract.PlanCriticFinding) []askcontract.PlanCriticFinding {
	seen := map[string]bool{}
	out := make([]askcontract.PlanCriticFinding, 0, len(findings))
	for _, finding := range findings {
		finding.Code = workflowissues.Code(strings.TrimSpace(string(finding.Code)))
		finding.Severity = workflowissues.Severity(strings.TrimSpace(string(finding.Severity)))
		finding.Message = strings.TrimSpace(finding.Message)
		finding.Path = strings.TrimSpace(finding.Path)
		if finding.Message == "" {
			continue
		}
		if finding.Severity == "" {
			finding.Severity = workflowissues.SeverityAdvisory
		}
		key := planCriticFindingKey(finding)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

func planCriticFindingKey(finding askcontract.PlanCriticFinding) string {
	return strings.Join([]string{strings.TrimSpace(string(finding.Code)), strings.TrimSpace(string(finding.Severity)), strings.TrimSpace(finding.Path), strings.TrimSpace(finding.Message)}, "|")
}

func planCriticFindingIsFatal(finding askcontract.PlanCriticFinding) bool {
	if finding.Recoverable {
		return false
	}
	if finding.Severity != "" {
		return finding.Severity == workflowissues.SeverityBlocking
	}
	if spec, ok := workflowissues.SpecFor(finding.Code); ok {
		return spec.DefaultSeverity == workflowissues.SeverityBlocking && !spec.DefaultRecoverable
	}
	return false
}

func planCriticFindingNeedsExecutionGate(plan askcontract.PlanResponse, finding askcontract.PlanCriticFinding) bool {
	switch finding.Code {
	case workflowissues.CodeMissingArtifactConsumer,
		workflowissues.CodeArtifactContractGap:
		return planNeedsArtifactContracts(plan) && len(plan.ExecutionModel.ArtifactContracts) == 0
	case workflowissues.CodeMissingRoleSelector:
		return planNeedsMultiRoleExecution(plan) && strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) == ""
	case workflowissues.CodeAmbiguousJoinContract,
		workflowissues.CodeWorkerJoinFanoutGap:
		return planNeedsMultiRoleExecution(plan) && len(plan.ExecutionModel.SharedStateContracts) == 0
	case workflowissues.CodeRoleCardinalityGap,
		workflowissues.CodeTopologyFidelityGap:
		return planNeedsMultiRoleExecution(plan) && (plan.AuthoringBrief.NodeCount <= 0 || strings.TrimSpace(plan.AuthoringBrief.Topology) == "unspecified")
	case workflowissues.CodeWeakVerificationStaging:
		return planNeedsStagedVerification(plan) && plan.ExecutionModel.Verification.ExpectedNodeCount <= 0
	default:
		return false
	}
}

func planNeedsArtifactContracts(plan askcontract.PlanResponse) bool {
	return plan.NeedsPrepare || len(plan.ArtifactKinds) > 0 || len(plan.ExecutionModel.ArtifactContracts) > 0
}

func hasPlanCapability(plan askcontract.PlanResponse, want string) bool {
	for _, capability := range plan.AuthoringBrief.RequiredCapabilities {
		if strings.TrimSpace(capability) == want {
			return true
		}
	}
	return false
}

func planNeedsMultiRoleExecution(plan askcontract.PlanResponse) bool {
	if strings.TrimSpace(plan.AuthoringBrief.Topology) == "multi-node" || strings.TrimSpace(plan.AuthoringBrief.Topology) == "ha" || plan.AuthoringBrief.NodeCount > 1 {
		return true
	}
	if strings.TrimSpace(plan.ExecutionModel.RoleExecution.WorkerFlow) != "" || plan.ExecutionModel.RoleExecution.PerNodeInvocation {
		return true
	}
	for _, capability := range plan.AuthoringBrief.RequiredCapabilities {
		if strings.TrimSpace(capability) == "kubeadm-join" {
			return true
		}
	}
	return false
}

func planNeedsStagedVerification(plan askcontract.PlanResponse) bool {
	if planNeedsArtifactContracts(plan) || planNeedsMultiRoleExecution(plan) || strings.TrimSpace(plan.AuthoringBrief.ModeIntent) == "prepare+apply" {
		return true
	}
	for _, capability := range plan.AuthoringBrief.RequiredCapabilities {
		switch strings.TrimSpace(capability) {
		case "kubeadm-bootstrap", "kubeadm-join":
			return true
		}
	}
	return false
}

func planCriticFindingIsRecoverable(finding askcontract.PlanCriticFinding) bool {
	if finding.Recoverable {
		return true
	}
	if spec, ok := workflowissues.SpecFor(finding.Code); ok {
		return spec.DefaultRecoverable
	}
	return finding.Recoverable
}

func legacyPlanCriticFinding(text string, severity workflowissues.Severity) askcontract.PlanCriticFinding {
	return askcontract.PlanCriticFinding{
		Code:        workflowissues.CodeAskUnclassifiedCriticFinding,
		Severity:    severity,
		Message:     text,
		Recoverable: true,
	}
}
