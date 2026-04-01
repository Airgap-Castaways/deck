package askpolicy

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestNormalizePlannedActionHandlesAddAlias(t *testing.T) {
	if got := normalizePlannedAction("add", "workflows/vars.yaml"); got != "create" {
		t.Fatalf("expected add to normalize to create, got %q", got)
	}
	if got := normalizePlannedAction("create_or_modify", "workflows/scenarios/apply.yaml"); got != "update" {
		t.Fatalf("expected create_or_modify to normalize to update, got %q", got)
	}
}

func TestMergeRequirementsWithPlanPromotesPrepareAndPlannedFiles(t *testing.T) {
	req := ScenarioRequirements{RequiredFiles: []string{"workflows/scenarios/apply.yaml"}, Connectivity: "offline"}
	merged := MergeRequirementsWithPlan(req, askcontract.PlanResponse{
		NeedsPrepare:      true,
		ArtifactKinds:     []string{"package"},
		EntryScenario:     "workflows/scenarios/apply.yaml",
		OfflineAssumption: "offline",
		Files:             []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
	})
	if !merged.NeedsPrepare || len(merged.ArtifactKinds) == 0 {
		t.Fatalf("expected prepare requirements, got %#v", merged)
	}
	if len(merged.RequiredFiles) != 2 {
		t.Fatalf("expected planned files merged into requirements, got %#v", merged.RequiredFiles)
	}
}

func TestBuildScenarioRequirementsPromotesComplexAskToComplete(t *testing.T) {
	req := BuildScenarioRequirements("create an air-gapped rhel9 3-node kubeadm cluster workflow with prepare and apply workflows", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if req.AcceptanceLevel != "complete" {
		t.Fatalf("expected complete acceptance for complex ask, got %#v", req)
	}
}

func TestBuildScenarioRequirementsUnderstandsThreeNodeSpelling(t *testing.T) {
	req := BuildScenarioRequirements("create an air-gapped rhel9 three-node kubeadm cluster workflow with prepare and apply", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if !containsString(req.ScenarioIntent, "multi-node") {
		t.Fatalf("expected multi-node intent, got %#v", req.ScenarioIntent)
	}
}

func TestBuildScenarioRequirementsCapturesSpecificTwoNodeOfflinePrompt(t *testing.T) {
	prompt := "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml. In prepare, stage kubeadm kubelet kubectl cri-tools containerd packages and Kubernetes control-plane images using typed steps only. In apply, use vars.role with allowed values control-plane and worker, bootstrap the control-plane with InitKubeadm writing /tmp/deck/join.txt, join the worker with JoinKubeadm using that same file, and run final CheckCluster only on the control-plane expecting total 2 nodes and controlPlaneReady 1. Do not use remote downloads during apply. Use workflows/vars.yaml if values repeat, and use workflows/components/ only if needed."
	req := BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if req.Connectivity != "offline" || !req.NeedsPrepare || req.AcceptanceLevel != "complete" {
		t.Fatalf("expected offline complete prepare/apply requirements, got %#v", req)
	}
	for _, want := range []string{"image", "package"} {
		if !containsString(req.ArtifactKinds, want) {
			t.Fatalf("expected artifact kind %q, got %#v", want, req.ArtifactKinds)
		}
	}
	for _, want := range []string{"kubeadm", "multi-node", "join", "node-count:2"} {
		if !containsString(req.ScenarioIntent, want) {
			t.Fatalf("expected scenario intent %q, got %#v", want, req.ScenarioIntent)
		}
	}
	for _, want := range []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml", "workflows/vars.yaml"} {
		if !containsString(req.RequiredFiles, want) {
			t.Fatalf("expected required file %q, got %#v", want, req.RequiredFiles)
		}
	}
}

func TestBuildScenarioRequirementsKeepsScenarioAndVarsTargetsForRefinePrompt(t *testing.T) {
	prompt := "Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values and preserve behavior."
	req := BuildScenarioRequirements(prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{HasWorkflowTree: true}, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}})
	for _, want := range []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"} {
		if !containsString(req.RequiredFiles, want) {
			t.Fatalf("expected required file %q, got %#v", want, req.RequiredFiles)
		}
	}
	if req.EntryScenario != "workflows/scenarios/control-plane-bootstrap.yaml" {
		t.Fatalf("expected scenario anchor entry, got %#v", req)
	}
}

func TestBuildScenarioRequirementsSupportsExplicitPrepareOnlyRequest(t *testing.T) {
	req := BuildScenarioRequirements("create a prepare workflow that downloads the runc binary into bundle storage", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "scenario", Path: "workflows/prepare.yaml"}})
	if req.EntryScenario != "" {
		t.Fatalf("expected no apply entry scenario for prepare-only request, got %#v", req)
	}
	if containsString(req.RequiredFiles, "workflows/scenarios/apply.yaml") {
		t.Fatalf("did not expect apply scenario requirement for prepare-only request, got %#v", req.RequiredFiles)
	}
	if !containsString(req.RequiredFiles, "workflows/prepare.yaml") {
		t.Fatalf("expected prepare workflow requirement for prepare-only request, got %#v", req.RequiredFiles)
	}
}

func TestBuildPlanDefaultsPreservesComplexityForComplexAsk(t *testing.T) {
	req := ScenarioRequirements{NeedsPrepare: true, ArtifactKinds: []string{"package", "image"}, ScenarioIntent: []string{"kubeadm", "multi-node", "join", "node-count:3"}, Connectivity: "offline", RequiredFiles: []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml", "workflows/vars.yaml"}}
	plan := BuildPlanDefaults(req, "create an air-gapped rhel9 3-node kubeadm workflow with prepare and apply", askintent.Decision{Route: askintent.RouteDraft}, askretrieve.WorkspaceSummary{})
	if plan.Complexity != "complex" {
		t.Fatalf("expected complex plan defaults, got %#v", plan)
	}
	if plan.AuthoringBrief.ModeIntent != "prepare+apply" {
		t.Fatalf("expected prepare+apply brief, got %#v", plan.AuthoringBrief)
	}
	if len(plan.ExecutionModel.ArtifactContracts) == 0 || plan.ExecutionModel.RoleExecution.RoleSelector != "vars.role" || plan.ExecutionModel.Verification.ExpectedNodeCount != 3 || plan.ExecutionModel.Verification.FinalVerificationRole != "control-plane" {
		t.Fatalf("expected execution model defaults for complex ask, got %#v", plan.ExecutionModel)
	}
}

func TestNormalizePlanCanonicalizesPlannerAuthoringBrief(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: "create 3-node kubeadm workflow",
		Intent:  "draft",
		AuthoringBrief: askcontract.AuthoringBrief{
			RouteIntent:          "Create staged offline workflows for kubeadm cluster bootstrap in this workspace",
			TargetScope:          "workspace-level",
			TargetPaths:          []string{"the apply scenario for this workspace"},
			ModeIntent:           "prepare and apply",
			Connectivity:         "apply runs air-gapped after prepare",
			CompletenessTarget:   "full",
			Topology:             "3-node kubeadm cluster",
			NodeCount:            3,
			RequiredCapabilities: []string{"kubeadm init/join", "offline package cache"},
		},
		Files: []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}},
	}, "create an air-gapped rhel9 3-node kubeadm workflow with prepare and apply package and image staging", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	brief := plan.AuthoringBrief
	if brief.TargetScope != "workspace" || brief.ModeIntent != "prepare+apply" || brief.Topology != "multi-node" || brief.CompletenessTarget != "complete" {
		t.Fatalf("expected canonical brief fields, got %#v", brief)
	}
	if len(brief.TargetPaths) != 2 {
		t.Fatalf("expected fallback target paths, got %#v", brief)
	}
	for _, want := range []string{"kubeadm-bootstrap", "kubeadm-join", "prepare-artifacts"} {
		found := false
		for _, capability := range brief.RequiredCapabilities {
			if capability == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in canonical capabilities, got %#v", want, brief.RequiredCapabilities)
		}
	}
	if len(plan.ExecutionModel.ArtifactContracts) == 0 {
		t.Fatalf("expected normalized execution model artifact contracts, got %#v", plan.ExecutionModel)
	}
	if plan.ExecutionModel.RoleExecution.RoleSelector != "vars.role" {
		t.Fatalf("expected fallback role selector, got %#v", plan.ExecutionModel)
	}
	if plan.ExecutionModel.Verification.ExpectedNodeCount != 3 {
		t.Fatalf("expected fallback expected node count, got %#v", plan.ExecutionModel)
	}
}

func TestNormalizePlanCanonicalizesExecutionModel(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:        "create 3-node kubeadm workflow",
		Intent:         "draft",
		AuthoringBrief: askcontract.AuthoringBrief{Topology: "multi-node", ModeIntent: "prepare+apply"},
		ExecutionModel: askcontract.ExecutionModel{
			ArtifactContracts:    []askcontract.ArtifactContract{{Kind: "Package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml", Description: "packages"}, {Kind: "unknown", ProducerPath: "bad", ConsumerPath: "bad"}},
			SharedStateContracts: []askcontract.SharedStateContract{{Name: "join-file", ProducerPath: "/tmp/deck/join.txt", ConsumerPaths: []string{" /tmp/deck/join.txt "}, AvailabilityModel: "published-for-worker-consumption"}, {Name: "", ProducerPath: "", AvailabilityModel: "weird"}},
			RoleExecution:        askcontract.RoleExecutionModel{RoleSelector: "", ControlPlaneFlow: "", WorkerFlow: "", PerNodeInvocation: false},
			Verification:         askcontract.VerificationStrategy{ExpectedNodeCount: 0},
		},
		Files: []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
	}, "create an air-gapped rhel9 3-node kubeadm workflow with prepare and apply", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	if len(plan.ExecutionModel.ArtifactContracts) == 0 || plan.ExecutionModel.ArtifactContracts[0].Kind != "package" {
		t.Fatalf("expected canonical artifact contract, got %#v", plan.ExecutionModel.ArtifactContracts)
	}
	if len(plan.ExecutionModel.SharedStateContracts) == 0 || len(plan.ExecutionModel.SharedStateContracts[0].ConsumerPaths) != 1 {
		t.Fatalf("expected canonical shared-state contract, got %#v", plan.ExecutionModel.SharedStateContracts)
	}
	if plan.ExecutionModel.RoleExecution.RoleSelector != "vars.role" || plan.ExecutionModel.Verification.ExpectedNodeCount != 3 || plan.ExecutionModel.Verification.FinalVerificationRole != "control-plane" {
		t.Fatalf("expected fallback execution details, got %#v", plan.ExecutionModel)
	}
}

func TestNormalizePlanAddsBlockingClarificationsForAmbiguousClusterRequests(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "create air-gapped kubeadm cluster workflow",
		Intent:              "draft",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		TargetOutcome:       "generate files",
		ValidationChecklist: []string{"lint"},
	}, "create air-gapped kubeadm cluster workflow", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if len(plan.Clarifications) == 0 {
		t.Fatalf("expected clarifications, got %#v", plan)
	}
	if len(plan.Blockers) == 0 {
		t.Fatalf("expected blockers derived from clarifications, got %#v", plan)
	}
}

func TestNormalizePlanAddsRuntimePlatformClarificationForPackageRequestWithoutDistro(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "create offline package staging workflow",
		Intent:              "draft",
		Files:               []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Action: "create"}},
		TargetOutcome:       "generate files",
		ValidationChecklist: []string{"lint"},
	}, "create offline package staging workflow", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if !hasClarification(plan.Clarifications, "runtime.platformFamily") {
		t.Fatalf("expected runtime platform clarification, got %#v", plan.Clarifications)
	}
}

func TestNormalizePlanAddsRefineVarsCompanionClarificationWhenVarsPathIsImplicit(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml"}, {Path: "workflows/vars.yaml"}, {Path: "workflows/scenarios/other.yaml"}}}
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "refactor control-plane bootstrap to use vars for repeated values",
		Intent:              "refine",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Action: "update"}, {Path: "workflows/scenarios/other.yaml", Action: "update"}},
		TargetOutcome:       "refine files",
		ValidationChecklist: []string{"lint"},
	}, "refactor workflows/scenarios/control-plane-bootstrap.yaml to use vars for repeated values", askretrieve.RetrievalResult{}, workspace, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}})
	if !hasClarification(plan.Clarifications, "refine.companionVars") {
		t.Fatalf("expected vars companion clarification, got %#v", plan.Clarifications)
	}
	if plan.AuthoringBrief.AnchorPaths[0] != "workflows/scenarios/control-plane-bootstrap.yaml" {
		t.Fatalf("expected scenario anchor path, got %#v", plan.AuthoringBrief)
	}
}

func TestNormalizePlanTracksRefineAnchorAndCompanionScope(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml"}, {Path: "workflows/vars.yaml"}, {Path: "workflows/scenarios/other.yaml"}}}
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "refactor scenario to use vars",
		Intent:              "refine",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Action: "update"}, {Path: "workflows/scenarios/other.yaml", Action: "update"}},
		TargetOutcome:       "refine files",
		ValidationChecklist: []string{"lint"},
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetPaths:           []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"},
			AllowedCompanionPaths: []string{"workflows/vars.yaml"},
		},
	}, "refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values", askretrieve.RetrievalResult{}, workspace, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}})
	if strings.Join(plan.AuthoringBrief.AnchorPaths, ",") != "workflows/scenarios/control-plane-bootstrap.yaml" {
		t.Fatalf("expected refine anchor path, got %#v", plan.AuthoringBrief)
	}
	if strings.Join(plan.AuthoringBrief.AllowedCompanionPaths, ",") != "workflows/vars.yaml" {
		t.Fatalf("expected vars companion path, got %#v", plan.AuthoringBrief)
	}
	if !containsString(plan.AuthoringBrief.DisallowedExpansionPaths, "workflows/scenarios/other.yaml") {
		t.Fatalf("expected unrelated scenario to be disallowed, got %#v", plan.AuthoringBrief)
	}
	if len(plan.Files) != 2 {
		t.Fatalf("expected refine files filtered to anchor and companion, got %#v", plan.Files)
	}
}

func TestNormalizePlanDropsSpuriousClusterClarificationsForJoinFileRefine(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}}}
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml for repeated join file values",
		Intent:              "refine",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "update"}, {Path: "workflows/vars.yaml", Action: "create"}},
		TargetOutcome:       "refine files",
		ValidationChecklist: []string{"lint"},
	}, "refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml for repeated join file values", askretrieve.RetrievalResult{}, workspace, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}})
	for _, unwanted := range []string{"cluster.implementation", "topology.kind", "topology.roleModel"} {
		if hasClarification(plan.Clarifications, unwanted) {
			t.Fatalf("expected refine vars prompt to avoid %s clarification, got %#v", unwanted, plan.Clarifications)
		}
	}
}

func TestNormalizePlanClearsSingleNodeWorkerExecutionForVerificationOnlyDraft(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: "Create a single-node apply workflow that verifies the cluster with CheckCluster expecting total 1 node and controlPlaneReady 1.",
		Intent:  "draft",
		AuthoringBrief: askcontract.AuthoringBrief{
			Topology:             "single-node",
			NodeCount:            1,
			ModeIntent:           "apply-only",
			RequiredCapabilities: []string{"cluster-verification"},
		},
		ExecutionModel:      askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", ControlPlaneFlow: "apply", WorkerFlow: "apply", PerNodeInvocation: true}},
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		EntryScenario:       "workflows/scenarios/apply.yaml",
		TargetOutcome:       "generate files",
		ValidationChecklist: []string{"lint"},
	}, "Create a single-node apply workflow that verifies the cluster with CheckCluster expecting total 1 node and controlPlaneReady 1.", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}})
	if plan.ExecutionModel.RoleExecution.RoleSelector != "" || plan.ExecutionModel.RoleExecution.WorkerFlow != "" || plan.ExecutionModel.RoleExecution.PerNodeInvocation {
		t.Fatalf("expected single-node verification plan to clear worker role execution, got %#v", plan.ExecutionModel.RoleExecution)
	}
	if plan.ExecutionModel.Verification.FinalVerificationRole != "local" {
		t.Fatalf("expected local final verification role, got %#v", plan.ExecutionModel.Verification)
	}
}

func TestValidatePlanStructureAllowsRecoverableExecutionDetailGaps(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
	}
	if err := ValidatePlanStructure(plan); err != nil {
		t.Fatalf("expected recoverable execution detail gaps to pass viability check, got %v", err)
	}
}

func TestValidatePlanStructureRejectsMissingViableEntryScenario(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 3},
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}},
	}
	if err := ValidatePlanStructure(plan); err == nil {
		t.Fatalf("expected missing entry scenario to fail viability check")
	}
}

func hasClarification(items []askcontract.PlanClarification, want string) bool {
	for _, item := range items {
		if item.ID == want {
			return true
		}
	}
	return false
}
