package askevidenceplan

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestBuildEvidencePlanMarksVersionedUnknownInstallRequestRequired(t *testing.T) {
	plan := BuildEvidencePlan("Install AcmeMesh 1.35.1 on Ubuntu 22.04 and list prerequisites", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion})
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

func TestBuildEvidencePlanTreatsRefineBootstrapPathAsLocal(t *testing.T) {
	plan := BuildEvidencePlan("Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values", askretrieve.WorkspaceSummary{HasWorkflowTree: true}, askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: "workflows/scenarios/control-plane-bootstrap.yaml"}})
	if plan.Decision != "unnecessary" {
		t.Fatalf("expected local refine prompt to avoid external docs, got %#v", plan)
	}
	if len(plan.Entities) != 0 {
		t.Fatalf("expected no external entities for local refine prompt, got %#v", plan.Entities)
	}
}

func TestBuildEvidencePlanTreatsInternalStepspecExplainAsLocal(t *testing.T) {
	plan := BuildEvidencePlan("Explain the typed step builders defined in internal/stepspec for DownloadPackage, InstallPackage, InitKubeadm, and JoinKubeadm.", askretrieve.WorkspaceSummary{HasWorkflowTree: true}, askintent.Decision{Route: askintent.RouteExplain, Target: askintent.Target{Kind: "component", Path: "internal/stepspec"}})
	if plan.Decision != "unnecessary" {
		t.Fatalf("expected internal code explain prompt to avoid external docs, got %#v", plan)
	}
}

func TestShouldUseLLMEvidencePlannerWhenExternalNeedLacksEntity(t *testing.T) {
	plan := BuildEvidencePlan("Install the latest release on Debian 12", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion})
	if !ShouldUseLLMEvidencePlanner(plan, "Install the latest release on Debian 12", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion}) {
		t.Fatalf("expected llm evidence planner fallback for ambiguous external request, got %#v", plan)
	}
}

func TestBuildEvidencePlanSkipsStopwordsWithoutAbortingEntityExtraction(t *testing.T) {
	plan := BuildEvidencePlan("Install the AcmeMesh on Debian 12", askretrieve.WorkspaceSummary{}, askintent.Decision{Route: askintent.RouteQuestion})
	found := false
	for _, entity := range plan.Entities {
		if entity.Name == "AcmeMesh" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected entity extraction to survive stopwords, got %#v", plan.Entities)
	}
}
