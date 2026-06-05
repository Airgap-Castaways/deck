package applycli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/logs"
	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type RunLogger interface {
	Dir() string
	EventSink() install.StepEventSink
	CloseWithResult(status string, err error) error
}

type ExecuteOptions struct {
	Request        ExecutionRequest
	BundleRoot     string
	Context        workflowcontext.Context
	WorkflowSource string
	Scenario       string
	DryRun         bool
	NonInteractive bool
	Verbosef       func(level int, format string, args ...any) error
	StdoutPrintf   func(format string, args ...any) error
	StdoutPrintln  func(args ...any) error
	InvocationID   string
	AdditionalSink install.StepEventSink
	NewRunLogger   func(workflowPath, workflowSource, scenario, bundleRoot, selectedPhase string) (RunLogger, error)
}

func Execute(ctx context.Context, opts ExecuteOptions) (err error) {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	request := opts.Request
	if request.Workflow == nil {
		return fmt.Errorf("workflow is nil")
	}
	if request.ExecutionWorkflow == nil {
		return fmt.Errorf("execution workflow is nil")
	}
	started := time.Now().UTC()
	defer func() {
		status := "ok"
		if err != nil {
			status = "failed"
		}
		_ = emitApplyDiagnostic(opts, 1, logs.CLIEvent{Component: "apply", Event: "run_completed", Attrs: map[string]any{"status": status, "duration_ms": time.Since(started).Milliseconds()}})
	}()
	if err := emitApplyDiagnostic(opts, 1, logs.CLIEvent{Component: "apply", Event: "run_requested", Attrs: map[string]any{"workflow": request.WorkflowPath, "phase": request.SelectedPhase, "state": request.StatePath, "bundle": strings.TrimSpace(opts.BundleRoot), "dry_run": opts.DryRun, "fresh": request.Fresh}}); err != nil {
		return err
	}
	if err := logApplyExecutionPlan(opts, request); err != nil {
		return err
	}
	if err := logApplyContext(opts, request); err != nil {
		return err
	}
	if opts.DryRun {
		return writeApplyDryRun(opts.StdoutPrintf, request, opts.Context.RenderMap())
	}
	if opts.NewRunLogger == nil {
		return fmt.Errorf("run logger factory is nil")
	}
	runLogger, err := opts.NewRunLogger(request.WorkflowPath, strings.TrimSpace(opts.WorkflowSource), strings.TrimSpace(opts.Scenario), strings.TrimSpace(opts.BundleRoot), request.SelectedPhase)
	if err != nil {
		return err
	}
	if err := emitApplyDiagnostic(opts, 1, logs.CLIEvent{Component: "apply", Event: "runlog_created", Attrs: map[string]any{"runlog": runLogger.Dir()}}); err != nil {
		return err
	}
	eventSink := combineStepEventSinks(runLogger.EventSink(), opts.AdditionalSink)
	defer func() {
		status := "ok"
		if err != nil {
			status = "failed"
		}
		closeErr := runLogger.CloseWithResult(status, err)
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	stateMetadata := install.StateMetadata{Workflow: install.StateWorkflow{Path: request.WorkflowPath, Source: strings.TrimSpace(opts.WorkflowSource)}}
	if request.Workflow != nil {
		stateMetadata.StateKey = request.Workflow.StateKey
		stateMetadata.Workflow.SHA256 = request.Workflow.WorkflowSHA256
	}
	if err := install.Run(ctx, request.ExecutionWorkflow, install.RunOptions{BundleRoot: opts.BundleRoot, StatePath: request.StatePath, StateMetadata: stateMetadata, DisableStateFallback: request.StateDirExplicit, StateMigrationSink: request.StateMigrationSink, Context: opts.Context, EventSink: eventSink, Fresh: request.Fresh, NonInteractive: opts.NonInteractive}); err != nil {
		return err
	}
	if opts.StdoutPrintln == nil {
		return nil
	}
	return opts.StdoutPrintln("apply: ok")
}

func writeApplyDryRun(stdoutPrintf func(format string, args ...any) error, request ExecutionRequest, context map[string]any) error {
	if stdoutPrintf == nil {
		return fmt.Errorf("stdout printf is nil")
	}
	wf := request.ExecutionWorkflow
	selectedPhaseName := request.SelectedPhase
	if wf == nil || len(wf.Phases) == 0 {
		if selectedPhaseName == "" {
			return errors.New("no phases found")
		}
		return fmt.Errorf("%s phase not found", selectedPhaseName)
	}

	state, err := LoadInstallDryRunState(request)
	if err != nil {
		return err
	}

	runtimeVars := map[string]any{}
	for key, value := range state.RuntimeVars {
		runtimeVars[key] = value
	}
	runtimeVars["host"] = install.CurrentHostFacts()

	completed := make(map[string]bool, len(state.CompletedPhases))
	for _, phaseName := range state.CompletedPhases {
		completed[phaseName] = true
	}

	for _, phase := range wf.Phases {
		if err := stdoutPrintf("PHASE=%s\n", phase.Name); err != nil {
			return err
		}
		if completed[phase.Name] {
			if err := stdoutPrintf("SKIP (completed phase)\n"); err != nil {
				return err
			}
			continue
		}
		for _, step := range phase.Steps {
			ok, evalErr := install.EvaluateWhenWithContext(step.When, wf.Vars, runtimeVars, context)
			if evalErr != nil {
				return fmt.Errorf("WHEN_EVAL_ERROR: step %s (%s): %w", step.ID, step.Kind, evalErr)
			}

			status := "PLAN"
			if !ok {
				status = "SKIP"
			}
			if err := stdoutPrintf("%s %s %s\n", step.ID, step.Kind, status); err != nil {
				return err
			}
		}
	}

	return nil
}

func combineStepEventSinks(sinks ...install.StepEventSink) install.StepEventSink {
	filtered := make([]install.StepEventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func(event install.StepEvent) {
		for _, sink := range filtered {
			sink(event)
		}
	}
}

func logApplyExecutionPlan(opts ExecuteOptions, request ExecutionRequest) error {
	wf := request.ExecutionWorkflow
	if wf == nil {
		return nil
	}
	phases := config.NormalizedPhases(wf)
	phaseCount, batchCount, stepCount, parallelBatchCount := summarizeApplyWorkflow(phases)
	if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "execution_plan", Attrs: map[string]any{"phases": phaseCount, "batches": batchCount, "steps": stepCount, "parallel_batches": parallelBatchCount}}); err != nil {
		return err
	}
	state, stateErr := LoadInstallDryRunState(request)
	if stateErr != nil {
		if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "state_snapshot_failed", Attrs: map[string]any{"state": request.StatePath, "error": stateErr}}); err != nil {
			return err
		}
	} else {
		if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "state_snapshot", Attrs: map[string]any{"state": request.StatePath, "completed_phases": len(state.CompletedPhases), "runtime_vars": len(state.RuntimeVars), "runtime_secrets": len(state.RuntimeSecrets)}}); err != nil {
			return err
		}
		if err := emitApplyDiagnostic(opts, 3, logs.CLIEvent{Level: "debug", Component: "apply", Event: "state_runtime", Attrs: map[string]any{"completed_phases": joinOrDash(cloneStrings(state.CompletedPhases)), "runtime_vars": joinOrDash(sortedAnyKeys(state.RuntimeVars)), "runtime_secrets": len(state.RuntimeSecrets)}}); err != nil {
			return err
		}
	}
	for _, phase := range phases {
		batches := workflowexec.BuildPhaseBatches(phase)
		if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "phase_plan", Attrs: map[string]any{"phase": phase.Name, "steps": len(phase.Steps), "batches": len(batches), "max_parallelism": phase.MaxParallelism}}); err != nil {
			return err
		}
		for _, batch := range batches {
			if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "batch_plan", Attrs: map[string]any{"phase": batch.PhaseName, "parallel_group": displayValueOrDash(batch.ParallelGroup), "parallel": batch.Parallel(), "steps": len(batch.Steps), "max_parallelism": batch.MaxParallelism}}); err != nil {
				return err
			}
			for _, step := range batch.Steps {
				if err := emitApplyDiagnostic(opts, 2, logs.CLIEvent{Level: "debug", Component: "apply", Event: "step_plan", Attrs: map[string]any{"phase": batch.PhaseName, "step": step.ID, "kind": step.Kind, "parallel_group": displayValueOrDash(step.ParallelGroup), "when": displayValueOrDash(step.When), "retry": step.Retry, "timeout": displayValueOrDash(step.Timeout), "register": len(step.Register)}}); err != nil {
					return err
				}
				if err := emitApplyDiagnostic(opts, 3, logs.CLIEvent{Level: "debug", Component: "apply", Event: "step_contract", Attrs: map[string]any{"phase": batch.PhaseName, "step": step.ID, "api_version": displayValueOrDash(step.APIVersion), "metadata_keys": joinOrDash(sortedAnyKeys(step.Metadata)), "register_keys": joinOrDash(sortedRegisterKeys(step.Register)), "spec_keys": joinOrDash(sortedAnyKeys(step.Spec))}}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func logApplyContext(opts ExecuteOptions, request ExecutionRequest) error {
	wf := request.Workflow
	if wf == nil {
		wf = request.ExecutionWorkflow
	}
	attrs := map[string]any{
		"workflow":        request.WorkflowPath,
		"workflow_source": displayValueOrDash(opts.WorkflowSource),
		"scenario":        displayValueOrDash(opts.Scenario),
		"selected_phase":  displayValueOrDash(request.SelectedPhase),
		"bundle":          displayValueOrDash(strings.TrimSpace(opts.BundleRoot)),
		"state":           displayValueOrDash(request.StatePath),
		"non_interactive": opts.NonInteractive,
	}
	if wf != nil {
		attrs["state_key"] = displayValueOrDash(wf.StateKey)
		attrs["workflow_sha256"] = displayValueOrDash(wf.WorkflowSHA256)
		attrs["vars"] = len(wf.Vars)
	}
	if err := emitApplyDiagnostic(opts, 3, logs.CLIEvent{Level: "debug", Component: "apply", Event: "execution_context", Attrs: attrs}); err != nil {
		return err
	}
	contextMap := opts.Context.RenderMap()
	if err := emitApplyDiagnostic(opts, 3, logs.CLIEvent{Level: "debug", Component: "apply", Event: "context_keys", Attrs: map[string]any{"keys": joinOrDash(sortedAnyKeys(contextMap))}}); err != nil {
		return err
	}
	if wf != nil {
		return emitApplyDiagnostic(opts, 3, logs.CLIEvent{Level: "debug", Component: "apply", Event: "workflow_vars", Attrs: map[string]any{"keys": joinOrDash(sortedAnyKeys(wf.Vars))}})
	}
	return nil
}

func summarizeApplyWorkflow(phases []config.Phase) (phaseCount int, batchCount int, stepCount int, parallelBatchCount int) {
	for _, phase := range phases {
		phaseCount++
		stepCount += len(phase.Steps)
		for _, batch := range workflowexec.BuildPhaseBatches(phase) {
			batchCount++
			if batch.Parallel() {
				parallelBatchCount++
			}
		}
	}
	return phaseCount, batchCount, stepCount, parallelBatchCount
}

func sortedAnyKeys(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := append([]string(nil), values...)
	slices.Sort(cloned)
	return cloned
}

func emitApplyDiagnostic(opts ExecuteOptions, level int, event logs.CLIEvent) error {
	if strings.TrimSpace(opts.InvocationID) != "" {
		attrs := make(map[string]any, len(event.Attrs)+1)
		for key, value := range event.Attrs {
			attrs[key] = value
		}
		attrs["invocation_id"] = strings.TrimSpace(opts.InvocationID)
		event.Attrs = attrs
	}
	return verboseEvent(opts.Verbosef, level, event)
}

func stateMigrationDiagnosticSink(component string, verbosef func(level int, format string, args ...any) error) func(source string, target string) {
	return func(source string, target string) {
		_ = verboseEvent(verbosef, 1, logs.CLIEvent{Component: strings.TrimSpace(component), Event: "state_migrated", Attrs: map[string]any{"source": source, "target": target}})
	}
}

func verboseEvent(fn func(level int, format string, args ...any) error, level int, event logs.CLIEvent) error {
	return logs.EmitCLIEventf(fn, level, event)
}
