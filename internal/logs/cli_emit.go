package logs

import (
	"fmt"
	"io"
	"strings"
)

func RenderCLIOrFallback(event CLIEvent) string {
	line, err := RenderDefaultCLI(event)
	if err == nil {
		return line
	}
	component := strings.TrimSpace(event.Component)
	if component == "" {
		component = "cli"
	}
	return FormatCLIText(CLIEvent{
		TS:        event.TS,
		Level:     "error",
		Component: component,
		Event:     "log_render_failed",
		Attrs: map[string]any{
			"error":          err.Error(),
			"original_event": strings.TrimSpace(event.Event),
		},
	})
}

func EmitCLIEventf(fn func(level int, format string, args ...any) error, level int, event CLIEvent) error {
	if fn == nil {
		return nil
	}
	return fn(level, "%s\n", RenderCLIOrFallback(event))
}

func WriteCLIEvent(w io.Writer, event CLIEvent) error {
	if w == nil {
		return nil
	}
	_, err := fmt.Fprintf(w, "%s\n", RenderCLIOrFallback(event))
	return err
}
