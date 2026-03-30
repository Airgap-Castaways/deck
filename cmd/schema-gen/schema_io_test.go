package main

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestFamilyPageSummaryUsesNaturalTitle(t *testing.T) {
	def := workflowexec.StepDefinition{Family: "host-check", FamilyTitle: "HostCheck", Kind: "CheckHost", Summary: "Validate host suitability checks on the current node."}
	got := familyPageSummary(def, "Host Check")
	want := "Reference for the `Host Check` family of typed workflow steps."
	if got != want {
		t.Fatalf("unexpected family summary: got %q want %q", got, want)
	}
}
