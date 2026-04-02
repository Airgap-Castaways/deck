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

func TestCompileBuildsCompleteTwoNodeOfflineDraft(t *testing.T) {
	program := askcontract.AuthoringProgram{
		Platform: askcontract.ProgramPlatform{Family: "rhel", Release: "9", RepoType: "rpm", BackendImage: "rockylinux:9"},
		Artifacts: askcontract.ProgramArtifacts{
			Packages:         []string{"kubeadm", "kubelet", "kubectl", "cri-tools", "containerd"},
			Images:           []string{"registry.k8s.io/kube-apiserver:v1.30.0", "registry.k8s.io/kube-controller-manager:v1.30.0"},
			PackageOutputDir: "packages/rpm/9",
			ImageOutputDir:   "images/control-plane",
		},
		Cluster: askcontract.ProgramCluster{
			JoinFile:          "/tmp/deck/join.txt",
			PodCIDR:           "10.244.0.0/16",
			KubernetesVersion: "v1.30.0",
			CriSocket:         "unix:///run/containerd/containerd.sock",
			RoleSelector:      "role",
			ControlPlaneCount: 1,
			WorkerCount:       1,
		},
		Verification: askcontract.ProgramVerification{
			ExpectedNodeCount:         2,
			ExpectedReadyCount:        2,
			ExpectedControlPlaneReady: 1,
			FinalVerificationRole:     "control-plane",
			Interval:                  "5s",
			Timeout:                   "10m",
		},
	}
	selection := askcontract.DraftSelection{
		Targets: []askcontract.DraftTargetSelection{
			{
				Path: "workflows/prepare.yaml",
				Builders: []askcontract.DraftBuilderSelection{
					{ID: "prepare.download-package"},
					{ID: "prepare.download-image"},
				},
			},
			{
				Path: "workflows/scenarios/apply.yaml",
				Builders: []askcontract.DraftBuilderSelection{
					{ID: "apply.install-package"},
					{ID: "apply.load-image"},
					{ID: "apply.init-kubeadm"},
					{ID: "apply.join-kubeadm"},
					{ID: "apply.check-cluster"},
				},
			},
		},
		Vars: map[string]any{
			"role":              "control-plane",
			"joinFile":          "/tmp/deck/join.txt",
			"podCIDR":           "10.244.0.0/16",
			"kubernetesVersion": "v1.30.0",
		},
	}
	docs, err := CompileWithProgram(program, selection)
	if err != nil {
		t.Fatalf("compile complete two-node offline draft: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected prepare, apply, and vars docs, got %#v", docs)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	for _, path := range []string{"workflows/prepare.yaml", "workflows/scenarios/apply.yaml", "workflows/vars.yaml"} {
		if _, ok := byPath[path]; !ok {
			t.Fatalf("expected %s in compiled documents, got %#v", path, byPath)
		}
	}
	prepare := byPath["workflows/prepare.yaml"].Workflow
	if prepare == nil || len(prepare.Phases) != 2 {
		t.Fatalf("expected prepare workflow phases, got %#v", prepare)
	}
	prepareSteps := map[string]askcontract.WorkflowStep{}
	for _, phase := range prepare.Phases {
		for _, step := range phase.Steps {
			prepareSteps[step.Kind] = step
		}
	}
	downloadPackages := prepareSteps["DownloadPackage"]
	if got := strings.Join(stringSlice(downloadPackages.Spec["packages"]), ","); got != "kubeadm,kubelet,kubectl,cri-tools,containerd" {
		t.Fatalf("expected complete package payload, got %#v", downloadPackages.Spec)
	}
	if outputDir, _ := downloadPackages.Spec["outputDir"].(string); outputDir != "packages/rpm/9" {
		t.Fatalf("expected package output dir, got %#v", downloadPackages.Spec)
	}
	downloadImages := prepareSteps["DownloadImage"]
	if got := strings.Join(stringSlice(downloadImages.Spec["images"]), ","); got != "registry.k8s.io/kube-apiserver:v1.30.0,registry.k8s.io/kube-controller-manager:v1.30.0" {
		t.Fatalf("expected complete image payload, got %#v", downloadImages.Spec)
	}
	if outputDir, _ := downloadImages.Spec["outputDir"].(string); outputDir != "images/control-plane" {
		t.Fatalf("expected image output dir, got %#v", downloadImages.Spec)
	}
	apply := byPath["workflows/scenarios/apply.yaml"].Workflow
	if apply == nil || len(apply.Phases) != 5 {
		t.Fatalf("expected apply workflow phases, got %#v", apply)
	}
	applySteps := map[string]askcontract.WorkflowStep{}
	for _, phase := range apply.Phases {
		for _, step := range phase.Steps {
			applySteps[step.Kind] = step
		}
	}
	install := applySteps["InstallPackage"]
	if got := strings.Join(stringSlice(install.Spec["packages"]), ","); got != "kubeadm,kubelet,kubectl,cri-tools,containerd" {
		t.Fatalf("expected install-package payload, got %#v", install.Spec)
	}
	if source := nestedMap(install.Spec, "source"); source["type"] != "local-repo" || source["path"] != "packages/rpm/9" {
		t.Fatalf("expected local package source, got %#v", install.Spec)
	}
	load := applySteps["LoadImage"]
	if got := strings.Join(stringSlice(load.Spec["images"]), ","); got != "registry.k8s.io/kube-apiserver:v1.30.0,registry.k8s.io/kube-controller-manager:v1.30.0" {
		t.Fatalf("expected load-image payload, got %#v", load.Spec)
	}
	if load.Spec["sourceDir"] != "images/control-plane" || load.Spec["runtime"] != "ctr" {
		t.Fatalf("expected image source dir and runtime, got %#v", load.Spec)
	}
	init := applySteps["InitKubeadm"]
	if init.When != `vars.role == "control-plane"` {
		t.Fatalf("expected control-plane init gate, got %#v", init)
	}
	for key, want := range map[string]any{"outputJoinFile": "/tmp/deck/join.txt", "podNetworkCIDR": "10.244.0.0/16", "kubernetesVersion": "v1.30.0", "criSocket": "unix:///run/containerd/containerd.sock"} {
		if init.Spec[key] != want {
			t.Fatalf("expected init %s=%v, got %#v", key, want, init.Spec)
		}
	}
	join := applySteps["JoinKubeadm"]
	if join.When != `vars.role == "worker"` || join.Spec["joinFile"] != "/tmp/deck/join.txt" {
		t.Fatalf("expected worker join gate and join file, got %#v", join)
	}
	check := applySteps["CheckCluster"]
	if check.When != `vars.role == "control-plane"` {
		t.Fatalf("expected control-plane verification gate, got %#v", check)
	}
	nodes, _ := check.Spec["nodes"].(map[string]any)
	if check.Spec["interval"] != "5s" || check.Spec["timeout"] != "10m" || nodes["total"] != 2 || nodes["ready"] != 2 || nodes["controlPlaneReady"] != 1 {
		t.Fatalf("expected check-cluster verification payload, got %#v", check.Spec)
	}
	vars := byPath["workflows/vars.yaml"].Vars
	if vars == nil || vars["role"] != "control-plane" || vars["joinFile"] != "/tmp/deck/join.txt" {
		t.Fatalf("expected vars companion document, got %#v", vars)
	}
}

func TestCompileAddsRoleVarWhenWorkflowUsesRoleSelector(t *testing.T) {
	program := askcontract.AuthoringProgram{
		Cluster:      askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", RoleSelector: "role", ControlPlaneCount: 1, WorkerCount: 1},
		Verification: askcontract.ProgramVerification{ExpectedNodeCount: 2, ExpectedReadyCount: 2, ExpectedControlPlaneReady: 1},
	}
	selection := askcontract.DraftSelection{Targets: []askcontract.DraftTargetSelection{
		{
			Path:     "workflows/scenarios/apply.yaml",
			Builders: []askcontract.DraftBuilderSelection{{ID: "apply.init-kubeadm"}, {ID: "apply.join-kubeadm"}, {ID: "apply.check-cluster"}},
		},
		{
			Path: "workflows/vars.yaml",
			Kind: "vars",
			Vars: map[string]any{"joinFile": "/tmp/deck/join.txt"},
		},
	}}
	docs, err := CompileWithProgram(program, selection)
	if err != nil {
		t.Fatalf("compile role-gated draft: %v", err)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	vars := byPath["workflows/vars.yaml"].Vars
	if vars == nil {
		t.Fatalf("expected vars document, got %#v", docs)
	}
	if vars["role"] != "control-plane" {
		t.Fatalf("expected code-owned role default in vars document, got %#v", vars)
	}
	if vars["joinFile"] != "/tmp/deck/join.txt" {
		t.Fatalf("expected existing vars preserved, got %#v", vars)
	}
}

func TestCompileCreatesVarsDocumentWhenRoleGatedWorkflowHasNoVarsTarget(t *testing.T) {
	program := askcontract.AuthoringProgram{
		Cluster:      askcontract.ProgramCluster{JoinFile: "/tmp/deck/join.txt", RoleSelector: "role", ControlPlaneCount: 1, WorkerCount: 1},
		Verification: askcontract.ProgramVerification{ExpectedNodeCount: 2, ExpectedReadyCount: 2, ExpectedControlPlaneReady: 1},
	}
	selection := askcontract.DraftSelection{Targets: []askcontract.DraftTargetSelection{{Path: "workflows/scenarios/apply.yaml", Builders: []askcontract.DraftBuilderSelection{{ID: "apply.init-kubeadm"}, {ID: "apply.join-kubeadm"}, {ID: "apply.check-cluster"}}}}}
	docs, err := CompileWithProgram(program, selection)
	if err != nil {
		t.Fatalf("compile role-gated workflow without vars target: %v", err)
	}
	byPath := map[string]askcontract.GeneratedDocument{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	vars := byPath["workflows/vars.yaml"].Vars
	if vars == nil || vars["role"] != "control-plane" {
		t.Fatalf("expected automatic vars companion for role-gated workflow, got %#v", docs)
	}
}

func TestDocumentKindPrefersCanonicalVarsAndComponentPaths(t *testing.T) {
	if got := documentKind("workflows/vars.yaml", "scenario"); got != "vars" {
		t.Fatalf("expected vars path to win over planner kind drift, got %q", got)
	}
	if got := documentKind("workflows/components/bootstrap.yaml", "scenario"); got != "component" {
		t.Fatalf("expected component path to win over planner kind drift, got %q", got)
	}
}

func TestNormalizeWhenRoleOverrideRejectsTemplateExpressions(t *testing.T) {
	program := askcontract.AuthoringProgram{Cluster: askcontract.ProgramCluster{RoleSelector: "role", ControlPlaneCount: 1, WorkerCount: 1}}
	if value, ok := normalizeWhenRoleOverride("{{ .vars.role }}", program); ok || value != nil {
		t.Fatalf("expected template whenRole override to be ignored, got value=%#v ok=%t", value, ok)
	}
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(item.(string)))
		}
		return out
	default:
		return nil
	}
}

func nestedMap(value map[string]any, key string) map[string]any {
	nested, _ := value[key].(map[string]any)
	return nested
}
