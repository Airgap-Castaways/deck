package askcontext

import (
	"strings"
	"testing"
)

func TestDiscoverCandidateStepsWithOptionsKeepsBootstrapKindsVisible(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("create an air-gapped rhel9 single-node kubeadm workflow", StepGuidanceOptions{ModeIntent: "apply-only", Topology: "single-node", RequiredCapabilities: []string{"kubeadm-bootstrap", "cluster-verification"}})
	seen := map[string]bool{}
	for _, item := range selected {
		seen[item.Step.Kind] = true
	}
	for _, want := range []string{"InitKubeadm", "CheckHost", "CheckKubernetesCluster"} {
		if !seen[want] {
			t.Fatalf("expected candidate %s, got %#v", want, selected)
		}
	}
}

func TestDiscoverCandidateStepsUsesPublicGroupTitles(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("use the host prep group for node prerequisites", StepGuidanceOptions{ModeIntent: "apply-only"})
	for _, item := range selected {
		if item.Step.Kind != "CheckHost" {
			continue
		}
		if !strings.Contains(item.WhyRelevant, "Host Prep group") {
			t.Fatalf("expected group-based relevance reason, got %#v", item)
		}
		return
	}
	t.Fatalf("expected CheckHost candidate for host prep prompt, got %#v", selected)
}
