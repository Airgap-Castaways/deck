package workflowexec

import (
	"os"
	"strings"
	"testing"
)

func TestDetectHostFactsCoversRuntimeHostDefinitions(t *testing.T) {
	facts := DetectHostFacts("linux", "x86_64", func(path string) ([]byte, error) {
		switch path {
		case "/etc/os-release":
			return []byte("ID=ubuntu\nVERSION=22.04.4 LTS\nVERSION_ID=22.04\nID_LIKE=debian\n"), nil
		case "/proc/sys/kernel/osrelease":
			return []byte("6.8.0-test\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	})

	for _, def := range RuntimeHostFieldDefinitions() {
		path := strings.TrimPrefix(def.Path, "runtime.host.")
		if _, ok := valueAtRuntimePath(facts, path); !ok {
			t.Fatalf("missing runtime host field %s", def.Path)
		}
	}
	checks := map[string]any{
		"os.name":        "linux",
		"os.id":          "ubuntu",
		"os.family":      "debian",
		"os.version":     "22.04.4 LTS",
		"os.versionId":   "22.04",
		"os.release":     "22.04",
		"os.idLike":      "debian",
		"arch":           "amd64",
		"kernel.release": "6.8.0-test",
	}
	for path, want := range checks {
		got, ok := valueAtRuntimePath(facts, path)
		if !ok {
			t.Fatalf("missing runtime host field %s", path)
		}
		if got != want {
			t.Fatalf("runtime host field %s = %#v, want %#v", path, got, want)
		}
	}
}

func valueAtRuntimePath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, key := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
