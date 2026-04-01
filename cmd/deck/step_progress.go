package main

import (
	"fmt"
	"strings"
	"time"
)

func formatStepProgressLine(command, stepID, kind, phase, status string, attempt int, reason, errText, startedAt, endedAt string) string {
	parts := []string{
		fmt.Sprintf("deck: %s step=%s", strings.TrimSpace(command), strings.TrimSpace(stepID)),
		fmt.Sprintf("kind=%s", strings.TrimSpace(kind)),
		fmt.Sprintf("phase=%s", displayValueOrDash(phase)),
		fmt.Sprintf("status=%s", displayValueOrDash(status)),
	}
	if attempt > 0 {
		parts = append(parts, fmt.Sprintf("attempt=%d", attempt))
	}
	if duration := formatEventDuration(startedAt, endedAt); duration != "" {
		parts = append(parts, fmt.Sprintf("duration=%s", duration))
	}
	if strings.TrimSpace(reason) != "" {
		parts = append(parts, fmt.Sprintf("reason=%s", strings.TrimSpace(reason)))
	}
	if strings.TrimSpace(errText) != "" {
		parts = append(parts, fmt.Sprintf("error=%s", strings.TrimSpace(errText)))
	}
	return strings.Join(parts, " ")
}

func formatEventDuration(startedAt, endedAt string) string {
	started := strings.TrimSpace(startedAt)
	ended := strings.TrimSpace(endedAt)
	if started == "" || ended == "" {
		return ""
	}
	start, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return ""
	}
	end, err := time.Parse(time.RFC3339Nano, ended)
	if err != nil || end.Before(start) {
		return ""
	}
	return end.Sub(start).String()
}
