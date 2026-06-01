package main

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/install"
	ctrllogs "github.com/Airgap-Castaways/deck/internal/logs"
)

func TestFormatWorkflowEventLineFiltersTextFieldsByVerbosity(t *testing.T) {
	ctrllogs.SetCLIFormat("text")
	ctrllogs.SetCLIColorEnabled(false)
	event := install.StepEvent{
		InvocationID:   "apply-1234",
		Event:          "step_skipped",
		StepID:         "join-worker",
		Kind:           "Command",
		Phase:          "install",
		Status:         "skipped",
		Reason:         "completed",
		Attempt:        1,
		StartedAt:      "2026-04-02T09:20:00Z",
		EndedAt:        "2026-04-02T09:20:01Z",
		BatchID:        "install:workers",
		ParallelGroup:  "workers",
		Parallel:       true,
		BatchSize:      2,
		MaxParallelism: 2,
	}

	line := formatWorkflowEventLine("apply", event, 0, "text")
	for _, want := range []string{"phase=install", "step=join-worker", "status=skipped", "reason=completed"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected compact line to include %q, got %q", want, line)
		}
	}
	for _, hidden := range []string{"kind=Command", "duration_ms=", "batch=install:workers", "invocation_id=apply-1234", "parallel_group=workers"} {
		if strings.Contains(line, hidden) {
			t.Fatalf("expected compact line to hide %q, got %q", hidden, line)
		}
	}

	line = formatWorkflowEventLine("apply", event, 1, "text")
	for _, want := range []string{"kind=Command", "duration_ms=1000"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected level 1 line to include %q, got %q", want, line)
		}
	}
	for _, hidden := range []string{"batch=install:workers", "invocation_id=apply-1234", "parallel_group=workers"} {
		if strings.Contains(line, hidden) {
			t.Fatalf("expected level 1 line to hide %q, got %q", hidden, line)
		}
	}

	line = formatWorkflowEventLine("apply", event, 2, "text")
	for _, want := range []string{"batch=install:workers", "parallel_group=workers", "batch_size=2", "max_parallelism=2", "invocation_id=apply-1234"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected level 2 line to include %q, got %q", want, line)
		}
	}
}

func TestFormatWorkflowEventLineKeepsJSONFields(t *testing.T) {
	ctrllogs.SetCLIFormat("json")
	t.Cleanup(func() { ctrllogs.SetCLIFormat("text") })
	event := install.StepEvent{InvocationID: "apply-1234", Event: "step_started", StepID: "install", Kind: "Command", Phase: "bootstrap", Status: "started", Attempt: 1, BatchID: "bootstrap"}

	line := formatWorkflowEventLine("apply", event, 0, "json")
	for _, want := range []string{"\"invocation_id\":\"apply-1234\"", "\"batch\":\"bootstrap\"", "\"attempt\":1", "\"kind\":\"Command\""} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected JSON line to keep %q, got %q", want, line)
		}
	}
}
