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

func TestManageServiceStep(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(dir, "systemctl.log")
	scriptPath := filepath.Join(binDir, "systemctl")
	script, err := os.ReadFile(filepath.Join("testdata", "systemctl_manage_service.sh"))
	if err != nil {
		t.Fatalf("read systemctl test script: %v", err)
	}
	rendered := strings.ReplaceAll(string(script), "__LOG_PATH__", logPath)
	//nolint:gosec // scriptPath stays under t.TempDir() for this test-only fake systemctl binary.
	if err := os.WriteFile(scriptPath, []byte(rendered), 0o755); err != nil {
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
