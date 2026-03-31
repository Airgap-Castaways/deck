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
