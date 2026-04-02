package askrefine

import (
	"errors"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestCandidatesForDocumentsIncludeFieldAndComponentCandidates(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Vars: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}, Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}}}}}}}
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
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Vars: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}, Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"podNetworkCIDR": "10.244.0.0/16"}}}}}}}
	resolved, err := ResolveCandidate(doc, askcontract.RefineTransformAction{Candidate: "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.podNetworkCIDR"})
	if err != nil {
		t.Fatalf("resolve candidate: %v", err)
	}
	if resolved.Type != "extract-var" || resolved.RawPath != "phases[0].steps[0].spec.podNetworkCIDR" || resolved.VarName != "podnetworkcidr" {
		t.Fatalf("expected resolved candidate fields, got %#v", resolved)
	}
}

func TestResolveCandidateAcceptsStructurallyValidExplicitExtractVarCandidate(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"kubernetesVersion": "1.35.1"}}}}}
	resolved, err := ResolveCandidate(doc, askcontract.RefineTransformAction{Candidate: "extract-var|workflows/scenarios/apply.yaml|steps[0].spec.kubernetesVersion", VarName: "kubernetesVersion"})
	if err != nil {
		t.Fatalf("resolve explicit extract-var candidate: %v", err)
	}
	if resolved.RawPath != "steps[0].spec.kubernetesVersion" || resolved.Type != "extract-var" {
		t.Fatalf("expected explicit extract-var candidate to resolve, got %#v", resolved)
	}
}

func TestCandidatesForDocumentsExcludeExtractVarForConstrainedAndNumericFields(t *testing.T) {
	doc := askcontract.GeneratedDocument{
		Path: "workflows/scenarios/apply.yaml",
		Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{
			Name: "verify",
			Steps: []askcontract.WorkflowStep{{
				ID:   "check",
				Kind: "CheckCluster",
				Spec: map[string]any{"interval": "5s", "timeout": "10m", "nodes": map[string]any{"total": 1}},
			}},
		}}},
	}
	items := CandidatesForDocuments(askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}}}, []askcontract.GeneratedDocument{doc})
	joined := []string{}
	for _, item := range items {
		joined = append(joined, item.ID)
	}
	text := strings.Join(joined, "\n")
	for _, unwanted := range []string{"extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.interval", "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.timeout", "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.nodes.total"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("did not expect %q in candidate ids, got %q", unwanted, text)
		}
	}
}

func TestCandidatesForDocumentsLimitExtractVarToRecommendedOrRepeatedValues(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Vars: map[string]any{"kubernetesVersion": "1.35.1"}, Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"kubernetesVersion": "1.35.1", "criSocket": "unix:///run/containerd/containerd.sock", "podNetworkCIDR": "10.244.0.0/16"}}}}}}}
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}}, VarsRecommendation: []string{"kubernetesVersion"}}
	items := CandidatesForDocuments(plan, []askcontract.GeneratedDocument{doc})
	joined := []string{}
	for _, item := range items {
		joined = append(joined, item.ID)
	}
	text := strings.Join(joined, "\n")
	if !strings.Contains(text, "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.kubernetesVersion") {
		t.Fatalf("expected kubernetesVersion extract-var candidate, got %q", text)
	}
	for _, unwanted := range []string{"extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.criSocket", "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.podNetworkCIDR"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("did not expect %q in candidate ids, got %q", unwanted, text)
		}
	}
}

func TestCandidatesForDocumentsUseRecommendationOnlyWhenNothingRepeats(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"criSocket": "unix:///run/containerd/containerd.sock"}}}}}}}
	plan := askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{TargetPaths: []string{"workflows/scenarios/apply.yaml"}}, VarsRecommendation: []string{"criSocket"}}
	items := CandidatesForDocuments(plan, []askcontract.GeneratedDocument{doc})
	joined := []string{}
	for _, item := range items {
		joined = append(joined, item.ID)
	}
	text := strings.Join(joined, "\n")
	if !strings.Contains(text, "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.criSocket") {
		t.Fatalf("expected criSocket extract-var candidate when nothing repeats, got %q", text)
	}
}

func TestResolveCandidateMarksUnsupportedExtractVarAsIgnorable(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "verify", Steps: []askcontract.WorkflowStep{{ID: "check", Kind: "CheckCluster", Spec: map[string]any{"interval": "5s"}}}}}}}
	_, err := ResolveCandidate(doc, askcontract.RefineTransformAction{Candidate: "extract-var|workflows/scenarios/apply.yaml|phases[0].steps[0].spec.interval"})
	var unknown UnknownCandidateError
	if err == nil || !strings.Contains(err.Error(), "unknown refine transform candidate") || !strings.Contains(err.Error(), "workflows/scenarios/apply.yaml") {
		t.Fatalf("expected unknown candidate error, got %v", err)
	}
	if !errors.As(err, &unknown) || !unknown.Ignorable {
		t.Fatalf("expected ignorable unknown extract-var candidate, got %#v", err)
	}
}

func TestResolveCandidateRejectsUnsupportedRawExtractVar(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "bootstrap", Steps: []askcontract.WorkflowStep{{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"criSocket": "unix:///run/containerd/containerd.sock"}}}}}}}
	_, err := ResolveCandidate(doc, askcontract.RefineTransformAction{Type: "extract-var", RawPath: "phases[0].steps[0].spec.criSocket", VarName: "criSocket"})
	var unknown UnknownCandidateError
	if !errors.As(err, &unknown) || !unknown.Ignorable {
		t.Fatalf("expected unsupported raw extract-var to be ignorable, got %#v", err)
	}
}
