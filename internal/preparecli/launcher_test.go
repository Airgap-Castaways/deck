package preparecli

import (
	"strings"
	"testing"
)

func TestRenderLauncherScriptUsesStructuredErrors(t *testing.T) {
	script := renderLauncherScript()
	for _, want := range []string{
		`component="launcher"`,
		`log_error unsupported_os os "$os_name"`,
		`log_error unsupported_arch architecture "$arch_name"`,
		`log_error runtime_binary_not_executable path "outputs/bin/$deck_os/$deck_arch/deck"`,
		`log_error runtime_binary_missing os "$deck_os" architecture "$deck_arch" path "outputs/bin/$deck_os/$deck_arch/deck"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in launcher script", want)
		}
	}
}
