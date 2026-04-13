package askpolicy

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestBuildContractGraphForMultiNodeArtifacts(t *testing.T) {
	facts := InferFacts("create an air-gapped three-node kubeadm cluster with workers", []string{"package", "image"}, "offline")
	graph := BuildContractGraph(facts, RequirementLike{
		Connectivity:   "offline",
		NeedsPrepare:   true,
		ArtifactKinds:  []string{"package", "image"},
		ScenarioIntent: []string{"kubeadm", "multi-node", "join", "node-count:3"},
		EntryScenario:  "workflows/scenarios/apply.yaml",
	}, askretrieve.WorkspaceSummary{})
	if len(graph.Artifacts) != 2 {
		t.Fatalf("expected artifact contracts, got %#v", graph.Artifacts)
	}
	if len(graph.SharedState) != 1 || graph.RoleExecution.RoleSelector != "vars.role" {
		t.Fatalf("expected role-aware graph, got %#v", graph)
	}
	if graph.Verification.ExpectedControlPlaneReady != 1 {
		t.Fatalf("expected control plane verification, got %#v", graph.Verification)
	}
}
