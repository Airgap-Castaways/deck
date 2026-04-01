package askpolicy

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestBuildEvidencePlanMarksVersionedUnknownInstallRequestRequired(t *testing.T) {
	plan := BuildEvidencePlan("Install AcmeMesh 1.37 on Ubuntu 22.04 and list prerequisites", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion})
	if plan.Decision != "required" || !plan.FreshnessSensitive || !plan.InstallEvidence || !plan.CompatibilityEvidence {
		t.Fatalf("expected required external evidence plan, got %#v", plan)
	}
	if len(plan.Entities) == 0 || plan.Entities[0].Name != "AcmeMesh" {
		t.Fatalf("expected unknown app entity extraction, got %#v", plan.Entities)
	}
}

func TestBuildEvidencePlanTreatsWorkspaceReviewAsUnnecessary(t *testing.T) {
	plan := BuildEvidencePlan("Review workflows/scenarios/apply.yaml in this workspace", askretrieve.WorkspaceSummary{HasWorkflowTree: true}, askintent.Decision{Route: askintent.RouteReview})
	if plan.Decision != "unnecessary" {
		t.Fatalf("expected local workspace prompt to avoid external docs, got %#v", plan)
	}
	if len(plan.Entities) != 0 {
		t.Fatalf("expected no external entities, got %#v", plan.Entities)
	}
}

func TestShouldUseLLMEvidencePlannerWhenExternalNeedLacksEntity(t *testing.T) {
	plan := BuildEvidencePlan("Install the latest release on Debian 12", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion})
	if !ShouldUseLLMEvidencePlanner(plan, "Install the latest release on Debian 12", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion}) {
		t.Fatalf("expected llm evidence planner fallback for ambiguous external request, got %#v", plan)
	}
}
