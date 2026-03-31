package askcli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestRunInteractiveClarificationsUsesDefaultsAndAnswers(t *testing.T) {
	plan := askcontract.PlanResponse{Request: "create cluster workflow", Intent: "draft", Clarifications: []askcontract.PlanClarification{
		{ID: "topology.kind", Question: "Choose topology", Options: []string{"single-node", "multi-node"}, RecommendedDefault: "multi-node", BlocksGeneration: true},
	}}
	stdin := strings.NewReader("\n")
	stdout := &bytes.Buffer{}
	updated, aborted, err := runInteractiveClarifications(stdin, stdout, plan)
	if err != nil {
		t.Fatalf("runInteractiveClarifications: %v", err)
	}
	if aborted {
		t.Fatalf("expected clarifications to complete")
	}
	if got := updated.Clarifications[0].Answer; got != "multi-node" {
		t.Fatalf("expected default answer, got %q", got)
	}
	if hasBlockingClarifications(updated) {
		t.Fatalf("expected all blocking clarifications resolved: %#v", updated.Clarifications)
	}
}

func TestRunInteractiveClarificationsCanAbort(t *testing.T) {
	plan := askcontract.PlanResponse{Clarifications: []askcontract.PlanClarification{{ID: "refine.anchorPath", Question: "Choose target", Options: []string{"workflows/scenarios/apply.yaml"}, BlocksGeneration: true}}}
	updated, aborted, err := runInteractiveClarifications(strings.NewReader("q\n"), &bytes.Buffer{}, plan)
	if err != nil {
		t.Fatalf("runInteractiveClarifications: %v", err)
	}
	if !aborted {
		t.Fatalf("expected interactive abort")
	}
	if updated.Clarifications[0].Answer != "" {
		t.Fatalf("expected unanswered clarification after abort, got %#v", updated.Clarifications)
	}
}
