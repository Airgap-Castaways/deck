package askknowledge

import (
	"strings"
	"testing"
)

func TestCurrentIncludesComponentAndConstraintKnowledge(t *testing.T) {
	bundle := Current()
	if bundle.Components.FragmentExample == "" || !strings.Contains(bundle.Components.FragmentRule, "fragment") {
		t.Fatalf("expected component fragment guidance, got %#v", bundle.Components)
	}
	found := false
	for _, item := range bundle.Constraints {
		if item.StepKind == "DownloadPackage" && item.Path == "spec.backend.runtime" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected constrained field catalog to include DownloadPackage runtime")
	}
}
