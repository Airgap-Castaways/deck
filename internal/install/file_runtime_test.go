package install

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestRun_CreateSymlink(t *testing.T) {
	t.Run("creates a new symlink", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "target.txt")
		linkPath := filepath.Join(dir, "link.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": target},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected symlink success, got %v", err)
		}

		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("lstat symlink path: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink mode, got %v", info.Mode())
		}
		actualTarget, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("readlink symlink path: %v", err)
		}
		if actualTarget != target {
			t.Fatalf("expected symlink target %q, got %q", target, actualTarget)
		}
	})

	t.Run("createParent creates destination parent", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "target.txt")
		linkPath := filepath.Join(dir, "nested", "path", "link.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": target, "createParent": true},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected symlink success, got %v", err)
		}
		if _, err := os.Stat(filepath.Dir(linkPath)); err != nil {
			t.Fatalf("expected created parent dir, got %v", err)
		}
	})

	t.Run("requireTarget rejects missing target", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		missingTarget := filepath.Join(dir, "missing.txt")
		linkPath := filepath.Join(dir, "link.txt")

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": missingTarget, "requireTarget": true},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{StatePath: statePath})
		if err == nil {
			t.Fatalf("expected requireTarget failure")
		}
		if !strings.Contains(err.Error(), "symlink target does not exist") {
			t.Fatalf("expected missing target error, got %v", err)
		}
	})

	t.Run("ignoreMissingTarget skips missing target", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		missingTarget := filepath.Join(dir, "missing.txt")
		linkPath := filepath.Join(dir, "link.txt")

		wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "install", Steps: []config.Step{{
			ID:   "CreateSymlink",
			Kind: "CreateSymlink",
			Spec: map[string]any{"path": linkPath, "target": missingTarget, "ignoreMissingTarget": true},
		}}}}}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected missing target skip, got %v", err)
		}
		if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
			t.Fatalf("expected no symlink created, got err=%v", err)
		}
	})

	t.Run("force replaces existing destination path", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "target.txt")
		linkPath := filepath.Join(dir, "link.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}
		if err := os.WriteFile(linkPath, []byte("existing"), 0o644); err != nil {
			t.Fatalf("write existing path: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": target, "force": true},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected symlink success, got %v", err)
		}
		actualTarget, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("expected destination replaced with symlink, got %v", err)
		}
		if actualTarget != target {
			t.Fatalf("expected symlink target %q, got %q", target, actualTarget)
		}
	})

	t.Run("force does not replace existing directory", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "target.txt")
		linkPath := filepath.Join(dir, "existing-dir")
		nested := filepath.Join(linkPath, "keep.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}
		if err := os.MkdirAll(linkPath, 0o755); err != nil {
			t.Fatalf("mkdir existing directory: %v", err)
		}
		if err := os.WriteFile(nested, []byte("keep"), 0o644); err != nil {
			t.Fatalf("write nested file: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": target, "force": true},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{StatePath: statePath})
		if err == nil {
			t.Fatalf("expected failure when destination is directory")
		}
		if !strings.Contains(err.Error(), "destination is a directory and cannot be replaced") {
			t.Fatalf("expected safe directory replacement error, got %v", err)
		}
		if _, statErr := os.Stat(nested); statErr != nil {
			t.Fatalf("expected directory contents preserved, got %v", statErr)
		}
	})

	t.Run("existing correct symlink is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		target := filepath.Join(dir, "target.txt")
		linkPath := filepath.Join(dir, "link.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write target: %v", err)
		}
		if err := os.Symlink(target, linkPath); err != nil {
			t.Fatalf("create initial symlink: %v", err)
		}

		before, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("lstat initial symlink: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "CreateSymlink",
					Kind: "CreateSymlink",
					Spec: map[string]any{"path": linkPath, "target": target},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected idempotent symlink success, got %v", err)
		}

		after, err := os.Lstat(linkPath)
		if err != nil {
			t.Fatalf("lstat symlink after run: %v", err)
		}
		if !before.ModTime().Equal(after.ModTime()) {
			t.Fatalf("expected symlink to be unchanged")
		}
		actualTarget, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("readlink symlink after run: %v", err)
		}
		if actualTarget != target {
			t.Fatalf("expected symlink target %q, got %q", target, actualTarget)
		}
	})
}

func TestRun_FileRespectsParentContext(t *testing.T) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "download",
				Kind: "CopyFile",
				Spec: map[string]any{"source": map[string]any{"url": srv.URL + "/files/payload.txt"}, "path": filepath.Join(dir, "payload.txt")},
			}},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := Run(ctx, wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected download cancellation")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded in error, got %v", err)
	}
}

func TestRun_DownloadFileRegistersOutputs(t *testing.T) {
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:       "download",
				Kind:     "CopyFile",
				Spec:     map[string]any{"source": map[string]any{"url": srv.URL + "/files/payload.txt"}, "path": filepath.Join(dir, "payload.txt")},
				Register: map[string]string{"downloadPath": "path"},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	stateRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var st State
	if err := json.Unmarshal(stateRaw, &st); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if st.RuntimeVars["downloadPath"] != filepath.Join(dir, "payload.txt") {
		t.Fatalf("expected registered path, got %#v", st.RuntimeVars["downloadPath"])
	}
}

func TestResolveSourceBytes_PreservesContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolveSourceBytes(ctx, map[string]any{
		"fetch": map[string]any{
			"sources": []any{map[string]any{"type": "online", "url": srv.URL}},
		},
	}, "files/payload.txt")
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if strings.Contains(err.Error(), "E_INSTALL_SOURCE_NOT_FOUND") {
		t.Fatalf("expected cancellation to not be mapped to source-not-found, got %v", err)
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("expected canceled context in error, got %v", err)
	}
}

func TestExecuteStep_CopyFileDecodeError(t *testing.T) {
	_, err := executeWorkflowStep(context.Background(), config.Step{Kind: "CopyFile", Spec: map[string]any{"source": 42, "path": "/tmp/out"}}, map[string]any{"source": 42, "path": "/tmp/out"}, workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "CopyFile"}, ExecutionContext{})
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode CopyFile spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureDirStep(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b")
	if err := runEnsureDir(map[string]any{"path": target, "mode": "0750"}); err != nil {
		t.Fatalf("runEnsureDir failed: %v", err)
	}
	if err := runEnsureDir(map[string]any{"path": target, "mode": "0750"}); err != nil {
		t.Fatalf("runEnsureDir second pass failed: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("unexpected mode: %o", info.Mode().Perm())
	}
}

func TestInstallFileStep(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installed.txt")
	spec := map[string]any{"path": target, "content": "hello", "mode": "0640"}
	if err := runWriteFile(spec); err != nil {
		t.Fatalf("runWriteFile failed: %v", err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := runWriteFile(spec); err != nil {
		t.Fatalf("runWriteFile second pass failed: %v", err)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("expected idempotent write to keep mtime")
	}
}

func TestTemplateFileStep(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "templated.txt")
	if err := runTemplateFile(map[string]any{"path": target, "template": "line", "mode": "0644"}); err != nil {
		t.Fatalf("runTemplateFile failed: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(raw) != "line\n" {
		t.Fatalf("unexpected content: %q", string(raw))
	}
}
