package applycli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type PlanCommandOptions struct {
	WorkflowPath    string
	Scenario        string
	SelectedPhase   string
	Output          string
	Fresh           bool
	VarOverrides    map[string]any
	VarsFiles       []string
	Hostname        string
	DetectHostname  func() (string, error)
	Verbosef        func(level int, format string, args ...any) error
	StdoutPrintf    func(format string, args ...any) error
	JSONEncoderFunc func() *json.Encoder
	ResolveOutput   func(string) (string, error)
}

type ApplyCommandOptions struct {
	WorkflowPath   string
	BundleRoot     string
	WorkflowSource string
	Scenario       string
	SelectedPhase  string
	Fresh          bool
	DryRun         bool
	VarOverrides   map[string]any
	VarsFiles      []string
	Hostname       string
	DetectHostname func() (string, error)
	Verbosef       func(level int, format string, args ...any) error
	StdoutPrintf   func(format string, args ...any) error
	StdoutPrintln  func(args ...any) error
	AdditionalSink install.StepEventSink
	NewRunLogger   func(workflowPath, workflowSource, scenario, bundleRoot, selectedPhase string) (RunLogger, error)
}

func RunPlanCommand(ctx context.Context, opts PlanCommandOptions) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if opts.ResolveOutput == nil {
		return fmt.Errorf("resolve output format is nil")
	}
	resolvedOutput, err := opts.ResolveOutput(opts.Output)
	if err != nil {
		return err
	}
	resolvedRequest, err := ResolveExecutionRequest(ctx, ExecutionRequestOptions{
		CommandName:                  "diff",
		WorkflowPath:                 strings.TrimSpace(opts.WorkflowPath),
		VarOverrides:                 opts.VarOverrides,
		VarsFiles:                    append([]string(nil), opts.VarsFiles...),
		NodeScopedVars:               true,
		Hostname:                     opts.Hostname,
		DetectHostname:               opts.DetectHostname,
		Fresh:                        opts.Fresh,
		SelectedPhase:                strings.TrimSpace(opts.SelectedPhase),
		DefaultPhase:                 "",
		BuildExecutionWorkflow:       true,
		ResolveStatePath:             true,
		StatePathFromExecutionTarget: true,
	})
	if err != nil {
		return err
	}
	execContext, err := buildPlanExecutionContext(&resolvedRequest, strings.TrimSpace(opts.Scenario))
	if err != nil {
		return err
	}
	if err := applyContextStateKey(&resolvedRequest, execContext); err != nil {
		return err
	}
	execContext.Paths.StateFile = resolvedRequest.StatePath
	return ExecutePlan(ctx, PlanOptions{
		Request:         resolvedRequest,
		Context:         execContext.RenderMap(),
		Output:          resolvedOutput,
		Verbosef:        opts.Verbosef,
		StdoutPrintf:    opts.StdoutPrintf,
		JSONEncoderFunc: opts.JSONEncoderFunc,
	})
}

func RunApplyCommand(ctx context.Context, opts ApplyCommandOptions) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	resolvedRequest, err := ResolveExecutionRequest(ctx, ExecutionRequestOptions{
		CommandName:                  "apply",
		WorkflowPath:                 strings.TrimSpace(opts.WorkflowPath),
		AllowRemoteWorkflow:          true,
		VarOverrides:                 opts.VarOverrides,
		VarsFiles:                    append([]string(nil), opts.VarsFiles...),
		NodeScopedVars:               true,
		Hostname:                     opts.Hostname,
		DetectHostname:               opts.DetectHostname,
		Fresh:                        opts.Fresh,
		SelectedPhase:                strings.TrimSpace(opts.SelectedPhase),
		DefaultPhase:                 "",
		BuildExecutionWorkflow:       true,
		ResolveStatePath:             true,
		StatePathFromExecutionTarget: false,
	})
	if err != nil {
		return err
	}
	execContext, bundleRoot, err := buildApplyExecutionContext(resolvedRequest, opts)
	if err != nil {
		return err
	}
	if err := applyContextStateKey(&resolvedRequest, execContext); err != nil {
		return err
	}
	execContext.Paths.StateFile = resolvedRequest.StatePath
	return Execute(ctx, ExecuteOptions{
		Request:        resolvedRequest,
		BundleRoot:     bundleRoot,
		Context:        execContext,
		WorkflowSource: strings.TrimSpace(opts.WorkflowSource),
		Scenario:       strings.TrimSpace(opts.Scenario),
		DryRun:         opts.DryRun,
		Verbosef:       opts.Verbosef,
		StdoutPrintf:   opts.StdoutPrintf,
		StdoutPrintln:  opts.StdoutPrintln,
		AdditionalSink: opts.AdditionalSink,
		NewRunLogger:   opts.NewRunLogger,
	})
}

func buildPlanExecutionContext(request *ExecutionRequest, scenario string) (workflowcontext.Context, error) {
	if request == nil {
		return workflowcontext.Context{}, fmt.Errorf("execution request is nil")
	}
	bundleRoot := ""
	if !IsHTTPWorkflowPath(request.WorkflowPath) {
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

func buildApplyExecutionContext(request ExecutionRequest, opts ApplyCommandOptions) (workflowcontext.Context, string, error) {
	bundleRoot := strings.TrimSpace(opts.BundleRoot)
	if bundleRoot == "" && !IsHTTPWorkflowPath(request.WorkflowPath) {
		inferred, err := inferBundleRootFromWorkflowPath(request.WorkflowPath)
		if err != nil {
			return workflowcontext.Context{}, "", err
		}
		bundleRoot = inferred
	}
	workflowSource := workflowcontext.SourceForWorkflowPath(request.WorkflowPath)
	if strings.TrimSpace(opts.WorkflowSource) == "server" {
		workflowSource = workflowcontext.SourceServer
	}
	return workflowcontext.Context{
		Command: workflowcontext.CommandApply,
		Workflow: workflowcontext.Workflow{
			Source:   workflowSource,
			Path:     request.WorkflowPath,
			Scenario: strings.TrimSpace(opts.Scenario),
		},
		Paths: workflowcontext.Paths{BundleRoot: bundleRoot},
	}, bundleRoot, nil
}

func applyContextStateKey(request *ExecutionRequest, execContext workflowcontext.Context) error {
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
	statePath, err := ResolveInstallStatePath(request.Workflow)
	if err != nil {
		return err
	}
	request.StatePath = statePath
	return nil
}

func inferBundleRootFromWorkflowPath(workflowPath string) (string, error) {
	trimmed := strings.TrimSpace(workflowPath)
	if trimmed == "" || IsHTTPWorkflowPath(trimmed) {
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
