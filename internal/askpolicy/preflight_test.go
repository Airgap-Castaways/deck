package askpolicy

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func TestBuildAuthoringPreflightKeepsExplicitRefineScope(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: workspacepaths.CanonicalApplyWorkflow, Content: "version: v1alpha1\n"}, {Path: workspacepaths.CanonicalVarsWorkflow, Content: "podCIDR: 10.244.0.0/16\n"}, {Path: "workflows/scenarios/other.yaml", Content: "version: v1alpha1\n"}}}
	decision := askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: workspacepaths.CanonicalApplyWorkflow}}
	plan, _ := BuildAuthoringPreflight("refine workflows/scenarios/apply.yaml to hoist repeated values into workflows/vars.yaml", askretrieve.RetrievalResult{}, workspace, decision, nil)
	if !containsString(plan.AuthoringBrief.TargetPaths, workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("expected explicit scenario target in preflight: %#v", plan.AuthoringBrief.TargetPaths)
	}
	if !containsString(plan.AuthoringBrief.TargetPaths, workspacepaths.CanonicalVarsWorkflow) {
		t.Fatalf("expected explicit vars companion in preflight: %#v", plan.AuthoringBrief.TargetPaths)
	}
	if containsString(plan.AuthoringBrief.TargetPaths, "workflows/scenarios/other.yaml") {
		t.Fatalf("did not expect unrelated scenario in preflight scope: %#v", plan.AuthoringBrief.TargetPaths)
	}
}

func TestBuildAuthoringPreflightAddsRuntimeClarificationForPackageDraftWithoutPlatform(t *testing.T) {
	decision := askintent.Decision{Route: askintent.RouteDraft}
	plan, _ := BuildAuthoringPreflight("create an apply-only workflow that installs the docker package", askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, decision, nil)
	if !PlanNeedsClarification(plan) {
		t.Fatalf("expected runtime clarification, got %#v", plan.Clarifications)
	}
	found := false
	for _, item := range plan.Clarifications {
		if item.ID == "runtime.platformFamily" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected runtime.platformFamily clarification, got %#v", plan.Clarifications)
	}
}

func TestBuildAuthoringPreflightSuppressesTopologyClarificationsForAnchoredRefine(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: workspacepaths.CanonicalApplyWorkflow, Content: "version: v1alpha1\n"}}}
	decision := askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: workspacepaths.CanonicalApplyWorkflow}}
	plan, _ := BuildAuthoringPreflight("refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml for repeated values", askretrieve.RetrievalResult{}, workspace, decision, nil)
	for _, item := range plan.Clarifications {
		switch item.ID {
		case "topology.kind", "topology.roleModel", "topology.nodeCount", "cluster.implementation", "runtime.platformFamily":
			t.Fatalf("did not expect topology/runtime clarification on anchored refine: %#v", plan.Clarifications)
		}
	}
}

func TestBuildAuthoringPreflightDoesNotPullCanonicalApplyIntoExplicitRefine(t *testing.T) {
	workspace := askretrieve.WorkspaceSummary{HasWorkflowTree: true, Files: []askretrieve.WorkspaceFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Content: "version: v1alpha1\n"}, {Path: workspacepaths.CanonicalVarsWorkflow, Content: "kubernetesVersion: v1.30.1\n"}}}
	decision := askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}}
	plan, _ := BuildAuthoringPreflight("Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values", askretrieve.RetrievalResult{}, workspace, decision, nil)
	if containsString(plan.AuthoringBrief.TargetPaths, workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("did not expect canonical apply scenario in explicit refine scope: %#v", plan.AuthoringBrief.TargetPaths)
	}
	if !containsString(plan.AuthoringBrief.TargetPaths, "workflows/scenarios/control-plane-bootstrap.yaml") || !containsString(plan.AuthoringBrief.TargetPaths, workspacepaths.CanonicalVarsWorkflow) {
		t.Fatalf("expected refine scope to stay on explicit scenario plus vars: %#v", plan.AuthoringBrief.TargetPaths)
	}
}
