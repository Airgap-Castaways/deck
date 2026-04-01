package main

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/prepare"
)

func verbosePrepareStepSink() prepare.StepEventSink {
	return func(event prepare.StepEvent) {
		_ = stderrPrintf("%s\n", formatStepProgressLine("prepare", event.StepID, event.Kind, event.Phase, strings.TrimSpace(event.Status), event.Attempt, event.Reason, event.Error, event.StartedAt, event.EndedAt))
	}
}
