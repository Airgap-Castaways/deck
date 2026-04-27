package main

import (
	"github.com/Airgap-Castaways/deck/internal/prepare"
)

func verbosePrepareStepSink(env *cliEnv) prepare.StepEventSink {
	return func(event prepare.StepEvent) {
		_ = env.stderrPrintf("%s\n", formatWorkflowEventLine("prepare", event))
	}
}
