package test

import (
	"os"
	"os/exec"
	"path/filepath"
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

func projectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, ".."))
}
