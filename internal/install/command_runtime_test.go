package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_CommandErrorCodes(t *testing.T) {
	t.Run("non-zero exit", func(t *testing.T) {
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
			Phases: []config.Phase{{
				Name:  "install",
				Steps: []config.Step{{ID: "cmd", Kind: "Command", Spec: map[string]any{"command": []any{"false"}}}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if err == nil {
			t.Fatalf("expected run command failure")
		}
		if !strings.Contains(err.Error(), "E_INSTALL_RUNCOMMAND_FAILED") {
			t.Fatalf("expected E_INSTALL_RUNCOMMAND_FAILED, got %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
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
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "cmd",
					Kind: "Command",
					Spec: map[string]any{"command": []any{"sleep", "1"}, "timeout": "10ms"},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if err == nil {
			t.Fatalf("expected run command timeout")
		}
		if !strings.Contains(err.Error(), "E_INSTALL_RUNCOMMAND_TIMEOUT") {
			t.Fatalf("expected E_INSTALL_RUNCOMMAND_TIMEOUT, got %v", err)
		}
	})

	t.Run("timeout classification", func(t *testing.T) {
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

		fakeList := filepath.Join(dir, "fake-images-timeout.sh")
		script := "#!/usr/bin/env bash\nset -euo pipefail\nsleep 1\n"
		if err := os.WriteFile(fakeList, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake list script: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "verify-images",
					Kind: "VerifyImage",
					Spec: map[string]any{
						"images":  []any{"registry.k8s.io/pause:3.10.1"},
						"command": []any{fakeList},
						"timeout": "20ms",
					},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if err == nil {
			t.Fatalf("expected verify images timeout")
		}
		if !errcode.Is(err, errCodeInstallImagesCmdFailed) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallImagesCmdFailed, err)
		}
		if !strings.Contains(err.Error(), errCodeInstallImagesCmdFailed) {
			t.Fatalf("expected verify images error code, got %v", err)
		}
		if !strings.Contains(err.Error(), "image verification timed out") {
			t.Fatalf("expected timeout classification, got %v", err)
		}
	})
}

func TestRun_RetrySemantics(t *testing.T) {
	t.Run("retry succeeds on second attempt", func(t *testing.T) {
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

		marker := filepath.Join(dir, "marker")
		scriptPath := filepath.Join(dir, "fail-once.sh")
		script := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ ! -f \"" + marker + "\" ]]; then\n  touch \"" + marker + "\"\n  exit 1\nfi\nexit 0\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			t.Fatalf("write script: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name:  "install",
				Steps: []config.Step{{ID: "retry-cmd", Kind: "Command", Retry: 1, Spec: map[string]any{"command": []any{scriptPath}}}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
			t.Fatalf("expected retry success, got %v", err)
		}
	})

	t.Run("retry exhausted keeps failure", func(t *testing.T) {
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

		counterPath := filepath.Join(dir, "counter")
		scriptPath := filepath.Join(dir, "always-fail.sh")
		script := "#!/usr/bin/env bash\nset -euo pipefail\ncount=0\nif [[ -f \"" + counterPath + "\" ]]; then\n  count=$(cat \"" + counterPath + "\")\nfi\ncount=$((count+1))\necho \"${count}\" > \"" + counterPath + "\"\nexit 1\n"
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			t.Fatalf("write script: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name:  "install",
				Steps: []config.Step{{ID: "retry-cmd", Kind: "Command", Retry: 1, Spec: map[string]any{"command": []any{scriptPath}}}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if err == nil {
			t.Fatalf("expected failure after retry exhaustion")
		}

		counterRaw, err := os.ReadFile(counterPath)
		if err != nil {
			t.Fatalf("read counter: %v", err)
		}
		if strings.TrimSpace(string(counterRaw)) != "2" {
			t.Fatalf("expected 2 attempts with retry=1, got %q", strings.TrimSpace(string(counterRaw)))
		}
	})
}

func TestRun_RetryStopsWhenParentContextDone(t *testing.T) {
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

	counterPath := filepath.Join(dir, "counter")
	scriptPath := filepath.Join(dir, "slow-fail.sh")
	script := "#!/usr/bin/env bash\nset -euo pipefail\ncount=0\nif [[ -f \"" + counterPath + "\" ]]; then\n  count=$(cat \"" + counterPath + "\")\nfi\ncount=$((count+1))\necho \"${count}\" > \"" + counterPath + "\"\nsleep 30\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "retry-cmd", Kind: "Command", Retry: 4, Spec: map[string]any{"command": []any{scriptPath}, "timeout": "5s"}}},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := Run(ctx, wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected parent context cancellation")
	}

	counterRaw, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("read counter: %v", readErr)
	}
	if strings.TrimSpace(string(counterRaw)) != "1" {
		t.Fatalf("expected exactly one attempt when parent context ends, got %q", strings.TrimSpace(string(counterRaw)))
	}
}

func TestRun_CommandParentCancelNotRelabeledAsTimeout(t *testing.T) {
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
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "cmd", Kind: "Command", Spec: map[string]any{"command": []any{"true"}, "timeout": "3s"}}},
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected canceled context error")
	}
	if strings.Contains(err.Error(), errCodeInstallCommandTimeout) {
		t.Fatalf("expected parent cancellation to not be mapped to timeout, got %v", err)
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("expected canceled context in error, got %v", err)
	}
}

func TestCommandOutputWithContext_TimeoutReturnsSentinel(t *testing.T) {
	_, err := runCommandOutputWithContext(context.Background(), []string{"sleep", "1"}, 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, ErrStepCommandTimeout) {
		t.Fatalf("expected step timeout sentinel, got %v", err)
	}
}

func TestCommandOutputWithContext_RejectsNilContext(t *testing.T) {
	_, err := runCommandOutputWithContext(nilContextForInstallTest(), []string{"true"}, time.Second)
	if err == nil {
		t.Fatalf("expected nil context error")
	}
	if !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommandSupportsEnvAndSudo(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(dir, "command.log")
	sudoScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'sudo:%s\\n' \"$*\" >> \"" + logPath + "\"\nexec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "sudo"), []byte(sudoScript), 0o755); err != nil {
		t.Fatalf("write sudo script: %v", err)
	}
	commandPath := filepath.Join(binDir, "print-env")
	commandScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'env:%s\\n' \"${DECK_TEST_ENV:-missing}\" >> \"" + logPath + "\"\n"
	if err := os.WriteFile(commandPath, []byte(commandScript), 0o755); err != nil {
		t.Fatalf("write command script: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	if err := runCommand(context.Background(), map[string]any{
		"command": []any{"print-env"},
		"env":     map[string]any{"DECK_TEST_ENV": "present"},
		"sudo":    true,
	}); err != nil {
		t.Fatalf("runCommand failed: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logText := string(raw)
	if !strings.Contains(logText, "sudo:print-env") {
		t.Fatalf("expected sudo invocation, got %q", logText)
	}
	if !strings.Contains(logText, "env:present") {
		t.Fatalf("expected env to be passed, got %q", logText)
	}
}
