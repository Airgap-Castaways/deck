package askpattern

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func TestComposeSelectsRoleAwarePattern(t *testing.T) {
	patterns := Compose(
		askpolicy.ScenarioRequirements{NeedsPrepare: true, ArtifactKinds: []string{"package"}, ScenarioIntent: []string{"kubeadm", "multi-node", "join"}},
		askretrieve.WorkspaceSummary{},
		askcontract.PlanResponse{AuthoringBrief: askcontract.AuthoringBrief{Topology: "multi-node", NodeCount: 3}, ExecutionModel: askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}}},
	)
	seen := map[string]bool{}
	for _, pattern := range patterns {
		seen[pattern.Name] = true
	}
	for _, want := range []string{"role-aware-apply", "artifact-staging", "cluster-bootstrap"} {
		if !seen[want] {
			t.Fatalf("expected pattern %q, got %#v", want, patterns)
		}
	}
}
