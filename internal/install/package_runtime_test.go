package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

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
	envMarker := filepath.Join(dir, "apt-env.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" > \"" + marker + "\"\nprintf 'DEBIAN_FRONTEND=%s\nAPT_LISTCHANGES_FRONTEND=%s\nNEEDRESTART_MODE=%s\nNEEDRESTART_SUSPEND=%s\n' \"${DEBIAN_FRONTEND:-}\" \"${APT_LISTCHANGES_FRONTEND:-}\" \"${NEEDRESTART_MODE:-}\" \"${NEEDRESTART_SUSPEND:-}\" > \"" + envMarker + "\"\nexit 0\n"
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
	envRaw, err := os.ReadFile(envMarker)
	if err != nil {
		t.Fatalf("read env marker: %v", err)
	}
	envText := string(envRaw)
	for _, want := range []string{
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
		"NEEDRESTART_MODE=l",
		"NEEDRESTART_SUSPEND=1",
	} {
		if !strings.Contains(envText, want) {
			t.Fatalf("expected apt env %q, got %q", want, envText)
		}
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
	envMarker := filepath.Join(dir, "apt-local-env.txt")
	fakeApt := filepath.Join(binDir, "apt-get")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho \"$*\" > \"" + marker + "\"\nprintf 'DEBIAN_FRONTEND=%s\nAPT_LISTCHANGES_FRONTEND=%s\nNEEDRESTART_MODE=%s\nNEEDRESTART_SUSPEND=%s\n' \"${DEBIAN_FRONTEND:-}\" \"${APT_LISTCHANGES_FRONTEND:-}\" \"${NEEDRESTART_MODE:-}\" \"${NEEDRESTART_SUSPEND:-}\" > \"" + envMarker + "\"\nexit 0\n"
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
	envRaw, err := os.ReadFile(envMarker)
	if err != nil {
		t.Fatalf("read env marker: %v", err)
	}
	envText := string(envRaw)
	for _, want := range []string{
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
		"NEEDRESTART_MODE=l",
		"NEEDRESTART_SUSPEND=1",
	} {
		if !strings.Contains(envText, want) {
			t.Fatalf("expected apt env %q, got %q", want, envText)
		}
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
