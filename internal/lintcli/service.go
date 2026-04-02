package lintcli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/logs"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Report struct {
	Status     string    `json:"status"`
	Mode       string    `json:"mode"`
	Root       string    `json:"root,omitempty"`
	Entrypoint string    `json:"entrypoint,omitempty"`
	Scenario   string    `json:"scenario,omitempty"`
	Workflows  []string  `json:"workflows"`
	Summary    Summary   `json:"summary"`
	Contracts  Contracts `json:"contracts"`
	Findings   []Finding `json:"findings"`
}

type Summary struct {
	WorkflowCount int `json:"workflowCount"`
	WarningCount  int `json:"warningCount"`
	ErrorCount    int `json:"errorCount"`
}

type Contracts struct {
	SupportedVersion string   `json:"supportedVersion"`
	SupportedModes   []string `json:"supportedModes"`
	TopLevelModes    []string `json:"topLevelModes"`
	ImportRule       string   `json:"importRule"`
	InvariantNotes   []string `json:"invariantNotes"`
}

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Path     string `json:"path,omitempty"`
	Phase    string `json:"phase,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type Options struct {
	Root            string
	File            string
	Scenario        string
	Output          string
	Verbosef        func(level int, format string, args ...any) error
	StdoutPrintf    func(format string, args ...any) error
	JSONEncoderFunc func() *json.Encoder
	WorkflowRootDir string
	ScenarioDirName string
}

func Execute(ctx context.Context, opts Options) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if opts.StdoutPrintf == nil {
		return fmt.Errorf("stdout printf is nil")
	}
	resolvedOutput := strings.TrimSpace(opts.Output)
	report, err := BuildReport(ctx, opts)
	if err != nil {
		return err
	}
	if err := logReport(opts.Verbosef, report); err != nil {
		return err
	}
	if resolvedOutput == "json" {
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
	if ctx == nil {
		return Report{}, fmt.Errorf("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	resolvedFile := strings.TrimSpace(opts.File)
	resolvedScenario := strings.TrimSpace(opts.Scenario)
	resolvedRoot := strings.TrimSpace(opts.Root)
	if resolvedRoot == "" {
		resolvedRoot = "."
	}
	if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "lint", Event: "lint_requested", Attrs: map[string]any{"root": resolvedRoot, "file": resolvedFile, "scenario": resolvedScenario}}); err != nil {
		return Report{}, err
	}
	if resolvedScenario != "" {
		if resolvedFile != "" {
			return Report{}, fmt.Errorf("lint accepts either --file or a scenario name, not both")
		}
		resolvedPath, err := resolveScenarioPath(resolvedRoot, resolvedScenario, opts.WorkflowRootDir, opts.ScenarioDirName)
		if err != nil {
			return Report{}, err
		}
		if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "lint", Event: "entrypoint_selected", Attrs: map[string]any{"entrypoint": resolvedPath}}); err != nil {
			return Report{}, err
		}
		files, err := validate.EntrypointWithContext(ctx, resolvedPath)
		if err != nil {
			return Report{}, err
		}
		return finalizeReport(ctx, Report{Mode: "scenario", Root: resolvedRoot, Entrypoint: resolvedPath, Scenario: resolvedScenario, Workflows: files, Summary: Summary{WorkflowCount: len(files)}})
	}
	if resolvedFile != "" {
		if isLocalComponentWorkflowPath(resolvedFile, opts.WorkflowRootDir) {
			return Report{}, fmt.Errorf("lint entrypoints must live under %s/%s/: %s", opts.WorkflowRootDir, opts.ScenarioDirName, resolvedFile)
		}
		if isLocalScenarioWorkflowPath(resolvedFile, opts.WorkflowRootDir, opts.ScenarioDirName) {
			if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "lint", Event: "entrypoint_selected", Attrs: map[string]any{"entrypoint": resolvedFile}}); err != nil {
				return Report{}, err
			}
			files, err := validate.EntrypointWithContext(ctx, resolvedFile)
			if err != nil {
				return Report{}, err
			}
			return finalizeReport(ctx, Report{Mode: "entrypoint", Entrypoint: resolvedFile, Workflows: files, Summary: Summary{WorkflowCount: len(files)}})
		}
		if err := validate.FileWithContext(ctx, resolvedFile); err != nil {
			return Report{}, err
		}
		wf, err := config.Load(ctx, resolvedFile)
		if err != nil {
			return Report{}, err
		}
		if err := validate.Workflow(resolvedFile, wf); err != nil {
			return Report{}, err
		}
		return finalizeReport(ctx, Report{Mode: "file", Entrypoint: resolvedFile, Workflows: []string{resolvedFile}, Summary: Summary{WorkflowCount: 1}})
	}
	files, err := validate.WorkspaceWithContext(ctx, resolvedRoot)
	if err != nil {
		return Report{}, err
	}
	if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "lint", Event: "workspace_loaded", Attrs: map[string]any{"workspace": resolvedRoot, "workflows": len(files)}}); err != nil {
		return Report{}, err
	}
	return finalizeReport(ctx, Report{Mode: "workspace", Root: resolvedRoot, Workflows: files, Summary: Summary{WorkflowCount: len(files)}})
}

func finalizeReport(ctx context.Context, report Report) (Report, error) {
	if len(report.Workflows) == 0 {
		report.Workflows = []string{}
	}
	findings, err := validate.AnalyzeFilesWithContext(ctx, report.Workflows)
	if err != nil {
		return Report{}, err
	}
	report.Findings = make([]Finding, 0, len(findings))
	for _, finding := range findings {
		report.Findings = append(report.Findings, Finding{Severity: finding.Severity, Code: finding.Code, Message: finding.Message, Hint: finding.Hint, Path: finding.Path, Phase: finding.Phase, StepID: finding.StepID, Kind: finding.Kind})
	}
	report.Contracts = Contracts{SupportedVersion: validate.SupportedWorkflowVersion(), SupportedModes: validate.SupportedWorkflowRoles(), TopLevelModes: validate.WorkflowTopLevelModes(), ImportRule: validate.WorkflowImportRule(), InvariantNotes: validate.WorkflowInvariantNotes()}
	report.Summary.WorkflowCount = max(report.Summary.WorkflowCount, len(report.Workflows))
	for _, finding := range report.Findings {
		if strings.EqualFold(strings.TrimSpace(finding.Severity), "error") {
			report.Summary.ErrorCount++
		} else {
			report.Summary.WarningCount++
		}
	}
	report.Status = "ok"
	return report, nil
}

func writeTextReport(stdoutPrintf func(format string, args ...any) error, report Report) error {
	if report.Summary.WorkflowCount == 1 && report.Entrypoint != "" && report.Mode == "file" {
		if err := stdoutPrintf("lint: ok (%s)\n", report.Entrypoint); err != nil {
			return err
		}
	} else {
		if err := stdoutPrintf("lint: ok (%d workflows)\n", report.Summary.WorkflowCount); err != nil {
			return err
		}
	}
	return stdoutPrintf("SUMMARY mode=%s workflows=%d warnings=%d errors=%d supportedVersion=%s modes=%s topLevelModes=%s\n", report.Mode, report.Summary.WorkflowCount, report.Summary.WarningCount, report.Summary.ErrorCount, report.Contracts.SupportedVersion, strings.Join(report.Contracts.SupportedModes, ","), strings.Join(report.Contracts.TopLevelModes, ","))
}

func logReport(verbose func(level int, format string, args ...any) error, report Report) error {
	if err := verboseEvent(verbose, 2, logs.CLIEvent{Level: "debug", Component: "lint", Event: "summary", Attrs: map[string]any{"mode": report.Mode, "workflows": report.Summary.WorkflowCount, "warnings": report.Summary.WarningCount, "errors": report.Summary.ErrorCount, "version": report.Contracts.SupportedVersion}}); err != nil {
		return err
	}
	for _, workflow := range report.Workflows {
		if err := verboseEvent(verbose, 2, logs.CLIEvent{Level: "debug", Component: "lint", Event: "workflow", Attrs: map[string]any{"workflow": workflow}}); err != nil {
			return err
		}
	}
	if err := verboseEvent(verbose, 3, logs.CLIEvent{Level: "debug", Component: "lint", Event: "contract", Attrs: map[string]any{"import_rule": report.Contracts.ImportRule, "top_level_modes": strings.Join(report.Contracts.TopLevelModes, ",")}}); err != nil {
		return err
	}
	for _, note := range report.Contracts.InvariantNotes {
		if err := verboseEvent(verbose, 3, logs.CLIEvent{Level: "debug", Component: "lint", Event: "invariant", Attrs: map[string]any{"note": note}}); err != nil {
			return err
		}
	}
	for _, finding := range report.Findings {
		if err := verboseEvent(verbose, 2, logs.CLIEvent{Level: "debug", Component: "lint", Event: "finding", Attrs: map[string]any{"code": finding.Code, "severity": finding.Severity, "path": displayValueOrDash(finding.Path), "phase": displayValueOrDash(finding.Phase), "step": displayValueOrDash(finding.StepID), "kind": displayValueOrDash(finding.Kind)}}); err != nil {
			return err
		}
		if strings.TrimSpace(finding.Hint) != "" {
			if err := verboseEvent(verbose, 3, logs.CLIEvent{Level: "debug", Component: "lint", Event: "finding_hint", Attrs: map[string]any{"code": finding.Code, "hint": finding.Hint}}); err != nil {
				return err
			}
		}
	}
	return nil
}

func verboseEvent(fn func(level int, format string, args ...any) error, level int, event logs.CLIEvent) error {
	line, err := logs.RenderDefaultCLI(event)
	if err != nil {
		line = logs.FormatCLIText(logs.CLIEvent{Level: "error", Component: "lint", Event: "log_render_failed", Attrs: map[string]any{"error": err.Error(), "original_event": event.Event}})
	}
	return verbosef(fn, level, "%s\n", line)
}

func resolveScenarioPath(root, scenario, workflowRootDir, scenarioDirName string) (string, error) {
	trimmed := strings.TrimSpace(scenario)
	if trimmed == "" {
		return "", fmt.Errorf("scenario name is required")
	}
	if strings.Contains(trimmed, "..") || strings.Contains(trimmed, "\\") || strings.Contains(trimmed, "/") {
		return "", fmt.Errorf("scenario shorthand must not contain path separators: %s", trimmed)
	}
	resolvedRoot := strings.TrimSpace(root)
	if resolvedRoot == "" {
		resolvedRoot = "."
	}
	workflowDir := workspacepaths.WorkflowScenariosPath(resolvedRoot)
	if strings.TrimSpace(workflowRootDir) != "" && workflowRootDir != workspacepaths.WorkflowRootDir {
		workflowDir = filepath.Join(resolvedRoot, workflowRootDir, scenarioDirName)
	}
	for _, suffix := range []string{"", ".yaml", ".yml"} {
		candidate := filepath.Join(workflowDir, trimmed+suffix)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("scenario not found under %s: %s", workflowDir, trimmed)
}

func isLocalComponentWorkflowPath(path, _ string) bool {
	return workspacepaths.IsComponentWorkflowPath(path)
}

func isLocalScenarioWorkflowPath(path, workflowRootDir, scenarioDirName string) bool {
	if (strings.TrimSpace(workflowRootDir) == "" || workflowRootDir == workspacepaths.WorkflowRootDir) && (strings.TrimSpace(scenarioDirName) == "" || scenarioDirName == workspacepaths.WorkflowScenariosDir) {
		return workspacepaths.IsScenarioWorkflowPath(path)
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return false
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	marker := string(filepath.Separator) + workflowRootDir + string(filepath.Separator) + scenarioDirName + string(filepath.Separator)
	return strings.Contains(resolved, marker)
}

func verbosef(fn func(level int, format string, args ...any) error, level int, format string, args ...any) error {
	if fn == nil {
		return nil
	}
	return fn(level, format, args...)
}

func displayValueOrDash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
