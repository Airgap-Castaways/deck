package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/taedi90/deck/internal/config"
)

func TestRun_InstallTools(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	sysctlPath := filepath.Join(dir, "sysctl.conf")
	modprobePath := filepath.Join(dir, "modules.conf")
	joinPath := filepath.Join(dir, "join.txt")

	wf := &config.Workflow{
		Version: "v1",
		Context: config.Context{StateFile: statePath},
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "install-packages", Kind: "InstallPackages", Spec: map[string]any{"packages": []any{"containerd"}}},
				{ID: "write-file", Kind: "WriteFile", Spec: map[string]any{"path": fileA, "content": "hello world"}},
				{ID: "edit-file", Kind: "EditFile", Spec: map[string]any{"path": fileA, "edits": []any{map[string]any{"op": "replace", "match": "world", "with": "deck"}}}},
				{ID: "copy-file", Kind: "CopyFile", Spec: map[string]any{"src": fileA, "dest": fileB}},
				{ID: "sysctl", Kind: "Sysctl", Spec: map[string]any{"writeFile": sysctlPath, "values": map[string]any{"net.ipv4.ip_forward": "1"}}},
				{ID: "modprobe", Kind: "Modprobe", Spec: map[string]any{"modules": []any{"overlay", "br_netfilter"}, "persistFile": modprobePath}},
				{ID: "run-cmd", Kind: "RunCommand", Spec: map[string]any{"command": []any{"true"}}},
				{ID: "kubeadm-init", Kind: "KubeadmInit", Spec: map[string]any{"outputJoinFile": joinPath}},
				{ID: "kubeadm-join", Kind: "KubeadmJoin", Spec: map[string]any{"joinFile": joinPath}},
			},
		}},
	}

	if err := Run(wf, RunOptions{}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	contentA, err := os.ReadFile(fileA)
	if err != nil {
		t.Fatalf("read fileA: %v", err)
	}
	if string(contentA) != "hello deck" {
		t.Fatalf("unexpected edited content: %q", string(contentA))
	}

	if _, err := os.Stat(fileB); err != nil {
		t.Fatalf("copied file missing: %v", err)
	}
	if _, err := os.Stat(sysctlPath); err != nil {
		t.Fatalf("sysctl file missing: %v", err)
	}
	if _, err := os.Stat(modprobePath); err != nil {
		t.Fatalf("modprobe persist file missing: %v", err)
	}
	if _, err := os.Stat(joinPath); err != nil {
		t.Fatalf("join file missing: %v", err)
	}

	rawState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var st State
	if err := json.Unmarshal(rawState, &st); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if len(st.CompletedSteps) != 9 {
		t.Fatalf("expected 9 completed steps, got %d", len(st.CompletedSteps))
	}
}

func TestRun_ResumeFromFailedStep(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")

	wf := &config.Workflow{
		Version: "v1",
		Context: config.Context{StateFile: statePath},
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "s1", Kind: "WriteFile", Spec: map[string]any{"path": first, "content": "ok"}},
				{ID: "s2", Kind: "RunCommand", Spec: map[string]any{"command": []any{"false"}}},
				{ID: "s3", Kind: "WriteFile", Spec: map[string]any{"path": second, "content": "done"}},
			},
		}},
	}

	if err := Run(wf, RunOptions{}); err == nil {
		t.Fatalf("expected failure on s2")
	}

	rawState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var st State
	if err := json.Unmarshal(rawState, &st); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if len(st.CompletedSteps) != 1 || st.CompletedSteps[0] != "s1" {
		t.Fatalf("unexpected completed steps after failure: %#v", st.CompletedSteps)
	}
	if st.FailedStep != "s2" {
		t.Fatalf("expected failed step s2, got %q", st.FailedStep)
	}

	wf.Phases[0].Steps[1].Spec = map[string]any{"command": []any{"true"}}
	if err := Run(wf, RunOptions{}); err != nil {
		t.Fatalf("resume run failed: %v", err)
	}

	if _, err := os.Stat(second); err != nil {
		t.Fatalf("expected second file after resume: %v", err)
	}

	rawState, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read final state: %v", err)
	}
	var final State
	if err := json.Unmarshal(rawState, &final); err != nil {
		t.Fatalf("parse final state: %v", err)
	}
	if len(final.CompletedSteps) != 3 {
		t.Fatalf("expected all steps completed, got %d", len(final.CompletedSteps))
	}
	if final.FailedStep != "" {
		t.Fatalf("expected empty failed step, got %q", final.FailedStep)
	}
}
