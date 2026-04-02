package main

import (
	"github.com/Airgap-Castaways/deck/internal/prepare"
)

func verbosePrepareStepSink() prepare.StepEventSink {
	return func(event prepare.StepEvent) {
		_ = stderrPrintf("%s\n", formatWorkflowEventLine("prepare", event))
	}
}
