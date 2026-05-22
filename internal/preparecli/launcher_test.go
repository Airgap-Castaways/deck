package preparecli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderLauncherScriptUsesStructuredErrors(t *testing.T) {
	script := renderLauncherScript()
	for _, want := range []string{
		`component="launcher"`,
		`printf ' %s="%s"' "$1" "$2" >&2`,
		`log_error unsupported_os os "$os_name"`,
		`log_error unsupported_arch architecture "$arch_name"`,
		`log_error runtime_binary_not_executable path "outputs/bin/$deck_os/$deck_arch/deck"`,
		`log_error runtime_binary_missing os "$deck_os" architecture "$deck_arch" path "outputs/bin/$deck_os/$deck_arch/deck"`,
		`script_dir_part=${script_path%/*}`,
		`-*) script_dir_part=./$script_dir_part ;;`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in launcher script", want)
		}
	}
	for _, forbidden := range []string{` tr `, ` sed `, `dirname`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("launcher script must not require %q: %s", forbidden, script)
		}
	}
}

func TestRenderLauncherScriptRunsWithoutTextUtilities(t *testing.T) {
	unamePath, err := exec.LookPath("uname")
	if err != nil {
		t.Fatalf("find uname: %v", err)
	}
	root := t.TempDir()
	binDir := filepath.Join(root, "outputs", "bin", "linux", "amd64")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir runtime bin dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "outputs", "bin", "linux", "arm64"), 0o755); err != nil {
		t.Fatalf("mkdir arm64 runtime bin dir: %v", err)
	}
	runtimeScript := "#!/bin/sh\nprintf 'runtime:%s\\n' \"$1\"\n"
	for _, arch := range []string{"amd64", "arm64"} {
		path := filepath.Join(root, "outputs", "bin", "linux", arch, "deck")
		if err := os.WriteFile(path, []byte(runtimeScript), 0o755); err != nil {
			t.Fatalf("write runtime binary: %v", err)
		}
	}
	launcherPath := filepath.Join(root, "deck")
	if err := os.WriteFile(launcherPath, []byte(renderLauncherScript()), 0o755); err != nil {
		t.Fatalf("write launcher: %v", err)
	}
	pathDir := filepath.Join(root, "path")
	if err := os.MkdirAll(pathDir, 0o755); err != nil {
		t.Fatalf("mkdir restricted path: %v", err)
	}
	if err := os.Symlink(unamePath, filepath.Join(pathDir, "uname")); err != nil {
		t.Fatalf("symlink uname: %v", err)
	}

	cmd := exec.Command(launcherPath, "version")
	cmd.Env = []string{"PATH=" + pathDir}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed without text utilities: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "runtime:version"; got != want {
		t.Fatalf("launcher output = %q, want %q", got, want)
	}
}
