package install

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestRun_InstallTools(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	bundle := filepath.Join(dir, "bundle")
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

	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	sysctlPath := filepath.Join(dir, "sysctl.conf")
	modprobePath := filepath.Join(dir, "modules.conf")
	joinPath := filepath.Join(dir, "join.txt")
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeApt := filepath.Join(binDir, "apt-get")
	if err := os.WriteFile(fakeApt, []byte("#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))
	kubeadm := useStubInitJoinKubeadm()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "install-packages", Kind: "InstallPackage", Spec: map[string]any{"packages": []any{"containerd"}}},
				{ID: "write-file", Kind: "WriteFile", Spec: map[string]any{"path": fileA, "content": "hello world"}},
				{ID: "edit-file", Kind: "EditFile", Spec: map[string]any{"path": fileA, "edits": []any{map[string]any{"op": "replace", "match": "world", "replaceWith": "deck"}}}},
				{ID: "copy-file", Kind: "CopyFile", Spec: map[string]any{"source": map[string]any{"path": fileA}, "path": fileB}},
				{ID: "Sysctl", Kind: "Sysctl", Spec: map[string]any{"writeFile": sysctlPath, "values": map[string]any{"net.ipv4.ip_forward": "1"}}},
				{ID: "modprobe", Kind: "KernelModule", Spec: map[string]any{"name": "overlay", "persistFile": modprobePath}},
				{ID: "run-cmd", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}},
				{ID: "kubeadm-init", Kind: "InitKubeadm", Spec: map[string]any{"outputJoinFile": joinPath}},
				{ID: "kubeadm-join", Kind: "JoinKubeadm", Spec: map[string]any{"joinFile": joinPath}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath, kubeadm: kubeadm}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	contentA, err := os.ReadFile(fileA)
	if err != nil {
		t.Fatalf("read fileA: %v", err)
	}
	if string(contentA) != "hello deck\n" {
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
	if len(st.CompletedPhases) != 1 || st.CompletedPhases[0] != "install" {
		t.Fatalf("unexpected completed phases: %#v", st.CompletedPhases)
	}
	if st.Phase != "completed" {
		t.Fatalf("expected final phase state completed, got %q", st.Phase)
	}
}

func TestRun_DefaultStatePathUsesHomeStateKey(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	bundle := filepath.Join(dir, "bundle")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
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
	t.Setenv("HOME", home)

	wf := &config.Workflow{
		StateKey: "state-key-default-path-test",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "s1", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	statePath := filepath.Join(home, ".local", "state", "deck", "state", wf.StateKey+".json")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file missing at expected home path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, ".deck", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected bundle state file, err=%v", err)
	}
}

func TestRun_ManifestIntegrityVerified(t *testing.T) {
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
	if err := os.WriteFile(artifact, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("hello")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "s1", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected install success, got %v", err)
	}
}

func TestRun_ManifestIntegrityMismatch(t *testing.T) {
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
	if err := os.WriteFile(artifact, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("different")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "s1", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected manifest integrity error")
	}
	if !strings.Contains(err.Error(), "E_BUNDLE_INTEGRITY") {
		t.Fatalf("expected E_BUNDLE_INTEGRITY error, got %v", err)
	}
}

func TestRun_NoPhasesFails(t *testing.T) {
	err := Run(context.Background(), &config.Workflow{Version: "v1"}, RunOptions{})
	if err == nil {
		t.Fatalf("expected no phases error")
	}
	if !strings.Contains(err.Error(), "no phases found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ResumeFromFailedStep(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	bundle := filepath.Join(dir, "bundle")
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

	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "s1", Kind: "WriteFile", Spec: map[string]any{"path": first, "content": "ok"}},
				{ID: "s2", Kind: "Command", Spec: map[string]any{"command": []any{"false"}}},
				{ID: "s3", Kind: "WriteFile", Spec: map[string]any{"path": second, "content": "done"}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err == nil {
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
	if len(st.CompletedPhases) != 0 {
		t.Fatalf("unexpected completed phases after failure: %#v", st.CompletedPhases)
	}
	if st.FailedPhase != "install" {
		t.Fatalf("expected failed phase install, got %q", st.FailedPhase)
	}

	wf.Phases[0].Steps[1].Spec = map[string]any{"command": []any{"true"}}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
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
	if len(final.CompletedPhases) != 1 || final.CompletedPhases[0] != "install" {
		t.Fatalf("unexpected completed phases after resume: %#v", final.CompletedPhases)
	}
	if final.FailedPhase != "" {
		t.Fatalf("expected empty failed phase, got %q", final.FailedPhase)
	}
}

func TestRun_UnsupportedInstallKindFails(t *testing.T) {
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
			Steps: []config.Step{{ID: "x", Kind: "UnknownKind", Spec: map[string]any{}}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected unsupported kind error")
	}
	if !strings.Contains(err.Error(), "E_INSTALL_KIND_UNSUPPORTED") {
		t.Fatalf("expected E_INSTALL_KIND_UNSUPPORTED, got %v", err)
	}
}

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

func TestRun_ExposesTypedErrorCodes(t *testing.T) {
	t.Run("unsupported kind", func(t *testing.T) {
		dir := t.TempDir()
		bundle := filepath.Join(dir, "bundle")
		statePath := filepath.Join(dir, "state", "state.json")
		if err := os.MkdirAll(bundle, 0o755); err != nil {
			t.Fatalf("mkdir bundle: %v", err)
		}
		artifact := filepath.Join(bundle, "files", "placeholder.txt")
		if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
			t.Fatalf("mkdir files: %v", err)
		}
		if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		if err := writeManifestForTest(bundle, "files/placeholder.txt", []byte("ok")); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name:  "install",
				Steps: []config.Step{{ID: "x", Kind: "UnknownKind", Spec: map[string]any{}}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if !errcode.Is(err, errCodeInstallKindUnsupported) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallKindUnsupported, err)
		}
	})

	t.Run("command timeout", func(t *testing.T) {
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
		if !errcode.Is(err, errCodeInstallCommandTimeout) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallCommandTimeout, err)
		}
	})
}

func TestRun_SystemValidationErrorsExposeTypedCodes(t *testing.T) {
	t.Run("sysctl missing path", func(t *testing.T) {
		err := runSysctl(context.Background(), map[string]any{"values": map[string]any{"net.ipv4.ip_forward": 1}})
		if !errcode.Is(err, errCodeInstallSysctlPathMiss) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallSysctlPathMiss, err)
		}
	})

	t.Run("manage service missing name", func(t *testing.T) {
		err := runManageService(context.Background(), map[string]any{"state": "started"})
		if !errcode.Is(err, errCodeInstallManageServiceNameMiss) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallManageServiceNameMiss, err)
		}
	})

	t.Run("systemd unit missing content", func(t *testing.T) {
		err := runWriteSystemdUnit(context.Background(), map[string]any{"path": "/etc/systemd/system/demo.service"})
		if !errcode.Is(err, errCodeInstallWriteSystemdUnitInput) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallWriteSystemdUnitInput, err)
		}
	})
}

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

func TestExecuteStep_CopyFileDecodeError(t *testing.T) {
	_, err := executeWorkflowStep(context.Background(), config.Step{Kind: "CopyFile", Spec: map[string]any{"source": 42, "path": "/tmp/out"}}, map[string]any{"source": 42, "path": "/tmp/out"}, workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "CopyFile"}, ExecutionContext{})
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode CopyFile spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_WriteContainerdConfigDefaultGenerationRespectsParentContext(t *testing.T) {
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

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeWriteContainerdConfig := filepath.Join(binDir, "containerd")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"config\" && \"${2:-}\" == \"default\" ]]; then\n  sleep 1\n  echo 'version = 2'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeWriteContainerdConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake containerd: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	target := filepath.Join(dir, "containerd", "config.toml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "containerd-config", Kind: "WriteContainerdConfig", Spec: map[string]any{"path": target, "timeout": "5s"}}},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := Run(ctx, wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected canceled containerd config generation")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded in error, got %v", err)
	}
}

func TestRun_WriteContainerdConfigDefaultGenerationTimeoutUsesTimeoutClassification(t *testing.T) {
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

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeWriteContainerdConfig := filepath.Join(binDir, "containerd")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"config\" && \"${2:-}\" == \"default\" ]]; then\n  sleep 1\n  echo 'version = 2'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeWriteContainerdConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake containerd: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	target := filepath.Join(dir, "containerd", "config.toml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "containerd-config", Kind: "WriteContainerdConfig", Spec: map[string]any{"path": target, "timeout": "20ms"}}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected containerd config timeout")
	}
	if !strings.Contains(err.Error(), "containerd config default generation timed out") {
		t.Fatalf("expected timeout classification, got %v", err)
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

func TestRun_PackagesExecutesPackageManager(t *testing.T) {
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

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-invoked.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" > \"" + marker + "\"\nexit 0\n"
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{"packages": []any{"containerd", "kubelet"}},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected install packages success, got %v", err)
	}

	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if !strings.Contains(args, "install -y containerd kubelet") {
		t.Fatalf("unexpected apt-get args: %q", args)
	}
}

func TestRun_PackagesSourcePathValidation(t *testing.T) {
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
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"source":   map[string]any{"type": "local-repo", "path": filepath.Join(dir, "missing")},
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected source validation error")
	}
	if !errcode.Is(err, errCodeInstallPkgSourceInvalid) {
		t.Fatalf("expected typed code %s, got %v", errCodeInstallPkgSourceInvalid, err)
	}
	if !strings.Contains(err.Error(), "E_INSTALL_PACKAGES_SOURCE_INVALID") {
		t.Fatalf("expected E_INSTALL_PACKAGES_SOURCE_INVALID, got %v", err)
	}
}

func TestRun_InstallPackagesFromLocalRepo(t *testing.T) {
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

	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	debA := filepath.Join(repoDir, "containerd_1.0.0_amd64.deb")
	debB := filepath.Join(repoDir, "kubelet_1.30.1_amd64.deb")
	if err := os.WriteFile(debA, []byte("deb-a"), 0o644); err != nil {
		t.Fatalf("write debA: %v", err)
	}
	if err := os.WriteFile(debB, []byte("deb-b"), 0o644); err != nil {
		t.Fatalf("write debB: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	marker := filepath.Join(dir, "apt-local-invoked.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" > \"" + marker + "\"\nexit 0\n"
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "install-pkgs",
				Kind: "InstallPackage",
				Spec: map[string]any{
					"packages": []any{"containerd", "kubelet"},
					"source":   map[string]any{"type": "local-repo", "path": repoDir},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected local repo install success, got %v", err)
	}

	raw, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	args := strings.TrimSpace(string(raw))
	if !strings.Contains(args, "install -y") {
		t.Fatalf("unexpected apt-get args: %q", args)
	}
	if !strings.Contains(args, debA) || !strings.Contains(args, debB) {
		t.Fatalf("local deb artifacts were not passed to apt-get: %q", args)
	}
}

func TestRun_PackagesTimeoutUsesTimeoutClassification(t *testing.T) {
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

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nsleep 1\n"
	if err := os.WriteFile(fakeApt, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake apt-get: %v", err)
	}
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:      "install-pkgs",
				Kind:    "InstallPackage",
				Timeout: "20ms",
				Spec:    map[string]any{"packages": []any{"containerd"}},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected install packages timeout")
	}
	if !strings.Contains(err.Error(), errCodeInstallPkgFailed) {
		t.Fatalf("expected package install error code, got %v", err)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout classification, got %v", err)
	}
	if strings.Contains(err.Error(), "package installation failed") {
		t.Fatalf("expected timeout path instead of generic failure, got %v", err)
	}
}

func TestTemplate_RenderVarsAndRuntime(t *testing.T) {
	wf := &config.Workflow{Vars: map[string]any{"kubernetesVersion": "v1.30.1", "registry": map[string]any{"host": "registry.k8s.io"}}}
	runtimeVars := map[string]any{"joinFile": "/tmp/join.txt"}

	rendered, err := renderSpec(map[string]any{
		"path": "{{ .runtime.joinFile }}",
		"nested": map[string]any{
			"image": "{{ .vars.registry.host }}/kube-apiserver:{{ .vars.kubernetesVersion }}",
		},
		"items": []any{
			"{{ .vars.kubernetesVersion }}",
			map[string]any{"join": "{{ .runtime.joinFile }}"},
			123,
		},
	}, wf, runtimeVars)
	if err != nil {
		t.Fatalf("renderSpec failed: %v", err)
	}

	if got := rendered["path"]; got != "/tmp/join.txt" {
		t.Fatalf("unexpected rendered path: %#v", got)
	}
	nested, ok := rendered["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested should be map, got %#v", rendered["nested"])
	}
	if got := nested["image"]; got != "registry.k8s.io/kube-apiserver:v1.30.1" {
		t.Fatalf("unexpected rendered image: %#v", got)
	}
	items, ok := rendered["items"].([]any)
	if !ok {
		t.Fatalf("items should be slice, got %#v", rendered["items"])
	}
	if got := items[0]; got != "v1.30.1" {
		t.Fatalf("unexpected rendered items[0]: %#v", got)
	}
	itemMap, ok := items[1].(map[string]any)
	if !ok || itemMap["join"] != "/tmp/join.txt" {
		t.Fatalf("unexpected rendered items[1]: %#v", items[1])
	}
	if got := items[2]; got != 123 {
		t.Fatalf("unexpected rendered items[2]: %#v", got)
	}

	_, err = renderSpec(map[string]any{"content": "{{ .runtime.missing }}"}, wf, runtimeVars)
	if err == nil {
		t.Fatalf("expected unresolved template reference error")
	}
	if !strings.Contains(err.Error(), "spec.content") {
		t.Fatalf("expected error to include spec path, got %v", err)
	}
}

func writeTestTarGz(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()
	for name, content := range files {
		raw := []byte(content)
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(raw))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, strings.NewReader(content)); err != nil {
			return err
		}
	}
	return nil
}

func TestManageServiceStep(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(dir, "systemctl.log")
	scriptPath := filepath.Join(binDir, "systemctl")
	script := "#!/usr/bin/env bash\nset -euo pipefail\ncontains_unit() {\n  local list=\"${1:-}\"\n  local unit=\"${2:-}\"\n  [[ -n \"${unit}\" ]] || return 1\n  if [[ \",${list},\" == *\",${unit},\"* ]]; then\n    return 0\n  fi\n  if [[ \"${unit}\" == *.service ]]; then\n    local base=\"${unit%.service}\"\n    [[ \",${list},\" == *\",${base},\"* ]]\n    return\n  fi\n  [[ \",${list},\" == *\",${unit}.service,\"* ]]\n}\ncmd=\"${1:-}\"\ncase \"${cmd}\" in\n  is-enabled)\n    if contains_unit \"${SYSTEMCTL_ENABLED_UNITS:-}\" \"${2:-}\"; then\n      exit 0\n    fi\n    exit 1\n    ;;\n  is-active)\n    unit=\"${2:-}\"\n    if [[ \"${unit}\" == \"--quiet\" ]]; then\n      unit=\"${3:-}\"\n    fi\n    if contains_unit \"${SYSTEMCTL_ACTIVE_UNITS:-}\" \"${unit}\"; then\n      exit 0\n    fi\n    exit 1\n    ;;\n  list-unit-files)\n    if contains_unit \"${SYSTEMCTL_EXISTING_UNITS:-}\" \"${2:-}\"; then\n      printf '%s enabled\\n' \"${2:-}\"\n      exit 0\n    fi\n    exit 1\n    ;;\n  daemon-reload)\n    printf '%s\\n' \"$*\" >> \"" + logPath + "\"\n    exit 0\n    ;;\n  enable|disable|start|stop|restart|reload)\n    if contains_unit \"${SYSTEMCTL_MISSING_UNITS:-}\" \"${2:-}\"; then\n      printf 'Unit %s not found.\\n' \"${2:-}\" >&2\n      exit 1\n    fi\n    ;;\nesac\nprintf '%s\\n' \"$*\" >> \"" + logPath + "\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write systemctl script: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))
	readLog := func() string {
		raw, err := os.ReadFile(logPath)
		if err != nil {
			if os.IsNotExist(err) {
				return ""
			}
			t.Fatalf("read log: %v", err)
		}
		return string(raw)
	}
	resetLog := func() {
		_ = os.Remove(logPath)
	}

	t.Run("single-name preserves enable and start behavior", func(t *testing.T) {
		resetLog()
		t.Setenv("SYSTEMCTL_ENABLED_UNITS", "")
		t.Setenv("SYSTEMCTL_ACTIVE_UNITS", "")
		t.Setenv("SYSTEMCTL_EXISTING_UNITS", "")
		t.Setenv("SYSTEMCTL_MISSING_UNITS", "")

		if err := runManageService(context.Background(), map[string]any{"name": "containerd", "enabled": true, "state": "started"}); err != nil {
			t.Fatalf("runManageService failed: %v", err)
		}

		got := readLog()
		if !strings.Contains(got, "enable containerd") || !strings.Contains(got, "start containerd") {
			t.Fatalf("expected enable/start invocations, got %q", got)
		}
	})

	t.Run("multi-name disable and stop applies per service", func(t *testing.T) {
		resetLog()
		t.Setenv("SYSTEMCTL_ENABLED_UNITS", "firewalld,ufw")
		t.Setenv("SYSTEMCTL_ACTIVE_UNITS", "firewalld,ufw")
		t.Setenv("SYSTEMCTL_EXISTING_UNITS", "")
		t.Setenv("SYSTEMCTL_MISSING_UNITS", "")

		if err := runManageService(context.Background(), map[string]any{"names": []any{"firewalld", "ufw"}, "enabled": false, "state": "stopped"}); err != nil {
			t.Fatalf("runManageService failed: %v", err)
		}

		got := readLog()
		if !strings.Contains(got, "disable firewalld") || !strings.Contains(got, "disable ufw") {
			t.Fatalf("expected disable for each service, got %q", got)
		}
		if !strings.Contains(got, "stop firewalld") || !strings.Contains(got, "stop ufw") {
			t.Fatalf("expected stop for each service, got %q", got)
		}
	})

	t.Run("daemon reload runs before service operation", func(t *testing.T) {
		resetLog()
		t.Setenv("SYSTEMCTL_ENABLED_UNITS", "")
		t.Setenv("SYSTEMCTL_ACTIVE_UNITS", "")
		t.Setenv("SYSTEMCTL_EXISTING_UNITS", "")
		t.Setenv("SYSTEMCTL_MISSING_UNITS", "")

		if err := runManageService(context.Background(), map[string]any{"name": "containerd", "daemonReload": true, "state": "restarted"}); err != nil {
			t.Fatalf("runManageService failed: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(readLog()), "\n")
		if len(lines) < 2 {
			t.Fatalf("expected daemon-reload and restart commands, got %q", readLog())
		}
		if lines[0] != "daemon-reload" {
			t.Fatalf("expected daemon-reload first, got %q", lines[0])
		}
		if lines[1] != "restart containerd" {
			t.Fatalf("expected restart after daemon-reload, got %q", lines[1])
		}
	})

	t.Run("ifExists skips missing units", func(t *testing.T) {
		resetLog()
		t.Setenv("SYSTEMCTL_ENABLED_UNITS", "")
		t.Setenv("SYSTEMCTL_ACTIVE_UNITS", "firewalld")
		t.Setenv("SYSTEMCTL_EXISTING_UNITS", "firewalld.service")
		t.Setenv("SYSTEMCTL_MISSING_UNITS", "")

		if err := runManageService(context.Background(), map[string]any{"names": []any{"firewalld", "ufw"}, "state": "stopped", "ifExists": true}); err != nil {
			t.Fatalf("runManageService failed: %v", err)
		}

		got := readLog()
		if !strings.Contains(got, "stop firewalld") {
			t.Fatalf("expected firewalld stop call, got %q", got)
		}
		if strings.Contains(got, "ufw") {
			t.Fatalf("expected missing ufw service to be skipped, got %q", got)
		}
	})

	t.Run("ignoreMissing suppresses missing unit operation failures", func(t *testing.T) {
		resetLog()
		t.Setenv("SYSTEMCTL_ENABLED_UNITS", "")
		t.Setenv("SYSTEMCTL_ACTIVE_UNITS", "")
		t.Setenv("SYSTEMCTL_EXISTING_UNITS", "firewalld.service")
		t.Setenv("SYSTEMCTL_MISSING_UNITS", "ufw")

		if err := runManageService(context.Background(), map[string]any{"names": []any{"firewalld", "ufw"}, "state": "started", "ignoreMissing": true}); err != nil {
			t.Fatalf("runManageService failed: %v", err)
		}

		got := readLog()
		if !strings.Contains(got, "start firewalld") {
			t.Fatalf("expected start call for existing service, got %q", got)
		}
		if strings.Contains(got, "start ufw") {
			t.Fatalf("expected missing ufw start failure to be suppressed, got %q", got)
		}
	})
}

func TestRun_ManageServiceRegistersNamesOutput(t *testing.T) {
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

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(dir, "systemctl.log")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + logPath + "\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(script), 0o755); err != nil {
		t.Fatalf("write systemctl script: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:       "ManageService",
				Kind:     "ManageService",
				Spec:     map[string]any{"names": []any{"firewalld", "ufw"}, "state": "restarted", "ignoreMissing": true},
				Register: map[string]string{"managedManageServices": "names"},
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
	services, ok := st.RuntimeVars["managedManageServices"].([]any)
	if !ok || len(services) != 2 || services[0] != "firewalld" || services[1] != "ufw" {
		t.Fatalf("expected registered service names, got %#v", st.RuntimeVars["managedManageServices"])
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

func TestWriteSystemdUnitStep(t *testing.T) {
	t.Run("writes unit file with content", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "systemd", "demo.service")
		if err := runWriteSystemdUnit(context.Background(), map[string]any{"path": target, "content": "[Unit]\nDescription=demo"}); err != nil {
			t.Fatalf("runWriteSystemdUnit failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read unit file: %v", err)
		}
		if string(raw) != "[Unit]\nDescription=demo\n" {
			t.Fatalf("unexpected unit content: %q", string(raw))
		}
	})

	t.Run("writes unit file from template content", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "systemd", "templated.service")
		if err := runWriteSystemdUnit(context.Background(), map[string]any{"path": target, "template": "[ManageService]\nExecStart=/usr/bin/true"}); err != nil {
			t.Fatalf("runWriteSystemdUnit failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read unit file: %v", err)
		}
		if string(raw) != "[ManageService]\nExecStart=/usr/bin/true\n" {
			t.Fatalf("unexpected unit template content: %q", string(raw))
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "etc", "systemd", "system", "kubelet.service")
		if err := runWriteSystemdUnit(context.Background(), map[string]any{"path": target, "content": "[Install]"}); err != nil {
			t.Fatalf("runWriteSystemdUnit failed: %v", err)
		}
		if _, err := os.Stat(filepath.Dir(target)); err != nil {
			t.Fatalf("expected parent directory to exist: %v", err)
		}
	})
}

func TestRepoConfigStep(t *testing.T) {
	t.Run("rpm with explicit path", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.repo")
		spec := map[string]any{
			"format": "rpm",
			"path":   target,
			"repositories": []any{map[string]any{
				"id":       "offline-base",
				"name":     "offline-base",
				"baseurl":  "file:///srv/repo",
				"enabled":  true,
				"gpgcheck": false,
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		got := string(raw)
		if !strings.Contains(got, "[offline-base]") || !strings.Contains(got, "baseurl=file:///srv/repo") {
			t.Fatalf("unexpected repo config: %q", got)
		}
	})

	t.Run("deb list rendering", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl":   "http://repo.local/apt/bookworm",
				"trusted":   true,
				"suite":     "./",
				"component": "main",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb [trusted=yes] http://repo.local/apt/bookworm ./ main" {
			t.Fatalf("unexpected apt repo config: %q", string(raw))
		}
	})

	t.Run("auto format uses host family and default path", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "default.list")
		origDetect := repoConfigDetectHostFacts
		origDefaultPath := repoConfigDefaultPathFunc
		t.Cleanup(func() {
			repoConfigDetectHostFacts = origDetect
			repoConfigDefaultPathFunc = origDefaultPath
		})
		repoConfigDetectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "debian"}}
		}
		repoConfigDefaultPathFunc = func(format string) string {
			if format != "deb" {
				t.Fatalf("expected deb format, got %s", format)
			}
			return target
		}

		spec := map[string]any{
			"format": "auto",
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb http://repo.local/apt/bookworm ./" {
			t.Fatalf("unexpected apt auto-rendered config: %q", string(raw))
		}
	})

	t.Run("cleanup and backup paths", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.repo")
		legacyA := filepath.Join(dir, "legacy-a.repo")
		legacyB := filepath.Join(dir, "legacy-b.repo")
		if err := os.WriteFile(legacyA, []byte("[a]\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacyA: %v", err)
		}
		if err := os.WriteFile(legacyB, []byte("[b]\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacyB: %v", err)
		}

		spec := map[string]any{
			"format":       "rpm",
			"path":         target,
			"cleanupPaths": []any{filepath.Join(dir, "legacy-*.repo")},
			"backupPaths":  []any{filepath.Join(dir, "legacy-*.repo")},
			"repositories": []any{map[string]any{
				"id":      "offline",
				"baseurl": "file:///srv/repo",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if _, err := os.Stat(legacyA + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for legacyA: %v", err)
		}
		if _, err := os.Stat(legacyB + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for legacyB: %v", err)
		}
		if _, err := os.Stat(legacyA); !os.IsNotExist(err) {
			t.Fatalf("expected legacyA removed, err=%v", err)
		}
		if _, err := os.Stat(legacyB); !os.IsNotExist(err) {
			t.Fatalf("expected legacyB removed, err=%v", err)
		}
	})

	t.Run("disable existing rpm repositories", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "offline.repo")
		existing := filepath.Join(dir, "legacy.repo")
		if err := os.WriteFile(existing, []byte("[legacy]\nname=legacy\nenabled=1\n"), 0o644); err != nil {
			t.Fatalf("write legacy repo: %v", err)
		}

		spec := map[string]any{
			"format":          "rpm",
			"path":            target,
			"disableExisting": true,
			"backupPaths":     []any{existing},
			"repositories": []any{map[string]any{
				"id":      "offline",
				"baseurl": "file:///srv/repo",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}

		raw, err := os.ReadFile(existing)
		if err != nil {
			t.Fatalf("read legacy repo: %v", err)
		}
		if !strings.Contains(string(raw), "enabled=0") {
			t.Fatalf("expected existing repo to be disabled, got %q", string(raw))
		}
	})

	t.Run("disable existing deb source paths", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "offline.list")
		existing := filepath.Join(dir, "legacy.list")
		if err := os.WriteFile(existing, []byte("deb http://legacy.local stable main\n"), 0o644); err != nil {
			t.Fatalf("write legacy apt source: %v", err)
		}

		spec := map[string]any{
			"format":          "deb",
			"path":            target,
			"disableExisting": true,
			"backupPaths":     []any{existing},
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}

		if _, err := os.Stat(existing + ".deck.bak"); err != nil {
			t.Fatalf("missing backup for existing apt source: %v", err)
		}
		if _, err := os.Stat(existing); !os.IsNotExist(err) {
			t.Fatalf("expected existing apt source removed, err=%v", err)
		}

		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read new apt source: %v", err)
		}
		if strings.TrimSpace(string(raw)) != "deb http://repo.local/apt/bookworm ./" {
			t.Fatalf("unexpected apt repo config: %q", string(raw))
		}
	})

	t.Run("repository configure does not refresh cache inline", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		origRun := repoConfigRunTimedCommand
		t.Cleanup(func() { repoConfigRunTimedCommand = origRun })
		calls := make([]string, 0)
		repoConfigRunTimedCommand = func(_ context.Context, name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			if timeout < time.Second {
				t.Fatalf("unexpected timeout: %s", timeout)
			}
			return nil
		}

		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if len(calls) != 0 {
			t.Fatalf("expected no refresh commands during configure: %#v", calls)
		}
	})

	t.Run("repository configure only writes repo files", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "repo", "offline.list")
		origRun := repoConfigRunTimedCommand
		t.Cleanup(func() { repoConfigRunTimedCommand = origRun })
		calls := make([]string, 0)
		repoConfigRunTimedCommand = func(_ context.Context, name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{
			"format": "deb",
			"path":   target,
			"repositories": []any{map[string]any{
				"baseurl": "http://repo.local/apt/bookworm",
			}},
		}
		if err := runRepoConfig(context.Background(), spec); err != nil {
			t.Fatalf("runRepoConfig failed: %v", err)
		}
		if len(calls) != 0 {
			t.Fatalf("expected no refresh command sequence: %#v", calls)
		}
	})
}

func TestRefreshRepositoryStep(t *testing.T) {
	t.Run("apt clean only", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "apt", "clean": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "apt-get clean" {
			t.Fatalf("unexpected apt clean commands: %#v", calls)
		}
	})

	t.Run("apt clean plus update ordering", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "apt", "clean": true, "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 2 || calls[0] != "apt-get clean" || calls[1] != "apt-get update" {
			t.Fatalf("unexpected apt clean/update sequence: %#v", calls)
		}
	})

	t.Run("dnf clean only", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "dnf", "clean": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf clean all" {
			t.Fatalf("unexpected dnf clean commands: %#v", calls)
		}
	})

	t.Run("dnf update uses makecache behavior", func(t *testing.T) {
		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}

		spec := map[string]any{"manager": "dnf", "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf makecache -y" {
			t.Fatalf("expected makecache call, got %#v", calls)
		}
		for _, call := range calls {
			if strings.Contains(call, "dnf update") || strings.Contains(call, "dnf install") {
				t.Fatalf("dnf update must not perform package upgrade/install, got %q", call)
			}
		}
	})

	t.Run("auto manager resolves using host facts", func(t *testing.T) {
		origDetect := repoConfigDetectHostFacts
		t.Cleanup(func() { repoConfigDetectHostFacts = origDetect })

		calls := make([]string, 0)
		runner := func(name string, args []string, timeout time.Duration) error {
			calls = append(calls, name+" "+strings.Join(args, " "))
			return nil
		}
		repoConfigDetectHostFacts = func() map[string]any {
			return map[string]any{"os": map[string]any{"family": "rhel"}}
		}

		spec := map[string]any{"manager": "auto", "update": true}
		if err := runRefreshRepositoryWithRunner(spec, runner); err != nil {
			t.Fatalf("runRefreshRepository failed: %v", err)
		}

		if len(calls) != 1 || calls[0] != "dnf makecache -y" {
			t.Fatalf("expected auto/rhel to resolve to dnf makecache, got %#v", calls)
		}
	})
}

func TestWriteContainerdConfigStep(t *testing.T) {
	t.Run("updates existing config.toml fields", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "config.toml")
		initial := "version = 2\n[plugins.\"io.containerd.grpc.v1.cri\".registry]\n  config_path = \"\"\n[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc.options]\n  SystemdCgroup = false\n"
		if err := os.WriteFile(target, []byte(initial), 0o644); err != nil {
			t.Fatalf("write initial config: %v", err)
		}

		spec := map[string]any{"path": target, "rawSettings": []any{
			map[string]any{"op": "set", "key": "registry.configPath", "value": "/etc/containerd/certs.d"},
			map[string]any{"op": "set", "key": "runtime.runtimes.runc.options.SystemdCgroup", "value": true},
		}}
		if err := runWriteContainerdConfig(context.Background(), spec); err != nil {
			t.Fatalf("runWriteContainerdConfig failed: %v", err)
		}

		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		got := string(raw)
		if !strings.Contains(got, "/etc/containerd/certs.d") || !strings.Contains(got, "SystemdCgroup = true") {
			t.Fatalf("unexpected config content: %q", got)
		}
	})

	t.Run("writes single registry hosts file under explicit configPath", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "config.toml")
		configPath := filepath.Join(dir, "certs.d")
		if err := os.WriteFile(target, []byte("version = 2\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		spec := map[string]any{
			"path": configPath,
			"registryHosts": []any{
				map[string]any{
					"registry":     "registry.k8s.io",
					"server":       "https://registry.k8s.io",
					"host":         "http://127.0.0.1:5000",
					"capabilities": []any{"pull", "resolve"},
					"skipVerify":   true,
				},
			},
		}

		if err := runWriteContainerdRegistryHosts(spec); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts failed: %v", err)
		}

		hostsPath := filepath.Join(configPath, "registry.k8s.io", "hosts.toml")
		hostsRaw, err := os.ReadFile(hostsPath)
		if err != nil {
			t.Fatalf("read hosts.toml: %v", err)
		}
		expected := "server = \"https://registry.k8s.io\"\n\n[host.\"http://127.0.0.1:5000\"]\n  capabilities = [\"pull\", \"resolve\"]\n  skip_verify = true\n"
		if string(hostsRaw) != expected {
			t.Fatalf("unexpected hosts.toml content: %q", string(hostsRaw))
		}

		if err := runWriteContainerdRegistryHosts(spec); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts second pass failed: %v", err)
		}
		hostsRawAgain, err := os.ReadFile(hostsPath)
		if err != nil {
			t.Fatalf("read hosts.toml second pass: %v", err)
		}
		if string(hostsRawAgain) != expected {
			t.Fatalf("hosts.toml changed unexpectedly on second pass: %q", string(hostsRawAgain))
		}
	})

	t.Run("writes multiple registry hosts files with default configPath", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "containerd", "config.toml")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir config parent: %v", err)
		}
		if err := os.WriteFile(target, []byte("version = 2\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		defaultConfigPath := filepath.Join(filepath.Dir(target), "certs.d")
		firstHostsPath := filepath.Join(defaultConfigPath, "registry.k8s.io", "hosts.toml")
		secondHostsPath := filepath.Join(defaultConfigPath, "docker.io", "hosts.toml")

		spec := map[string]any{
			"path": target,
			"registryHosts": []any{
				map[string]any{
					"registry":     "registry.k8s.io",
					"server":       "https://registry.k8s.io",
					"host":         "http://127.0.0.1:5000",
					"capabilities": []any{"pull", "resolve"},
					"skipVerify":   true,
				},
				map[string]any{
					"registry":     "docker.io",
					"server":       "https://registry-1.docker.io",
					"host":         "http://127.0.0.1:5001",
					"capabilities": []any{"pull"},
					"skipVerify":   false,
				},
			},
		}

		if err := runWriteContainerdRegistryHosts(map[string]any{"path": defaultConfigPath, "registryHosts": spec["registryHosts"]}); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts failed: %v", err)
		}

		firstRaw, err := os.ReadFile(firstHostsPath)
		if err != nil {
			t.Fatalf("read first hosts.toml: %v", err)
		}
		if !strings.Contains(string(firstRaw), "server = \"https://registry.k8s.io\"") {
			t.Fatalf("unexpected first hosts.toml content: %q", string(firstRaw))
		}

		secondRaw, err := os.ReadFile(secondHostsPath)
		if err != nil {
			t.Fatalf("read second hosts.toml: %v", err)
		}
		if !strings.Contains(string(secondRaw), "server = \"https://registry-1.docker.io\"") || !strings.Contains(string(secondRaw), "skip_verify = false") {
			t.Fatalf("unexpected second hosts.toml content: %q", string(secondRaw))
		}

		configRaw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config.toml: %v", err)
		}
		if strings.Contains(string(configRaw), "config_path") {
			t.Fatalf("did not expect config_path to be injected when configPath is omitted: %q", string(configRaw))
		}
	})

	t.Run("applies structured edits for toml yaml and json", func(t *testing.T) {
		dir := t.TempDir()

		tomlPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(tomlPath, []byte("[plugins.\"io.containerd.grpc.v1.cri\".registry]\nconfig_path = \"\"\n"), 0o644); err != nil {
			t.Fatalf("write toml: %v", err)
		}
		if err := runEditTOML(map[string]any{"path": tomlPath, "edits": []any{map[string]any{"op": "set", "rawPath": `plugins."io.containerd.grpc.v1.cri".registry.config_path`, "value": "/etc/containerd/certs.d"}}}); err != nil {
			t.Fatalf("runEditTOML failed: %v", err)
		}
		tomlRaw, err := os.ReadFile(tomlPath)
		if err != nil {
			t.Fatalf("read toml: %v", err)
		}
		if !strings.Contains(string(tomlRaw), "/etc/containerd/certs.d") {
			t.Fatalf("unexpected toml content: %q", string(tomlRaw))
		}

		yamlPath := filepath.Join(dir, "kubeadm.yaml")
		if err := runEditYAML(map[string]any{"path": yamlPath, "createIfMissing": true, "edits": []any{map[string]any{"op": "set", "rawPath": "ClusterConfiguration.imageRepository", "value": "registry.local/k8s"}}}); err != nil {
			t.Fatalf("runEditYAML failed: %v", err)
		}
		yamlRaw, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("read yaml: %v", err)
		}
		if !strings.Contains(string(yamlRaw), "imageRepository: registry.local/k8s") {
			t.Fatalf("unexpected yaml content: %q", string(yamlRaw))
		}

		jsonPath := filepath.Join(dir, "cni.json")
		if err := os.WriteFile(jsonPath, []byte(`{"plugins":[{"type":"loopback"}]}`), 0o644); err != nil {
			t.Fatalf("write json: %v", err)
		}
		if err := runEditJSON(map[string]any{"path": jsonPath, "edits": []any{map[string]any{"op": "set", "rawPath": "plugins.0.type", "value": "bridge"}}}); err != nil {
			t.Fatalf("runEditJSON failed: %v", err)
		}
		jsonRaw, err := os.ReadFile(jsonPath)
		if err != nil {
			t.Fatalf("read json: %v", err)
		}
		if !strings.Contains(string(jsonRaw), `"type": "bridge"`) {
			t.Fatalf("unexpected json content: %q", string(jsonRaw))
		}
	})
}

func TestSwapStep(t *testing.T) {
	dir := t.TempDir()
	fstab := filepath.Join(dir, "fstab")
	content := "UUID=abc / ext4 defaults 0 1\n/swapfile none swap sw 0 0\n"
	if err := os.WriteFile(fstab, []byte(content), 0o644); err != nil {
		t.Fatalf("write fstab: %v", err)
	}
	if err := runSwap(context.Background(), map[string]any{"disable": false, "persist": true, "fstabPath": fstab}); err != nil {
		t.Fatalf("runSwap failed: %v", err)
	}
	raw, err := os.ReadFile(fstab)
	if err != nil {
		t.Fatalf("read fstab: %v", err)
	}
	if !strings.Contains(string(raw), "# /swapfile none swap sw 0 0") {
		t.Fatalf("expected swap line to be commented: %q", string(raw))
	}
}

func TestKernelModuleStep(t *testing.T) {
	dir := t.TempDir()
	persistPath := filepath.Join(dir, "modules-load.d", "k8s.conf")
	spec := map[string]any{"name": "overlay", "load": false, "persist": true, "persistFile": persistPath}
	if err := runKernelModule(context.Background(), spec); err != nil {
		t.Fatalf("runKernelModule failed: %v", err)
	}
	if err := runKernelModule(context.Background(), spec); err != nil {
		t.Fatalf("runKernelModule second pass failed: %v", err)
	}
	raw, err := os.ReadFile(persistPath)
	if err != nil {
		t.Fatalf("read persist file: %v", err)
	}
	if strings.Count(string(raw), "overlay") != 1 {
		t.Fatalf("expected single module line, got %q", string(raw))
	}

	multiPersistPath := filepath.Join(dir, "modules-load.d", "multi.conf")
	multiSpec := map[string]any{"names": []any{"overlay", "br_netfilter", "overlay"}, "load": false, "persist": true, "persistFile": multiPersistPath}
	if err := runKernelModule(context.Background(), multiSpec); err != nil {
		t.Fatalf("runKernelModule multi failed: %v", err)
	}
	multiRaw, err := os.ReadFile(multiPersistPath)
	if err != nil {
		t.Fatalf("read multi persist file: %v", err)
	}
	if strings.Count(string(multiRaw), "overlay") != 1 || strings.Count(string(multiRaw), "br_netfilter") != 1 {
		t.Fatalf("expected deduplicated module lines, got %q", string(multiRaw))
	}
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

func TestRun_RepositoryRequiresExplicitAction(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	target := filepath.Join(dir, "offline.list")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "repo-config",
				Kind: "ConfigureRepository",
				Spec: map[string]any{
					"format":       "deb",
					"path":         target,
					"repositories": []any{map[string]any{"id": "offline", "baseurl": "http://repo.local/debian"}},
				},
			}},
		}},
	}
	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
		t.Fatalf("expected repository step to run, got %v", err)
	}
}

func TestSysctlApplyStep(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(dir, "sysctl.log")
	scriptPath := filepath.Join(binDir, "sysctl")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + logPath + "\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write sysctl script: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	if err := runSysctlApply(context.Background(), map[string]any{"file": "/etc/sysctl.d/99-kubernetes-cri.conf"}); err != nil {
		t.Fatalf("runSysctlApply failed: %v", err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.TrimSpace(string(raw)) != "-p /etc/sysctl.d/99-kubernetes-cri.conf" {
		t.Fatalf("unexpected sysctl args: %q", string(raw))
	}
}
