package logs

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCLIText(t *testing.T) {
	line := FormatCLIText(CLIEvent{
		TS:        time.Date(2026, time.April, 2, 9, 20, 0, 0, time.UTC),
		Level:     "info",
		Component: "ask",
		Event:     "phase_started",
		Attrs: map[string]any{
			"phase":        "generation",
			"attempt":      1,
			"prompt":       "system line 1\nsystem line 2",
			"route":        "draft",
			"max_attempts": 3,
		},
	})
	for _, want := range []string{
		"ts=2026-04-02T09:20:00Z",
		"level=info",
		"component=ask",
		"event=phase_started",
		"attempt=1",
		"max_attempts=3",
		"phase=generation",
		"route=draft",
		`prompt="system line 1\nsystem line 2"`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in %q", want, line)
		}
	}
}
