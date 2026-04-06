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

func TestNormalizeAuthoringProgramUsesPreinstalledDefaultsForMinimalSingleNodeBootstrap(t *testing.T) {
	program := normalizeAuthoringProgram(askcontract.AuthoringProgram{Platform: askcontract.ProgramPlatform{Family: "rhel", Release: "kubernetes-1.35.1", RepoType: "pre-staged-offline"}, Cluster: askcontract.ProgramCluster{RoleSelector: "single-node"}, Verification: askcontract.ProgramVerification{ExpectedReadyCount: 1}}, askcontract.AuthoringBrief{Topology: "single-node", ModeIntent: "apply-only", NodeCount: 1, RequiredCapabilities: []string{"kubeadm-bootstrap", "cluster-verification"}}, askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 1, ExpectedControlPlaneReady: 1, FinalVerificationRole: "control-plane"}}, "Create a minimal single-node apply-only offline kubeadm workflow for Kubernetes 1.35.1 using only init-kubeadm and check-kubernetes-cluster builders")
	if program.Platform.Family != "custom" || program.Platform.RepoType != "none" || program.Platform.BackendImage != "none" {
		t.Fatalf("expected minimal preinstalled platform defaults, got %#v", program.Platform)
	}
	if program.Platform.Release != "unspecified" {
		t.Fatalf("expected unspecified release for minimal apply-only flow, got %#v", program.Platform)
	}
	if program.Artifacts.PackageOutputDir != "" || program.Artifacts.ImageOutputDir != "" {
		t.Fatalf("expected no artifact output dirs for minimal apply-only flow, got %#v", program.Artifacts)
	}
	if program.Verification.ExpectedReadyCount != 0 {
		t.Fatalf("expected ready count 0 for no-CNI minimal flow, got %#v", program.Verification)
	}
	if program.Cluster.RoleSelector != "" {
		t.Fatalf("expected no role selector for single-node flow, got %#v", program.Cluster)
	}
	if program.Cluster.KubernetesVersion != "v1.35.1" {
		t.Fatalf("expected inferred kubernetes version for minimal flow, got %#v", program.Cluster)
	}
	if program.Cluster.CriSocket != "unix:///run/containerd/containerd.sock" {
		t.Fatalf("expected default cri socket for kubeadm flow, got %#v", program.Cluster)
	}
}

func TestNormalizeAuthoringProgramSeedsPrepareArtifactsForKubeadmDraft(t *testing.T) {
	program := normalizeAuthoringProgram(
		askcontract.AuthoringProgram{},
		askcontract.AuthoringBrief{PlatformFamily: "custom", ModeIntent: "prepare+apply", NodeCount: 2, RequiredCapabilities: []string{"prepare-artifacts", "package-staging", "image-staging", "kubeadm-bootstrap", "kubeadm-join"}},
		askcontract.ExecutionModel{RoleExecution: askcontract.RoleExecutionModel{RoleSelector: "vars.role"}, Verification: askcontract.VerificationStrategy{ExpectedNodeCount: 2, ExpectedControlPlaneReady: 1, FinalVerificationRole: "control-plane"}},
		"Create an air-gapped kubeadm prepare and apply workflow with worker join",
	)
	if program.Platform.Family == "" || program.Platform.Family == "custom" {
		t.Fatalf("expected canonical prepare platform family, got %#v", program.Platform)
	}
	if program.Platform.Release == "" {
		t.Fatalf("expected canonical prepare platform release, got %#v", program.Platform)
	}
	if len(program.Artifacts.Packages) == 0 || len(program.Artifacts.Images) == 0 {
		t.Fatalf("expected seeded kubeadm prepare artifacts, got %#v", program.Artifacts)
	}
	if program.Artifacts.PackageOutputDir == "" || program.Artifacts.ImageOutputDir == "" {
		t.Fatalf("expected canonical artifact output dirs, got %#v", program.Artifacts)
	}
}
