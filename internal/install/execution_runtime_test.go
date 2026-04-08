package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRun_WhenAndRegisterSemantics(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	joinPath := filepath.Join(dir, "join.txt")
	registeredOutputPath := filepath.Join(dir, "registered.txt")
	skippedOutputPath := filepath.Join(dir, "skipped.txt")

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "init", Kind: "InitKubeadm", Spec: map[string]any{"outputJoinFile": joinPath}, Register: map[string]string{"workerJoinFile": "joinFile"}},
				{ID: "use-register", Kind: "WriteFile", When: "vars.role == \"control-plane\"", Spec: map[string]any{"path": registeredOutputPath, "content": "{{ .runtime.workerJoinFile }}"}},
				{ID: "skip-worker", Kind: "WriteFile", When: "vars.role == \"worker\"", Spec: map[string]any{"path": skippedOutputPath, "content": "worker"}},
			},
		}},
	}
	kubeadm := useStubInitJoinKubeadm()

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath, kubeadm: kubeadm}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	raw, err := os.ReadFile(registeredOutputPath)
	if err != nil {
		t.Fatalf("read registered output: %v", err)
	}
	if strings.TrimSpace(string(raw)) != joinPath {
		t.Fatalf("expected registered content to be %q, got %q", joinPath, strings.TrimSpace(string(raw)))
	}

	if _, err := os.Stat(skippedOutputPath); err == nil {
		t.Fatalf("expected skipped step output to not exist")
	}

	stateRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var st State
	if err := json.Unmarshal(stateRaw, &st); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if st.RuntimeVars["workerJoinFile"] != joinPath {
		t.Fatalf("expected runtime var workerJoinFile=%q, got %#v", joinPath, st.RuntimeVars["workerJoinFile"])
	}
	if len(st.CompletedPhases) != 1 || st.CompletedPhases[0] != "install" {
		t.Fatalf("unexpected completed phases: %#v", st.CompletedPhases)
	}
}

func TestRun_CheckHostAndRuntimeHost(t *testing.T) {
	t.Run("runtime host available without checkhost", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		outputPath := filepath.Join(dir, "runtime-host.txt")
		origDetect := detectHostFacts
		t.Cleanup(func() { detectHostFacts = origDetect })
		detectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "rhel"}, "arch": "amd64"}
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "runtime-branch",
					Kind: "WriteFile",
					When: "runtime.host.os.family == \"rhel\" && runtime.host.arch == \"amd64\"",
					Spec: map[string]any{"path": outputPath, "content": "ok"},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Fresh: true}); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if raw, err := os.ReadFile(outputPath); err != nil || strings.TrimSpace(string(raw)) != "ok" {
			t.Fatalf("expected runtime.host gated write, got err=%v content=%q", err, string(raw))
		}
	})

	t.Run("checkhost allowed in apply", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		outputPath := filepath.Join(dir, "checked.txt")
		origDetect := detectHostFacts
		t.Cleanup(func() { detectHostFacts = origDetect })
		detectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "rhel"}, "arch": "amd64"}
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{
					{ID: "check-host", Kind: "CheckHost", Register: map[string]string{"hostPassed": "passed"}, Spec: map[string]any{"checks": []any{"os", "arch"}}},
					{ID: "write", Kind: "WriteFile", When: "runtime.hostPassed == true && runtime.host.os.family == \"rhel\"", Spec: map[string]any{"path": outputPath, "content": "checked"}},
				},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Fresh: true}); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if raw, err := os.ReadFile(outputPath); err != nil || strings.TrimSpace(string(raw)) != "checked" {
			t.Fatalf("expected apply CheckHost gated write, got err=%v content=%q", err, string(raw))
		}
	})
}

func TestRun_ParallelGroupRunsConcurrentlyAndRegistersAfterBatch(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	starts := filepath.Join(dir, "starts.txt")
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	combined := filepath.Join(dir, "combined.txt")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:           "install",
			MaxParallelism: 2,
			Steps: []config.Step{
				{ID: "left", Kind: "WriteFile", ParallelGroup: "pair", Spec: map[string]any{"path": left, "content": "L"}, Register: map[string]string{"leftPath": "path"}},
				{ID: "right", Kind: "Command", ParallelGroup: "pair", Timeout: "2s", Spec: map[string]any{"command": []any{"sh", "-c", fmt.Sprintf("echo right >> '%s'; while [ $(wc -l < '%s' 2>/dev/null || printf 0) -lt 2 ]; do sleep 0.05; done; printf R > '%s'", shellEscapePath(starts), shellEscapePath(starts), shellEscapePath(right))}}},
				{ID: "left-sync", Kind: "Command", ParallelGroup: "pair", Timeout: "2s", Spec: map[string]any{"command": []any{"sh", "-c", fmt.Sprintf("echo left >> '%s'; while [ $(wc -l < '%s' 2>/dev/null || printf 0) -lt 2 ]; do sleep 0.05; done", shellEscapePath(starts), shellEscapePath(starts))}}},
				{ID: "combine", Kind: "WriteFile", Spec: map[string]any{"path": combined, "content": "{{ .runtime.leftPath }}"}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Fresh: true}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if _, err := os.Stat(right); err != nil {
		t.Fatalf("expected right command output: %v", err)
	}
	raw, err := os.ReadFile(combined)
	if err != nil {
		t.Fatalf("read combined: %v", err)
	}
	if strings.TrimSpace(string(raw)) != left {
		t.Fatalf("expected combined output to equal left path %q, got %q", left, strings.TrimSpace(string(raw)))
	}
}

func shellEscapePath(path string) string {
	return strings.ReplaceAll(path, "'", "'\\''")
}

func TestRun_InitKubeadmSkipDoesNotRegisterMissingJoinFile(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	adminConfDir := filepath.Join(dir, "etc", "kubernetes")
	if err := os.MkdirAll(adminConfDir, 0o755); err != nil {
		t.Fatalf("mkdir admin conf dir: %v", err)
	}
	adminConfPath := filepath.Join(adminConfDir, "admin.conf")
	if err := os.WriteFile(adminConfPath, []byte("apiVersion: v1\n"), 0o644); err != nil {
		t.Fatalf("write admin conf: %v", err)
	}
	prevAdminConfPath := kubeadmAdminConfPath
	kubeadmAdminConfPath = adminConfPath
	t.Cleanup(func() { kubeadmAdminConfPath = prevAdminConfPath })

	joinPath := filepath.Join(dir, "join.txt")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:       "init",
				Kind:     "InitKubeadm",
				Spec:     map[string]any{"outputJoinFile": joinPath},
				Register: map[string]string{"workerJoinFile": "joinFile"},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected register failure for skipped init without join file")
	}
	if !strings.Contains(err.Error(), "E_REGISTER_OUTPUT_NOT_FOUND") {
		t.Fatalf("expected E_REGISTER_OUTPUT_NOT_FOUND, got %v", err)
	}
}

func TestRun_WhenInvalidExpression(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "worker"},
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "bad-when",
				Kind: "Command",
				When: "vars.role = \"worker\"",
				Spec: map[string]any{"command": []any{"true"}},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected condition evaluation failure")
	}
	if !strings.Contains(err.Error(), "E_CONDITION_EVAL") {
		t.Fatalf("expected E_CONDITION_EVAL, got %v", err)
	}
}

func TestWhen_NamespaceEnforced(t *testing.T) {
	vars := map[string]any{"nodeRole": "worker"}
	runtimeVars := map[string]any{"hostPassed": true}
	ok, err := EvaluateWhen("vars.nodeRole == \"worker\"", vars, runtimeVars)
	if err != nil {
		t.Fatalf("expected vars namespace expression to pass, got %v", err)
	}
	if !ok {
		t.Fatalf("expected vars namespace expression to be true")
	}

	_, err = EvaluateWhen("nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected bare identifier to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"nodeRole\"; use vars.nodeRole") {
		t.Fatalf("expected bare identifier guidance, got %v", err)
	}

	_, err = EvaluateWhen("context.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected context namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"context.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}

	_, err = EvaluateWhen("other.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected unknown dotted namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"other.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}
}
