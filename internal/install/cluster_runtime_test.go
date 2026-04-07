package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_UpgradeAndClusterChecks(t *testing.T) {
	kubeadm := useStubUpgradeKubeadm()
	useStubCheckKubernetesCluster(t)

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state", "state.json")
	reportDir := filepath.Join(dir, "reports")
	wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "install", Steps: []config.Step{
		{ID: "upgrade", Kind: "UpgradeKubeadm", Spec: map[string]any{"kubernetesVersion": "v1.31.0"}},
		{ID: "check", Kind: "CheckKubernetesCluster", Spec: map[string]any{"reports": map[string]any{"nodesPath": filepath.Join(reportDir, "nodes.txt")}, "versions": map[string]any{"reportPath": filepath.Join(reportDir, "version.txt")}}},
	}}}}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, kubeadm: kubeadm}); err != nil {
		t.Fatalf("expected typed kubeadm upgrade and cluster check success, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(reportDir, "nodes.txt")); err != nil {
		t.Fatalf("expected nodes report, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(reportDir, "version.txt")); err != nil {
		t.Fatalf("expected version report, got %v", err)
	}
}

func TestRun_KubeadmMissingFileErrorCode(t *testing.T) {
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
				ID:   "join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{"joinFile": filepath.Join(dir, "missing-join.txt")},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected kubeadm join missing file error")
	}
	if !errcode.Is(err, errCodeInstallJoinFileMissing) {
		t.Fatalf("expected typed code %s, got %v", errCodeInstallJoinFileMissing, err)
	}
	if !strings.Contains(err.Error(), "E_INSTALL_KUBEADM_JOIN_FILE_NOT_FOUND") {
		t.Fatalf("expected E_INSTALL_KUBEADM_JOIN_FILE_NOT_FOUND, got %v", err)
	}
}

func TestRun_JoinKubeadmRequiresExistingJoinFile(t *testing.T) {
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
				ID:   "join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{"joinFile": filepath.Join(dir, "join.txt")},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected join file error")
	}
	if !strings.Contains(err.Error(), errCodeInstallJoinFileMissing) {
		t.Fatalf("expected kubeadm join file missing error, got %v", err)
	}
}

func TestRun_KubeadmRealMode(t *testing.T) {
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
	fakeKubeadmPath := filepath.Join(binDir, "kubeadm")
	fakeScript := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"init\" ]]; then\n  exit 0\nfi\nif [[ \"${1:-}\" == \"token\" && \"${2:-}\" == \"create\" ]]; then\n  echo \"W0000 warning: no default routes found\"\n  echo \"kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\"\n  exit 0\nfi\nif [[ \"${1:-}\" == \"join\" ]]; then\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(fakeKubeadmPath, []byte(fakeScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	joinPath := filepath.Join(dir, "join.txt")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{
				{ID: "kubeadm-init", Kind: "InitKubeadm", Spec: map[string]any{"outputJoinFile": joinPath}},
				{ID: "kubeadm-join", Kind: "JoinKubeadm", Spec: map[string]any{"joinFile": joinPath}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm mode run failed: %v", err)
	}

	joinRaw, err := os.ReadFile(joinPath)
	if err != nil {
		t.Fatalf("read join file: %v", err)
	}
	if got := string(joinRaw); got != "kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\n" {
		t.Fatalf("expected sanitized join file, got %q", got)
	}
}

func TestRun_KubeadmRealModeAcceptsJoinFileWithWarnings(t *testing.T) {
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
	fakeKubeadmPath := filepath.Join(binDir, "kubeadm")
	fakeScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" > \"" + filepath.Join(dir, "join-args.log") + "\"\nif [[ \"${1:-}\" == \"join\" ]]; then\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(fakeKubeadmPath, []byte(fakeScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	joinPath := filepath.Join(dir, "join.txt")
	joinLog := filepath.Join(dir, "join-args.log")
	joinRaw := "W0000 warning: no default routes found\nkubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\n"
	if err := os.WriteFile(joinPath, []byte(joinRaw), 0o644); err != nil {
		t.Fatalf("write join file: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{"joinFile": joinPath},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm join run failed: %v", err)
	}

	argsRaw, err := os.ReadFile(joinLog)
	if err != nil {
		t.Fatalf("read join args log: %v", err)
	}
	if got := strings.TrimSpace(string(argsRaw)); got != "join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake" {
		t.Fatalf("unexpected kubeadm join args: %q", got)
	}
}

func TestRun_KubeadmRealModeAcceptsWrappedJoinFileWithWarnings(t *testing.T) {
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
	fakeKubeadmPath := filepath.Join(binDir, "kubeadm")
	fakeScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" > \"" + filepath.Join(dir, "join-args.log") + "\"\nif [[ \"${1:-}\" == \"join\" ]]; then\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(fakeKubeadmPath, []byte(fakeScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, originalPath))

	joinPath := filepath.Join(dir, "join.txt")
	joinLog := filepath.Join(dir, "join-args.log")
	joinRaw := "W0000 warning: no default routes found\nkubeadm join 10.1.0.10:6443 \\\n  --token fake.token \\\n  --discovery-token-ca-cert-hash sha256:fake\n"
	if err := os.WriteFile(joinPath, []byte(joinRaw), 0o644); err != nil {
		t.Fatalf("write join file: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{"joinFile": joinPath},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("wrapped kubeadm join run failed: %v", err)
	}

	argsRaw, err := os.ReadFile(joinLog)
	if err != nil {
		t.Fatalf("read join args log: %v", err)
	}
	if got := strings.TrimSpace(string(argsRaw)); got != "join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake" {
		t.Fatalf("unexpected kubeadm join args: %q", got)
	}
}

func TestRun_KubeadmRealModeRejectsInvalidCommand(t *testing.T) {
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
	if err := os.WriteFile(joinPath, []byte("echo not-kubeadm\n"), 0o644); err != nil {
		t.Fatalf("write join file: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{"joinFile": joinPath},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected invalid join command error")
	}
	if !errcode.Is(err, errCodeInstallJoinCmdInvalid) {
		t.Fatalf("expected typed code %s, got %v", errCodeInstallJoinCmdInvalid, err)
	}
	if !strings.Contains(err.Error(), "E_INSTALL_KUBEADM_JOIN_COMMAND_INVALID") {
		t.Fatalf("expected E_INSTALL_KUBEADM_JOIN_COMMAND_INVALID, got %v", err)
	}
}

func TestRun_KubeadmRealModeSupportsJoinConfigFile(t *testing.T) {
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
	kubeadmLog := filepath.Join(dir, "kubeadm.log")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nif [[ \"${1:-}\" == \"join\" ]]; then\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	configPath := filepath.Join(dir, "kubeadm-join.yaml")
	if err := os.WriteFile(configPath, []byte("apiVersion: kubeadm.k8s.io/v1beta3\nkind: JoinConfiguration\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{
					"mode":           "real",
					"configFile":     configPath,
					"asControlPlane": true,
					"extraArgs":      []any{"--skip-phases=preflight"},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm join config run failed: %v", err)
	}

	logRaw, err := os.ReadFile(kubeadmLog)
	if err != nil {
		t.Fatalf("read kubeadm log: %v", err)
	}
	if got := strings.TrimSpace(string(logRaw)); got != "join --config "+configPath+" --control-plane --skip-phases=preflight" {
		t.Fatalf("unexpected kubeadm join args: %q", got)
	}
}

func TestRun_KubeadmRealModeSupportsAsControlPlaneJoinFile(t *testing.T) {
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
	kubeadmLog := filepath.Join(dir, "kubeadm.log")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nif [[ \"${1:-}\" == \"join\" ]]; then\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	joinPath := filepath.Join(dir, "join.txt")
	if err := os.WriteFile(joinPath, []byte("kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\n"), 0o644); err != nil {
		t.Fatalf("write join file: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{
					"mode":           "real",
					"joinFile":       joinPath,
					"asControlPlane": true,
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm join control-plane run failed: %v", err)
	}

	logRaw, err := os.ReadFile(kubeadmLog)
	if err != nil {
		t.Fatalf("read kubeadm log: %v", err)
	}
	if got := strings.TrimSpace(string(logRaw)); got != "join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake --control-plane" {
		t.Fatalf("unexpected kubeadm join args: %q", got)
	}
}

func TestRun_JoinKubeadmRejectsConflictingInputs(t *testing.T) {
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
	if err := os.WriteFile(joinPath, []byte("kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\n"), 0o644); err != nil {
		t.Fatalf("write join file: %v", err)
	}
	configPath := filepath.Join(dir, "kubeadm-join.yaml")
	if err := os.WriteFile(configPath, []byte("apiVersion: kubeadm.k8s.io/v1beta3\nkind: JoinConfiguration\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-join",
				Kind: "JoinKubeadm",
				Spec: map[string]any{
					"mode":       "real",
					"joinFile":   joinPath,
					"configFile": configPath,
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected conflicting join inputs error")
	}
	if !errcode.Is(err, errCodeInstallJoinInputConflict) {
		t.Fatalf("expected typed code %s, got %v", errCodeInstallJoinInputConflict, err)
	}
	if !strings.Contains(err.Error(), "E_INSTALL_KUBEADM_JOIN_INPUT_CONFLICT") {
		t.Fatalf("expected E_INSTALL_KUBEADM_JOIN_INPUT_CONFLICT, got %v", err)
	}
}

func TestRun_KubeadmRealModeSupportsImagePullAndConfigWrite(t *testing.T) {
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
	kubeadmLog := filepath.Join(dir, "kubeadm.log")
	fakeKubeadmPath := filepath.Join(binDir, "kubeadm")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nif [[ \"${1:-}\" == \"init\" ]]; then\n  exit 0\nfi\nif [[ \"${1:-}\" == \"token\" && \"${2:-}\" == \"create\" ]]; then\n  echo \"kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\"\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(fakeKubeadmPath, []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	fakeIPPath := filepath.Join(binDir, "ip")
	fakeIPScript := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"-4 route get 1.1.1.1\" ]]; then\n  echo \"1.1.1.1 via 10.0.2.2 dev eth0 src 10.20.30.40 uid 0\"\n  exit 0\nfi\nif [[ \"$*\" == \"-4 -o addr show scope global\" ]]; then\n  echo \"2: eth0    inet 10.20.30.40/24 brd 10.20.30.255 scope global dynamic eth0\"\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeIPPath, []byte(fakeIPScript), 0o755); err != nil {
		t.Fatalf("write fake ip: %v", err)
	}

	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	joinPath := filepath.Join(dir, "join.txt")
	configPath := filepath.Join(dir, "kubeadm-init.yaml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-init",
				Kind: "InitKubeadm",
				Spec: map[string]any{
					"mode":                  "real",
					"outputJoinFile":        joinPath,
					"configFile":            configPath,
					"configTemplate":        "default",
					"kubernetesVersion":     "v1.30.14",
					"podNetworkCIDR":        "10.244.0.0/16",
					"criSocket":             "unix:///run/containerd/containerd.sock",
					"ignorePreflightErrors": []any{"swap"},
					"extraArgs":             []any{"--skip-phases=addon/kube-proxy"},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm mode run failed: %v", err)
	}

	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	configText := string(configRaw)
	if !strings.Contains(configText, "advertiseAddress: 10.20.30.40") {
		t.Fatalf("expected detected advertise address in config, got %q", configText)
	}
	if !strings.Contains(configText, "kubernetesVersion: v1.30.14") {
		t.Fatalf("expected kubernetes version in config, got %q", configText)
	}
	if !strings.Contains(configText, "podSubnet: 10.244.0.0/16") {
		t.Fatalf("expected pod subnet in config, got %q", configText)
	}
	if !strings.Contains(configText, "criSocket: unix:///run/containerd/containerd.sock") {
		t.Fatalf("expected cri socket in config, got %q", configText)
	}

	logRaw, err := os.ReadFile(kubeadmLog)
	if err != nil {
		t.Fatalf("read kubeadm log: %v", err)
	}
	logText := string(logRaw)
	if !strings.Contains(logText, "init --config "+configPath+" --ignore-preflight-errors swap --skip-phases=addon/kube-proxy") {
		t.Fatalf("expected kubeadm init args with config file only, got %q", logText)
	}
}

func TestRun_InitKubeadmSkipsWhenAdminConfExists(t *testing.T) {
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
	kubeadmLog := filepath.Join(dir, "kubeadm.log")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

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
				ID:   "kubeadm-init",
				Kind: "InitKubeadm",
				Spec: map[string]any{
					"mode":           "real",
					"outputJoinFile": joinPath,
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected init skip success, got %v", err)
	}
	if _, err := os.Stat(joinPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no join file to be written on skip, got err=%v", err)
	}
	if _, err := os.Stat(kubeadmLog); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected kubeadm not to run on skip, got err=%v", err)
	}
}

func TestRun_InitKubeadmRunsWhenSkipDisabledAndAdminConfExists(t *testing.T) {
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
	kubeadmLog := filepath.Join(dir, "kubeadm.log")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nif [[ \"${1:-}\" == \"init\" ]]; then\n  exit 0\nfi\nif [[ \"${1:-}\" == \"token\" && \"${2:-}\" == \"create\" ]]; then\n  echo \"kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\"\n  exit 0\nfi\necho \"unsupported kubeadm invocation\" >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

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
				ID:   "kubeadm-init",
				Kind: "InitKubeadm",
				Spec: map[string]any{
					"mode":                  "real",
					"outputJoinFile":        joinPath,
					"skipIfAdminConfExists": false,
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("expected init to run when skip disabled, got %v", err)
	}
	joinRaw, err := os.ReadFile(joinPath)
	if err != nil {
		t.Fatalf("read join file: %v", err)
	}
	if !strings.Contains(string(joinRaw), "kubeadm join") {
		t.Fatalf("expected join file output, got %q", string(joinRaw))
	}
	logRaw, err := os.ReadFile(kubeadmLog)
	if err != nil {
		t.Fatalf("read kubeadm log: %v", err)
	}
	if !strings.Contains(string(logRaw), "init") {
		t.Fatalf("expected kubeadm init to run, got %q", string(logRaw))
	}
}

func TestRun_KubeadmAdvertiseAddressDetectionFallback(t *testing.T) {
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
	fakeKubeadmPath := filepath.Join(binDir, "kubeadm")
	fakeKubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"init\" ]]; then\n  exit 0\nfi\nif [[ \"${1:-}\" == \"token\" && \"${2:-}\" == \"create\" ]]; then\n  echo \"kubeadm join 10.1.0.10:6443 --token fake.token --discovery-token-ca-cert-hash sha256:fake\"\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeKubeadmPath, []byte(fakeKubeadmScript), 0o755); err != nil {
		t.Fatalf("write fake kubeadm: %v", err)
	}
	fakeIPPath := filepath.Join(binDir, "ip")
	fakeIPScript := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$*\" == \"-4 route get 1.1.1.1\" ]]; then\n  exit 1\nfi\nif [[ \"$*\" == \"-4 -o addr show scope global\" ]]; then\n  echo \"2: eth0    inet 172.16.0.25/24 brd 172.16.0.255 scope global dynamic eth0\"\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeIPPath, []byte(fakeIPScript), 0o755); err != nil {
		t.Fatalf("write fake ip: %v", err)
	}

	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	joinPath := filepath.Join(dir, "join.txt")
	configPath := filepath.Join(dir, "kubeadm-init.yaml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "install",
			Steps: []config.Step{{
				ID:   "kubeadm-init",
				Kind: "InitKubeadm",
				Spec: map[string]any{
					"mode":              "real",
					"outputJoinFile":    joinPath,
					"configFile":        configPath,
					"configTemplate":    "default",
					"kubernetesVersion": "v1.30.14",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath}); err != nil {
		t.Fatalf("real kubeadm mode run failed: %v", err)
	}

	configRaw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if !strings.Contains(string(configRaw), "advertiseAddress: 172.16.0.25") {
		t.Fatalf("expected fallback global IPv4 advertise address, got %q", string(configRaw))
	}
}

func TestRun_Kubeadm(t *testing.T) {
	t.Run("runs reset command and cleanup actions", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		binDir := filepath.Join(dir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}

		kubeadmLog := filepath.Join(dir, "kubeadm.log")
		systemctlLog := filepath.Join(dir, "systemctl.log")
		crictlLog := filepath.Join(dir, "crictl.log")

		kubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(kubeadmScript), 0o755); err != nil {
			t.Fatalf("write kubeadm script: %v", err)
		}
		systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + systemctlLog + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
			t.Fatalf("write systemctl script: %v", err)
		}
		crictlScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + crictlLog + "\"\nif [[ \"$*\" == *\"ps -a --name kube-apiserver -q\"* ]]; then\n  echo cid-apiserver\n  exit 0\nfi\nif [[ \"$*\" == *\"ps -a --name etcd -q\"* ]]; then\n  echo cid-etcd\n  exit 0\nfi\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "crictl"), []byte(crictlScript), 0o755); err != nil {
			t.Fatalf("write crictl script: %v", err)
		}
		t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

		removeDir := filepath.Join(dir, "remove-dir")
		removeFile := filepath.Join(dir, "remove-file.conf")
		if err := os.MkdirAll(removeDir, 0o755); err != nil {
			t.Fatalf("mkdir remove dir: %v", err)
		}
		if err := os.WriteFile(removeFile, []byte("stale"), 0o644); err != nil {
			t.Fatalf("write remove file: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "reset",
					Kind: "ResetKubeadm",
					Spec: map[string]any{
						"mode":                  "real",
						"force":                 true,
						"criSocket":             "unix:///run/containerd/containerd.sock",
						"removePaths":           []any{removeDir},
						"removeFiles":           []any{removeFile},
						"cleanupContainers":     []any{"kube-apiserver", "etcd"},
						"restartRuntimeService": "containerd",
					},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("kubeadm reset run failed: %v", err)
		}

		if _, err := os.Stat(removeDir); !os.IsNotExist(err) {
			t.Fatalf("expected remove dir deleted, err=%v", err)
		}
		if _, err := os.Stat(removeFile); !os.IsNotExist(err) {
			t.Fatalf("expected remove file deleted, err=%v", err)
		}

		kubeadmRaw, err := os.ReadFile(kubeadmLog)
		if err != nil {
			t.Fatalf("read kubeadm log: %v", err)
		}
		kubeadmArgs := strings.TrimSpace(string(kubeadmRaw))
		if !strings.Contains(kubeadmArgs, "reset --force --cri-socket unix:///run/containerd/containerd.sock") {
			t.Fatalf("unexpected kubeadm args: %q", kubeadmArgs)
		}

		systemctlRaw, err := os.ReadFile(systemctlLog)
		if err != nil {
			t.Fatalf("read systemctl log: %v", err)
		}
		systemctlLogText := string(systemctlRaw)
		if !strings.Contains(systemctlLogText, "stop kubelet") {
			t.Fatalf("expected kubelet stop command, got %q", systemctlLogText)
		}
		if !strings.Contains(systemctlLogText, "restart containerd") {
			t.Fatalf("expected runtime restart command, got %q", systemctlLogText)
		}

		crictlRaw, err := os.ReadFile(crictlLog)
		if err != nil {
			t.Fatalf("read crictl log: %v", err)
		}
		crictlLogText := string(crictlRaw)
		if !strings.Contains(crictlLogText, "ps -a --name kube-apiserver -q") || !strings.Contains(crictlLogText, "ps -a --name etcd -q") {
			t.Fatalf("expected crictl ps cleanup lookups, got %q", crictlLogText)
		}
		if !strings.Contains(crictlLogText, "rm -f cid-apiserver") || !strings.Contains(crictlLogText, "rm -f cid-etcd") {
			t.Fatalf("expected crictl rm cleanup calls, got %q", crictlLogText)
		}
	})

	t.Run("ignoreErrors tolerates kubeadm failure and continues cleanup", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		binDir := filepath.Join(dir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}

		systemctlLog := filepath.Join(dir, "systemctl.log")
		kubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nexit 1\n"
		if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(kubeadmScript), 0o755); err != nil {
			t.Fatalf("write kubeadm script: %v", err)
		}
		systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + systemctlLog + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
			t.Fatalf("write systemctl script: %v", err)
		}
		crictlScript := "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "crictl"), []byte(crictlScript), 0o755); err != nil {
			t.Fatalf("write crictl script: %v", err)
		}
		t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

		removeDir := filepath.Join(dir, "remove-dir")
		removeFile := filepath.Join(dir, "remove-file.conf")
		if err := os.MkdirAll(removeDir, 0o755); err != nil {
			t.Fatalf("mkdir remove dir: %v", err)
		}
		if err := os.WriteFile(removeFile, []byte("stale"), 0o644); err != nil {
			t.Fatalf("write remove file: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "reset",
					Kind: "ResetKubeadm",
					Spec: map[string]any{
						"mode":                  "real",
						"ignoreErrors":          true,
						"removePaths":           []any{removeDir},
						"removeFiles":           []any{removeFile},
						"restartRuntimeService": "containerd",
					},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("expected ignoreErrors success, got %v", err)
		}

		if _, err := os.Stat(removeDir); !os.IsNotExist(err) {
			t.Fatalf("expected remove dir deleted, err=%v", err)
		}
		if _, err := os.Stat(removeFile); !os.IsNotExist(err) {
			t.Fatalf("expected remove file deleted, err=%v", err)
		}

		systemctlRaw, err := os.ReadFile(systemctlLog)
		if err != nil {
			t.Fatalf("read systemctl log: %v", err)
		}
		systemctlLogText := string(systemctlRaw)
		if !strings.Contains(systemctlLogText, "restart containerd") {
			t.Fatalf("expected runtime restart command, got %q", systemctlLogText)
		}
	})

	t.Run("real mode passes extra args to kubeadm reset", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		binDir := filepath.Join(dir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}

		kubeadmLog := filepath.Join(dir, "kubeadm.log")
		kubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(kubeadmScript), 0o755); err != nil {
			t.Fatalf("write kubeadm script: %v", err)
		}
		systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
			t.Fatalf("write systemctl script: %v", err)
		}
		crictlScript := "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"
		if err := os.WriteFile(filepath.Join(binDir, "crictl"), []byte(crictlScript), 0o755); err != nil {
			t.Fatalf("write crictl script: %v", err)
		}
		t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "reset",
					Kind: "ResetKubeadm",
					Spec: map[string]any{
						"mode":      "real",
						"force":     true,
						"extraArgs": []any{"--cleanup-tmp-dir"},
					},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath}); err != nil {
			t.Fatalf("reset run failed: %v", err)
		}

		raw, err := os.ReadFile(kubeadmLog)
		if err != nil {
			t.Fatalf("read kubeadm log: %v", err)
		}
		if got := strings.TrimSpace(string(raw)); got != "reset --force --cleanup-tmp-dir" {
			t.Fatalf("unexpected kubeadm args: %q", got)
		}
	})

	t.Run("stub mode skips reset side effects", func(t *testing.T) {
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state", "state.json")
		binDir := filepath.Join(dir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("mkdir bin: %v", err)
		}

		kubeadmLog := filepath.Join(dir, "kubeadm.log")
		systemctlLog := filepath.Join(dir, "systemctl.log")
		crictlLog := filepath.Join(dir, "crictl.log")
		kubeadmScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + kubeadmLog + "\"\nexit 1\n"
		if err := os.WriteFile(filepath.Join(binDir, "kubeadm"), []byte(kubeadmScript), 0o755); err != nil {
			t.Fatalf("write kubeadm script: %v", err)
		}
		systemctlScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + systemctlLog + "\"\nexit 1\n"
		if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
			t.Fatalf("write systemctl script: %v", err)
		}
		crictlScript := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"" + crictlLog + "\"\nexit 1\n"
		if err := os.WriteFile(filepath.Join(binDir, "crictl"), []byte(crictlScript), 0o755); err != nil {
			t.Fatalf("write crictl script: %v", err)
		}
		t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

		removeDir := filepath.Join(dir, "remove-dir")
		removeFile := filepath.Join(dir, "remove-file.conf")
		if err := os.MkdirAll(removeDir, 0o755); err != nil {
			t.Fatalf("mkdir remove dir: %v", err)
		}
		if err := os.WriteFile(removeFile, []byte("stale"), 0o644); err != nil {
			t.Fatalf("write remove file: %v", err)
		}

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "install",
				Steps: []config.Step{{
					ID:   "reset",
					Kind: "ResetKubeadm",
					Spec: map[string]any{
						"force":                 true,
						"criSocket":             "unix:///run/containerd/containerd.sock",
						"extraArgs":             []any{"--cleanup-tmp-dir"},
						"removePaths":           []any{removeDir},
						"removeFiles":           []any{removeFile},
						"cleanupContainers":     []any{"kube-apiserver"},
						"restartRuntimeService": "containerd",
					},
				}},
			}},
		}
		kubeadm := useStubResetKubeadm()

		if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, kubeadm: kubeadm}); err != nil {
			t.Fatalf("expected stub reset success, got %v", err)
		}

		if _, err := os.Stat(kubeadmLog); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected kubeadm not to run in stub mode, err=%v", err)
		}
		if _, err := os.Stat(systemctlLog); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected systemctl not to run in stub mode, err=%v", err)
		}
		if _, err := os.Stat(crictlLog); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected crictl not to run in stub mode, err=%v", err)
		}
		if _, err := os.Stat(removeDir); err != nil {
			t.Fatalf("expected remove dir to remain in stub mode, err=%v", err)
		}
		if _, err := os.Stat(removeFile); err != nil {
			t.Fatalf("expected remove file to remain in stub mode, err=%v", err)
		}
	})
}
