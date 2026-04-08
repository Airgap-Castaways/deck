package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_Wait(t *testing.T) {
	t.Run("waits for file to appear", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "appears.txt")

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "wait-file",
					Kind: "WaitForFile",
					Spec: map[string]any{"path": target, "type": "file", "pollInterval": "10ms", "timeout": "1s"},
				}},
			}},
		}

		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = os.WriteFile(target, []byte("ok"), 0o644)
		}()

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected Wait success, got %v", err)
		}
	})

	t.Run("waits for path to disappear", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "gone.txt")
		if err := os.WriteFile(target, []byte("still here"), 0o644); err != nil {
			t.Fatalf("write initial target: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "wait-absent",
					Kind: "WaitForMissingFile",
					Spec: map[string]any{"path": target, "pollInterval": "10ms", "timeout": "1s"},
				}},
			}},
		}

		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = os.Remove(target)
		}()

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected Wait absent success, got %v", err)
		}
	})

	t.Run("waits for non-empty file", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "non-empty.txt")
		if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
			t.Fatalf("write empty file: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "wait-non-empty",
					Kind: "WaitForFile",
					Spec: map[string]any{"path": target, "type": "file", "nonEmpty": true, "pollInterval": "10ms", "timeout": "1s"},
				}},
			}},
		}

		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = os.WriteFile(target, []byte("ready"), 0o644)
		}()

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected Wait non-empty success, got %v", err)
		}
	})

	t.Run("type mismatch times out with clear error", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "typed")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:      "wait-type",
					Kind:    "WaitForFile",
					Timeout: "80ms",
					Spec:    map[string]any{"path": target, "type": "file", "pollInterval": "10ms"},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{StatePath: statePath})
		if err == nil {
			t.Fatalf("expected Wait timeout error")
		}
		if !errcode.Is(err, errCodeInstallWaitTimeout) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallWaitTimeout, err)
		}
		if !strings.Contains(err.Error(), errCodeInstallWaitTimeout) {
			t.Fatalf("expected %s, got %v", errCodeInstallWaitTimeout, err)
		}
		if !strings.Contains(err.Error(), target) {
			t.Fatalf("expected timeout error to include path, got %v", err)
		}
		if !strings.Contains(err.Error(), "exist as a file") {
			t.Fatalf("expected timeout error to include expected condition, got %v", err)
		}
	})

	t.Run("missing file glob succeeds after files disappear", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		manifestDir := filepath.Join(dir, "manifests")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatalf("mkdir manifests: %v", err)
		}
		target := filepath.Join(manifestDir, "pod.yaml")
		if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = os.Remove(target)
		}()

		wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "install", Steps: []config.Step{{
			ID:      "wait-glob",
			Kind:    "WaitForMissingFile",
			Timeout: "200ms",
			Spec:    map[string]any{"glob": filepath.Join(manifestDir, "*.yaml"), "interval": "10ms"},
		}}}}}
		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected wait glob success, got %v", err)
		}
	})
}

func TestRun_WaitRequiresRequiredFields(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	target := filepath.Join(dir, "appears.txt")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "wait-file",
				Kind: "WaitForFile",
				Spec: map[string]any{"path": target, "timeout": "10ms"},
			}},
		}},
	}
	err := Run(context.Background(), wf, RunOptions{StatePath: statePath})
	if err == nil {
		t.Fatalf("expected wait timeout error")
	}
	if !strings.Contains(err.Error(), errCodeInstallWaitTimeout) {
		t.Fatalf("expected wait timeout error, got %v", err)
	}
}
