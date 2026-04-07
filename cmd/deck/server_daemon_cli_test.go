package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerUpDaemonUsesWorkingDirectoryProperty(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "systemd-run.log")
	systemdRunScript := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$*\" > \"" + logPath + "\"\nprintf 'Running as unit: deck-server.service\\n'\n"
	if err := os.WriteFile(filepath.Join(binDir, "systemd-run"), []byte(systemdRunScript), 0o755); err != nil {
		t.Fatalf("write systemd-run script: %v", err)
	}
	systemctlScript := "#!/bin/sh\nset -eu\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write systemctl script: %v", err)
	}
	t.Setenv("PATH", binDir)

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	res := execute([]string{"server", "up", "--root", ".", "--addr", ":8080", "--daemon", "--unit", "deck-server"})
	if res.err != nil {
		t.Fatalf("expected daemon startup success, got %v stderr=%q", res.err, res.stderr)
	}
	if !strings.Contains(res.stdout, "server up: ok (deck-server.service)") {
		t.Fatalf("expected success output, got %q", res.stdout)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read systemd-run log: %v", err)
	}
	args := string(raw)
	if !strings.Contains(args, "WorkingDirectory="+root) {
		t.Fatalf("expected WorkingDirectory property, got %q", args)
	}
	if strings.Contains(args, "WorkingEnsureDirectory=") {
		t.Fatalf("unexpected WorkingEnsureDirectory property in %q", args)
	}
	if !strings.Contains(args, " server up --root . --addr :8080 ") {
		t.Fatalf("expected server up daemon command arguments, got %q", args)
	}
}

func TestServerUpDaemonReportsSystemdRunPropertyFailures(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	systemdRunScript := "#!/bin/sh\nset -eu\nprintf 'Unknown assignment: WorkingDirectory\\n' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "systemd-run"), []byte(systemdRunScript), 0o755); err != nil {
		t.Fatalf("write systemd-run script: %v", err)
	}
	systemctlScript := "#!/bin/sh\nset -eu\nexit 0\n"
	if err := os.WriteFile(filepath.Join(binDir, "systemctl"), []byte(systemctlScript), 0o755); err != nil {
		t.Fatalf("write systemctl script: %v", err)
	}
	t.Setenv("PATH", binDir)

	res := execute([]string{"server", "up", "--root", root, "--addr", ":8080", "--daemon", "--unit", "deck-server"})
	if res.err == nil {
		t.Fatalf("expected daemon startup error")
	}
	if !strings.Contains(res.err.Error(), "server up: Unknown assignment: WorkingDirectory") {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if !strings.Contains(res.stderr, "component=cli event=command_failed") {
		t.Fatalf("expected CLI failure event, got %q", res.stderr)
	}
}
