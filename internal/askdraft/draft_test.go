package askdraft

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestCompileBuildsWorkflowDocumentsFromBuilderSelections(t *testing.T) {
	program := askcontract.AuthoringProgram{
		Platform:     askcontract.ProgramPlatform{Family: "rhel", Release: "9", RepoType: "rpm", BackendImage: "rockylinux:9"},
		Artifacts:    askcontract.ProgramArtifacts{Packages: []string{"kubeadm", "kubelet", "kubectl"}, PackageOutputDir: "packages/rpm/9"},
		Cluster:      askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", ControlPlaneCount: 1},
		Verification: askcontract.ProgramVerification{ExpectedNodeCount: 1, ExpectedReadyCount: 1, ExpectedControlPlaneReady: 1},
	}
	selection := askcontract.DraftSelection{
		Targets: []askcontract.DraftTargetSelection{
			{
				Path: "workflows/prepare.yaml",
				Builders: []askcontract.DraftBuilderSelection{{
					ID:        "prepare.download-package",
					Overrides: map[string]any{},
				}},
			},
			{
				Path:     "workflows/scenarios/apply.yaml",
				Builders: []askcontract.DraftBuilderSelection{{ID: "apply.init-kubeadm", Overrides: map[string]any{}}, {ID: "apply.check-cluster", Overrides: map[string]any{}}},
			},
		},
		Vars: map[string]any{"role": "control-plane"},
	}
	docs, err := CompileWithProgram(program, selection)
	if err != nil {
		t.Fatalf("compile builder selection: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected prepare, apply, and vars docs, got %#v", docs)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	if byPath["workflows/prepare.yaml"].Workflow == nil || byPath["workflows/scenarios/apply.yaml"].Workflow == nil || byPath["workflows/vars.yaml"].Vars == nil {
		t.Fatalf("expected compiled workflow and vars documents, got %#v", byPath)
	}
	prepare := byPath["workflows/prepare.yaml"].Workflow
	if len(prepare.Phases) != 1 || prepare.Phases[0].Steps[0].Kind != "DownloadPackage" {
		t.Fatalf("expected typed prepare builder output, got %#v", prepare)
	}
	apply := byPath["workflows/scenarios/apply.yaml"].Workflow
	joinedKinds := []string{}
	for _, phase := range apply.Phases {
		for _, step := range phase.Steps {
			joinedKinds = append(joinedKinds, step.Kind)
		}
	}
	if !strings.Contains(strings.Join(joinedKinds, ","), "InitKubeadm") || !strings.Contains(strings.Join(joinedKinds, ","), "CheckCluster") {
		t.Fatalf("expected kubeadm builder steps, got %#v", apply)
	}
}

func TestCompileIgnoresDeprecatedLowLevelOverrides(t *testing.T) {
	program := askcontract.AuthoringProgram{
		Platform:     askcontract.ProgramPlatform{Family: "rhel", Release: "9", RepoType: "rpm", BackendImage: "rockylinux:9"},
		Artifacts:    askcontract.ProgramArtifacts{Packages: []string{"kubeadm"}, Images: []string{"registry.k8s.io/pause:3.10"}, PackageOutputDir: "packages/rpm/9", ImageOutputDir: "images/control-plane"},
		Cluster:      askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", RoleSelector: "role", ControlPlaneCount: 1, WorkerCount: 1},
		Verification: askcontract.ProgramVerification{ExpectedNodeCount: 2, ExpectedReadyCount: 2, ExpectedControlPlaneReady: 1, FinalVerificationRole: "control-plane"},
	}
	selection := askcontract.DraftSelection{Targets: []askcontract.DraftTargetSelection{
		{
			Path: "workflows/prepare.yaml",
			Builders: []askcontract.DraftBuilderSelection{{
				ID:        "prepare.download-package",
				Overrides: map[string]any{"outputDir": "/tmp/bad", "repoType": "yum", "distroFamily": "rhel", "distroRelease": "9"},
			}},
		},
		{
			Path: "workflows/scenarios/apply.yaml",
			Builders: []askcontract.DraftBuilderSelection{
				{ID: "apply.install-package", Overrides: map[string]any{"sourcePath": "/tmp/bad"}},
				{ID: "apply.load-image", Overrides: map[string]any{"runtime": "containerd"}},
				{ID: "apply.init-kubeadm", Overrides: map[string]any{"whenRole": "control-plane"}},
			},
		},
	}}
	docs, err := CompileWithProgram(program, selection)
	if err != nil {
		t.Fatalf("compile with deprecated overrides: %v", err)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	prepare := byPath["workflows/prepare.yaml"].Workflow
	if got := prepare.Phases[0].Steps[0].Spec["outputDir"]; got != "packages/rpm/9" {
		t.Fatalf("expected canonical package output dir, got %#v", got)
	}
	apply := byPath["workflows/scenarios/apply.yaml"].Workflow
	seenRuntime := false
	for _, phase := range apply.Phases {
		for _, step := range phase.Steps {
			if step.Kind == "LoadImage" {
				if step.Spec["runtime"] != "ctr" {
					t.Fatalf("expected canonical image runtime, got %#v", step.Spec)
				}
				seenRuntime = true
			}
		}
	}
	if !seenRuntime {
		t.Fatalf("expected LoadImage step in apply workflow")
	}
}

func TestCompileUsesKubeadmOverrideAliasesAndDropsStructuralNoise(t *testing.T) {
	selection := askcontract.DraftSelection{Targets: []askcontract.DraftTargetSelection{{
		Path: "workflows/scenarios/apply.yaml",
		Builders: []askcontract.DraftBuilderSelection{
			{ID: "apply.init-kubeadm", Overrides: map[string]any{"outputJoinFile": "/custom/join.txt", "kubernetesVersion": "v1.35.1", "criSocket": "unix:///run/containerd/containerd.sock", "phase": "ignored"}},
			{ID: "apply.check-cluster", Overrides: map[string]any{"nodeCount": 1, "readyCount": 1, "controlPlaneReady": 1, "phase": "ignored"}},
		},
	}}}
	docs, err := CompileWithProgram(askcontract.AuthoringProgram{}, selection)
	if err != nil {
		t.Fatalf("compile with kubeadm overrides: %v", err)
	}
	if len(docs) != 1 || docs[0].Workflow == nil {
		t.Fatalf("expected single workflow document, got %#v", docs)
	}
	apply := docs[0].Workflow
	joined := map[string]askcontract.WorkflowStep{}
	for _, phase := range apply.Phases {
		for _, step := range phase.Steps {
			joined[step.Kind] = step
		}
	}
	init := joined["InitKubeadm"]
	if init.Spec["outputJoinFile"] != "/custom/join.txt" || init.Spec["kubernetesVersion"] != "v1.35.1" || init.Spec["criSocket"] != "unix:///run/containerd/containerd.sock" {
		t.Fatalf("expected init-kubeadm overrides to materialize, got %#v", init.Spec)
	}
	check := joined["CheckCluster"]
	nodes, _ := check.Spec["nodes"].(map[string]any)
	if nodes["total"] != 1 || nodes["ready"] != 1 || nodes["controlPlaneReady"] != 1 {
		t.Fatalf("expected check-cluster overrides to materialize, got %#v", check.Spec)
	}
}
