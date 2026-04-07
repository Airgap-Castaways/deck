package logs

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCLIText(t *testing.T) {
	SetCLIColorEnabled(false)
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

func TestFormatCLITextWithColors(t *testing.T) {
	SetCLIColorEnabled(true)
	t.Cleanup(func() {
		SetCLIColorEnabled(false)
	})
	line := FormatCLIText(CLIEvent{
		TS:        time.Date(2026, time.April, 2, 9, 20, 0, 0, time.UTC),
		Level:     "error",
		Component: "apply",
		Event:     "step_failed",
		Attrs: map[string]any{
			"status": "failed",
			"phase":  "bootstrap",
			"step":   "kubeadm-init",
		},
	})
	for _, want := range []string{"\x1b[", "level", "error", "component", "apply", "event", "step_failed", "status", "failed"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in %q", want, line)
		}
	}
}

func TestWrapCLISubprocessWriter(t *testing.T) {
	SetCLIColorEnabled(true)
	t.Cleanup(func() {
		SetCLIColorEnabled(false)
	})
	var buf ansiTestBuffer
	wrapped := WrapCLISubprocessWriter("kubeadm", &buf)
	if wrapped == nil {
		t.Fatal("expected wrapped writer")
	}
	if _, err := wrapped.Write([]byte("line one\nline two")); err != nil {
		t.Fatalf("write wrapped output: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"\x1b[", "[kubeadm]", "line one", "line two"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
	if strings.Count(got, "[kubeadm]") != 2 {
		t.Fatalf("expected prefix for each line, got %q", got)
	}
}

func TestRenderCLIOrFallbackUsesEventComponent(t *testing.T) {
	SetCLIFormat("json")
	t.Cleanup(func() {
		SetCLIFormat("text")
	})
	line := RenderCLIOrFallback(CLIEvent{Component: "apply", Event: "step_started", Attrs: map[string]any{"payload": func() {}}})
	for _, want := range []string{"component=apply", "event=log_render_failed", "original_event=step_started"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in %q", want, line)
		}
	}
}

func TestEmitCLIEventf(t *testing.T) {
	var got string
	err := EmitCLIEventf(func(level int, format string, args ...any) error {
		if level != 2 {
			t.Fatalf("unexpected level: %d", level)
		}
		if format != "%s\n" {
			t.Fatalf("unexpected format: %q", format)
		}
		if len(args) != 1 {
			t.Fatalf("unexpected arg count: %d", len(args))
		}
		got, _ = args[0].(string)
		return nil
	}, 2, CLIEvent{Component: "cache", Event: "clean_planned"})
	if err != nil {
		t.Fatalf("EmitCLIEventf: %v", err)
	}
	if !strings.Contains(got, "component=cache") || !strings.Contains(got, "event=clean_planned") {
		t.Fatalf("unexpected rendered event: %q", got)
	}
	if err := EmitCLIEventf(nil, 1, CLIEvent{}); err != nil {
		t.Fatalf("EmitCLIEventf nil handler: %v", err)
	}
}

func TestWriteCLIEvent(t *testing.T) {
	var buf strings.Builder
	if err := WriteCLIEvent(&buf, CLIEvent{Component: "server", Event: "started"}); err != nil {
		t.Fatalf("WriteCLIEvent: %v", err)
	}
	if !strings.Contains(buf.String(), "component=server") || !strings.Contains(buf.String(), "event=started") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
	if err := WriteCLIEvent(nil, CLIEvent{}); err != nil {
		t.Fatalf("WriteCLIEvent nil writer: %v", err)
	}
}

type ansiTestBuffer struct {
	strings.Builder
}

func (ansiTestBuffer) SupportsANSI() bool {
	return true
}

func TestFormatCLIJSONIgnoresReservedAttrKeys(t *testing.T) {
	raw, err := FormatCLIJSON(CLIEvent{
		TS:        time.Date(2026, time.April, 2, 9, 20, 0, 0, time.UTC),
		Level:     "info",
		Component: "ask",
		Event:     "phase_started",
		Attrs: map[string]any{
			"event":     "shadowed",
			"component": "shadowed",
			"phase":     "generation",
		},
	})
	if err != nil {
		t.Fatalf("FormatCLIJSON: %v", err)
	}
	line := string(raw)
	if !strings.Contains(line, `"component":"ask"`) || !strings.Contains(line, `"event":"phase_started"`) {
		t.Fatalf("expected canonical component and event, got %s", line)
	}
	if strings.Contains(line, "shadowed") {
		t.Fatalf("expected reserved attrs to be filtered, got %s", line)
	}
}
