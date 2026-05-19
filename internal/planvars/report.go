package planvars

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/applycli"
	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

const (
	CommandApply   = "apply"
	CommandPrepare = "prepare"
)

type Options struct {
	Command         string
	WorkflowPath    string
	Scenario        string
	SelectedPhase   string
	PreparedRoot    string
	Fresh           bool
	VarOverrides    map[string]any
	VarsFiles       []string
	Output          string
	StdoutPrintf    func(format string, args ...any) error
	JSONEncoderFunc func() *json.Encoder
}

type Report struct {
	Command       string         `json:"command"`
	WorkflowPath  string         `json:"workflowPath"`
	SelectedPhase string         `json:"selectedPhase,omitempty"`
	StatePath     string         `json:"statePath,omitempty"`
	Vars          map[string]any `json:"vars"`
	Context       map[string]any `json:"context"`
	Runtime       RuntimeReport  `json:"runtime"`
}

type RuntimeReport struct {
	Initial map[string]any      `json:"initial"`
	Planned []PlannedRuntimeVar `json:"planned"`
}

type PlannedRuntimeVar struct {
	Key    string `json:"key"`
	Step   string `json:"step"`
	Output string `json:"output"`
	Phase  string `json:"phase,omitempty"`
}

func Execute(ctx context.Context, opts Options) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if opts.StdoutPrintf == nil {
		return fmt.Errorf("stdout printf is nil")
	}
	report, err := BuildReport(ctx, opts)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Output) == "json" {
		if opts.JSONEncoderFunc == nil {
			return fmt.Errorf("json encoder factory is nil")
		}
		enc := opts.JSONEncoderFunc()
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	return writeTextReport(opts.StdoutPrintf, report)
}

func BuildReport(ctx context.Context, opts Options) (Report, error) {
	switch strings.TrimSpace(opts.Command) {
	case "", CommandApply:
		return buildApplyReport(ctx, opts)
	case CommandPrepare:
		return buildPrepareReport(ctx, opts)
	default:
		return Report{}, fmt.Errorf("unsupported vars plan command: %s", opts.Command)
	}
}

func buildApplyReport(ctx context.Context, opts Options) (Report, error) {
	request, err := applycli.ResolveExecutionRequest(ctx, applycli.ExecutionRequestOptions{
		CommandName:                  CommandApply,
		WorkflowPath:                 strings.TrimSpace(opts.WorkflowPath),
		AllowRemoteWorkflow:          true,
		VarOverrides:                 opts.VarOverrides,
		VarsFiles:                    append([]string(nil), opts.VarsFiles...),
		NodeScopedVars:               true,
		Fresh:                        opts.Fresh,
		SelectedPhase:                strings.TrimSpace(opts.SelectedPhase),
		DefaultPhase:                 "",
		BuildExecutionWorkflow:       true,
		ResolveStatePath:             true,
		StatePathFromExecutionTarget: true,
	})
	if err != nil {
		return Report{}, err
	}
	execContext, err := buildApplyContext(request, strings.TrimSpace(opts.Scenario))
	if err != nil {
		return Report{}, err
	}
	if err := applyContextStateKey(&request, execContext); err != nil {
		return Report{}, err
	}
	execContext.Paths.StateFile = request.StatePath

	state, err := applycli.LoadInstallDryRunState(request)
	if err != nil {
		return Report{}, err
	}
	runtimeInitial := map[string]any{}
	for key, value := range state.RuntimeVars {
		runtimeInitial[key] = value
	}
	runtimeInitial["host"] = install.CurrentHostFacts()

	return Report{
		Command:       CommandApply,
		WorkflowPath:  request.WorkflowPath,
		SelectedPhase: request.SelectedPhase,
		StatePath:     request.StatePath,
		Vars:          cloneMap(request.ExecutionWorkflow.Vars),
		Context:       execContext.RenderMap(),
		Runtime:       RuntimeReport{Initial: runtimeInitial, Planned: plannedRuntimeVars(request.ExecutionWorkflow)},
	}, nil
}

func buildPrepareReport(ctx context.Context, opts Options) (Report, error) {
	workflowPath, err := discoverPrepareWorkflow(ctx)
	if err != nil {
		return Report{}, err
	}
	preparedRoot := strings.TrimSpace(opts.PreparedRoot)
	if preparedRoot == "" {
		preparedRoot = workspacepaths.DefaultPreparedRoot(".")
	}
	preparedRootAbs, err := filepath.Abs(preparedRoot)
	if err != nil {
		return Report{}, fmt.Errorf("resolve --root: %w", err)
	}
	wf, err := config.LoadWithOptions(ctx, workflowPath, config.LoadOptions{VarOverrides: opts.VarOverrides, VarsFiles: append([]string(nil), opts.VarsFiles...), NodeScopedVars: true})
	if err != nil {
		return Report{}, err
	}
	executionWorkflow, err := applycli.BuildExecutionWorkflow(wf, strings.TrimSpace(opts.SelectedPhase))
	if err != nil {
		return Report{}, err
	}
	execContext := workflowcontext.Context{
		Command: workflowcontext.CommandPrepare,
		Workflow: workflowcontext.Workflow{
			Source: workflowcontext.SourceFilesystem,
			Path:   workflowPath,
		},
		Paths: workflowcontext.Paths{BundleRoot: preparedRootAbs, OutputRoot: preparedRootAbs},
	}
	return Report{
		Command:       CommandPrepare,
		WorkflowPath:  workflowPath,
		SelectedPhase: strings.TrimSpace(opts.SelectedPhase),
		Vars:          cloneMap(executionWorkflow.Vars),
		Context:       execContext.RenderMap(),
		Runtime:       RuntimeReport{Initial: map[string]any{"host": install.CurrentHostFacts()}, Planned: plannedRuntimeVars(executionWorkflow)},
	}, nil
}

func buildApplyContext(request applycli.ExecutionRequest, scenario string) (workflowcontext.Context, error) {
	bundleRoot := ""
	if !applycli.IsHTTPWorkflowPath(request.WorkflowPath) {
		inferred, err := inferBundleRootFromWorkflowPath(request.WorkflowPath)
		if err != nil {
			return workflowcontext.Context{}, err
		}
		bundleRoot = inferred
	}
	return workflowcontext.Context{
		Command: workflowcontext.CommandApply,
		Workflow: workflowcontext.Workflow{
			Source:   workflowcontext.SourceForWorkflowPath(request.WorkflowPath),
			Path:     request.WorkflowPath,
			Scenario: strings.TrimSpace(scenario),
		},
		Paths: workflowcontext.Paths{BundleRoot: bundleRoot},
	}, nil
}

func applyContextStateKey(request *applycli.ExecutionRequest, execContext workflowcontext.Context) error {
	if request == nil || request.Workflow == nil {
		return fmt.Errorf("workflow is nil")
	}
	stateKey := config.StateKeyWithContext(request.Workflow.StateKey, execContext.StateFingerprint())
	if stateKey == "" {
		return fmt.Errorf("workflow state key is empty")
	}
	request.Workflow.StateKey = stateKey
	if request.ExecutionWorkflow != nil {
		request.ExecutionWorkflow.StateKey = stateKey
	}
	statePath, err := applycli.ResolveInstallStatePath(request.Workflow)
	if err != nil {
		return err
	}
	request.StatePath = statePath
	return nil
}

func inferBundleRootFromWorkflowPath(workflowPath string) (string, error) {
	trimmed := strings.TrimSpace(workflowPath)
	if trimmed == "" || applycli.IsHTTPWorkflowPath(trimmed) {
		return "", nil
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve workflow path: %w", err)
	}
	workflowDir := string(filepath.Separator) + workspacepaths.WorkflowRootDir + string(filepath.Separator)
	idx := strings.LastIndex(abs, workflowDir)
	if idx < 0 {
		return "", nil
	}
	return abs[:idx], nil
}

func discoverPrepareWorkflow(ctx context.Context) (string, error) {
	workflowDir := filepath.Join(".", workspacepaths.WorkflowRootDir)
	absWorkflowDir, err := filepath.Abs(workflowDir)
	if err != nil {
		return "", fmt.Errorf("resolve workflow directory: %w", err)
	}
	info, err := os.Stat(absWorkflowDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("workflow directory not found: %s", absWorkflowDir)
	}
	preferred := workspacepaths.CanonicalPrepareWorkflowPath(filepath.Dir(absWorkflowDir))
	preferredInfo, statErr := os.Stat(preferred)
	if statErr != nil || preferredInfo.IsDir() {
		return "", fmt.Errorf("prepare workflow not found: %s", preferred)
	}
	if _, loadErr := config.LoadWithOptions(ctx, preferred, config.LoadOptions{NodeScopedVars: true}); loadErr != nil {
		return "", loadErr
	}
	return preferred, nil
}

func plannedRuntimeVars(wf *config.Workflow) []PlannedRuntimeVar {
	if wf == nil {
		return nil
	}
	planned := make([]PlannedRuntimeVar, 0)
	for _, phase := range config.NormalizedPhases(wf) {
		for _, step := range phase.Steps {
			keys := make([]string, 0, len(step.Register))
			for key := range step.Register {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				planned = append(planned, PlannedRuntimeVar{Key: key, Step: step.ID, Output: step.Register[key], Phase: phase.Name})
			}
		}
	}
	return planned
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func writeTextReport(stdoutPrintf func(format string, args ...any) error, report Report) error {
	if err := stdoutPrintf("VARS command=%s workflow=%s selectedPhase=%s\n", report.Command, report.WorkflowPath, displayValueOrDash(report.SelectedPhase)); err != nil {
		return err
	}
	if err := stdoutPrintf("\nvars:\n"); err != nil {
		return err
	}
	if err := writeMap(stdoutPrintf, "  ", report.Vars); err != nil {
		return err
	}
	if err := stdoutPrintf("\ncontext:\n"); err != nil {
		return err
	}
	if err := writeMap(stdoutPrintf, "  ", report.Context); err != nil {
		return err
	}
	if err := stdoutPrintf("\nruntime:\n  initial:\n"); err != nil {
		return err
	}
	if err := writeMap(stdoutPrintf, "    ", report.Runtime.Initial); err != nil {
		return err
	}
	if err := stdoutPrintf("\n  planned:\n"); err != nil {
		return err
	}
	if len(report.Runtime.Planned) == 0 {
		return stdoutPrintf("    -\n")
	}
	for _, planned := range report.Runtime.Planned {
		if err := stdoutPrintf("    %s:\n", planned.Key); err != nil {
			return err
		}
		if err := stdoutPrintf("      step: %s\n", planned.Step); err != nil {
			return err
		}
		if strings.TrimSpace(planned.Phase) != "" {
			if err := stdoutPrintf("      phase: %s\n", planned.Phase); err != nil {
				return err
			}
		}
		if err := stdoutPrintf("      output: %s\n", planned.Output); err != nil {
			return err
		}
	}
	return nil
}

func writeMap(stdoutPrintf func(format string, args ...any) error, indent string, values map[string]any) error {
	if len(values) == 0 {
		return stdoutPrintf("%s-\n", indent)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		value := values[key]
		if nested, ok := value.(map[string]any); ok {
			if err := stdoutPrintf("%s%s:\n", indent, key); err != nil {
				return err
			}
			if err := writeMap(stdoutPrintf, indent+"  ", nested); err != nil {
				return err
			}
			continue
		}
		if err := stdoutPrintf("%s%s: %s\n", indent, key, formatValue(value)); err != nil {
			return err
		}
	}
	return nil
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return "null"
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(raw)
	}
}

func displayValueOrDash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}
