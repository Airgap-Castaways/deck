package validate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WorkspaceWithContext(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestEntrypointWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := EntrypointWithContext(ctx, filepath.Join(t.TempDir(), "workflows", "scenarios", "apply.yaml"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestAnalyzeFilesWithContextCanceled(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "workflow.yaml")
	if err := os.WriteFile(path, []byte("version: v1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := AnalyzeFilesWithContext(ctx, []string{path})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestWorkspaceWithContextAllowsScenarioOnlyWorkspace(t *testing.T) {
	root := t.TempDir()
	scenarioDir := filepath.Join(root, "workflows", "scenarios")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	applyPath := filepath.Join(scenarioDir, "apply.yaml")
	content := "version: v1alpha1\nsteps:\n  - id: check\n    kind: CheckHost\n    spec:\n      checks: [os]\n"
	if err := os.WriteFile(applyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	files, err := WorkspaceWithContext(context.Background(), root)
	if err != nil {
		t.Fatalf("workspace validate scenario-only root: %v", err)
	}
	if len(files) != 1 || filepath.Clean(files[0]) != filepath.Clean(applyPath) {
		t.Fatalf("unexpected validated files: %#v", files)
	}
}
