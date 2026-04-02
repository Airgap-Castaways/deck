package install

type StepEvent struct {
	Event          string
	StepID         string
	Kind           string
	Phase          string
	Status         string
	Reason         string
	Attempt        int
	StartedAt      string
	EndedAt        string
	Error          string
	BatchID        string
	ParallelGroup  string
	Parallel       bool
	BatchSize      int
	MaxParallelism int
	FailedStep     string
}

type StepEventSink func(StepEvent)

func emitStepEvent(sink StepEventSink, event StepEvent) {
	if sink == nil {
		return
	}
	sink(event)
}
