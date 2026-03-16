package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/taedi90/deck/internal/applycli"
	"github.com/taedi90/deck/internal/config"
	"github.com/taedi90/deck/internal/filemode"
	"github.com/taedi90/deck/internal/install"
	"github.com/taedi90/deck/internal/validate"
)

type diffOptions struct {
	workflowPath  string
	server        string
	session       string
	apiToken      string
	selectedPhase string
	output        string
	varOverrides  map[string]string
}

func newPlanCommand() *cobra.Command {
	vars := &varFlag{}
	cmd := &cobra.Command{
		Use:     "plan",
		Aliases: []string{"diff"},
		Short:   "Show the planned apply step execution",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workflowPath, err := cmdFlagValue(cmd, "file")
			if err != nil {
				return err
			}
			server, err := cmdFlagValue(cmd, "server")
			if err != nil {
				return err
			}
			session, err := cmdFlagValue(cmd, "session")
			if err != nil {
				return err
			}
			apiToken, err := cmdFlagValue(cmd, "api-token")
			if err != nil {
				return err
			}
			selectedPhase, err := cmdFlagValue(cmd, "phase")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return runDiffWithOptions(cmd.Context(), diffOptions{
				workflowPath:  workflowPath,
				server:        server,
				session:       session,
				apiToken:      apiToken,
				selectedPhase: selectedPhase,
				output:        output,
				varOverrides:  vars.AsMap(),
			})
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringP("file", "f", "", "path to workflow file")
	cmd.Flags().String("server", "", "site server URL (defaults to saved server when --session is set)")
	cmd.Flags().String("session", "", "site session id for assisted mode")
	cmd.Flags().String("api-token", "", "bearer token for assisted site APIs (defaults to saved token)")
	cmd.Flags().String("phase", "", "phase name to plan (defaults to all phases)")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	return cmd
}

func runDiffWithOptions(ctx context.Context, opts diffOptions) error {
	assistedConfig, assistedMode, err := resolveAssistedExecutionConfig(opts.server, opts.session, opts.apiToken)
	if err != nil {
		return err
	}
	workflowPath := strings.TrimSpace(opts.workflowPath)
	selectedPhase := strings.TrimSpace(opts.selectedPhase)
	if assistedMode {
		return runAssistedAction(assistedConfig, "diff", func(assistedCtx assistedExecutionContext) error {
			return executeDiff(ctx, assistedCtx.WorkflowPath, selectedPhase, opts.output, varsAsAnyMap(opts.varOverrides))
		})
	}
	if workflowPath == "" {
		return errors.New("--file (or -f) is required")
	}
	return executeDiff(ctx, workflowPath, selectedPhase, opts.output, varsAsAnyMap(opts.varOverrides))
}

func executeDiff(ctx context.Context, workflowPath, selectedPhase, output string, varOverrides map[string]any) error {
	resolvedRequest, err := applycli.ResolveExecutionRequest(ctx, applycli.ExecutionRequestOptions{
		CommandName:                  "diff",
		WorkflowPath:                 workflowPath,
		VarOverrides:                 varOverrides,
		SelectedPhase:                selectedPhase,
		DefaultPhase:                 "",
		BuildExecutionWorkflow:       true,
		ResolveStatePath:             true,
		StatePathFromExecutionTarget: true,
	})
	if err != nil {
		return err
	}
	applyExecutionWorkflow := resolvedRequest.ExecutionWorkflow

	state, err := applycli.LoadInstallDryRunState(applyExecutionWorkflow)
	if err != nil {
		return err
	}
	completed := make(map[string]bool, len(state.CompletedSteps))
	for _, stepID := range state.CompletedSteps {
		completed[stepID] = true
	}
	runtimeVars := map[string]any{}
	for k, v := range state.RuntimeVars {
		runtimeVars[k] = v
	}
	statePath := resolvedRequest.StatePath
	ctxData := map[string]any{"bundleRoot": "", "stateFile": statePath}
	type diffStep struct {
		Phase  string `json:"phase,omitempty"`
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Action string `json:"action"`
		Reason string `json:"reason,omitempty"`
	}
	steps := make([]diffStep, 0)
	for _, phase := range applyExecutionWorkflow.Phases {
		for _, step := range phase.Steps {
			entry := diffStep{Phase: phase.Name, ID: step.ID, Kind: step.Kind}
			if completed[step.ID] {
				entry.Action = "skip"
				entry.Reason = "completed"
				steps = append(steps, entry)
				continue
			}
			ok, evalErr := install.EvaluateWhen(step.When, applyExecutionWorkflow.Vars, runtimeVars, ctxData)
			if evalErr != nil {
				return fmt.Errorf("WHEN_EVAL_ERROR: step %s (%s): %w", step.ID, step.Kind, evalErr)
			}
			if !ok {
				entry.Action = "skip"
				entry.Reason = "when"
				steps = append(steps, entry)
				continue
			}
			entry.Action = "run"
			steps = append(steps, entry)
		}
	}

	if output == "json" {
		payload := struct {
			Phase     string     `json:"phase"`
			StatePath string     `json:"statePath"`
			Steps     []diffStep `json:"steps"`
		}{Phase: resolvedRequest.SelectedPhase, StatePath: statePath, Steps: steps}
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(payload)
	}
	multiPhase := len(applyExecutionWorkflow.Phases) > 1
	currentPhase := ""
	for _, s := range steps {
		if multiPhase && s.Phase != currentPhase {
			currentPhase = s.Phase
			if err := stdoutPrintf("PHASE=%s\n", currentPhase); err != nil {
				return err
			}
		}
		if s.Action == "skip" && s.Reason != "" {
			if err := stdoutPrintf("%s %s SKIP (%s)\n", s.ID, s.Kind, s.Reason); err != nil {
				return err
			}
			continue
		}
		if err := stdoutPrintf("%s %s %s\n", s.ID, s.Kind, strings.ToUpper(s.Action)); err != nil {
			return err
		}
	}
	return nil
}

var doctorVarRefPattern = regexp.MustCompile(`\.vars\.([A-Za-z_][A-Za-z0-9_]*)`)

type doctorReport struct {
	Timestamp string         `json:"timestamp"`
	Workflow  string         `json:"workflow"`
	Summary   doctorSummary  `json:"summary"`
	Checks    []doctorCheck  `json:"checks"`
	Vars      map[string]any `json:"vars"`
}

type doctorSummary struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type doctorCheck struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Value   string   `json:"value"`
	Status  string   `json:"status"`
	Message string   `json:"message,omitempty"`
	UsedBy  []string `json:"used_by,omitempty"`
}

type doctorOptions struct {
	workflowPath string
	server       string
	session      string
	apiToken     string
	outPath      string
	varOverrides map[string]string
}

func newDoctorCommand() *cobra.Command {
	vars := &varFlag{}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check referenced artifact inputs before apply",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workflowPath, err := cmdFlagValue(cmd, "file")
			if err != nil {
				return err
			}
			server, err := cmdFlagValue(cmd, "server")
			if err != nil {
				return err
			}
			session, err := cmdFlagValue(cmd, "session")
			if err != nil {
				return err
			}
			apiToken, err := cmdFlagValue(cmd, "api-token")
			if err != nil {
				return err
			}
			outPath, err := cmdFlagValue(cmd, "out")
			if err != nil {
				return err
			}
			return runDoctorWithOptions(cmd.Context(), doctorOptions{
				workflowPath: workflowPath,
				server:       server,
				session:      session,
				apiToken:     apiToken,
				outPath:      outPath,
				varOverrides: vars.AsMap(),
			})
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringP("file", "f", "", "path or URL to workflow file")
	cmd.Flags().String("server", "", "site server URL (defaults to saved server when --session is set)")
	cmd.Flags().String("session", "", "site session id for assisted mode")
	cmd.Flags().String("api-token", "", "bearer token for assisted site APIs (defaults to saved token)")
	cmd.Flags().String("out", "", "output report path (required)")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	return cmd
}

func runDoctorWithOptions(ctx context.Context, opts doctorOptions) error {
	resolvedOut := strings.TrimSpace(opts.outPath)
	assistedConfig, assistedMode, err := resolveAssistedExecutionConfig(opts.server, opts.session, opts.apiToken)
	if err != nil {
		return err
	}
	if resolvedOut == "" && !assistedMode {
		return errors.New("--out is required")
	}
	if resolvedOut == "" && assistedMode {
		resolvedOut = filepath.Join(assistedDataRoot(), "reports", strings.TrimSpace(opts.session), "doctor-local.json")
	}

	if assistedMode {
		return runAssistedAction(assistedConfig, "doctor", func(assistedCtx assistedExecutionContext) error {
			return executeDoctor(ctx, assistedCtx.WorkflowPath, varsAsAnyMap(opts.varOverrides), resolvedOut)
		})
	}

	return executeDoctor(ctx, strings.TrimSpace(opts.workflowPath), varsAsAnyMap(opts.varOverrides), resolvedOut)
}

func executeDoctor(ctx context.Context, workflowPath string, varOverrides map[string]any, resolvedOut string) error {
	resolvedRequest, err := applycli.ResolveExecutionRequest(ctx, applycli.ExecutionRequestOptions{
		CommandName:                "doctor",
		WorkflowPath:               strings.TrimSpace(workflowPath),
		DiscoverWorkflow:           func(context.Context) (string, error) { return applycli.DiscoverApplyWorkflow(ctx, ".") },
		AllowRemoteWorkflow:        true,
		NormalizeLocalWorkflowPath: true,
		VarOverrides:               varOverrides,
	})
	if err != nil {
		return err
	}
	resolvedWorkflowPath := resolvedRequest.WorkflowPath
	wf := resolvedRequest.Workflow

	checks := make([]doctorCheck, 0)
	checkByName := map[string]*doctorCheck{}
	addCheck := func(c doctorCheck) {
		if existing, ok := checkByName[c.Name]; ok {
			existing.UsedBy = append(existing.UsedBy, c.UsedBy...)
			sort.Strings(existing.UsedBy)
			existing.UsedBy = dedupeStrings(existing.UsedBy)
			if existing.Status == "passed" && c.Status == "failed" {
				existing.Status = "failed"
				existing.Message = c.Message
				existing.Value = c.Value
				existing.Kind = c.Kind
			}
			return
		}
		checks = append(checks, c)
		checkByName[c.Name] = &checks[len(checks)-1]
	}

	refs := collectDoctorArtifactVarRefs(wf)
	for name, usedBy := range refs {
		v, ok := wf.Vars[name]
		if !ok {
			addCheck(doctorCheck{Name: "vars." + name, Kind: "var", Status: "failed", Message: "missing", UsedBy: usedBy})
			continue
		}
		s, ok := v.(string)
		if !ok {
			addCheck(doctorCheck{Name: "vars." + name, Kind: "var", Status: "failed", Message: "not a string", UsedBy: usedBy})
			continue
		}
		resolved := strings.TrimSpace(s)
		if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
			status, msg := doctorCheckHTTPReachable(resolved)
			addCheck(doctorCheck{Name: "vars." + name, Kind: "http", Value: resolved, Status: status, Message: msg, UsedBy: usedBy})
			continue
		}
		status, msg := doctorCheckPathExists(resolved)
		addCheck(doctorCheck{Name: "vars." + name, Kind: "path", Value: resolved, Status: status, Message: msg, UsedBy: usedBy})
	}

	report := doctorReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Workflow:  resolvedWorkflowPath,
		Checks:    checks,
		Vars:      wf.Vars,
	}
	for _, c := range checks {
		if c.Status == "failed" {
			report.Summary.Failed++
		} else {
			report.Summary.Passed++
		}
	}

	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode doctor report: %w", err)
	}
	if err := filemode.WritePrivateFile(resolvedOut, raw); err != nil {
		return fmt.Errorf("write doctor report: %w", err)
	}

	if err := stdoutPrintf("doctor: wrote %s\n", resolvedOut); err != nil {
		return err
	}
	if report.Summary.Failed > 0 {
		return fmt.Errorf("doctor: failed (%d failed checks)", report.Summary.Failed)
	}
	return nil
}

func collectDoctorArtifactVarRefs(wf *config.Workflow) map[string][]string {
	refs := map[string]map[string]bool{}
	if wf == nil {
		return map[string][]string{}
	}
	for _, phase := range wf.Phases {
		for _, step := range phase.Steps {
			fetchRaw, ok := step.Spec["fetch"].(map[string]any)
			if !ok {
				continue
			}
			sourcesRaw, ok := fetchRaw["sources"].([]any)
			if !ok {
				continue
			}
			for _, raw := range sourcesRaw {
				s, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				for _, key := range []string{"path", "url"} {
					vRaw, ok := s[key].(string)
					if !ok {
						continue
					}
					v := strings.TrimSpace(vRaw)
					if v == "" {
						continue
					}
					matches := doctorVarRefPattern.FindAllStringSubmatch(v, -1)
					for _, m := range matches {
						if len(m) != 2 {
							continue
						}
						name := m[1]
						if refs[name] == nil {
							refs[name] = map[string]bool{}
						}
						refs[name][step.ID] = true
					}
				}
			}
		}
	}
	out := map[string][]string{}
	for name, usedBy := range refs {
		steps := make([]string, 0, len(usedBy))
		for stepID := range usedBy {
			steps = append(steps, stepID)
		}
		sort.Strings(steps)
		out[name] = steps
	}
	return out
}

func doctorCheckPathExists(path string) (string, string) {
	if strings.TrimSpace(path) == "" {
		return "failed", "empty path"
	}
	if _, err := os.Stat(path); err != nil {
		return "failed", err.Error()
	}
	return "passed", ""
}

func doctorCheckHTTPReachable(url string) (string, string) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "failed", err.Error()
	}
	resp, err := client.Do(req)
	if err != nil {
		return "failed", err.Error()
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "failed", fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	return "passed", ""
}

type applyOptions struct {
	workflowPath  string
	server        string
	session       string
	apiToken      string
	selectedPhase string
	prefetch      bool
	dryRun        bool
	varOverrides  map[string]string
	positional    []string
}

func newApplyCommand() *cobra.Command {
	vars := &varFlag{}
	cmd := &cobra.Command{
		Use:   "apply [workflow] [bundle]",
		Short: "Execute an apply file against a bundle",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 2 {
				return errors.New("apply accepts at most two positional arguments: [workflow] [bundle]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowPath, err := cmdFlagValue(cmd, "file")
			if err != nil {
				return err
			}
			server, err := cmdFlagValue(cmd, "server")
			if err != nil {
				return err
			}
			session, err := cmdFlagValue(cmd, "session")
			if err != nil {
				return err
			}
			apiToken, err := cmdFlagValue(cmd, "api-token")
			if err != nil {
				return err
			}
			selectedPhase, err := cmdFlagValue(cmd, "phase")
			if err != nil {
				return err
			}
			prefetch, err := cmdFlagBoolValue(cmd, "prefetch")
			if err != nil {
				return err
			}
			dryRun, err := cmdFlagBoolValue(cmd, "dry-run")
			if err != nil {
				return err
			}
			return runApplyWithOptions(cmd.Context(), applyOptions{
				workflowPath:  workflowPath,
				server:        server,
				session:       session,
				apiToken:      apiToken,
				selectedPhase: selectedPhase,
				prefetch:      prefetch,
				dryRun:        dryRun,
				varOverrides:  vars.AsMap(),
				positional:    args,
			})
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringP("file", "f", "", "path or URL to workflow file")
	cmd.Flags().String("server", "", "site server URL (defaults to saved server when --session is set)")
	cmd.Flags().String("session", "", "site session id for assisted mode")
	cmd.Flags().String("api-token", "", "bearer token for assisted site APIs (defaults to saved token)")
	cmd.Flags().String("phase", "", "phase name to execute (defaults to all phases)")
	cmd.Flags().Bool("prefetch", false, "execute File download steps before other steps")
	cmd.Flags().Bool("dry-run", false, "print apply plan without executing steps")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	return cmd
}

func runApplyWithOptions(ctx context.Context, opts applyOptions) error {
	if len(opts.positional) > 2 {
		return errors.New("apply accepts at most two positional arguments: [workflow] [bundle]")
	}
	positionalArgs := make([]string, 0, len(opts.positional))
	for _, arg := range opts.positional {
		positionalArgs = append(positionalArgs, strings.TrimSpace(arg))
	}

	assistedConfig, assistedMode, err := resolveAssistedExecutionConfig(opts.server, opts.session, opts.apiToken)
	if err != nil {
		return err
	}
	if assistedMode {
		return runAssistedAction(assistedConfig, "apply", func(assistedCtx assistedExecutionContext) error {
			return executeApply(ctx, assistedCtx.WorkflowPath, assistedCtx.BundleRoot, strings.TrimSpace(opts.selectedPhase), opts.prefetch, opts.dryRun, varsAsAnyMap(opts.varOverrides))
		})
	}

	workflowPath, bundleRoot, err := applycli.ResolveWorkflowAndBundle(ctx, strings.TrimSpace(opts.workflowPath), positionalArgs)
	if err != nil {
		return err
	}
	return executeApply(ctx, workflowPath, bundleRoot, strings.TrimSpace(opts.selectedPhase), opts.prefetch, opts.dryRun, varsAsAnyMap(opts.varOverrides))
}

func executeApply(ctx context.Context, workflowPath, bundleRoot, selectedPhase string, prefetch, dryRun bool, varOverrides map[string]any) error {
	resolvedRequest, err := applycli.ResolveExecutionRequest(ctx, applycli.ExecutionRequestOptions{
		CommandName:                  "apply",
		WorkflowPath:                 workflowPath,
		AllowRemoteWorkflow:          true,
		VarOverrides:                 varOverrides,
		SelectedPhase:                selectedPhase,
		DefaultPhase:                 "",
		BuildExecutionWorkflow:       true,
		ResolveStatePath:             true,
		StatePathFromExecutionTarget: false,
	})
	if err != nil {
		return err
	}

	wf := resolvedRequest.Workflow
	applyExecutionWorkflow := resolvedRequest.ExecutionWorkflow
	statePath := resolvedRequest.StatePath
	if dryRun {
		return runApplyDryRun(applyExecutionWorkflow, resolvedRequest.SelectedPhase, bundleRoot)
	}

	if prefetch {
		prefetchWorkflow := applycli.BuildPrefetchWorkflow(wf)
		if len(prefetchWorkflow.Phases) > 0 && len(prefetchWorkflow.Phases[0].Steps) > 0 {
			if err := install.Run(ctx, prefetchWorkflow, install.RunOptions{BundleRoot: bundleRoot, StatePath: statePath}); err != nil {
				return err
			}
		}
	}

	if err := install.Run(ctx, applyExecutionWorkflow, install.RunOptions{BundleRoot: bundleRoot, StatePath: statePath}); err != nil {
		return err
	}

	return stdoutPrintln("apply: ok")
}

func runApplyDryRun(wf *config.Workflow, selectedPhaseName string, bundleRoot string) error {
	if wf == nil || len(wf.Phases) == 0 {
		if selectedPhaseName == "" {
			return errors.New("no phases found")
		}
		return fmt.Errorf("%s phase not found", selectedPhaseName)
	}

	state, err := applycli.LoadInstallDryRunState(wf)
	if err != nil {
		return err
	}

	runtimeVars := map[string]any{}
	for key, value := range state.RuntimeVars {
		runtimeVars[key] = value
	}

	completed := make(map[string]bool, len(state.CompletedSteps))
	for _, stepID := range state.CompletedSteps {
		completed[stepID] = true
	}

	statePath, err := applycli.ResolveInstallStatePath(wf)
	if err != nil {
		return err
	}
	ctxData := map[string]any{"bundleRoot": bundleRoot, "stateFile": statePath}

	for _, phase := range wf.Phases {
		if err := stdoutPrintf("PHASE=%s\n", phase.Name); err != nil {
			return err
		}
		for _, step := range phase.Steps {
			if completed[step.ID] {
				if err := stdoutPrintf("%s %s SKIP (completed)\n", step.ID, step.Kind); err != nil {
					return err
				}
				continue
			}

			ok, evalErr := install.EvaluateWhen(step.When, wf.Vars, runtimeVars, ctxData)
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

func executeLint(ctx context.Context, root string, file string, scenario string) error {
	resolvedFile := strings.TrimSpace(file)
	resolvedScenario := strings.TrimSpace(scenario)
	if resolvedScenario != "" {
		if resolvedFile != "" {
			return fmt.Errorf("lint accepts either --file or a scenario name, not both")
		}
		resolvedPath, err := resolveLintScenarioPath(root, resolvedScenario)
		if err != nil {
			return err
		}
		files, err := validate.Entrypoint(resolvedPath)
		if err != nil {
			return err
		}
		return stdoutPrintf("lint: ok (%d workflows)\n", len(files))
	}
	if resolvedFile != "" {
		if isLocalComponentWorkflowPath(resolvedFile) {
			return fmt.Errorf("lint entrypoints must live under workflows/scenarios/: %s", resolvedFile)
		}
		if isLocalScenarioWorkflowPath(resolvedFile) {
			files, err := validate.Entrypoint(resolvedFile)
			if err != nil {
				return err
			}
			return stdoutPrintf("lint: ok (%d workflows)\n", len(files))
		}
		if err := validate.File(resolvedFile); err != nil {
			return err
		}
		wf, err := config.Load(ctx, resolvedFile)
		if err != nil {
			return err
		}
		if err := validate.Workflow(resolvedFile, wf); err != nil {
			return err
		}
		return stdoutPrintf("lint: ok (%s)\n", resolvedFile)
	}

	files, err := validate.Workspace(root)
	if err != nil {
		return err
	}
	return stdoutPrintf("lint: ok (%d workflows)\n", len(files))
}

func resolveLintScenarioPath(root string, scenario string) (string, error) {
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
	workflowDir := filepath.Join(resolvedRoot, workflowRootDir, workflowScenariosDir)
	for _, suffix := range []string{"", ".yaml", ".yml"} {
		candidate := filepath.Join(workflowDir, trimmed+suffix)
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("scenario not found under %s: %s", workflowDir, trimmed)
}

func isLocalComponentWorkflowPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return false
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	marker := string(filepath.Separator) + workflowRootDir + string(filepath.Separator) + workflowComponentsDir + string(filepath.Separator)
	return strings.Contains(resolved, marker)
}

func isLocalScenarioWorkflowPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return false
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	marker := string(filepath.Separator) + workflowRootDir + string(filepath.Separator) + workflowScenariosDir + string(filepath.Separator)
	return strings.Contains(resolved, marker)
}
