package main

import (
	"github.com/Airgap-Castaways/deck/internal/prepare"
)

func verbosePrepareStepSink(env *cliEnv, invocationID string) prepare.StepEventSink {
	if env == nil || env.verbosity < 1 {
		return nil
	}
	return func(event prepare.StepEvent) {
		event.InvocationID = invocationID
		_ = env.stderrPrintf("%s\n", formatWorkflowEventLine("prepare", event))
	}
}
