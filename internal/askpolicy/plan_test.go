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
	prompt := "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml. In prepare, stage kubeadm kubelet kubectl cri-tools containerd packages and Kubernetes control-plane images using typed steps only. In apply, use vars.role with allowed values control-plane and worker, bootstrap the control-plane with InitKubeadm writing /tmp/deck/join.txt, join the worker with JoinKubeadm using that same file, and run final CheckKubernetesCluster only on the control-plane expecting total 2 nodes and controlPlaneReady 1. Do not use remote downloads during apply. Use workflows/vars.yaml if values repeat, and use workflows/components/ only if needed."
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

func TestBuildScenarioRequirementsIgnoresExternalEvidenceArtifactKinds(t *testing.T) {
	retrieval := askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}", Evidence: &askretrieve.EvidenceSummary{ArtifactKinds: []string{"package"}, OfflineHints: []string{"Treat gathered installation artifacts as offline bundle inputs for prepare before apply."}}}}}
	req := BuildScenarioRequirements("create a minimal single-node apply-only kubeadm workflow using only init-kubeadm and check-kubernetes-cluster builders", retrieval, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if req.NeedsPrepare {
		t.Fatalf("expected external evidence artifact hints not to force prepare, got %#v", req)
	}
	if len(req.ArtifactKinds) != 0 {
		t.Fatalf("expected external evidence artifact kinds not to enter requirements, got %#v", req.ArtifactKinds)
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
	if len(brief.TargetPaths) != 3 || !containsString(brief.TargetPaths, "workflows/prepare.yaml") || !containsString(brief.TargetPaths, "workflows/scenarios/apply.yaml") || !containsString(brief.TargetPaths, "workflows/vars.yaml") {
		t.Fatalf("expected canonical fallback target paths including vars companion, got %#v", brief)
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

func TestNormalizePlanReplacesLocalOnlyJoinContractForMultiNodeFlow(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: "create 2-node kubeadm workflow",
		Intent:  "draft",
		AuthoringBrief: askcontract.AuthoringBrief{
			Topology:             "multi-node",
			NodeCount:            2,
			ModeIntent:           "prepare+apply",
			RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join", "cluster-verification"},
		},
		ExecutionModel: askcontract.ExecutionModel{
			SharedStateContracts: []askcontract.SharedStateContract{{
				Name:              "join-file",
				ProducerPath:      "workflows/scenarios/apply.yaml",
				ConsumerPaths:     []string{"workflows/scenarios/apply.yaml"},
				AvailabilityModel: "local-only",
				Description:       "worker reuses the same local path",
			}},
			RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true},
		},
		Files: []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}, {Path: "workflows/vars.yaml"}},
	}, "Create an offline RHEL 9 kubeadm workflow for exactly 2 nodes: 1 control-plane and 1 worker. Generate both workflows/prepare.yaml and workflows/scenarios/apply.yaml.", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	if len(plan.ExecutionModel.SharedStateContracts) == 0 {
		t.Fatalf("expected normalized shared-state contract, got %#v", plan.ExecutionModel)
	}
	join := plan.ExecutionModel.SharedStateContracts[0]
	if join.AvailabilityModel != "published-for-worker-consumption" {
		t.Fatalf("expected published join contract, got %#v", join)
	}
	if join.ProducerPath != "/tmp/deck/join.txt" {
		t.Fatalf("expected canonical join producer path, got %#v", join)
	}
	if len(join.ConsumerPaths) != 1 || join.ConsumerPaths[0] != "/tmp/deck/join.txt" {
		t.Fatalf("expected canonical join consumer paths, got %#v", join)
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
	if len(plan.Blockers) != 0 {
		t.Fatalf("expected clarifications to stay separate from blockers, got %#v", plan)
	}
}

func TestNormalizePlanKeepsExplicitBlockersButDoesNotMirrorClarifications(t *testing.T) {
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             "create air-gapped kubeadm cluster workflow",
		Intent:              "draft",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		TargetOutcome:       "generate files",
		ValidationChecklist: []string{"lint"},
		Blockers:            []string{"unsupported authoring coverage"},
	}, "create air-gapped kubeadm cluster workflow", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if len(plan.Blockers) != 1 || plan.Blockers[0] != "unsupported authoring coverage" {
		t.Fatalf("expected explicit blocker to remain, got %#v", plan.Blockers)
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

func TestNormalizePlanSkipsRuntimePlatformClarificationForMinimalApplyOnlySingleNodeBootstrap(t *testing.T) {
	prompt := "Create a minimal single-node apply-only offline kubeadm workflow for Kubernetes 1.35.1 using only init-kubeadm and check-kubernetes-cluster builders"
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:             prompt,
		Intent:              "draft",
		Files:               []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		TargetOutcome:       "generate files",
		ValidationChecklist: []string{"lint"},
	}, prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if hasClarification(plan.Clarifications, "runtime.platformFamily") {
		t.Fatalf("expected no runtime platform clarification for minimal apply-only flow, got %#v", plan.Clarifications)
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

func TestNormalizePlanUsesCodeOwnedTwoNodeRoleClarificationShape(t *testing.T) {
	prompt := "Create an offline RHEL 9 kubeadm workflow for 2 nodes"
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: prompt,
		Intent:  "draft",
		Clarifications: []askcontract.PlanClarification{{
			ID:                 "topology.roleModel",
			Question:           "Planner drifted into a vague role-model question",
			Kind:               "enum",
			Options:            []string{"weird-option", "custom"},
			RecommendedDefault: "weird-option",
		}},
		Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
	}, prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	var roleClarification askcontract.PlanClarification
	for _, item := range plan.Clarifications {
		if item.ID == "topology.roleModel" {
			roleClarification = item
			break
		}
	}
	if roleClarification.ID == "" {
		t.Fatalf("expected role-model clarification, got %#v", plan.Clarifications)
	}
	if strings.Join(roleClarification.Options, ",") != "1cp-1worker,custom" {
		t.Fatalf("expected code-owned two-node options, got %#v", roleClarification)
	}
	if roleClarification.RecommendedDefault != "1cp-1worker" {
		t.Fatalf("expected code-owned two-node default, got %#v", roleClarification)
	}
	if !strings.Contains(strings.ToLower(roleClarification.Question), "role") {
		t.Fatalf("expected canonical role-model question, got %#v", roleClarification)
	}
}

func TestNormalizePlanReconcilesTwoNodeClarificationAnswers(t *testing.T) {
	prompt := "Create an offline RHEL 9 kubeadm workflow for 2 nodes"
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: prompt,
		Intent:  "draft",
		AuthoringBrief: askcontract.AuthoringBrief{
			Topology:             "unspecified",
			NodeCount:            3,
			ModeIntent:           "prepare+apply",
			RequiredCapabilities: []string{"kubeadm-bootstrap", "kubeadm-join", "cluster-verification"},
		},
		AuthoringProgram: askcontract.AuthoringProgram{
			Cluster:      askcontract.ProgramCluster{ControlPlaneCount: 2, WorkerCount: 1},
			Verification: askcontract.ProgramVerification{ExpectedNodeCount: 3, ExpectedControlPlaneReady: 2},
		},
		ExecutionModel: askcontract.ExecutionModel{
			RoleExecution: askcontract.RoleExecutionModel{PerNodeInvocation: false},
			Verification:  askcontract.VerificationStrategy{ExpectedNodeCount: 3, ExpectedControlPlaneReady: 2},
		},
		Clarifications: []askcontract.PlanClarification{{ID: "topology.nodeCount", Answer: "2"}, {ID: "topology.roleModel", Answer: "1cp-1worker"}},
		Files:          []askcontract.PlanFile{{Path: "workflows/prepare.yaml", Action: "create"}, {Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
	}, prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if plan.AuthoringBrief.Topology != "multi-node" || plan.AuthoringBrief.NodeCount != 2 {
		t.Fatalf("expected reconciled two-node topology, got %#v", plan.AuthoringBrief)
	}
	if plan.AuthoringProgram.Cluster.ControlPlaneCount != 1 || plan.AuthoringProgram.Cluster.WorkerCount != 1 {
		t.Fatalf("expected reconciled cluster counts, got %#v", plan.AuthoringProgram.Cluster)
	}
	if plan.ExecutionModel.Verification.ExpectedNodeCount != 2 || plan.ExecutionModel.Verification.ExpectedControlPlaneReady != 1 {
		t.Fatalf("expected reconciled verification counts, got %#v", plan.ExecutionModel.Verification)
	}
	if plan.AuthoringProgram.Verification.ExpectedNodeCount != 2 || plan.AuthoringProgram.Verification.ExpectedControlPlaneReady != 1 {
		t.Fatalf("expected reconciled program verification counts, got %#v", plan.AuthoringProgram.Verification)
	}
	if plan.ExecutionModel.RoleExecution.RoleSelector != "vars.role" || !plan.ExecutionModel.RoleExecution.PerNodeInvocation {
		t.Fatalf("expected reconciled role execution, got %#v", plan.ExecutionModel.RoleExecution)
	}
}

func TestNormalizePlanDropsDirectoryLikePlannerPaths(t *testing.T) {
	prompt := "Create an offline RHEL 9 kubeadm workflow for 2 nodes"
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: prompt,
		Intent:  "draft",
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetPaths:           []string{"workflows/scenarios/apply.yaml"},
			AnchorPaths:           []string{"workflows/components/"},
			AllowedCompanionPaths: []string{"workflows/components/", "workflows/scenarios/"},
		},
		Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
	}, prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if len(plan.AuthoringBrief.AnchorPaths) != 1 || plan.AuthoringBrief.AnchorPaths[0] != "workflows/scenarios/apply.yaml" {
		t.Fatalf("expected directory-like anchor path to be dropped in favor of scenario target, got %#v", plan.AuthoringBrief)
	}
	if len(plan.AuthoringBrief.AllowedCompanionPaths) != 0 {
		t.Fatalf("expected directory-like companion paths to be dropped, got %#v", plan.AuthoringBrief)
	}
}

func TestNormalizePlanDropsGlobLikePlannerPaths(t *testing.T) {
	prompt := "refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values"
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml"}, {Path: "workflows/vars.yaml"}, {Path: "workflows/components/bootstrap.yaml"}}}
	plan := NormalizePlan(askcontract.PlanResponse{
		Request: prompt,
		Intent:  "refine",
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetPaths:           []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/scenarios/*.yaml", "workflows/vars.yaml"},
			AllowedCompanionPaths: []string{"workflows/components/*.yaml", "workflows/vars.yaml"},
		},
		Files:         []askcontract.PlanFile{{Path: "workflows/scenarios/*.yaml", Action: "update"}, {Path: "workflows/scenarios/control-plane-bootstrap.yaml", Action: "update"}, {Path: "workflows/vars.yaml", Action: "update"}},
		EntryScenario: "workflows/scenarios/*.yaml",
	}, prompt, askretrieve.RetrievalResult{}, workspace, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}})
	for _, unwanted := range []string{"workflows/scenarios/*.yaml", "workflows/components/*.yaml"} {
		if containsString(plan.AuthoringBrief.TargetPaths, unwanted) || containsString(plan.AuthoringBrief.AllowedCompanionPaths, unwanted) {
			t.Fatalf("expected glob-like path %q to be dropped, got %#v", unwanted, plan.AuthoringBrief)
		}
	}
	if plan.EntryScenario != "workflows/scenarios/control-plane-bootstrap.yaml" {
		t.Fatalf("expected normalized concrete entry scenario, got %#v", plan.EntryScenario)
	}
	for _, file := range plan.Files {
		if strings.Contains(file.Path, "*") {
			t.Fatalf("expected glob-like planned files to be dropped, got %#v", plan.Files)
		}
	}
}

func TestAskcontractPathAllowedRejectsGlobLikePaths(t *testing.T) {
	for _, path := range []string{"workflows/scenarios/*.yaml", "workflows/components/*.yaml", "workflows/components/**/*.yaml"} {
		if askcontractPathAllowed(path) {
			t.Fatalf("expected glob-like path %q to be rejected", path)
		}
	}
}

func TestNormalizePlanAddsVarsFileForRoleGatedDrafts(t *testing.T) {
	prompt := "Create an offline RHEL 9 kubeadm workflow for 2 nodes"
	plan := NormalizePlan(askcontract.PlanResponse{
		Request:        prompt,
		Intent:         "draft",
		AuthoringBrief: askcontract.AuthoringBrief{Topology: "multi-node", NodeCount: 2, ModeIntent: "apply-only", TargetPaths: []string{"workflows/scenarios/apply.yaml"}},
		ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role", PerNodeInvocation: true, WorkerFlow: "join"}},
		Files:          []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
	}, prompt, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft})
	if !containsString(plan.AuthoringBrief.TargetPaths, "workflows/vars.yaml") {
		t.Fatalf("expected role-gated draft to plan vars companion, got %#v", plan.AuthoringBrief)
	}
	if !planHasFilePath(plan.Files, "workflows/vars.yaml") {
		t.Fatalf("expected role-gated draft to include vars file in plan, got %#v", plan.Files)
	}
	if !containsString(plan.VarsRecommendation, "role") {
		t.Fatalf("expected role vars recommendation, got %#v", plan.VarsRecommendation)
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
		Request: "Create a single-node apply workflow that verifies the cluster with CheckKubernetesCluster expecting total 1 node and controlPlaneReady 1.",
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
	}, "Create a single-node apply workflow that verifies the cluster with CheckKubernetesCluster expecting total 1 node and controlPlaneReady 1.", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/apply.yaml"}})
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

func TestValidatePlanStructureRejectsPackageStagingWithoutPackages(t *testing.T) {
	plan := askcontract.PlanResponse{
		NeedsPrepare:        true,
		ArtifactKinds:       []string{"package"},
		AuthoringBrief:      askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 2, RequiredCapabilities: []string{"prepare-artifacts", "package-staging", "kubeadm-bootstrap", "cluster-verification"}},
		AuthoringProgram:    askcontract.AuthoringProgram{Platform: askcontract.ProgramPlatform{Family: "rhel", Release: "9"}, Cluster: askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt"}, Verification: askcontract.ProgramVerification{ExpectedNodeCount: 2}},
		ExecutionModel:      askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "package", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 2}},
		Files:               []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		EntryScenario:       "workflows/scenarios/apply.yaml",
		ValidationChecklist: []string{"lint"},
	}
	if err := ValidatePlanStructure(plan); err == nil || !strings.Contains(err.Error(), "artifacts.packages") {
		t.Fatalf("expected missing package payload to fail viability check, got %v", err)
	}
}

func TestValidatePlanStructureRejectsImageStagingWithoutImages(t *testing.T) {
	plan := askcontract.PlanResponse{
		NeedsPrepare:        true,
		ArtifactKinds:       []string{"image"},
		AuthoringBrief:      askcontract.AuthoringBrief{ModeIntent: "prepare+apply", Topology: "multi-node", NodeCount: 2, RequiredCapabilities: []string{"prepare-artifacts", "image-staging", "kubeadm-bootstrap", "cluster-verification"}},
		AuthoringProgram:    askcontract.AuthoringProgram{Platform: askcontract.ProgramPlatform{Family: "rhel", Release: "9"}, Cluster: askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt"}, Verification: askcontract.ProgramVerification{ExpectedNodeCount: 2}},
		ExecutionModel:      askcontract.ExecutionModel{ArtifactContracts: []askcontract.ArtifactContract{{Kind: "image", ProducerPath: "workflows/prepare.yaml", ConsumerPath: "workflows/scenarios/apply.yaml"}}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 2}},
		Files:               []askcontract.PlanFile{{Path: "workflows/prepare.yaml"}, {Path: "workflows/scenarios/apply.yaml"}},
		EntryScenario:       "workflows/scenarios/apply.yaml",
		ValidationChecklist: []string{"lint"},
	}
	if err := ValidatePlanStructure(plan); err == nil || !strings.Contains(err.Error(), "artifacts.images") {
		t.Fatalf("expected missing image payload to fail viability check, got %v", err)
	}
}

func TestEvaluatePlanConformanceDoesNotRequireVarsFromChecklistMentionOnly(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files:          []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "create"}},
		ValidationChecklist: []string{
			"Do not add prepare workflow, components, or vars unless later requested.",
		},
	}
	gen := askcontract.GenerationResponse{Files: []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}}
	result := EvaluatePlanConformance(plan, gen, askintent.Decision{Route: askintent.RouteDraft})
	for _, finding := range result.Findings {
		if finding.Code == "vars_required_by_checklist" {
			t.Fatalf("expected checklist mention alone not to require vars file, got %#v", result.Findings)
		}
	}
}

func TestEvaluatePlanConformanceRequiresVarsWhenPlanned(t *testing.T) {
	plan := askcontract.PlanResponse{
		AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml", "workflows/vars.yaml"}},
		EntryScenario:  "workflows/scenarios/apply.yaml",
		Files: []askcontract.PlanFile{
			{Path: "workflows/scenarios/apply.yaml", Action: "create"},
			{Path: "workflows/vars.yaml", Action: "create"},
		},
	}
	gen := askcontract.GenerationResponse{Files: []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps: []\n"}}}
	result := EvaluatePlanConformance(plan, gen, askintent.Decision{Route: askintent.RouteDraft})
	found := false
	for _, finding := range result.Findings {
		if finding.Code == "vars_required_by_checklist" {
			found = true
			if finding.Path != "workflows/vars.yaml" {
				t.Fatalf("expected vars path finding, got %#v", finding)
			}
		}
	}
	if !found {
		t.Fatalf("expected planned vars file to remain required, got %#v", result.Findings)
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
