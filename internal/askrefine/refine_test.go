package askrefine

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestCandidatesForDocumentsIncludeFieldAndComponentCandidates(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}}}}}}}
	items := CandidatesForDocuments(askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}}}, []askcontract.GeneratedDocument{doc})
	joined := []string{}
	for _, item := range items {
		joined = append(joined, item.ID)
	}
	text := strings.Join(joined, "\n")
	for _, want := range []string{"extract-component|workflows/scenarios/apply.yaml|phases[0]", "set-field|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.podNetworkCIDR", "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.podNetworkCIDR"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in candidate ids, got %q", want, text)
		}
	}
}

func TestResolveCandidateFillsTransformPaths(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}}}}}}}
	resolved, err := ResolveCandidate(doc, askcontract.RefineTransformAction{Candidate: "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.podNetworkCIDR"})
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if resolved.Type != "extract-var" || resolved.RawPath != "phases[0].steps[0].spec.podNetworkCIDR" || resolved.VarName != "podnetworkcidr" {
		t.Fatalf("expected resolved candidate fields, got %#v", resolved)
	}
}
