package main

import (
	"strings"
	"time"

	"github.com/Airgap-Castaways/deck/internal/install"
	"github.com/Airgap-Castaways/deck/internal/logs"
)

func formatWorkflowEventLine(command string, event install.StepEvent, verbosity int, logFormat string) string {
	attrs := workflowEventAttrs(event)
	if strings.TrimSpace(logFormat) != "json" {
		attrs = filterWorkflowEventAttrs(attrs, verbosity)
	}
	return formatCLIEvent(logs.CLIEvent{
		TS:        eventTimestamp(event),
		Level:     eventLevel(event),
		Component: command,
		Event:     eventName(event),
		Attrs:     attrs,
	})
}

func workflowEventAttrs(event install.StepEvent) map[string]any {
	attrs := map[string]any{
		"phase":  displayValueOrDash(event.Phase),
		"status": displayValueOrDash(event.Status),
	}
	if event.InvocationID != "" {
		attrs["invocation_id"] = event.InvocationID
	}
	if event.StepID != "" {
		attrs["step"] = event.StepID
	}
	if event.Kind != "" {
		attrs["kind"] = event.Kind
	}
	if event.Attempt > 0 {
		attrs["attempt"] = event.Attempt
	}
	if event.BatchID != "" {
		attrs["batch"] = event.BatchID
	}
	if event.ParallelGroup != "" {
		attrs["parallel_group"] = event.ParallelGroup
	}
	if event.Parallel {
		attrs["parallel"] = true
	}
	if event.BatchSize > 0 {
		attrs["batch_size"] = event.BatchSize
	}
	if event.MaxParallelism > 0 {
		attrs["max_parallelism"] = event.MaxParallelism
	}
	if event.Reason != "" {
		attrs["reason"] = event.Reason
	}
	if event.Error != "" {
		attrs["error"] = event.Error
	}
	if event.FailedStep != "" {
		attrs["failed_step"] = event.FailedStep
	}
	if durationMS := formatEventDurationMS(event.StartedAt, event.EndedAt); durationMS >= 0 {
		attrs["duration_ms"] = durationMS
	}
	return attrs
}

func filterWorkflowEventAttrs(attrs map[string]any, verbosity int) map[string]any {
	if verbosity >= 2 {
		return attrs
	}
	allowed := map[string]bool{"phase": true, "step": true, "status": true, "reason": true}
	if verbosity >= 1 {
		allowed["kind"] = true
		allowed["duration_ms"] = true
		allowed["failed_step"] = true
		allowed["error"] = true
	}
	filtered := make(map[string]any, len(allowed))
	for key, value := range attrs {
		if allowed[key] {
			filtered[key] = value
		}
	}
	return filtered
}

func formatEventDurationMS(startedAt, endedAt string) int64 {
	started := strings.TrimSpace(startedAt)
	ended := strings.TrimSpace(endedAt)
	if started == "" || ended == "" {
		return -1
	}
	start, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return -1
	}
	end, err := time.Parse(time.RFC3339Nano, ended)
	if err != nil || end.Before(start) {
		return -1
	}
	return end.Sub(start).Milliseconds()
}

func eventTimestamp(event install.StepEvent) time.Time {
	for _, raw := range []string{event.EndedAt, event.StartedAt} {
		if raw == "" {
			continue
		}
		if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

func eventLevel(event install.StepEvent) string {
	if event.Status == "failed" || event.Error != "" {
		return "error"
	}
	return "info"
}

func eventName(event install.StepEvent) string {
	if event.Event != "" {
		return event.Event
	}
	if event.StepID == "" {
		return "batch_" + displayValueOrDash(event.Status)
	}
	return "step_" + displayValueOrDash(event.Status)
}
