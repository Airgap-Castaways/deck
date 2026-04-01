package askrepair

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
)

func TestTryAutoRepairFillsMissingInitJoinFile(t *testing.T) {
	files := []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"}}
	diags := []askdiagnostic.Diagnostic{{RepairOp: "fill-field", File: "workflows/scenarios/apply.yaml", StepID: "init", StepKind: "InitKubeadm", Path: "spec.outputJoinFile", Message: "InitKubeadm requires spec.outputJoinFile"}}
	repaired, notes, applied, err := TryAutoRepair(t.TempDir(), files, diags, []string{"workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("try auto repair: %v", err)
	}
	if !applied || len(repaired) != 1 || !strings.Contains(repaired[0].Content, "outputJoinFile: /tmp/deck/join.txt") {
		t.Fatalf("expected outputJoinFile auto repair, got %#v notes=%#v applied=%t", repaired, notes, applied)
	}
}

func TestTryAutoRepairRenamesDuplicateStepIDs(t *testing.T) {
	files := []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nphases:\n  - name: control-plane\n    steps:\n      - id: preflight\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n  - name: worker\n    steps:\n      - id: preflight\n        kind: CheckHost\n        spec:\n          checks: [os, arch, swap]\n"}}
	diags := []askdiagnostic.Diagnostic{{RepairOp: "rename-step", File: "workflows/scenarios/apply.yaml", Message: "workflow reuses step id preflight"}}
	repaired, _, applied, err := TryAutoRepair(t.TempDir(), files, diags, []string{"workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("try auto repair duplicate ids: %v", err)
	}
	if !applied || len(repaired) != 1 || !strings.Contains(repaired[0].Content, "worker-preflight") {
		t.Fatalf("expected duplicate step ids to be renamed, got %#v", repaired)
	}
}

func TestTryAutoRepairMigratesInstallPackageSourcePath(t *testing.T) {
	files := []askcontract.GeneratedFile{{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: install\n    kind: InstallPackage\n    spec:\n      packages: [kubeadm]\n      sourcePath: /tmp/packages\n"}}
	diags := []askdiagnostic.Diagnostic{{RepairOp: "remove-field", File: "workflows/scenarios/apply.yaml", StepID: "install", StepKind: "InstallPackage", Path: "spec.sourcePath", Message: "InstallPackage: Additional property sourcePath is not allowed"}}
	repaired, _, applied, err := TryAutoRepair(t.TempDir(), files, diags, []string{"workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("try auto repair sourcePath migration: %v", err)
	}
	if !applied || len(repaired) != 1 || !strings.Contains(repaired[0].Content, "source:") || !strings.Contains(repaired[0].Content, "path: /tmp/packages") {
		t.Fatalf("expected sourcePath migration repair, got %#v", repaired)
	}
}

func TestTryAutoRepairRespectsRepairPaths(t *testing.T) {
	files := []askcontract.GeneratedFile{
		{Path: "workflows/scenarios/apply.yaml", Content: "version: v1alpha1\nsteps:\n  - id: init\n    kind: InitKubeadm\n    spec:\n      podNetworkCIDR: 10.244.0.0/16\n"},
		{Path: "workflows/prepare.yaml", Content: "version: v1alpha1\nsteps:\n  - id: download\n    kind: DownloadImage\n    spec:\n      images: [registry.k8s.io/pause:3.9]\n"},
	}
	diags := []askdiagnostic.Diagnostic{
		{RepairOp: "fill-field", File: "workflows/scenarios/apply.yaml", StepID: "init", StepKind: "InitKubeadm", Path: "spec.outputJoinFile", Message: "InitKubeadm requires spec.outputJoinFile"},
		{RepairOp: "fix-literal", File: "workflows/prepare.yaml", StepID: "download", StepKind: "DownloadImage", Path: "spec.backend.engine", Allowed: []string{"go-containerregistry"}, Message: "invalid backend"},
	}
	repaired, _, applied, err := TryAutoRepair(t.TempDir(), files, diags, []string{"workflows/scenarios/apply.yaml"})
	if err != nil {
		t.Fatalf("try auto repair scoped paths: %v", err)
	}
	if !applied {
		t.Fatalf("expected scoped repair to apply")
	}
	byPath := map[string]string{}
	for _, file := range repaired {
		byPath[file.Path] = file.Content
	}
	if !strings.Contains(byPath["workflows/scenarios/apply.yaml"], "outputJoinFile: /tmp/deck/join.txt") {
		t.Fatalf("expected in-scope repair to apply, got %q", byPath["workflows/scenarios/apply.yaml"])
	}
	if strings.Contains(byPath["workflows/prepare.yaml"], "backend:") {
		t.Fatalf("expected out-of-scope repair to be skipped, got %q", byPath["workflows/prepare.yaml"])
	}
}
