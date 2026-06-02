package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/userdirs"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type State struct {
	Version         int                      `json:"-"`
	Kind            string                   `json:"-"`
	StateKey        string                   `json:"-"`
	Workflow        StateWorkflow            `json:"-"`
	Status          string                   `json:"-"`
	CreatedAt       string                   `json:"-"`
	UpdatedAt       string                   `json:"-"`
	Phase           string                   `json:"phase,omitempty"`
	CompletedPhases []string                 `json:"completedPhases,omitempty"`
	FailedPhase     string                   `json:"failedPhase,omitempty"`
	RuntimeVars     map[string]any           `json:"runtimeVars,omitempty"`
	RuntimeSecrets  map[string]RuntimeSecret `json:"runtimeSecrets,omitempty"`
	Error           string                   `json:"error,omitempty"`
}

type RuntimeSecret struct {
	Phase  string `json:"phase,omitempty"`
	StepID string `json:"stepID,omitempty"`
	Output string `json:"output,omitempty"`
}

type StateWorkflow struct {
	Path   string `json:"path,omitempty"`
	Source string `json:"source,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type StateMetadata struct {
	StateKey string
	Workflow StateWorkflow
	Now      func() time.Time
}

type stateFileV2 struct {
	Version      int            `json:"version"`
	Kind         string         `json:"kind"`
	StateKey     string         `json:"stateKey"`
	Workflow     StateWorkflow  `json:"workflow,omitempty"`
	Status       string         `json:"status"`
	CurrentPhase string         `json:"currentPhase,omitempty"`
	CreatedAt    string         `json:"createdAt"`
	UpdatedAt    string         `json:"updatedAt"`
	Phases       []statePhaseV2 `json:"phases,omitempty"`
	Runtime      stateRuntimeV2 `json:"runtime,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type statePhaseV2 struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type stateRuntimeV2 struct {
	Vars    map[string]any           `json:"vars,omitempty"`
	Secrets map[string]RuntimeSecret `json:"secrets,omitempty"`
}

type legacyState struct {
	Phase           string                   `json:"phase,omitempty"`
	CompletedPhases []string                 `json:"completedPhases,omitempty"`
	FailedPhase     string                   `json:"failedPhase,omitempty"`
	RuntimeVars     map[string]any           `json:"runtimeVars,omitempty"`
	RuntimeSecrets  map[string]RuntimeSecret `json:"runtimeSecrets,omitempty"`
	Error           string                   `json:"error,omitempty"`
	CompletedSteps  []string                 `json:"completedSteps,omitempty"`
	SkippedSteps    []string                 `json:"skippedSteps,omitempty"`
	FailedStep      string                   `json:"failedStep,omitempty"`
}

func LoadState(path string) (*State, error) {
	content, err := fsutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{CompletedPhases: []string{}, RuntimeVars: map[string]any{}}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}

	return parseStateBytes(content)
}

func (st *State) UnmarshalJSON(content []byte) error {
	loaded, err := parseStateBytes(content)
	if err != nil {
		return err
	}
	*st = *loaded
	return nil
}

func parseStateBytes(content []byte) (*State, error) {
	var header struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
	}
	if err := json.Unmarshal(content, &header); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	if header.Version == 2 || header.Kind == "deck.applyState" {
		return loadStateV2(content)
	}
	return loadStateV1(content)
}

func loadStateV1(content []byte) (*State, error) {
	var legacy legacyState
	var st State
	if err := json.Unmarshal(content, &legacy); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	st = State{
		Version:         1,
		Phase:           legacy.Phase,
		CompletedPhases: legacy.CompletedPhases,
		FailedPhase:     legacy.FailedPhase,
		RuntimeVars:     legacy.RuntimeVars,
		RuntimeSecrets:  legacy.RuntimeSecrets,
		Error:           legacy.Error,
	}
	if len(st.CompletedPhases) == 0 && len(legacy.CompletedSteps) > 0 {
		st.CompletedPhases = []string{}
	}
	if st.CompletedPhases == nil {
		st.CompletedPhases = []string{}
	}
	if st.RuntimeVars == nil {
		st.RuntimeVars = map[string]any{}
	}
	if st.RuntimeSecrets == nil {
		st.RuntimeSecrets = map[string]RuntimeSecret{}
	}
	return &st, nil
}

func loadStateV2(content []byte) (*State, error) {
	var file stateFileV2
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	st := State{
		Version:        file.Version,
		Kind:           file.Kind,
		StateKey:       file.StateKey,
		Workflow:       file.Workflow,
		Status:         file.Status,
		CreatedAt:      file.CreatedAt,
		UpdatedAt:      file.UpdatedAt,
		Phase:          file.CurrentPhase,
		RuntimeVars:    file.Runtime.Vars,
		RuntimeSecrets: file.Runtime.Secrets,
		Error:          file.Error,
	}
	for _, phase := range file.Phases {
		name := strings.TrimSpace(phase.Name)
		if name == "" {
			continue
		}
		switch strings.TrimSpace(phase.Status) {
		case "succeeded":
			st.CompletedPhases = append(st.CompletedPhases, name)
		case "failed":
			st.FailedPhase = name
		}
	}
	if st.Phase == "" {
		st.Phase = phaseFromStatus(st.Status, st.FailedPhase)
	}
	normalizeState(&st)
	return &st, nil
}

func SaveState(path string, st *State) error {
	return SaveStateWithMetadata(path, st, StateMetadata{})
}

func SaveStateWithMetadata(path string, st *State, metadata StateMetadata) error {
	file := stateFileFromState(path, st, metadata)
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}
	return SaveRawStateFile(path, raw)
}

func stateFileFromState(path string, st *State, metadata StateMetadata) stateFileV2 {
	if st == nil {
		st = &State{}
	}
	now := time.Now().UTC()
	if metadata.Now != nil {
		now = metadata.Now().UTC()
	}
	updatedAt := now.Format(time.RFC3339)
	createdAt := strings.TrimSpace(st.CreatedAt)
	if createdAt == "" {
		createdAt = updatedAt
	}
	stateKey := strings.TrimSpace(metadata.StateKey)
	if stateKey == "" {
		stateKey = strings.TrimSpace(st.StateKey)
	}
	if stateKey == "" {
		stateKey = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	workflow := metadata.Workflow
	if workflow == (StateWorkflow{}) {
		workflow = st.Workflow
	}
	status := stateStatus(st)
	return stateFileV2{
		Version:      2,
		Kind:         "deck.applyState",
		StateKey:     stateKey,
		Workflow:     workflow,
		Status:       status,
		CurrentPhase: strings.TrimSpace(st.Phase),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Phases:       phasesForState(st, updatedAt),
		Runtime:      stateRuntimeV2{Vars: st.RuntimeVars, Secrets: st.RuntimeSecrets},
		Error:        strings.TrimSpace(st.Error),
	}
}

func phasesForState(st *State, completedAt string) []statePhaseV2 {
	if st == nil {
		return nil
	}
	phases := make([]statePhaseV2, 0, len(st.CompletedPhases)+1)
	seen := map[string]bool{}
	for _, name := range st.CompletedPhases {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		phases = append(phases, statePhaseV2{Name: trimmed, Status: "succeeded", CompletedAt: completedAt})
	}
	failed := strings.TrimSpace(st.FailedPhase)
	if failed != "" && !seen[failed] {
		phases = append(phases, statePhaseV2{Name: failed, Status: "failed"})
	}
	return phases
}

func stateStatus(st *State) string {
	if st == nil {
		return "running"
	}
	if strings.TrimSpace(st.FailedPhase) != "" || strings.TrimSpace(st.Error) != "" {
		return "failed"
	}
	if strings.TrimSpace(st.Phase) == "completed" {
		return "succeeded"
	}
	return "running"
}

func phaseFromStatus(status string, failedPhase string) string {
	switch strings.TrimSpace(status) {
	case "succeeded":
		return "completed"
	case "failed":
		return strings.TrimSpace(failedPhase)
	default:
		return ""
	}
}

func normalizeState(st *State) {
	if st == nil {
		return
	}
	if st.CompletedPhases == nil {
		st.CompletedPhases = []string{}
	}
	if st.RuntimeVars == nil {
		st.RuntimeVars = map[string]any{}
	}
	if st.RuntimeSecrets == nil {
		st.RuntimeSecrets = map[string]RuntimeSecret{}
	}
}

func SaveRawStateFile(path string, raw []byte) error {
	if err := filemode.EnsureParentPrivateDir(path); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	tmp := path + ".tmp"
	if err := filemode.WritePrivateFile(tmp, raw); err != nil {
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

func DefaultStatePath(wf *config.Workflow) (string, error) {
	return StatePathInDir(wf, workspacepaths.ApplyStateDir("."))
}

func StatePathInDir(wf *config.Workflow, stateDir string) (string, error) {
	if wf == nil {
		return "", fmt.Errorf("workflow is nil")
	}
	stateKey := strings.TrimSpace(wf.StateKey)
	if stateKey == "" {
		return "", fmt.Errorf("workflow state key is empty")
	}
	resolvedDir := strings.TrimSpace(stateDir)
	if resolvedDir == "" {
		resolvedDir = workspacepaths.ApplyStateDir(".")
	}
	return filepath.Join(resolvedDir, stateKey+".json"), nil
}

func XDGStatePath(wf *config.Workflow) (string, error) {
	if wf == nil {
		return "", fmt.Errorf("workflow is nil")
	}
	stateKey := strings.TrimSpace(wf.StateKey)
	if stateKey == "" {
		return "", fmt.Errorf("workflow state key is empty")
	}
	return userdirs.StateFile(stateKey + ".json")
}

func XDGApplyStatePath(wf *config.Workflow) (string, error) {
	if wf == nil {
		return "", fmt.Errorf("workflow is nil")
	}
	stateKey := strings.TrimSpace(wf.StateKey)
	if stateKey == "" {
		return "", fmt.Errorf("workflow state key is empty")
	}
	root, err := userdirs.StateRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "state", workspacepaths.ApplyStateDirRel, stateKey+".json"), nil
}

func LegacyStatePath(wf *config.Workflow) (string, error) {
	if wf == nil {
		return "", fmt.Errorf("workflow is nil")
	}
	stateKey := strings.TrimSpace(wf.StateKey)
	if stateKey == "" {
		return "", fmt.Errorf("workflow state key is empty")
	}
	return userdirs.LegacyStateFile(stateKey + ".json")
}

func resolveStateReadPath(wf *config.Workflow, preferredPath string, allowFallback bool, migrationSink func(source string, target string)) (string, error) {
	resolved := strings.TrimSpace(preferredPath)
	if resolved == "" {
		return preferredPath, nil
	}
	if _, err := os.Stat(resolved); err == nil {
		return resolved, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat state file: %w", err)
	}
	if !allowFallback {
		return resolved, nil
	}
	if wf == nil || strings.TrimSpace(wf.StateKey) == "" {
		return resolved, nil
	}
	fallbackPath, migrated, err := resolveFallbackStateReadPath(wf, resolved)
	if err != nil {
		return "", err
	}
	if migrated {
		if migrationSink != nil {
			migrationSink(fallbackPath, resolved)
		}
		return resolved, nil
	}
	return fallbackPath, nil
}

func ResolveStateReadPathForWorkflow(wf *config.Workflow, preferredPath string) (string, error) {
	return resolveStateReadPath(wf, preferredPath, true, nil)
}

func ResolveStateReadPathForWorkflowNoFallback(wf *config.Workflow, preferredPath string) (string, error) {
	return resolveStateReadPath(wf, preferredPath, false, nil)
}

func ResolveStateReadPathForWorkflowWithMigrationSink(wf *config.Workflow, preferredPath string, migrationSink func(source string, target string)) (string, error) {
	return resolveStateReadPath(wf, preferredPath, true, migrationSink)
}
