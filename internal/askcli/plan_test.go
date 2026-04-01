package askcli

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestNeedsComplexPlannerKeepsGenericKubeadmClusterPrompt(t *testing.T) {
	decision := askintent.Decision{Route: askintent.RouteDraft}
	workspace := askretrieve.WorkspaceSummary{}
	if !needsComplexPlanner("create an air-gapped rhel9 kubeadm cluster workflow", workspace, decision) {
		t.Fatalf("expected generic kubeadm cluster prompt to require planner")
	}
}

func TestNeedsComplexPlannerSkipsExplicitSingleNodeKubeadmPrompt(t *testing.T) {
	decision := askintent.Decision{Route: askintent.RouteDraft}
	workspace := askretrieve.WorkspaceSummary{}
	if needsComplexPlanner("create an air-gapped rhel9 single-node kubeadm workflow", workspace, decision) {
		t.Fatalf("expected explicit single-node kubeadm prompt to avoid planner")
	}
}

func TestNeedsComplexPlannerKeepsExplicitMultiNodeTopologyPrompt(t *testing.T) {
	decision := askintent.Decision{Route: askintent.RouteDraft}
	workspace := askretrieve.WorkspaceSummary{}
	if !needsComplexPlanner("create an air-gapped rhel9 3-node kubeadm cluster workflow", workspace, decision) {
		t.Fatalf("expected explicit multi-node kubeadm prompt to require planner")
	}
}

func TestNeedsComplexPlannerKeepsGenericAirGappedClusterPrompt(t *testing.T) {
	decision := askintent.Decision{Route: askintent.RouteDraft}
	workspace := askretrieve.WorkspaceSummary{}
	if !needsComplexPlanner("create air-gapped cluster workflow", workspace, decision) {
		t.Fatalf("expected generic air-gapped cluster prompt to require planner")
	}
}
