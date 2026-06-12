package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Airgap-Castaways/deck/internal/applycli"
	"github.com/Airgap-Castaways/deck/internal/planvars"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type diffOptions struct {
	workflowPath  string
	scenario      string
	source        string
	stateDir      string
	selectedPhase string
	output        string
	varOverrides  map[string]string
	varsFiles     []string
}

type planVarsOptions struct {
	command       string
	workflowPath  string
	scenario      string
	source        string
	selectedPhase string
	preparedRoot  string
	output        string
	varOverrides  map[string]string
	varsFiles     []string
}

func newPlanCommand(env *cliEnv) *cobra.Command {
	vars := &varFlag{}
	varsFiles := &stringSliceFlag{}
	var stateDir string
	cmd := &cobra.Command{
		Use:     "plan",
		Aliases: []string{"diff"},
		Short:   "Show the planned apply step execution",
		RunE: func(cmd *cobra.Command, _ []string) error {
			workflowPath, err := cmdFlagValue(cmd, "workflow")
			if err != nil {
				return err
			}
			scenario, err := cmdFlagValue(cmd, "scenario")
			if err != nil {
				return err
			}
			source, err := cmdFlagValue(cmd, "source")
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
			return runDiffWithOptions(env, cmd.Context(), diffOptions{
				workflowPath:  workflowPath,
				scenario:      scenario,
				source:        source,
				stateDir:      stateDir,
				selectedPhase: selectedPhase,
				output:        output,
				varOverrides:  vars.AsMap(),
				varsFiles:     varsFiles.Values(),
			})
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().String("workflow", "", "path or URL to workflow file")
	cmd.Flags().String("scenario", "", "scenario name to plan")
	cmd.Flags().String("source", scenarioSourceLocal, "scenario source (local|server)")
	cmd.Flags().String("phase", "", "phase name to plan (defaults to all phases)")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "directory for apply state files (overrides local .deck/state/apply or remote XDG state)")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	cmd.Flags().VarP(varsFiles, "vars-file", "f", "vars file overlay relative to workflows/ (repeatable)")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	registerScenarioSourceCompletion(cmd, "source", false)
	registerScenarioNameCompletion(cmd, "scenario", "source", "", false)
	cmd.AddCommand(newPlanVarsCommand(env))
	return cmd
}

func newPlanVarsCommand(env *cliEnv) *cobra.Command {
	vars := &varFlag{}
	varsFiles := &stringSliceFlag{}
	cmd := &cobra.Command{
		Use:   "vars",
		Short: "Show effective vars, context, and initial runtime values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			command, err := cmdFlagValue(cmd, "command")
			if err != nil {
				return err
			}
			workflowPath, err := cmdFlagValue(cmd, "workflow")
			if err != nil {
				return err
			}
			scenario, err := cmdFlagValue(cmd, "scenario")
			if err != nil {
				return err
			}
			source, err := cmdFlagValue(cmd, "source")
			if err != nil {
				return err
			}
			selectedPhase, err := cmdFlagValue(cmd, "phase")
			if err != nil {
				return err
			}
			preparedRoot, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return runPlanVarsWithOptions(env, cmd.Context(), planVarsOptions{
				command:       command,
				workflowPath:  workflowPath,
				scenario:      scenario,
				source:        source,
				selectedPhase: selectedPhase,
				preparedRoot:  preparedRoot,
				output:        output,
				varOverrides:  vars.AsMap(),
				varsFiles:     varsFiles.Values(),
			})
		},
	}
	cmd.Flags().String("command", planvars.CommandApply, "execution command to inspect (apply|prepare)")
	cmd.Flags().String("workflow", "", "path or URL to apply workflow file")
	cmd.Flags().String("scenario", "", "scenario name to inspect for apply")
	cmd.Flags().String("source", scenarioSourceLocal, "scenario source for apply (local|server)")
	cmd.Flags().String("phase", "", "phase name to inspect (defaults to all phases)")
	cmd.Flags().String("root", workspacepaths.DefaultPreparedRoot("."), "prepared bundle output directory for --command prepare")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	cmd.Flags().VarP(varsFiles, "vars-file", "f", "vars file overlay relative to workflows/ (repeatable)")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	registerScenarioSourceCompletion(cmd, "source", false)
	registerScenarioNameCompletion(cmd, "scenario", "source", "", false)
	return cmd
}

func runPlanVarsWithOptions(env *cliEnv, ctx context.Context, opts planVarsOptions) error {
	resolvedOutput, err := resolveOutputFormat(opts.output)
	if err != nil {
		return err
	}
	command := strings.TrimSpace(opts.command)
	if command == "" {
		command = planvars.CommandApply
	}
	workflowPath := strings.TrimSpace(opts.workflowPath)
	if command == planvars.CommandApply {
		workflowPath, err = resolvePlanWorkflowPath(ctx, workflowPath, strings.TrimSpace(opts.scenario), strings.TrimSpace(opts.source))
		if err != nil {
			return err
		}
	}
	return planvars.Execute(ctx, planvars.Options{
		Command:         command,
		WorkflowPath:    workflowPath,
		Scenario:        strings.TrimSpace(opts.scenario),
		SelectedPhase:   strings.TrimSpace(opts.selectedPhase),
		PreparedRoot:    strings.TrimSpace(opts.preparedRoot),
		VarOverrides:    varsAsAnyMap(opts.varOverrides),
		VarsFiles:       append([]string(nil), opts.varsFiles...),
		Output:          resolvedOutput,
		StdoutPrintf:    env.stdoutPrintf,
		JSONEncoderFunc: env.stdoutJSONEncoder,
	})
}

func runDiffWithOptions(env *cliEnv, ctx context.Context, opts diffOptions) error {
	if err := env.commandStarted("plan"); err != nil {
		return err
	}
	workflowPath, err := resolvePlanWorkflowPath(ctx, strings.TrimSpace(opts.workflowPath), strings.TrimSpace(opts.scenario), strings.TrimSpace(opts.source))
	if err != nil {
		return err
	}
	selectedPhase := strings.TrimSpace(opts.selectedPhase)
	return executeDiff(env, ctx, workflowPath, strings.TrimSpace(opts.scenario), selectedPhase, opts.output, opts.stateDir, opts.varsFiles, varsAsAnyMap(opts.varOverrides))
}

func executeDiff(env *cliEnv, ctx context.Context, workflowPath, scenario, selectedPhase, output string, stateDir string, varsFiles []string, varOverrides map[string]any) error {
	stateDir = strings.TrimSpace(stateDir)
	return applycli.RunPlanCommand(ctx, applycli.PlanCommandOptions{
		WorkflowPath:     workflowPath,
		Scenario:         scenario,
		SelectedPhase:    selectedPhase,
		Output:           output,
		StateDir:         stateDir,
		StateDirExplicit: stateDir != "",
		VarOverrides:     varOverrides,
		VarsFiles:        append([]string(nil), varsFiles...),
		Verbosef:         env.verbosef,
		StdoutPrintf:     env.stdoutPrintf,
		JSONEncoderFunc:  env.stdoutJSONEncoder,
		ResolveOutput:    resolveOutputFormat,
	})
}

type applyOptions struct {
	workflowPath   string
	scenario       string
	source         string
	selectedPhase  string
	fresh          bool
	stateDir       string
	dryRun         bool
	nonInteractive bool
	varOverrides   map[string]string
	varsFiles      []string
	positional     []string
}

func newApplyCommand(env *cliEnv) *cobra.Command {
	vars := &varFlag{}
	varsFiles := &stringSliceFlag{}
	var stateDir string
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
			workflowPath, err := cmdFlagValue(cmd, "workflow")
			if err != nil {
				return err
			}
			scenario, err := cmdFlagValue(cmd, "scenario")
			if err != nil {
				return err
			}
			source, err := cmdFlagValue(cmd, "source")
			if err != nil {
				return err
			}
			selectedPhase, err := cmdFlagValue(cmd, "phase")
			if err != nil {
				return err
			}
			fresh, err := cmdFlagBoolValue(cmd, "fresh")
			if err != nil {
				return err
			}
			dryRun, err := cmdFlagBoolValue(cmd, "dry-run")
			if err != nil {
				return err
			}
			nonInteractive, err := cmdFlagBoolValue(cmd, "non-interactive")
			if err != nil {
				return err
			}
			return runApplyWithOptions(env, cmd.Context(), applyOptions{
				workflowPath:   workflowPath,
				scenario:       scenario,
				source:         source,
				selectedPhase:  selectedPhase,
				fresh:          fresh,
				stateDir:       stateDir,
				dryRun:         dryRun,
				nonInteractive: nonInteractive,
				varOverrides:   vars.AsMap(),
				varsFiles:      varsFiles.Values(),
				positional:     args,
			})
		},
	}
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().String("workflow", "", "path or URL to workflow file")
	cmd.Flags().String("scenario", "", "scenario name to execute")
	cmd.Flags().String("source", scenarioSourceLocal, "scenario source (local|server)")
	cmd.Flags().String("phase", "", "phase name to execute (defaults to all phases)")
	cmd.Flags().Bool("fresh", false, "clear saved apply state before execution")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "directory for apply state files (overrides local .deck/state/apply or remote XDG state)")
	cmd.Flags().Bool("dry-run", false, "print apply plan without executing steps")
	cmd.Flags().Bool("non-interactive", false, "fail or use defaults for operator interaction steps instead of prompting")
	cmd.Flags().VarP(varsFiles, "vars-file", "f", "vars file overlay relative to workflows/ (repeatable)")
	cmd.Flags().Var(vars, "var", "set variable override (key=value), repeatable")
	registerScenarioSourceCompletion(cmd, "source", false)
	registerScenarioNameCompletion(cmd, "scenario", "source", "", false)
	return cmd
}

func runApplyWithOptions(env *cliEnv, ctx context.Context, opts applyOptions) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if len(opts.positional) > 2 {
		return errors.New("apply accepts at most two positional arguments: [workflow] [bundle]")
	}
	positionalArgs := make([]string, 0, len(opts.positional))
	for _, arg := range opts.positional {
		positionalArgs = append(positionalArgs, strings.TrimSpace(arg))
	}
	if err := env.commandStarted("apply"); err != nil {
		return err
	}

	workflowPath, bundleRoot, err := resolveApplyWorkflowAndBundle(ctx, opts, positionalArgs)
	if err != nil {
		return err
	}
	invocationID := newInvocationID("apply")
	return applycli.RunApplyCommand(ctx, applycli.ApplyCommandOptions{
		WorkflowPath:     workflowPath,
		BundleRoot:       bundleRoot,
		WorkflowSource:   inferWorkflowSource(workflowPath, strings.TrimSpace(opts.source)),
		Scenario:         strings.TrimSpace(opts.scenario),
		SelectedPhase:    opts.selectedPhase,
		Fresh:            opts.fresh,
		StateDir:         strings.TrimSpace(opts.stateDir),
		StateDirExplicit: strings.TrimSpace(opts.stateDir) != "",
		DryRun:           opts.dryRun,
		NonInteractive:   opts.nonInteractive,
		VarOverrides:     varsAsAnyMap(opts.varOverrides),
		VarsFiles:        append([]string(nil), opts.varsFiles...),
		Verbosef:         env.verbosef,
		StdoutPrintf:     env.stdoutPrintf,
		StdoutPrintln:    env.stdoutPrintln,
		InvocationID:     invocationID,
		AdditionalSink:   verboseApplyStepSink(env, invocationID),
		NewRunLogger: func(workflowPath, workflowSource, scenario, bundleRoot, selectedPhase string) (applycli.RunLogger, error) {
			return newApplyRunLogger(env, workflowPath, workflowSource, scenario, bundleRoot, selectedPhase)
		},
	})
}
