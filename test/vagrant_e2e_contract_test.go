package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibvirtEnvScriptContracts(t *testing.T) {
	root := projectRoot(t)
	scriptPath := filepath.Join(root, "test", "vagrant", "libvirt-env.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("stat libvirt env helper: %v", err)
	}

	cmd := exec.Command("bash", "-lc", "source '"+scriptPath+"'; declare -F prepare_libvirt_environment >/dev/null; test -n \"${DECK_LIBVIRT_POOL_NAME}\"; test -n \"${DECK_LIBVIRT_URI}\"; test -n \"${DECK_LIBVIRT_POOL_PATH}\"; test -n \"${DECK_VAGRANT_HOME}\"")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("libvirt env script contract check failed: %v\n%s", err, string(out))
	}
}

func TestLibvirtEnvScriptDoesNotChangeCallerShellOptions(t *testing.T) {
	root := projectRoot(t)
	scriptPath := filepath.Join(root, "test", "vagrant", "libvirt-env.sh")
	cmd := exec.Command("bash", "-lc", "set +e +u; set +o pipefail; source '"+scriptPath+"'; set -o | grep -E 'errexit|nounset|pipefail'")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("libvirt env sourcing changed caller shell options: %v\n%s", err, string(out))
	}
	got := strings.Fields(string(out))
	if len(got) != 6 {
		t.Fatalf("unexpected shell option output: %q", string(out))
	}
	for i := 0; i < len(got); i += 2 {
		if got[i+1] != "off" {
			t.Fatalf("expected %s to remain off, got %q", got[i], string(out))
		}
	}
}

func TestPrepareLibvirtEnvironmentReturnsInsteadOfExiting(t *testing.T) {
	root := projectRoot(t)
	scriptPath := filepath.Join(root, "test", "vagrant", "libvirt-env.sh")
	stubDir := t.TempDir()
	virshPath := filepath.Join(stubDir, "virsh")
	stub := "#!/usr/bin/env bash\nset -euo pipefail\ncmd=\"${3:-}\"\ncase \"${cmd}\" in\n  pool-info|pool-define|pool-build|pool-start|pool-autostart)\n    exit 0\n    ;;\n  pool-dumpxml)\n    cat <<'EOF'\n<pool type='dir'>\n  <name>deck</name>\n  <target>\n    <path>/tmp/not-the-expected-path</path>\n  </target>\n</pool>\nEOF\n    ;;\n  *)\n    exit 0\n    ;;\nesac\n"
	if err := os.WriteFile(virshPath, []byte(stub), 0o755); err != nil {
		t.Fatalf("write virsh stub: %v", err)
	}
	cmd := exec.Command("bash", "-lc", "PATH='"+stubDir+":/usr/bin:/bin'; source '"+scriptPath+"'; prepare_libvirt_environment; rc=$?; printf 'rc=%s\nafter\n' \"${rc}\"")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare_libvirt_environment should return to caller: %v\n%s", err, string(out))
	}
	got := string(out)
	if !strings.Contains(got, "[deck] libvirt pool path mismatch:") {
		t.Fatalf("expected libvirt pool mismatch, got %q", got)
	}
	if !strings.Contains(got, "rc=1") || !strings.Contains(got, "after") {
		t.Fatalf("expected function return without shell exit, got %q", got)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, ".."))
}
