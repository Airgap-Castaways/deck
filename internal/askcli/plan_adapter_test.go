package askcli

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestAdaptPlanBoundaryCanonicalizesPlannerAliases(t *testing.T) {
	plan := adaptPlanBoundary(askcontract.PlanResponse{
		Clarifications: []askcontract.PlanClarification{
			{ID: "platform-family", Answer: "rhel"},
			{ID: "repo-access-mode", Answer: "filesystem-path"},
			{Question: "Which role layout should the plan use for control-plane and worker nodes?", Options: []string{"1 control-plane + 2 workers"}, Answer: "1 control-plane + 2 workers"},
		},
		Files: []askcontract.PlanFile{{Path: "workflows/scenarios/apply.yaml", Action: "modify"}, {Path: "workflows/vars.yaml", Action: "add"}, {Path: "workflows/prepare.yaml", Action: "create_or_modify"}},
	})
	if got := plan.Clarifications[0].ID; got != "runtime.platformFamily" {
		t.Fatalf("expected canonical platform clarification id, got %q", got)
	}
	if got := plan.Clarifications[1].ID; got != "repo-delivery" {
		t.Fatalf("expected canonical repo clarification id, got %q", got)
	}
	if got := plan.Clarifications[2].ID; got != "topology.roleModel" || plan.Clarifications[2].Answer != "1cp-2workers" {
		t.Fatalf("expected canonical role model clarification, got %#v", plan.Clarifications[2])
	}
	if got := plan.Files[0].Action; got != "update" {
		t.Fatalf("expected canonical modify action, got %q", got)
	}
	if got := plan.Files[1].Action; got != "create" {
		t.Fatalf("expected canonical add action, got %q", got)
	}
	if got := plan.Files[2].Action; got != "update" {
		t.Fatalf("expected canonical create_or_modify action, got %q", got)
	}
}

func TestAdaptPlanBoundaryPreservesUnknownClarificationIDs(t *testing.T) {
	plan := adaptPlanBoundary(askcontract.PlanResponse{Clarifications: []askcontract.PlanClarification{{ID: "artifact-publish-model", Answer: "local-http-server"}}})
	if got := plan.Clarifications[0].ID; got != "artifact-publish-model" {
		t.Fatalf("expected unknown id to be preserved, got %q", got)
	}
}
