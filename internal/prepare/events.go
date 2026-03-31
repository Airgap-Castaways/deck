package prepare

import "github.com/Airgap-Castaways/deck/internal/install"

type StepEvent = install.StepEvent

type StepEventSink = install.StepEventSink

func emitStepEvent(sink StepEventSink, event StepEvent) {
	if sink == nil {
		return
	}
	sink(event)
}
