package askauthoring

import "testing"

func TestInferFactsRecognizesKoreanClusterPromptAmbiguities(t *testing.T) {
	facts := InferFacts("3 노드 쿠버네티스 클러스터링 워크플로우를 구성해줘", nil, "offline")
	if facts.NodeCount != 3 {
		t.Fatalf("expected korean node count detection, got %#v", facts)
	}
	for _, want := range []string{"role-model", "cluster-implementation"} {
		if !contains(facts.Ambiguities, want) {
			t.Fatalf("expected ambiguity %q, got %#v", want, facts.Ambiguities)
		}
	}
	if len(facts.Clarifications) == 0 {
		t.Fatalf("expected clarifications for korean cluster prompt, got %#v", facts)
	}
}

func TestInferFactsDoesNotTreatJoinFileRefactorAsClusterAuthoring(t *testing.T) {
	facts := InferFacts("refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml for repeated join file values", nil, "unspecified")
	if contains(facts.Ambiguities, "cluster-implementation") || contains(facts.Ambiguities, "cluster-topology") || contains(facts.Ambiguities, "role-model") {
		t.Fatalf("expected join-file refine prompt to avoid cluster authoring ambiguities, got %#v", facts)
	}
}

func TestInferFactsTreatsCheckClusterPromptAsVerificationOnly(t *testing.T) {
	facts := InferFacts("Create a single-node apply workflow that verifies the cluster with CheckCluster expecting total 1 node and controlPlaneReady 1.", nil, "unspecified")
	if !contains(facts.Capabilities, "cluster-verification") {
		t.Fatalf("expected verification capability, got %#v", facts)
	}
	if contains(facts.Ambiguities, "cluster-implementation") {
		t.Fatalf("expected verification-only prompt not to require cluster implementation clarification, got %#v", facts)
	}
}
