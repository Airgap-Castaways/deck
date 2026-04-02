package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunPrepareCreatesPreparedBundleDir(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow scenarios: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workflowsDir, "components"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/source.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir, "vars.yaml"), []byte("kubernetesVersion: v1.30.1\n"), 0o644); err != nil {
		t.Fatalf("write vars workflow: %v", err)
	}
	fragmentDir := filepath.Join(workflowsDir, "components", "offline-multinode")
	if err := os.MkdirAll(fragmentDir, 0o755); err != nil {
		t.Fatalf("mkdir fragment dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fragmentDir, "apply-common.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write workflow fragment: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})

	preparedRoot := filepath.Join(root, "outputs")
	if _, err := runWithCapturedStdout([]string{"prepare", "--root", preparedRoot, "--bundle-binary-source", "local"}); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	for _, required := range []string{
		filepath.Join("files", "source.bin"),
		filepath.Join("bin", runtime.GOOS, runtime.GOARCH, "deck"),
	} {
		if _, err := os.Stat(filepath.Join(preparedRoot, required)); err != nil {
			t.Fatalf("missing prepared path %s: %v", required, err)
		}
	}
	for _, required := range []string{"deck", filepath.Join(".deck", "manifest.json"), filepath.Join("workflows", "prepare.yaml")} {
		if _, err := os.Stat(filepath.Join(root, required)); err != nil {
			t.Fatalf("missing workspace path %s: %v", required, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "deck"))
	if err != nil {
		t.Fatalf("read deck launcher: %v", err)
	}
	if !strings.Contains(string(raw), "outputs/bin/") {
		t.Fatalf("expected launcher to point at outputs/bin, got %q", string(raw))
	}
}

func TestRunPrepareDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir, "vars.yaml"), []byte("x: y\n"), 0o644); err != nil {
		t.Fatalf("write vars workflow: %v", err)
	}
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})

	preparedRoot := filepath.Join(root, "outputs")
	planOut, err := runWithCapturedStdout([]string{"prepare", "--dry-run", "--bundle-binary-source", "local"})
	if err != nil {
		t.Fatalf("prepare dry-run failed: %v", err)
	}
	if !strings.Contains(planOut, "PREPARE_WORKFLOW=") {
		t.Fatalf("expected dry-run plan output, got %q", planOut)
	}
	if _, statErr := os.Stat(preparedRoot); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run must not create prepared root, stat err=%v", statErr)
	}
}

func TestRunPrepareVerboseDiagnostics(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir, "vars.yaml"), []byte("x: y\n"), 0o644); err != nil {
		t.Fatalf("write vars workflow: %v", err)
	}
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	res := execute([]string{"prepare", "--dry-run", "--v=1", "--bundle-binary-source", "local"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if !strings.Contains(res.stdout, "PREPARE_WORKFLOW=") {
		t.Fatalf("unexpected stdout: %q", res.stdout)
	}
	for _, want := range []string{"component=prepare", "event=run_requested", "root=outputs", "event=workflow_selected", "event=vars_selected", "event=apply_selected", "event=prepared_root", "event=dry_run"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}

	res = execute([]string{"prepare", "--dry-run", "--v=2", "--bundle-binary-source", "local"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	for _, want := range []string{"component=prepare", "event=workflow_includes", "count=3", "event=cache_artifact", "step=seed", "type=file", "action=FETCH", "event=cache_plan", "fetch=1", "reuse=0"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
}

func TestRunPrepareVerboseStepDiagnostics(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	res := execute([]string{"prepare", "--root", filepath.Join(root, "outputs"), "--v=1", "--bundle-binary-source", "local"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if !strings.Contains(res.stdout, "prepare: ok") {
		t.Fatalf("unexpected stdout: %q", res.stdout)
	}
	for _, want := range []string{
		"component=prepare event=batch_started",
		"component=prepare event=step_started",
		"step=seed",
		"batch=prepare",
		"component=prepare event=step_succeeded",
		"duration_ms=",
		"component=prepare event=batch_succeeded batch=prepare",
	} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
}

func TestRunPrepareEmitsDefaultProgressLog(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	res := execute([]string{"prepare", "--root", filepath.Join(root, "outputs"), "--bundle-binary-source", "local"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if !strings.Contains(res.stderr, "component=prepare event=step_started") || !strings.Contains(res.stderr, "step=seed") || !strings.Contains(res.stderr, "batch=prepare") {
		t.Fatalf("expected default step progress on stderr, got %q", res.stderr)
	}
	if !strings.Contains(res.stderr, "component=prepare event=batch_succeeded batch=prepare") || !strings.Contains(res.stderr, "duration_ms=") {
		t.Fatalf("expected completion progress with duration, got %q", res.stderr)
	}
}

func TestRunPrepareSupportsJSONLogFormat(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	res := execute([]string{"prepare", "--root", filepath.Join(root, "outputs"), "--bundle-binary-source", "local", "--log-format=json"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	lines := strings.Split(strings.TrimSpace(res.stderr), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected stderr log lines")
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("expected JSON log line, got %q: %v", line, err)
		}
	}
}

func TestRunPrepareSucceedsWithoutApplyWorkflow(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writePrepareDownloadWorkflowFixture(t, root, "files/seed.bin")
	if err := os.WriteFile(filepath.Join(workflowsDir, "vars.yaml"), []byte("x: y\n"), 0o644); err != nil {
		t.Fatalf("write vars workflow: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})

	preparedRoot := filepath.Join(root, "outputs")
	out, err := runWithCapturedStdout([]string{"prepare", "--root", preparedRoot, "--bundle-binary-source", "local"})
	if err != nil {
		t.Fatalf("prepare failed without apply workflow: %v", err)
	}
	if !strings.Contains(out, "prepare: ok") {
		t.Fatalf("unexpected output: %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(preparedRoot, "workflows", "scenarios", "apply.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("apply workflow must remain optional, stat err=%v", statErr)
	}
}

func TestRunPrepareVarFlagOverridesWorkflowVars(t *testing.T) {
	root := t.TempDir()
	workflowsDir := filepath.Join(root, "workflows")
	if err := os.MkdirAll(filepath.Join(workflowsDir, "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workflowsDir, "components"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	packPath := filepath.Join(root, "workflows", "prepare.yaml")
	packBody := fmt.Sprintf(`version: v1alpha1
vars:
  relPath: default.bin
phases:
  - name: prepare
    steps:
      - id: p1
        kind: DownloadFile
        spec:
          source:
            path: files/source.bin
          fetch:
            sources:
              - type: local
                path: %q
          outputPath: files/{{ .vars.relPath  }}
`, filepath.Join(root, "seed"))
	if err := os.WriteFile(packPath, []byte(packBody), 0o644); err != nil {
		t.Fatalf("write prepare workflow: %v", err)
	}
	seedDir := filepath.Join(root, "seed", "files")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "source.bin"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir, "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowsDir, "vars.yaml"), []byte("kubernetesVersion: v1.30.1\n"), 0o644); err != nil {
		t.Fatalf("write vars workflow: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})

	preparedRoot := filepath.Join(root, "outputs")
	if _, err := runWithCapturedStdout([]string{"prepare", "--root", preparedRoot, "--bundle-binary-source", "local", "--var", "relPath=override.bin"}); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(preparedRoot, "files", "override.bin")); err != nil {
		t.Fatalf("expected override output in prepared root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(preparedRoot, "files", "default.bin")); !os.IsNotExist(err) {
		t.Fatalf("unexpected default output in prepared root: %v", err)
	}
}
