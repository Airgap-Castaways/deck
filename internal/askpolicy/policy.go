package askpolicy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askauthoring"
	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

type ScenarioRequirements struct {
	AcceptanceLevel     string
	Connectivity        string
	NeedsPrepare        bool
	ArtifactKinds       []string
	RequiredFiles       []string
	EntryScenario       string
	TypedPreference     bool
	VarsAdvisories      []string
	ComponentAdvisories []string
	ScenarioIntent      []string
}

func BuildAuthoringBrief(prompt string, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) askcontract.AuthoringBrief {
	req := BuildScenarioRequirements(prompt, retrieval, workspace, decision)
	return BriefFromRequirements(req, decision)
}

func InferRequestComplexity(prompt string, req ScenarioRequirements) string {
	return inferRequestComplexity(prompt, req)
}

func DefaultValidationChecklist(req ScenarioRequirements) []string {
	return defaultValidationChecklist(req)
}

func BriefFromRequirements(req ScenarioRequirements, decision askintent.Decision) askcontract.AuthoringBrief {
	brief := askcontract.AuthoringBrief{
		RouteIntent:          string(decision.Route),
		TargetScope:          inferTargetScope(req, decision),
		TargetPaths:          briefTargetPaths(req),
		ModeIntent:           inferModeIntent(req),
		Connectivity:         strings.TrimSpace(req.Connectivity),
		CompletenessTarget:   strings.TrimSpace(req.AcceptanceLevel),
		Topology:             inferTopology(req),
		NodeCount:            inferNodeCount(req),
		RequiredCapabilities: inferRequiredCapabilities(req),
	}
	brief.TargetPaths = dedupeStrings(brief.TargetPaths)
	brief.RequiredCapabilities = dedupeStrings(brief.RequiredCapabilities)
	return brief
}

func ExecutionModelFromRequirements(req ScenarioRequirements) askcontract.ExecutionModel {
	facts := askauthoring.InferFacts(strings.Join(req.ScenarioIntent, " ")+" "+strings.Join(req.ArtifactKinds, " "), req.ArtifactKinds, req.Connectivity)
	graph := askauthoring.BuildContractGraph(facts, askauthoring.RequirementLike{
		Connectivity:   req.Connectivity,
		NeedsPrepare:   req.NeedsPrepare,
		ArtifactKinds:  req.ArtifactKinds,
		ScenarioIntent: req.ScenarioIntent,
		RequiredFiles:  req.RequiredFiles,
		EntryScenario:  req.EntryScenario,
	}, askretrieve.WorkspaceSummary{})
	model := graph.ExecutionModel()
	if model.Verification.ExpectedNodeCount <= 0 {
		model.Verification.ExpectedNodeCount = expectedNodeCountFromRequirements(req)
	}
	if model.Verification.ExpectedControlPlaneReady <= 0 {
		model.Verification.ExpectedControlPlaneReady = expectedControlPlaneReadyFromRequirements(req)
	}
	if strings.TrimSpace(model.RoleExecution.ControlPlaneFlow) == "" {
		model.RoleExecution.ControlPlaneFlow = controlPlaneFlowFromRequirements(req)
	}
	if strings.TrimSpace(model.RoleExecution.WorkerFlow) == "" {
		model.RoleExecution.WorkerFlow = workerFlowFromRequirements(req)
	}
	if strings.TrimSpace(model.Verification.BootstrapPhase) == "" {
		model.Verification.BootstrapPhase = bootstrapPhaseFromRequirements(req)
	}
	if strings.TrimSpace(model.Verification.FinalPhase) == "" {
		model.Verification.FinalPhase = finalPhaseFromRequirements(req)
	}
	if strings.TrimSpace(model.Verification.FinalVerificationRole) == "" {
		model.Verification.FinalVerificationRole = finalVerificationRoleFromRequirements(req)
	}
	return model
}

type EvaluationFinding struct {
	Severity string
	Code     string
	Message  string
	Fix      string
	Path     string
}

type EvaluationResult struct {
	Findings []EvaluationFinding
}

func BuildScenarioRequirements(prompt string, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) ScenarioRequirements {
	requestedMode := requestedWorkflowMode(prompt)
	artifactKinds := mergedArtifactKinds(prompt, retrieval)
	facts := askauthoring.InferFacts(prompt, artifactKinds, InferOfflineAssumption(prompt))
	needsPrepare := facts.NeedsPrepare
	req := ScenarioRequirements{
		AcceptanceLevel:     inferAcceptanceLevel(prompt, workspace, decision),
		Connectivity:        facts.Connectivity,
		NeedsPrepare:        needsPrepare,
		ArtifactKinds:       artifactKinds,
		RequiredFiles:       nil,
		EntryScenario:       "",
		TypedPreference:     typedPreferenceRequested(prompt),
		VarsAdvisories:      inferVarsRecommendation(prompt),
		ComponentAdvisories: inferComponentRecommendation(prompt),
		ScenarioIntent:      append([]string(nil), facts.Intents...),
	}
	if req.AcceptanceLevel == "starter" {
		req.ComponentAdvisories = nil
	}
	if requestedMode != "prepare-only" {
		req.RequiredFiles = append(req.RequiredFiles, "workflows/scenarios/apply.yaml")
		req.EntryScenario = "workflows/scenarios/apply.yaml"
	}
	if needsPrepare || requestedMode == "prepare-only" {
		req.RequiredFiles = append(req.RequiredFiles, "workflows/prepare.yaml")
	}
	if strings.Contains(strings.ToLower(prompt), "vars") || len(req.VarsAdvisories) > 0 {
		req.RequiredFiles = append(req.RequiredFiles, "workflows/vars.yaml")
	}
	for _, path := range askintent.ExtractWorkflowPaths(prompt) {
		req.RequiredFiles = append(req.RequiredFiles, path)
		if strings.HasPrefix(path, "workflows/scenarios/") || path == "workflows/prepare.yaml" {
			req.EntryScenario = path
		}
	}
	if workspace.HasWorkflowTree && decision.Route == askintent.RouteRefine {
		// retain known required files for refine if prepare already exists
		if workspace.HasPrepare && !containsString(req.RequiredFiles, "workflows/prepare.yaml") && needsPrepare {
			req.RequiredFiles = append(req.RequiredFiles, "workflows/prepare.yaml")
		}
	}
	req.RequiredFiles = dedupeStrings(req.RequiredFiles)
	return req
}

func requestedWorkflowMode(prompt string) string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	mentionsPrepare := strings.Contains(lower, "prepare workflow") || strings.Contains(lower, "prepare-only") || strings.Contains(lower, "prepare only")
	mentionsApply := strings.Contains(lower, "apply workflow") || strings.Contains(lower, "apply-only") || strings.Contains(lower, "apply only") || strings.Contains(lower, "scenario workflow")
	switch {
	case mentionsPrepare && !mentionsApply:
		return "prepare-only"
	case mentionsApply && !mentionsPrepare:
		return "apply-only"
	default:
		return "workspace"
	}
}

func controlPlaneFlowFromRequirements(req ScenarioRequirements) string {
	if containsString(req.ScenarioIntent, "kubeadm") {
		return "preflight -> runtime setup -> InitKubeadm -> bootstrap verification"
	}
	return ""
}

func workerFlowFromRequirements(req ScenarioRequirements) string {
	if containsString(req.ScenarioIntent, "join") || containsString(req.ScenarioIntent, "multi-node") || containsString(req.ScenarioIntent, "ha") {
		return "preflight -> runtime setup -> JoinKubeadm -> final cluster verification"
	}
	return ""
}

func bootstrapPhaseFromRequirements(req ScenarioRequirements) string {
	if containsString(req.ScenarioIntent, "kubeadm") {
		return "bootstrap-control-plane"
	}
	return ""
}

func finalPhaseFromRequirements(req ScenarioRequirements) string {
	if containsString(req.ScenarioIntent, "kubeadm") {
		return "verify-cluster"
	}
	return ""
}

func finalVerificationRoleFromRequirements(req ScenarioRequirements) string {
	if containsString(req.ScenarioIntent, "multi-node") || containsString(req.ScenarioIntent, "ha") || containsString(req.ScenarioIntent, "join") {
		return "control-plane"
	}
	if containsString(req.ScenarioIntent, "kubeadm") {
		return "control-plane"
	}
	return "local"
}

func expectedNodeCountFromRequirements(req ScenarioRequirements) int {
	if n := inferNodeCount(req); n > 0 {
		return n
	}
	return 0
}

func expectedControlPlaneReadyFromRequirements(req ScenarioRequirements) int {
	if containsString(req.ScenarioIntent, "ha") {
		if n := inferNodeCount(req); n >= 3 {
			return 3
		}
		return 1
	}
	if containsString(req.ScenarioIntent, "kubeadm") {
		return 1
	}
	return 0
}

func dedupeArtifactContracts(contracts []askcontract.ArtifactContract) []askcontract.ArtifactContract {
	seen := map[string]bool{}
	out := make([]askcontract.ArtifactContract, 0, len(contracts))
	for _, item := range contracts {
		key := item.Kind + "|" + item.ProducerPath + "|" + item.ConsumerPath
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func MergeRequirementsWithPlan(req ScenarioRequirements, plan askcontract.PlanResponse) ScenarioRequirements {
	if strings.TrimSpace(plan.OfflineAssumption) != "" {
		req.Connectivity = strings.TrimSpace(plan.OfflineAssumption)
	}
	if plan.NeedsPrepare {
		req.NeedsPrepare = true
	}
	if len(plan.ArtifactKinds) > 0 {
		req.ArtifactKinds = NormalizeArtifactKinds(append(req.ArtifactKinds, plan.ArtifactKinds...))
	}
	if len(plan.VarsRecommendation) > 0 {
		req.VarsAdvisories = dedupeStrings(append(req.VarsAdvisories, plan.VarsRecommendation...))
	}
	if len(plan.ComponentRecommendation) > 0 {
		req.ComponentAdvisories = dedupeStrings(append(req.ComponentAdvisories, plan.ComponentRecommendation...))
	}
	if strings.TrimSpace(plan.EntryScenario) != "" {
		req.EntryScenario = strings.TrimSpace(plan.EntryScenario)
	}
	for _, file := range plan.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path != "" {
			req.RequiredFiles = append(req.RequiredFiles, path)
		}
	}
	req.RequiredFiles = dedupeStrings(req.RequiredFiles)
	return req
}

func EvaluateGeneration(req ScenarioRequirements, plan askcontract.PlanResponse, gen askcontract.GenerationResponse) EvaluationResult {
	findings := make([]EvaluationFinding, 0)
	brief := plan.AuthoringBrief
	executionModel := plan.ExecutionModel
	if strings.TrimSpace(brief.ModeIntent) == "prepare+apply" {
		if len(preparePathsFromGeneration(gen.Files)) == 0 {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "brief_prepare_missing", Message: "request expects both prepare and apply workflows but generated output is missing workflows/prepare.yaml", Fix: "Return both a prepare workflow and at least one apply scenario when the request asks for prepare and apply", Path: "workflows/prepare.yaml"})
		}
		if len(scenarioLikePathsWithName(gen.Files, "apply")) == 0 {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "brief_apply_missing", Message: "request expects both prepare and apply workflows but generated output is missing an apply scenario", Fix: "Return a scenario entrypoint under workflows/scenarios/ for apply execution", Path: "workflows/scenarios/apply.yaml"})
		}
	}
	if strings.TrimSpace(brief.TargetScope) == "workspace" && len(brief.TargetPaths) > 1 {
		generatedPaths := generatedMap(gen.Files)
		for _, path := range brief.TargetPaths {
			if _, ok := generatedPaths[path]; !ok {
				severity := "blocking"
				code := "brief_target_missing"
				message := fmt.Sprintf("generated output is missing expected target from authoring brief: %s", path)
				fix := "Return the expected workflow files for the full workspace-scoped request"
				if isAdvisoryComponentTarget(path, brief) {
					severity = "advisory"
					code = "brief_component_target_advisory"
					message = fmt.Sprintf("authoring brief recommends extracted component target but schema-valid entry workflows take priority for the first draft: %s", path)
					fix = "Start with schema-valid entry workflows, then extract reusable component files after the draft validates"
				}
				findings = append(findings, EvaluationFinding{Severity: severity, Code: code, Message: message, Fix: fix, Path: path})
			}
		}
	}
	generatedPaths := generatedMap(gen.Files)
	for _, contract := range executionModel.ArtifactContracts {
		producer := filepath.ToSlash(strings.TrimSpace(contract.ProducerPath))
		consumer := filepath.ToSlash(strings.TrimSpace(contract.ConsumerPath))
		if producer != "" {
			if _, ok := generatedPaths[producer]; !ok {
				severity := "blocking"
				code := "execution_model_producer_missing"
				message := fmt.Sprintf("generated output is missing artifact producer required by execution model: %s", producer)
				fix := "Generate the workflow file that produces staged artifacts before apply consumes them"
				if isAdvisoryComponentTarget(producer, brief) {
					severity = "advisory"
					code = "execution_model_component_producer_advisory"
					message = fmt.Sprintf("execution model recommends an extracted component artifact producer, but schema-valid entry workflows take priority for the first draft: %s", producer)
					fix = "Keep the producer contract clear in entry workflows first, then extract the component file after validation passes"
				}
				findings = append(findings, EvaluationFinding{Severity: severity, Code: code, Message: message, Fix: fix, Path: producer})
			}
		}
		if consumer != "" {
			if _, ok := generatedPaths[consumer]; !ok {
				severity := "blocking"
				code := "execution_model_consumer_missing"
				message := fmt.Sprintf("generated output is missing artifact consumer required by execution model: %s", consumer)
				fix := "Generate the workflow file that consumes the staged artifacts described by the execution model"
				if isAdvisoryComponentTarget(consumer, brief) {
					severity = "advisory"
					code = "execution_model_component_consumer_advisory"
					message = fmt.Sprintf("execution model recommends an extracted component artifact consumer, but schema-valid entry workflows take priority for the first draft: %s", consumer)
					fix = "Keep the consumer contract clear in entry workflows first, then extract the component file after validation passes"
				}
				findings = append(findings, EvaluationFinding{Severity: severity, Code: code, Message: message, Fix: fix, Path: consumer})
			}
		}
	}
	for _, contract := range executionModel.SharedStateContracts {
		if !generationAppearsToHandleSharedState(gen.Files, contract) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "execution_model_shared_state_missing", Message: fmt.Sprintf("execution model expects shared-state handling for %s but generated output does not clearly model production and consumption", strings.TrimSpace(contract.Name)), Fix: "Add explicit production and consumption steps or clearly model the shared availability contract in the affected workflow", Path: "workflows/scenarios/apply.yaml"})
		}
		if strings.EqualFold(strings.TrimSpace(contract.AvailabilityModel), "published-for-worker-consumption") && !generationAppearsToPublishSharedState(gen.Files, contract) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "execution_model_shared_state_publish_missing", Message: fmt.Sprintf("execution model expects published shared-state availability for %s but generated output does not show an explicit publication or unambiguous handoff", strings.TrimSpace(contract.Name)), Fix: "Publish the shared-state artifact explicitly with a typed file or directory step before consumer steps run", Path: "workflows/scenarios/apply.yaml"})
		}
	}
	if executionModel.RoleExecution.PerNodeInvocation && strings.TrimSpace(executionModel.RoleExecution.RoleSelector) != "" {
		if !generationAppearsRoleAware(gen.Files, executionModel.RoleExecution.RoleSelector) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "execution_model_role_selector_missing", Message: fmt.Sprintf("execution model expects role-aware per-node invocation via %s but generated workflows do not appear to branch on it", executionModel.RoleExecution.RoleSelector), Fix: "Add role-aware conditions or separate role-specific phases that use the execution model role selector", Path: "workflows/scenarios/apply.yaml"})
		}
		if generationViolatesFinalVerificationRole(gen.Files, executionModel.Verification.FinalVerificationRole) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "execution_model_final_verify_role_mismatch", Message: fmt.Sprintf("final cluster verification does not appear to run on the expected %s role", executionModel.Verification.FinalVerificationRole), Fix: "Move final CheckCluster verification to the role required by the execution model or make the role gate explicit", Path: "workflows/scenarios/apply.yaml"})
		}
	}
	if generationViolatesVerificationExpectations(gen.Files, executionModel.Verification) {
		findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "execution_model_verification_mismatch", Message: fmt.Sprintf("generated CheckCluster expectations do not match the execution model verification contract (expected nodes=%d controlPlaneReady=%d)", executionModel.Verification.ExpectedNodeCount, executionModel.Verification.ExpectedControlPlaneReady), Fix: "Align final CheckCluster node expectations with the execution model topology contract", Path: "workflows/scenarios/apply.yaml"})
	}
	if hasCapability(brief.RequiredCapabilities, "cluster-verification") && !hasKind(generatedWorkflowSteps(gen.Files), "CheckCluster") {
		findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "missing_cluster_verification", Message: "request requires cluster verification but generated workflow does not include a CheckCluster step", Fix: "Use the typed CheckCluster step when the plan requires cluster verification", Path: "workflows/scenarios/apply.yaml"})
	}
	if req.TypedPreference && countCommands(gen.Files) > 0 {
		alternatives := askcontext.StrongTypedAlternativesWithOptions(plan.Request, askcontext.StepGuidanceOptions{ModeIntent: plan.AuthoringBrief.ModeIntent, Topology: plan.AuthoringBrief.Topology, RequiredCapabilities: plan.AuthoringBrief.RequiredCapabilities})
		if len(alternatives) > 0 {
			kinds := make([]string, 0, len(alternatives))
			for _, step := range alternatives {
				kinds = append(kinds, step.Kind)
			}
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "typed_preference", Message: fmt.Sprintf("request prefers typed steps but generation still relies on Command; consider %s before falling back to Command", strings.Join(dedupeStrings(kinds), ", "))})
		}
	}
	applyPaths := scenarioLikePathsWithName(gen.Files, "apply")
	preparePaths := preparePathsFromGeneration(gen.Files)
	if strings.EqualFold(strings.TrimSpace(req.Connectivity), "offline") {
		for _, file := range gen.Files {
			clean := filepath.ToSlash(strings.TrimSpace(file.Path))
			if !containsString(applyPaths, clean) {
				continue
			}
			if onlineActivityDetected(file.Content) {
				findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "offline_apply_online_retrieval", Message: fmt.Sprintf("offline apply workflow performs online retrieval: %s", clean), Fix: "Move online downloads, pulls, or mirror refresh work into prepare and keep apply offline", Path: clean})
			}
		}
	}
	for _, file := range gen.Files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		for _, violation := range constrainedLiteralViolations(file.Content) {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "constrained_literal_template", Message: fmt.Sprintf("constrained typed field uses vars template in %s: %s", clean, violation), Fix: fmt.Sprintf("Keep %s as a literal allowed value instead of a vars template", violation), Path: clean})
		}
		if clean == "workflows/prepare.yaml" && prepareUsesCommandForArtifacts(file.Content, req.ArtifactKinds) {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "prepare_command_artifacts", Message: fmt.Sprintf("prepare workflow uses Command for artifact collection where a typed prepare step should be used: %s", clean), Fix: "Use typed prepare steps such as DownloadImage or DownloadPackage instead of Command for artifact collection in prepare", Path: clean})
		}
	}
	if req.NeedsPrepare && len(preparePaths) == 0 {
		findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "missing_prepare", Message: "artifact-requiring request is missing workflows/prepare.yaml", Fix: "Add workflows/prepare.yaml when packages, images, binaries, or bundles must be prepared before apply", Path: "workflows/prepare.yaml"})
	}
	if req.NeedsPrepare && len(req.ArtifactKinds) > 0 && len(preparePaths) > 0 {
		generated := generatedMap(gen.Files)
		for _, path := range preparePaths {
			if !prepareAppearsArtifactOriented(generated[path].Content, req.ArtifactKinds) {
				findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "prepare_not_artifact_oriented", Message: fmt.Sprintf("prepare workflow does not appear to stage requested artifacts: %s", path), Fix: "Ensure prepare collects or stages the packages, images, binaries, or bundles required before apply", Path: path})
			}
		}
	}
	if scenarioAppearsIncomplete(req, gen.Files) {
		findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "scenario_intent_incomplete", Message: fmt.Sprintf("generated workflow does not fully match requested scenario intent: %s", strings.TrimSpace(plan.Request)), Fix: "Add the missing install or bootstrap stages required by the requested scenario before returning the workflow"})
	}
	if !generatedHas(gen.Files, "workflows/vars.yaml") {
		for _, reason := range req.VarsAdvisories {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "vars_advisory", Message: reason})
		}
		for _, reason := range inferVarsAdvisories(gen.Files) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "vars_repetition", Message: reason})
		}
	}
	if !hasGeneratedComponents(gen.Files) {
		for _, reason := range req.ComponentAdvisories {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "component_advisory", Message: reason})
		}
		for _, reason := range inferComponentAdvisories(gen.Files) {
			findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "component_repetition", Message: reason})
		}
	}
	return EvaluationResult{Findings: findings}
}

func isAdvisoryComponentTarget(path string, brief askcontract.AuthoringBrief) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if !strings.HasPrefix(path, "workflows/components/") {
		return false
	}
	if strings.TrimSpace(brief.TargetScope) == "component" {
		return false
	}
	return true
}

func generationAppearsToHandleJoinState(files []askcontract.GeneratedFile) bool {
	for _, step := range generatedWorkflowSteps(files) {
		if strings.EqualFold(step.Kind, "InitKubeadm") {
			if value, ok := stringSpec(step.Spec, "outputJoinFile"); ok && strings.TrimSpace(value) != "" {
				return true
			}
		}
		if strings.EqualFold(step.Kind, "JoinKubeadm") {
			if value, ok := stringSpec(step.Spec, "joinFile"); ok && strings.TrimSpace(value) != "" {
				return true
			}
		}
	}
	return false
}

func generationAppearsToHandleSharedState(files []askcontract.GeneratedFile, contract askcontract.SharedStateContract) bool {
	name := strings.ToLower(strings.TrimSpace(contract.Name))
	producerPath := strings.ToLower(strings.TrimSpace(contract.ProducerPath))
	for _, step := range generatedWorkflowSteps(files) {
		for _, field := range []string{"outputJoinFile", "joinFile", "path", "sourcePath", "targetPath"} {
			if value, ok := stringSpec(step.Spec, field); ok {
				lower := strings.ToLower(strings.TrimSpace(value))
				if producerPath != "" && lower == producerPath {
					return true
				}
				if name != "" && strings.Contains(lower, name) {
					return true
				}
			}
		}
	}
	if name == "join-file" {
		return generationAppearsToHandleJoinState(files)
	}
	return false
}

func generationAppearsToPublishJoinState(files []askcontract.GeneratedFile, producerPath string) bool {
	producerPath = strings.ToLower(strings.TrimSpace(producerPath))
	for _, step := range generatedWorkflowSteps(files) {
		if strings.EqualFold(step.Kind, "InitKubeadm") {
			if value, ok := stringSpec(step.Spec, "outputJoinFile"); ok {
				lower := strings.ToLower(strings.TrimSpace(value))
				if lower != "" && (producerPath == "" || lower == producerPath) {
					return true
				}
			}
		}
	}
	for _, step := range generatedWorkflowSteps(files) {
		if strings.EqualFold(step.Kind, "CopyFile") || strings.EqualFold(step.Kind, "EnsureDirectory") || strings.EqualFold(step.Kind, "WriteFile") {
			for _, field := range []string{"path", "targetPath", "sourcePath"} {
				if value, ok := stringSpec(step.Spec, field); ok {
					lower := strings.ToLower(strings.TrimSpace(value))
					if producerPath != "" && lower == producerPath {
						return true
					}
					if strings.Contains(lower, "join") {
						return true
					}
				}
			}
		}
	}
	return false
}

func generationAppearsToPublishSharedState(files []askcontract.GeneratedFile, contract askcontract.SharedStateContract) bool {
	if canonicalSharedStateName(contract.Name) == "join-file" {
		return generationAppearsToPublishJoinState(files, contract.ProducerPath)
	}
	name := strings.ToLower(strings.TrimSpace(contract.Name))
	producerPath := strings.ToLower(strings.TrimSpace(contract.ProducerPath))
	for _, step := range generatedWorkflowSteps(files) {
		for _, field := range []string{"path", "sourcePath", "targetPath", "outputJoinFile"} {
			if value, ok := stringSpec(step.Spec, field); ok {
				lower := strings.ToLower(strings.TrimSpace(value))
				if (name != "" && strings.Contains(lower, name)) || (producerPath != "" && lower == producerPath) {
					return true
				}
			}
		}
	}
	return false
}

func generationViolatesFinalVerificationRole(files []askcontract.GeneratedFile, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" || role == "any" || role == "local" {
		return false
	}
	for _, step := range generatedWorkflowSteps(files) {
		if !strings.EqualFold(step.Kind, "CheckCluster") {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(step.When)), strings.ToLower(role)) {
			return false
		}
	}
	return true
}

func generationViolatesVerificationExpectations(files []askcontract.GeneratedFile, verification askcontract.VerificationStrategy) bool {
	if verification.ExpectedNodeCount <= 0 || verification.ExpectedControlPlaneReady <= 0 {
		return false
	}
	for _, step := range generatedWorkflowSteps(files) {
		if !strings.EqualFold(step.Kind, "CheckCluster") {
			continue
		}
		total, totalOK := intSpec(step.Spec, "nodes", "total")
		cp, cpOK := intSpec(step.Spec, "nodes", "controlPlaneReady")
		if totalOK && cpOK && total == verification.ExpectedNodeCount && cp == verification.ExpectedControlPlaneReady {
			return false
		}
	}
	return true
}

func generationAppearsRoleAware(files []askcontract.GeneratedFile, roleSelector string) bool {
	selector := strings.ToLower(strings.TrimSpace(roleSelector))
	if selector == "" {
		return true
	}
	for _, step := range generatedWorkflowSteps(files) {
		if strings.Contains(strings.ToLower(strings.TrimSpace(step.When)), selector) {
			return true
		}
	}
	return false
}

func generatedWorkflowSteps(files []askcontract.GeneratedFile) []askcontract.WorkflowStep {
	out := []askcontract.WorkflowStep{}
	for _, file := range files {
		if file.Delete {
			continue
		}
		doc, err := askir.ParseDocument(file.Path, []byte(file.Content))
		if err != nil || doc.Workflow == nil {
			continue
		}
		out = append(out, doc.Workflow.Steps...)
		for _, phase := range doc.Workflow.Phases {
			out = append(out, phase.Steps...)
		}
	}
	return out
}

func stringSpec(spec map[string]any, keys ...string) (string, bool) {
	value, ok := nestedSpec(spec, keys...)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func intSpec(spec map[string]any, keys ...string) (int, bool) {
	value, ok := nestedSpec(spec, keys...)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func nestedSpec(spec map[string]any, keys ...string) (any, bool) {
	current := any(spec)
	for _, key := range keys {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func defaultValidationChecklist(req ScenarioRequirements) []string {
	checklist := []string{"Workflow schema validates", "Entrypoint scenarios are loadable", "Planned files are generated"}
	if req.NeedsPrepare {
		checklist = append(checklist, "Artifact-requiring offline flows include a prepare workflow before apply")
	}
	if len(req.VarsAdvisories) > 0 {
		checklist = append(checklist, "Review whether repeated configurable values belong in workflows/vars.yaml")
	}
	if len(req.ComponentAdvisories) > 0 {
		checklist = append(checklist, "Review whether reusable repeated logic belongs in workflows/components/")
	}
	return checklist
}

func mergedArtifactKinds(prompt string, retrieval askretrieve.RetrievalResult) []string {
	_ = retrieval
	return InferArtifactKinds(strings.TrimSpace(prompt), nil)
}

func inferVarsRecommendation(prompt string) []string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if strings.Contains(lower, "vars") || strings.Contains(lower, "variable") || strings.Contains(lower, "variables") || strings.Contains(lower, "repeated") || strings.Contains(lower, "parameter") {
		return []string{"Use workflows/vars.yaml for repeated package, image, path, or version values."}
	}
	return nil
}

func inferComponentRecommendation(prompt string) []string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if strings.Contains(lower, "component") || strings.Contains(lower, "components") || strings.Contains(lower, "reusable") || strings.Contains(lower, "shared fragment") {
		return []string{"Consider workflows/components/ for reusable repeated logic across phases or scenarios."}
	}
	return nil
}

func inferAcceptanceLevel(prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) string {
	facts := askauthoring.InferFacts(prompt, InferArtifactKinds(prompt, nil), InferOfflineAssumption(prompt))
	if facts.Topology == "multi-node" || facts.Topology == "ha" || facts.NodeCount > 1 {
		if decision.Route == askintent.RouteRefine && workspace.HasWorkflowTree {
			return "refine"
		}
		return "complete"
	}
	score := 0
	if facts.NeedsPrepare {
		score++
	}
	if len(facts.ArtifactKinds) > 1 {
		score++
	}
	if len(facts.Capabilities) > 2 {
		score++
	}
	if len(facts.Ambiguities) > 0 {
		score++
	}
	if score >= 2 {
		if decision.Route == askintent.RouteRefine && workspace.HasWorkflowTree {
			return "refine"
		}
		return "complete"
	}
	if decision.Route == askintent.RouteRefine {
		if !workspace.HasWorkflowTree {
			return "starter"
		}
		return "refine"
	}
	if decision.Route == askintent.RouteDraft && !workspace.HasWorkflowTree {
		return "starter"
	}
	return "complete"
}

func inferRequestComplexity(prompt string, req ScenarioRequirements) string {
	facts := askauthoring.InferFacts(prompt, req.ArtifactKinds, req.Connectivity)
	score := 0
	if req.NeedsPrepare {
		score++
	}
	if facts.RequestedMode == "workspace" && req.NeedsPrepare && strings.TrimSpace(req.EntryScenario) != "" {
		score++
	}
	if facts.Topology == "multi-node" || facts.Topology == "ha" {
		score += 2
	}
	if containsString(facts.Intents, "kubeadm") {
		score++
	}
	if facts.MultiRoleRequested {
		score++
	}
	if len(req.ArtifactKinds) > 1 {
		score++
	}
	switch {
	case score >= 4:
		return "complex"
	case score >= 2:
		return "medium"
	default:
		return "simple"
	}
}

func inferModeIntent(req ScenarioRequirements) string {
	hasPrepare := containsString(req.RequiredFiles, "workflows/prepare.yaml") || req.NeedsPrepare
	hasApply := containsString(req.RequiredFiles, "workflows/scenarios/apply.yaml") || strings.TrimSpace(req.EntryScenario) != ""
	switch {
	case hasPrepare && hasApply:
		return "prepare+apply"
	case hasPrepare:
		return "prepare-only"
	case hasApply:
		return "apply-only"
	default:
		return "workspace"
	}
}

func inferTargetScope(req ScenarioRequirements, decision askintent.Decision) string {
	if len(briefTargetPaths(req)) > 1 {
		return "workspace"
	}
	if decision.Target.Kind == "scenario" && strings.TrimSpace(decision.Target.Path) != "" && inferModeIntent(req) != "prepare+apply" {
		return "scenario"
	}
	if decision.Target.Kind == "vars" && len(briefTargetPaths(req)) == 1 {
		return "vars"
	}
	if decision.Target.Kind == "component" && len(briefTargetPaths(req)) == 1 {
		return "component"
	}
	if inferModeIntent(req) == "prepare+apply" || len(req.RequiredFiles) > 1 {
		return "workspace"
	}
	return "workspace"
}

func briefTargetPaths(req ScenarioRequirements) []string {
	paths := append([]string(nil), req.RequiredFiles...)
	if strings.TrimSpace(req.EntryScenario) != "" {
		paths = append(paths, req.EntryScenario)
	}
	return dedupeStrings(paths)
}

func inferTopology(req ScenarioRequirements) string {
	for _, intent := range req.ScenarioIntent {
		switch strings.ToLower(strings.TrimSpace(intent)) {
		case "ha":
			return "ha"
		case "multi-node":
			return "multi-node"
		case "single-node":
			return "single-node"
		}
	}
	return "unspecified"
}

func inferNodeCount(req ScenarioRequirements) int {
	for _, intent := range req.ScenarioIntent {
		lower := strings.ToLower(strings.TrimSpace(intent))
		if strings.HasPrefix(lower, "node-count:") {
			value := strings.TrimSpace(strings.TrimPrefix(lower, "node-count:"))
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				return n
			}
		}
	}
	if inferTopology(req) == "single-node" {
		return 1
	}
	return 0
}

func inferRequiredCapabilities(req ScenarioRequirements) []string {
	facts := askauthoring.InferFacts(strings.Join(req.ScenarioIntent, " "), req.ArtifactKinds, req.Connectivity)
	capabilities := append([]string(nil), facts.Capabilities...)
	if req.NeedsPrepare && strings.EqualFold(strings.TrimSpace(req.Connectivity), "offline") {
		capabilities = append(capabilities, "prepare-artifacts")
		if containsString(req.ArtifactKinds, "package") || containsString(req.ScenarioIntent, "kubeadm") {
			capabilities = append(capabilities, "package-staging")
		}
		if containsString(req.ArtifactKinds, "image") || containsString(req.ScenarioIntent, "kubeadm") {
			capabilities = append(capabilities, "image-staging")
		}
	}
	return dedupeStrings(capabilities)
}

func typedPreferenceRequested(prompt string) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	return strings.Contains(lower, "typed step") || strings.Contains(lower, "typed steps") || strings.Contains(lower, "where possible")
}

func generatedMap(files []askcontract.GeneratedFile) map[string]askcontract.GeneratedFile {
	out := map[string]askcontract.GeneratedFile{}
	for _, file := range files {
		if file.Delete {
			continue
		}
		out[filepath.ToSlash(strings.TrimSpace(file.Path))] = file
	}
	return out
}

func generatedHas(files []askcontract.GeneratedFile, want string) bool {
	for _, file := range files {
		if file.Delete {
			continue
		}
		if filepath.ToSlash(strings.TrimSpace(file.Path)) == want {
			return true
		}
	}
	return false
}

func countCommands(files []askcontract.GeneratedFile) int {
	count := 0
	for _, file := range files {
		if file.Delete {
			continue
		}
		for _, line := range strings.Split(file.Content, "\n") {
			if strings.TrimSpace(line) == "kind: Command" {
				count++
			}
		}
	}
	return count
}

func scenarioLikePathsWithName(files []askcontract.GeneratedFile, name string) []string {
	paths := []string{}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, file := range files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		if strings.HasPrefix(clean, "workflows/scenarios/") && strings.Contains(strings.ToLower(clean), name) {
			paths = append(paths, clean)
		}
	}
	return dedupeStrings(paths)
}

func preparePathsFromGeneration(files []askcontract.GeneratedFile) []string {
	paths := []string{}
	for _, file := range files {
		if filepath.ToSlash(strings.TrimSpace(file.Path)) == "workflows/prepare.yaml" {
			paths = append(paths, "workflows/prepare.yaml")
		}
	}
	return dedupeStrings(paths)
}

func onlineActivityDetected(content string) bool {
	if doc, err := askir.ParseDocument("workflows/scenarios/apply.yaml", []byte(content)); err == nil && doc.Workflow != nil {
		for _, step := range generatedWorkflowSteps([]askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: content}}) {
			switch strings.TrimSpace(step.Kind) {
			case "DownloadPackage", "DownloadImage", "RefreshRepository":
				return true
			case "Command":
				if commandSpecLooksOnline(step.Spec) {
					return true
				}
			}
		}
	}
	lower := strings.ToLower(content)
	hints := []string{"curl", "wget", "dnf download", "apt-get download", "docker pull", "ctr image pull", "podman pull", "repo sync", "refreshrepository", "downloadpackage", "downloadimage"}
	for _, hint := range hints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func prepareAppearsArtifactOriented(content string, artifactKinds []string) bool {
	steps := generatedWorkflowSteps([]askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: content}})
	if len(steps) > 0 {
		for _, kind := range artifactKinds {
			switch strings.ToLower(strings.TrimSpace(kind)) {
			case "package":
				if hasKind(steps, "DownloadPackage") {
					return true
				}
			case "image":
				if hasKind(steps, "DownloadImage") {
					return true
				}
			case "repository-mirror":
				if hasKind(steps, "DownloadPackage") || hasKind(steps, "ConfigureRepository") {
					return true
				}
			}
		}
	}
	lower := strings.ToLower(content)
	hasPackages := strings.Contains(lower, "kind: downloadpackage") || strings.Contains(lower, "packages:")
	hasImages := strings.Contains(lower, "kind: downloadimage") || strings.Contains(lower, "images:")
	hasOutputDir := strings.Contains(lower, "outputdir:")
	for _, kind := range artifactKinds {
		switch strings.ToLower(strings.TrimSpace(kind)) {
		case "package":
			if hasPackages || strings.Contains(lower, "installpackage") || strings.Contains(lower, "/prepared/packages") || hasOutputDir {
				return true
			}
		case "image":
			if hasImages || strings.Contains(lower, "loadimage") || strings.Contains(lower, "/prepared/images") || hasOutputDir {
				return true
			}
		case "binary", "archive", "bundle", "repository-mirror":
			if strings.Contains(lower, "writefile") || strings.Contains(lower, "/prepared/files") || strings.Contains(lower, "configurerepository") || strings.Contains(lower, "command") {
				return true
			}
		}
	}
	return len(artifactKinds) == 0
}

func constrainedLiteralViolations(content string) []string {
	violations := []string{}
	for _, step := range askcatalog.Current().StepKinds() {
		for _, field := range step.ConstrainedLiteralFields {
			if fieldUsesVarsTemplate(content, field.Path) {
				violations = append(violations, field.Path)
			}
		}
	}
	return dedupeStrings(violations)
}

func fieldUsesVarsTemplate(content string, path string) bool {
	segments := strings.Split(strings.TrimSpace(path), ".")
	if len(segments) < 2 {
		return false
	}
	keys := segments[1:]
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != keys[0]+":" {
			continue
		}
		baseIndent := len(line) - len(strings.TrimLeft(line, " "))
		idx := i + 1
		for segIdx := 1; segIdx < len(keys) && idx < len(lines); {
			line = lines[idx]
			idx++
			if strings.TrimSpace(line) == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " "))
			trimmed = strings.TrimSpace(line)
			if indent <= baseIndent {
				break
			}
			want := keys[segIdx] + ":"
			if trimmed == want {
				baseIndent = indent
				segIdx++
				continue
			}
			if segIdx == len(keys)-1 && strings.HasPrefix(trimmed, want) {
				value := strings.TrimSpace(strings.TrimPrefix(trimmed, want))
				return strings.Contains(value, "{{ .vars.")
			}
		}
	}
	return false
}

func prepareUsesCommandForArtifacts(content string, artifactKinds []string) bool {
	steps := generatedWorkflowSteps([]askcontract.GeneratedFile{{Path: "workflows/prepare.yaml", Content: content}})
	foundCommand := false
	for _, step := range steps {
		if strings.TrimSpace(step.Kind) != "Command" {
			continue
		}
		foundCommand = true
		if commandSpecLooksOnline(step.Spec) {
			return true
		}
	}
	lower := strings.ToLower(content)
	if !foundCommand && !strings.Contains(lower, "kind: command") {
		return false
	}
	if containsString(artifactKinds, "image") && (strings.Contains(lower, "docker pull") || strings.Contains(lower, "docker save") || strings.Contains(lower, "ctr image pull") || strings.Contains(lower, "podman pull")) {
		return true
	}
	if containsString(artifactKinds, "package") && (strings.Contains(lower, "dnf download") || strings.Contains(lower, "apt-get download")) {
		return true
	}
	return false
}

func inferVarsAdvisories(files []askcontract.GeneratedFile) []string {
	counts := map[string]int{}
	for _, file := range files {
		for _, line := range strings.Split(file.Content, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if !strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "- ") {
				continue
			}
			if strings.Contains(trimmed, "{{") {
				continue
			}
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if idx := strings.Index(value, ":"); idx >= 0 {
				value = strings.TrimSpace(value[idx+1:])
			}
			value = strings.Trim(value, `"'`)
			if len(value) < 4 {
				continue
			}
			if looksInterestingRepeatedValue(value) {
				counts[value]++
			}
		}
	}
	advisories := []string{}
	for value, count := range counts {
		if count >= 2 {
			advisories = append(advisories, fmt.Sprintf("Repeated configurable value %q appears multiple times; consider moving it into workflows/vars.yaml", value))
		}
	}
	return dedupeStrings(advisories)
}

func inferComponentAdvisories(files []askcontract.GeneratedFile) []string {
	sequenceCounts := map[string]int{}
	for _, doc := range generatedWorkflowDocs(files) {
		kinds := []string{}
		if doc.Workflow == nil {
			continue
		}
		for _, step := range doc.Workflow.Steps {
			if strings.TrimSpace(step.Kind) != "" {
				kinds = append(kinds, strings.TrimSpace(step.Kind))
			}
		}
		for _, phase := range doc.Workflow.Phases {
			for _, step := range phase.Steps {
				if strings.TrimSpace(step.Kind) != "" {
					kinds = append(kinds, strings.TrimSpace(step.Kind))
				}
			}
		}
		if len(kinds) >= 2 {
			sequenceCounts[strings.Join(kinds, ">")] += 1
		}
	}
	advisories := []string{}
	for seq, count := range sequenceCounts {
		if count >= 2 {
			advisories = append(advisories, fmt.Sprintf("Repeated step sequence %q appears across generated files; consider moving it into workflows/components/", seq))
		}
	}
	return dedupeStrings(advisories)
}

func looksInterestingRepeatedValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "/") || strings.Contains(lower, ".") || strings.Contains(lower, "v1.") || strings.Contains(lower, "registry") || strings.Contains(lower, "kube") || strings.Contains(lower, "containerd")
}

func scenarioAppearsIncomplete(req ScenarioRequirements, files []askcontract.GeneratedFile) bool {
	if req.AcceptanceLevel == "starter" {
		return false
	}
	steps := generatedWorkflowSteps(files)
	hasInit := hasKind(steps, "InitKubeadm") || hasKind(steps, "UpgradeKubeadm")
	hasCheck := hasKind(steps, "CheckCluster")
	for _, intent := range req.ScenarioIntent {
		if intent == "kubeadm" {
			if !hasInit {
				return true
			}
			if !hasCheck {
				return true
			}
		}
	}
	if len(req.ArtifactKinds) > 0 && req.NeedsPrepare && len(preparePathsFromGeneration(files)) == 0 {
		return true
	}
	return false
}

func commandSpecLooksOnline(spec map[string]any) bool {
	value, ok := spec["command"]
	if !ok {
		return false
	}
	text := strings.ToLower(fmt.Sprint(value))
	for _, hint := range []string{"curl", "wget", "docker pull", "ctr image pull", "podman pull", "dnf download", "apt-get download", "repo sync"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

func hasKind(steps []askcontract.WorkflowStep, kind string) bool {
	for _, step := range steps {
		if strings.EqualFold(strings.TrimSpace(step.Kind), kind) {
			return true
		}
	}
	return false
}

func hasCapability(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func hasGeneratedComponents(files []askcontract.GeneratedFile) bool {
	for _, file := range files {
		if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(file.Path)), "workflows/components/") {
			return true
		}
	}
	return false
}

func generatedWorkflowDocs(files []askcontract.GeneratedFile) []askcontract.GeneratedDocument {
	out := []askcontract.GeneratedDocument{}
	for _, file := range files {
		if file.Delete {
			continue
		}
		doc, err := askir.ParseDocument(file.Path, []byte(file.Content))
		if err != nil {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
