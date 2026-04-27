package main

import (
	"context"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/applycli"
	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/lintcli"
)

func verboseApplyStepSink(env *cliEnv) install.StepEventSink {
	return func(event install.StepEvent) {
		_ = env.stderrPrintf("%s\n", formatWorkflowEventLine("apply", event))
	}
}

func displayValueOrDash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func resolvePlanWorkflowPath(ctx context.Context, workflowPath, scenario, source string) (string, error) {
	return applycli.ResolvePlanWorkflowPath(ctx, applycli.InvocationOptions{
		WorkflowPath:     workflowPath,
		Scenario:         scenario,
		Source:           source,
		DefaultLocalRoot: ".",
		ResolveScenario:  resolveScenarioWorkflowReference,
	})
}

func resolveApplyWorkflowAndBundle(ctx context.Context, opts applyOptions, positionalArgs []string) (string, string, error) {
	return applycli.ResolveApplyWorkflowAndBundle(ctx, applycli.InvocationOptions{
		WorkflowPath:    opts.workflowPath,
		Scenario:        opts.scenario,
		Source:          opts.source,
		PositionalArgs:  positionalArgs,
		ResolveScenario: resolveScenarioWorkflowReference,
	})
}

func executeLint(env *cliEnv, ctx context.Context, root string, file string, scenario string, output string) error {
	resolvedOutput, err := resolveOutputFormat(output)
	if err != nil {
		return err
	}
	return lintcli.Execute(ctx, lintcli.Options{
		Root:            root,
		File:            file,
		Scenario:        scenario,
		Output:          resolvedOutput,
		Verbosef:        env.verbosef,
		StdoutPrintf:    env.stdoutPrintf,
		JSONEncoderFunc: env.stdoutJSONEncoder,
		WorkflowRootDir: workflowRootDir,
		ScenarioDirName: workflowScenariosDir,
	})
}
