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

func TestDiscoverCandidateStepsUsesHyphenatedGroupIDs(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("use the host-prep group for node prerequisites", StepGuidanceOptions{ModeIntent: "apply-only"})
	for _, item := range selected {
		if item.Step.Kind != "CheckHost" {
			continue
		}
		if !strings.Contains(item.WhyRelevant, "Host Prep group") {
			t.Fatalf("expected hyphenated group id to map to public group reason, got %#v", item)
		}
		return
	}
	t.Fatalf("expected CheckHost candidate for hyphenated group prompt, got %#v", selected)
}

func TestDiscoverCandidateStepsUsesLegacyGroupAliases(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("use artifact staging for offline packages", StepGuidanceOptions{ModeIntent: "prepare+apply"})
	for _, item := range selected {
		if item.Step.Kind != "DownloadPackage" {
			continue
		}
		if !strings.Contains(item.WhyRelevant, "Packages and Repositories group alias") {
			t.Fatalf("expected legacy alias relevance reason, got %#v", item)
		}
		return
	}
	t.Fatalf("expected DownloadPackage candidate for artifact staging alias prompt, got %#v", selected)
}

func TestDiscoverCandidateStepsUsesNewDomainGroupIDs(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("show container-images steps for offline image flow", StepGuidanceOptions{ModeIntent: "prepare+apply"})
	seen := map[string]bool{}
	for _, item := range selected {
		seen[item.Step.Kind] = true
	}
	for _, want := range []string{"DownloadImage", "LoadImage", "VerifyImage"} {
		if !seen[want] {
			t.Fatalf("expected %s candidate for container-images group, got %#v", want, selected)
		}
	}
}

func TestDiscoverCandidateStepsBoostsAirGappedPackageStaging(t *testing.T) {
	selected := DiscoverCandidateStepsWithOptions("create an air-gapped offline package staging flow", StepGuidanceOptions{ModeIntent: "prepare+apply"})
	for _, item := range selected {
		if item.Step.Kind != "DownloadPackage" {
			continue
		}
		if !strings.Contains(item.WhyRelevant, "supports air-gapped artifact flow") {
			t.Fatalf("expected air-gapped artifact boost for DownloadPackage, got %#v", item)
		}
		return
	}
	t.Fatalf("expected DownloadPackage candidate for air-gapped package staging prompt, got %#v", selected)
}

func TestPackageArtifactPromptUsesWholeWords(t *testing.T) {
	for _, prompt := range []string{"air-gapped report", "postage review", "reportFile output"} {
		if isPackageArtifactPrompt(prompt) {
			t.Fatalf("expected %q not to match package artifact prompt", prompt)
		}
	}
	for _, prompt := range []string{"air-gapped repo setup", "package staging", "prepare artifact bundle"} {
		if !isPackageArtifactPrompt(prompt) {
			t.Fatalf("expected %q to match package artifact prompt", prompt)
		}
	}
}

func TestMatchingGroupAliasUsesWholeWords(t *testing.T) {
	aliases := []string{"artifact staging", "waits"}
	if got := matchingGroupAlias("adapt the workflow for reports", aliases); got != "" {
		t.Fatalf("expected no substring alias match, got %q", got)
	}
	if got := matchingGroupAlias("use artifact-staging for offline packages", aliases); got != "artifact staging" {
		t.Fatalf("expected phrase alias match across punctuation, got %q", got)
	}
}
