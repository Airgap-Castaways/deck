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
