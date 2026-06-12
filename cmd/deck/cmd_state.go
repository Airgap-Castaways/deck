package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Airgap-Castaways/deck/internal/applycli"
	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type stateWorkflowOptions struct {
	workflowPath string
	scenario     string
	source       string
	root         string
	server       string
	stateDir     string
	output       string
	varOverrides map[string]string
	varsFiles    []string
}

type stateListOptions struct {
	stateDir string
	output   string
}

type stateClearOptions struct {
	workflow stateWorkflowOptions
	all      bool
	yes      bool
}

type stateEntry struct {
	StateKey        string   `json:"stateKey"`
	Status          string   `json:"status"`
	WorkflowPath    string   `json:"workflowPath,omitempty"`
	WorkflowSource  string   `json:"workflowSource,omitempty"`
	WorkflowSHA256  string   `json:"workflowSha256,omitempty"`
	CurrentPhase    string   `json:"currentPhase,omitempty"`
	CompletedPhases []string `json:"completedPhases,omitempty"`
	CompletedCount  int      `json:"completedPhaseCount"`
	RuntimeVarKeys  []string `json:"runtimeVarKeys,omitempty"`
	UpdatedAt       string   `json:"updatedAt,omitempty"`
	Version         int      `json:"version"`
	Path            string   `json:"path"`
}

func newStateCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{Use: "state", Short: "Inspect and clear saved apply state"}
	cmd.AddCommand(newStateShowCommand(env), newStateListCommand(env), newStateClearCommand(env))
	return cmd
}

func newStateShowCommand(env *cliEnv) *cobra.Command {
	vars := &varFlag{}
	varsFiles := &stringSliceFlag{}
	opts := stateWorkflowOptions{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show apply state selected by workflow inputs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.varOverrides = vars.AsMap()
			opts.varsFiles = varsFiles.Values()
			return runStateShow(env, cmd.Context(), opts)
		},
	}
	addStateWorkflowFlags(cmd, &opts, vars, varsFiles)
	return cmd
}

func newStateListCommand(env *cliEnv) *cobra.Command {
	opts := stateListOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List apply state files",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStateList(env, opts)
		},
	}
	cmd.Flags().StringVar(&opts.stateDir, "state-dir", "", "directory for apply state files (defaults to .deck/state/apply)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "text", "output format (text|json)")
	return cmd
}

func newStateClearCommand(env *cliEnv) *cobra.Command {
	vars := &varFlag{}
	varsFiles := &stringSliceFlag{}
	opts := stateClearOptions{}
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Delete apply state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.workflow.varOverrides = vars.AsMap()
			opts.workflow.varsFiles = varsFiles.Values()
			return runStateClear(env, cmd.Context(), opts)
		},
	}
	addStateWorkflowFlags(cmd, &opts.workflow, vars, varsFiles)
	cmd.Flags().BoolVar(&opts.all, "all", false, "delete all apply state files in the selected state directory")
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "confirm state deletion without prompting")
	return cmd
}

func addStateWorkflowFlags(cmd *cobra.Command, opts *stateWorkflowOptions, vars *varFlag, varsFiles *stringSliceFlag) {
	cmd.Flags().StringVar(&opts.workflowPath, "workflow", "", "path or URL to workflow file")
	cmd.Flags().StringVar(&opts.scenario, "scenario", "", "scenario name")
	cmd.Flags().StringVar(&opts.source, "source", scenarioSourceLocal, "scenario source (local|server)")
	cmd.Flags().StringVar(&opts.root, "root", "", "local workflow root containing workflows/")
	cmd.Flags().StringVar(&opts.server, "server", "", "remote workflow server URL")
	cmd.Flags().StringVar(&opts.stateDir, "state-dir", "", "directory for apply state files (overrides local .deck/state/apply or remote XDG state)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "text", "output format (text|json)")
	cmd.Flags().VarP(varsFiles, "vars-file", "f", "vars overlay path relative to the selected workflow root (workflows/), repeatable")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	registerScenarioSourceCompletion(cmd, "source", false)
	registerScenarioNameCompletionWithSourceLocator(cmd, "scenario", "source", "root", "server", false)
}

func runStateShow(env *cliEnv, ctx context.Context, opts stateWorkflowOptions) error {
	request, err := resolveStateRequest(ctx, opts)
	if err != nil {
		return err
	}
	state, err := install.LoadState(request.StatePath)
	if err != nil {
		return err
	}
	entry := stateEntryFromState(request.StatePath, state)
	if strings.TrimSpace(entry.WorkflowPath) == "" {
		entry.WorkflowPath = request.WorkflowPath
	}
	if strings.TrimSpace(entry.StateKey) == "" && request.Workflow != nil {
		entry.StateKey = request.Workflow.StateKey
	}
	return writeStateEntries(env, strings.TrimSpace(opts.output), []stateEntry{entry}, false)
}

func runStateList(env *cliEnv, opts stateListOptions) error {
	stateDir := strings.TrimSpace(opts.stateDir)
	if stateDir == "" {
		stateDir = workspacepaths.ApplyStateDir(".")
	}
	entries, err := readStateEntries(stateDir)
	if err != nil {
		return err
	}
	return writeStateEntries(env, strings.TrimSpace(opts.output), entries, true)
}

func runStateClear(_ *cliEnv, ctx context.Context, opts stateClearOptions) error {
	if !opts.yes {
		return fmt.Errorf("state clear requires --yes")
	}
	workflowSelected := strings.TrimSpace(opts.workflow.workflowPath) != "" || strings.TrimSpace(opts.workflow.scenario) != ""
	if opts.all && workflowSelected {
		return fmt.Errorf("state clear accepts either --all or --workflow/--scenario, not both")
	}
	if !opts.all && !workflowSelected {
		return fmt.Errorf("state clear requires --all or --workflow/--scenario")
	}
	if opts.all {
		stateDir := strings.TrimSpace(opts.workflow.stateDir)
		if stateDir == "" {
			stateDir = workspacepaths.ApplyStateDir(".")
		}
		return clearStateDir(stateDir)
	}
	request, err := resolveStateRequest(ctx, opts.workflow)
	if err != nil {
		return err
	}
	paths := []string{request.StatePath}
	if !request.StateDirExplicit && request.Workflow != nil {
		if xdgPath, err := install.XDGStatePath(request.Workflow); err == nil {
			paths = append(paths, xdgPath)
		}
		if legacyPath, err := install.LegacyStatePath(request.Workflow); err == nil {
			paths = append(paths, legacyPath)
		}
	}
	return removeStateFiles(paths)
}

func resolveStateRequest(ctx context.Context, opts stateWorkflowOptions) (applycli.ExecutionRequest, error) {
	workflowPath, err := resolvePlanWorkflowPath(ctx, strings.TrimSpace(opts.workflowPath), strings.TrimSpace(opts.scenario), strings.TrimSpace(opts.source), strings.TrimSpace(opts.root), strings.TrimSpace(opts.server))
	if err != nil {
		return applycli.ExecutionRequest{}, err
	}
	stateDir := strings.TrimSpace(opts.stateDir)
	return applycli.ResolveStateRequest(ctx, applycli.StateRequestOptions{
		WorkflowPath:     workflowPath,
		StateDir:         stateDir,
		StateDirExplicit: stateDir != "",
		VarOverrides:     varsAsAnyMap(opts.varOverrides),
		VarsFiles:        append([]string(nil), opts.varsFiles...),
	})
}

func readStateEntries(stateDir string) ([]stateEntry, error) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read state directory: %w", err)
	}
	result := make([]stateEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		path := filepath.Join(stateDir, entry.Name())
		state, err := install.LoadState(path)
		if err != nil {
			continue
		}
		result = append(result, stateEntryFromState(path, state))
	}
	slices.SortFunc(result, func(a, b stateEntry) int { return strings.Compare(a.Path, b.Path) })
	return result, nil
}

func stateEntryFromState(path string, state *install.State) stateEntry {
	if state == nil {
		state = &install.State{}
	}
	stateKey := strings.TrimSpace(state.StateKey)
	if stateKey == "" {
		stateKey = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	runtimeVarKeys := make([]string, 0, len(state.RuntimeVars))
	for key := range state.RuntimeVars {
		runtimeVarKeys = append(runtimeVarKeys, key)
	}
	slices.Sort(runtimeVarKeys)
	return stateEntry{
		StateKey:        stateKey,
		Status:          displayStateStatus(state),
		WorkflowPath:    state.Workflow.Path,
		WorkflowSource:  state.Workflow.Source,
		WorkflowSHA256:  state.Workflow.SHA256,
		CurrentPhase:    state.Phase,
		CompletedPhases: append([]string(nil), state.CompletedPhases...),
		CompletedCount:  len(state.CompletedPhases),
		RuntimeVarKeys:  runtimeVarKeys,
		UpdatedAt:       state.UpdatedAt,
		Version:         displayStateVersion(state),
		Path:            path,
	}
}

func displayStateStatus(state *install.State) string {
	if state == nil {
		return "running"
	}
	if strings.TrimSpace(state.Status) != "" {
		return state.Status
	}
	if strings.TrimSpace(state.FailedPhase) != "" || strings.TrimSpace(state.Error) != "" {
		return "failed"
	}
	if strings.TrimSpace(state.Phase) == "completed" {
		return "succeeded"
	}
	return "running"
}

func displayStateVersion(state *install.State) int {
	if state == nil || state.Version == 0 {
		return 1
	}
	return state.Version
}

func writeStateEntries(env *cliEnv, output string, entries []stateEntry, list bool) error {
	resolvedOutput, err := resolveOutputFormat(output)
	if err != nil {
		return err
	}
	if resolvedOutput == "json" {
		encoder := env.stdoutJSONEncoder()
		encoder.SetIndent("", "  ")
		if list {
			return encoder.Encode(entries)
		}
		if len(entries) == 0 {
			return encoder.Encode(nil)
		}
		return encoder.Encode(entries[0])
	}
	if list {
		for _, entry := range entries {
			if err := env.stdoutPrintf("%s status=%s workflow=%s completed=%d updated=%s path=%s\n", entry.StateKey, entry.Status, displayValueOrDash(entry.WorkflowPath), entry.CompletedCount, displayValueOrDash(entry.UpdatedAt), entry.Path); err != nil {
				return err
			}
		}
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	entry := entries[0]
	if err := env.stdoutPrintf("state=%s\n", entry.Path); err != nil {
		return err
	}
	if err := env.stdoutPrintf("stateKey=%s\n", entry.StateKey); err != nil {
		return err
	}
	if err := env.stdoutPrintf("version=%d\n", entry.Version); err != nil {
		return err
	}
	if err := env.stdoutPrintf("workflow=%s\n", displayValueOrDash(entry.WorkflowPath)); err != nil {
		return err
	}
	if err := env.stdoutPrintf("status=%s\n", entry.Status); err != nil {
		return err
	}
	if err := env.stdoutPrintf("currentPhase=%s\n", displayValueOrDash(entry.CurrentPhase)); err != nil {
		return err
	}
	if err := env.stdoutPrintf("completedPhases=%s\n", strings.Join(entry.CompletedPhases, ",")); err != nil {
		return err
	}
	if err := env.stdoutPrintf("runtimeVarKeys=%s\n", strings.Join(entry.RuntimeVarKeys, ",")); err != nil {
		return err
	}
	return env.stdoutPrintf("updatedAt=%s\n", displayValueOrDash(entry.UpdatedAt))
}

func clearStateDir(stateDir string) error {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read state directory: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(stateDir, entry.Name()))
	}
	return removeStateFiles(paths)
}

func removeStateFiles(paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		if err := os.Remove(trimmed); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove state file %s: %w", trimmed, err)
		}
	}
	return nil
}
