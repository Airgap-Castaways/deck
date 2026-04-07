package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildServerDaemonArgsUsesWorkingDirectoryProperty(t *testing.T) {
	args := buildServerDaemonArgs("deck-server", "/tmp/deck", "/tmp/current/dir", serverUpOptions{
		root:          ".",
		addr:          ":8080",
		auditMaxSize:  50,
		auditMaxFiles: 10,
	})
	if got, want := args[:10], []string{
		"--unit", "deck-server",
		"--property", "WorkingDirectory=/tmp/current/dir",
		"--service-type=simple",
		"/tmp/deck",
		"server", "up",
		"--root", ".",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected daemon args prefix:\n got: %#v\nwant: %#v", got, want)
	}
	if strings.Contains(strings.Join(args, " "), "WorkingEnsureDirectory=") {
		t.Fatalf("unexpected WorkingEnsureDirectory property in %#v", args)
	}
}

func TestBuildServerDaemonArgsEscapesWorkingDirectoryPercents(t *testing.T) {
	args := buildServerDaemonArgs("deck-server", "/tmp/deck", "/tmp/100%/cwd", serverUpOptions{
		root:          ".",
		addr:          ":8080",
		auditMaxSize:  50,
		auditMaxFiles: 10,
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "WorkingDirectory=/tmp/100%%/cwd") {
		t.Fatalf("expected escaped percent in args, got %#v", args)
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
