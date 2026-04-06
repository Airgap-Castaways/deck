package askcli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestApplyPlanAnswersInfersRefineRouteFromPlannedUpdates(t *testing.T) {
	plan := askcontract.PlanResponse{
		Request: "Refine workflows/scenarios/worker-join.yaml to keep the anchor file stable and move reusable repeated logic into workflows/components if needed",
		Intent:  "Refine worker-join scenario composition for stability and reuse.",
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetScope:           "scenario",
			TargetPaths:           []string{"workflows/scenarios/worker-join.yaml"},
			AnchorPaths:           []string{"workflows/scenarios/worker-join.yaml"},
			AllowedCompanionPaths: []string{"workflows/components/", "workflows/vars.yaml"},
			ModeIntent:            "workspace",
			CompletenessTarget:    "refine",
		},
		Files: []askcontract.PlanFile{{Path: "workflows/scenarios/worker-join.yaml", Action: "update"}},
		Clarifications: []askcontract.PlanClarification{
			{ID: "cluster.implementation", Question: "Which implementation should the plan use?", Kind: "enum", Options: []string{"custom", "kubeadm"}, RecommendedDefault: "kubeadm", BlocksGeneration: true},
			{ID: "refine.componentPath", Question: "Which component path should the plan allow?", Kind: "path", Options: []string{"workflows/components/worker-join-shared.yaml", "none"}, RecommendedDefault: "workflows/components/worker-join-shared.yaml", BlocksGeneration: true},
		},
	}
	updated, err := applyPlanAnswers(plan, []string{"cluster.implementation=kubeadm"})
	if err != nil {
		t.Fatalf("apply plan answers: %v", err)
	}
	if !hasClarification(updated.Clarifications, "refine.componentPath") {
		t.Fatalf("expected refine.componentPath to remain after normalize, got %#v", updated.Clarifications)
	}
	if !hasAnsweredClarification(updated.Clarifications, "cluster.implementation", "kubeadm") {
		t.Fatalf("expected stored cluster implementation answer, got %#v", updated.Clarifications)
	}
}

func TestRunInteractiveClarificationsReevaluatesBlockingItemsAfterEachAnswer(t *testing.T) {
	plan := askcontract.PlanResponse{
		Request: "Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values",
		Intent:  "Refactor scenario literals into shared vars.",
		AuthoringBrief: askcontract.AuthoringBrief{
			TargetScope:        "scenario",
			TargetPaths:        []string{"workflows/scenarios/control-plane-bootstrap.yaml", "workflows/vars.yaml"},
			AnchorPaths:        []string{"workflows/scenarios/control-plane-bootstrap.yaml"},
			ModeIntent:         "apply-only",
			CompletenessTarget: "refine",
		},
		Files: []askcontract.PlanFile{{Path: "workflows/scenarios/control-plane-bootstrap.yaml", Action: "update"}},
		Clarifications: []askcontract.PlanClarification{
			{ID: "cluster.implementation", Question: "Which implementation should the plan use?", Kind: "enum", Options: []string{"custom", "kubeadm"}, RecommendedDefault: "kubeadm", BlocksGeneration: true},
			{ID: "refine.componentPath", Question: "Which component path should the plan allow?", Kind: "path", Options: []string{"workflows/components/bootstrap-shared.yaml", "none"}, RecommendedDefault: "none", BlocksGeneration: true},
		},
	}
	updated, aborted, err := runInteractiveClarifications(strings.NewReader("\n"), &bytes.Buffer{}, plan)
	if err != nil {
		t.Fatalf("runInteractiveClarifications: %v", err)
	}
	if aborted {
		t.Fatalf("expected clarifications to complete")
	}
	if hasClarification(updated.Clarifications, "refine.componentPath") {
		t.Fatalf("expected stale refine.componentPath clarification to be dropped after re-evaluation, got %#v", updated.Clarifications)
	}
	if !hasAnsweredClarification(updated.Clarifications, "cluster.implementation", "kubeadm") {
		t.Fatalf("expected cluster implementation answer to persist, got %#v", updated.Clarifications)
	}
}

func hasClarification(items []askcontract.PlanClarification, want string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.ID) == want {
			return true
		}
	}
	return false
}

func hasAnsweredClarification(items []askcontract.PlanClarification, want string, answer string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.ID) == want && strings.TrimSpace(item.Answer) == answer {
			return true
		}
	}
	return false
}
