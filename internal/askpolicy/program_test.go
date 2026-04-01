package askpolicy

import (
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestNormalizeAuthoringProgramBuildsDefaultsFromBriefAndExecution(t *testing.T) {
	program := normalizeAuthoringProgram(askcontract.AuthoringProgram{}, askcontract.AuthoringBrief{PlatformFamily: "rhel", Topology: "multi-node", NodeCount: 2, RequiredCapabilities: []string{"package-staging", "kubeadm-bootstrap", "cluster-verification"}}, askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 2, ExpectedControlPlaneReady: 1, FinalVerificationRole: "control-plane"}}, "Create an offline RHEL 9 kubeadm workflow")
	if program.Platform.Family != "rhel" || program.Platform.Release != "9" || program.Platform.RepoType != "rpm" {
		t.Fatalf("expected normalized platform defaults, got %#v", program.Platform)
	}
	if program.Cluster.JoinFile != "/tmp/deck/join.txt" || program.Cluster.RoleSelector != "role" {
		t.Fatalf("expected normalized cluster defaults, got %#v", program.Cluster)
	}
	if program.Verification.ExpectedReadyCount != 2 || program.Verification.Timeout != "10m" {
		t.Fatalf("expected normalized verification defaults, got %#v", program.Verification)
	}
}
