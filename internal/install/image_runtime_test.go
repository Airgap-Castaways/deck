package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_Image(t *testing.T) {
	t.Run("success", func(t *testing.T) {
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

		fakeList := filepath.Join(dir, "fake-images-list.sh")
		script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'registry.k8s.io/pause:3.10.1\\nregistry.k8s.io/kube-apiserver:v1.30.14\\n'\n"
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
					},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
			t.Fatalf("verify images should succeed: %v", err)
		}
	})

	t.Run("missing image", func(t *testing.T) {
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

		fakeList := filepath.Join(dir, "fake-images-list.sh")
		script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf 'registry.k8s.io/kube-apiserver:v1.30.14\\n'\n"
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
					},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
		if err == nil {
			t.Fatalf("expected missing image error")
		}
		if !errcode.Is(err, errCodeInstallImagesNotFound) {
			t.Fatalf("expected typed code %s, got %v", errCodeInstallImagesNotFound, err)
		}
		if !strings.Contains(err.Error(), "E_INSTALL_VERIFY_IMAGES_NOT_FOUND") {
			t.Fatalf("expected E_INSTALL_VERIFY_IMAGES_NOT_FOUND, got %v", err)
		}
	})
}

func TestRunLoadImageUsesPlatformArchiveSuffix(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	logPath := filepath.Join(dir, "archives.txt")

	err := runLoadImage(context.Background(), bundle, map[string]any{
		"images":    []any{"registry.k8s.io/pause:3.9"},
		"platforms": []any{"linux/amd64", "linux/arm64"},
		"command": []any{
			"sh",
			"-c",
			"printf '%s\n' \"$1\" >> \"$2\"",
			"sh",
			"{archive}",
			logPath,
		},
	})
	if err != nil {
		t.Fatalf("runLoadImage failed: %v", err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read archive log: %v", err)
	}
	want := strings.Join([]string{
		filepath.ToSlash(filepath.Join(bundle, "images", "registry.k8s.io_pause_3.9_linux_amd64.tar")),
		filepath.ToSlash(filepath.Join(bundle, "images", "registry.k8s.io_pause_3.9_linux_arm64.tar")),
	}, "\n") + "\n"
	if string(raw) != want {
		t.Fatalf("unexpected archive paths:\n%s\nwant:\n%s", string(raw), want)
	}
}
