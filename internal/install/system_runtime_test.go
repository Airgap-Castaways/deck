package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/errcode"
)

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

func TestSwapStep(t *testing.T) {
	dir := t.TempDir()
	fstab := filepath.Join(dir, "fstab")
	content := "UUID=abc / ext4 defaults 0 1\n/swapfile none swap sw 0 0\n"
	if err := os.WriteFile(fstab, []byte(content), 0o644); err != nil {
		t.Fatalf("write fstab: %v", err)
	}
	if err := runSwap(context.Background(), map[string]any{"disable": true, "persist": true, "fstabPath": fstab}); err != nil {
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
