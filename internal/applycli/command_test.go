package applycli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestClearSelectedApplyStateIgnoresUnresolvedFallbackPaths(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_STATE_HOME", "")

	statePath := filepath.Join(t.TempDir(), "selected.json")
	if err := os.WriteFile(statePath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write selected state: %v", err)
	}

	err := clearSelectedApplyState(ExecutionRequest{
		Workflow:  &config.Workflow{StateKey: "selected"},
		StatePath: statePath,
	})
	if err != nil {
		t.Fatalf("clear selected state: %v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected selected state to be cleared, stat err: %v", err)
	}
}
