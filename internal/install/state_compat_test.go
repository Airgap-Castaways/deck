package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestResolveStateReadPathMigratesLegacyHomeStateFile(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Chdir(workspace)

	wf := &config.Workflow{StateKey: "legacy-state-key"}
	preferred, err := DefaultStatePath(wf)
	if err != nil {
		t.Fatalf("DefaultStatePath failed: %v", err)
	}
	legacyPath, err := LegacyStatePath(wf)
	if err != nil {
		t.Fatalf("LegacyStatePath failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy state dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("{\n  \"completedSteps\": [\"s1\"]\n}\n"), 0o600); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	resolved, err := ResolveStateReadPathForWorkflow(wf, preferred)
	if err != nil {
		t.Fatalf("ResolveStateReadPathForWorkflow failed: %v", err)
	}
	if resolved != preferred {
		t.Fatalf("expected migrated state path, got %q want %q", resolved, preferred)
	}
	if _, err := os.Stat(preferred); err != nil {
		t.Fatalf("migrated state missing: %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy state should not be deleted: %v", err)
	}
}

func TestResolveStateReadPathMigratesOldXDGBeforeLegacy(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Chdir(workspace)

	wf := &config.Workflow{StateKey: "xdg-state-key"}
	preferred, err := DefaultStatePath(wf)
	if err != nil {
		t.Fatalf("DefaultStatePath failed: %v", err)
	}
	xdgPath, err := XDGStatePath(wf)
	if err != nil {
		t.Fatalf("XDGStatePath failed: %v", err)
	}
	legacyPath, err := LegacyStatePath(wf)
	if err != nil {
		t.Fatalf("LegacyStatePath failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatalf("mkdir XDG state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy state dir: %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("{\"phase\":\"xdg\"}"), 0o600); err != nil {
		t.Fatalf("write XDG state: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("{\"phase\":\"legacy\"}"), 0o600); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	resolved, err := ResolveStateReadPathForWorkflow(wf, preferred)
	if err != nil {
		t.Fatalf("ResolveStateReadPathForWorkflow failed: %v", err)
	}
	if resolved != preferred {
		t.Fatalf("expected migrated state path, got %q want %q", resolved, preferred)
	}
	raw, err := os.ReadFile(preferred)
	if err != nil {
		t.Fatalf("read migrated state: %v", err)
	}
	if string(raw) != "{\"phase\":\"xdg\"}" {
		t.Fatalf("expected XDG state to win, got %q", string(raw))
	}
}

func TestResolveStateReadPathNoFallbackUsesPreferredPath(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Chdir(workspace)

	wf := &config.Workflow{StateKey: "explicit-state-key"}
	preferred := filepath.Join(workspace, "explicit", wf.StateKey+".json")
	xdgPath, err := XDGStatePath(wf)
	if err != nil {
		t.Fatalf("XDGStatePath failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatalf("mkdir XDG state dir: %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("{\"phase\":\"xdg\"}"), 0o600); err != nil {
		t.Fatalf("write XDG state: %v", err)
	}

	resolved, err := ResolveStateReadPathForWorkflowNoFallback(wf, preferred)
	if err != nil {
		t.Fatalf("ResolveStateReadPathForWorkflowNoFallback failed: %v", err)
	}
	if resolved != preferred {
		t.Fatalf("expected preferred state path, got %q want %q", resolved, preferred)
	}
	if _, err := os.Stat(preferred); !os.IsNotExist(err) {
		t.Fatalf("preferred state should not be migrated, err=%v", err)
	}
}

func TestResolveStateReadPathMigratesOldXDGToRemoteApplyState(t *testing.T) {
	home := t.TempDir()
	stateHome := filepath.Join(home, ".local", "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", stateHome)

	wf := &config.Workflow{StateKey: "remote-state-key"}
	preferred, err := XDGApplyStatePath(wf)
	if err != nil {
		t.Fatalf("XDGApplyStatePath failed: %v", err)
	}
	xdgPath, err := XDGStatePath(wf)
	if err != nil {
		t.Fatalf("XDGStatePath failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatalf("mkdir XDG state dir: %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("{\"phase\":\"completed\"}"), 0o600); err != nil {
		t.Fatalf("write XDG state: %v", err)
	}

	var migratedSource, migratedTarget string
	resolved, err := ResolveStateReadPathForWorkflowWithMigrationSink(wf, preferred, func(source string, target string) {
		migratedSource = source
		migratedTarget = target
	})
	if err != nil {
		t.Fatalf("ResolveStateReadPathForWorkflowWithMigrationSink failed: %v", err)
	}
	if resolved != preferred {
		t.Fatalf("expected migrated state path, got %q want %q", resolved, preferred)
	}
	if migratedSource != xdgPath || migratedTarget != preferred {
		t.Fatalf("unexpected migration event: source=%q target=%q", migratedSource, migratedTarget)
	}
	if _, err := os.Stat(preferred); err != nil {
		t.Fatalf("migrated remote state missing: %v", err)
	}
}

func TestXDGApplyStatePathFallsBackToHomeWhenXDGStateHomeUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")

	wf := &config.Workflow{StateKey: "home-fallback-key"}
	statePath, err := XDGApplyStatePath(wf)
	if err != nil {
		t.Fatalf("XDGApplyStatePath failed: %v", err)
	}
	expected := filepath.Join(home, ".local", "state", "deck", "state", "apply", wf.StateKey+".json")
	if statePath != expected {
		t.Fatalf("state path mismatch: got %q want %q", statePath, expected)
	}
}

func TestResolveStateReadPathMigrationFailureFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	wf := &config.Workflow{StateKey: "migration-failure-key"}
	xdgPath, err := XDGStatePath(wf)
	if err != nil {
		t.Fatalf("XDGStatePath failed: %v", err)
	}
	if err := os.MkdirAll(xdgPath, 0o755); err != nil {
		t.Fatalf("mkdir invalid XDG state file path: %v", err)
	}
	preferred := filepath.Join(t.TempDir(), "target", wf.StateKey+".json")

	_, err = ResolveStateReadPathForWorkflow(wf, preferred)
	if err == nil {
		t.Fatalf("expected migration failure")
	}
	if !strings.Contains(err.Error(), "read fallback state file") {
		t.Fatalf("expected migration error, got %v", err)
	}
}

func TestSaveStateWritesV2MetadataAndLoadStateReadsIt(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state-key.json")
	now := time.Date(2026, 6, 2, 1, 2, 3, 0, time.UTC)
	state := &State{
		Phase:           "completed",
		CompletedPhases: []string{"install"},
		RuntimeVars:     map[string]any{"public": "value"},
		RuntimeSecrets:  map[string]RuntimeSecret{"token": {Phase: "install", StepID: "secret", Output: "value"}},
	}
	metadata := StateMetadata{
		StateKey: "state-key",
		Workflow: StateWorkflow{Path: "workflows/scenarios/apply.yaml", Source: "filesystem", SHA256: "abc123"},
		Now:      func() time.Time { return now },
	}

	if err := SaveStateWithMetadata(statePath, state, metadata); err != nil {
		t.Fatalf("SaveStateWithMetadata failed: %v", err)
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var file stateFileV2
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if file.Version != 2 || file.Kind != "deck.applyState" || file.StateKey != "state-key" {
		t.Fatalf("unexpected v2 identity: %+v", file)
	}
	if file.Workflow.Path != metadata.Workflow.Path || file.Workflow.Source != metadata.Workflow.Source || file.Workflow.SHA256 != metadata.Workflow.SHA256 {
		t.Fatalf("unexpected workflow metadata: %+v", file.Workflow)
	}
	if file.Status != "succeeded" || file.CurrentPhase != "completed" || file.UpdatedAt != "2026-06-02T01:02:03Z" {
		t.Fatalf("unexpected status metadata: %+v", file)
	}
	if len(file.Phases) != 1 || file.Phases[0].Name != "install" || file.Phases[0].Status != "succeeded" {
		t.Fatalf("unexpected phase metadata: %+v", file.Phases)
	}

	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded.Version != 2 || loaded.StateKey != "state-key" || loaded.Status != "succeeded" {
		t.Fatalf("unexpected loaded metadata: %+v", loaded)
	}
	if len(loaded.CompletedPhases) != 1 || loaded.CompletedPhases[0] != "install" {
		t.Fatalf("unexpected loaded phases: %+v", loaded.CompletedPhases)
	}
}
