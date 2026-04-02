package main

import (
	"bytes"
	"os"
	"strings"

	ctrllogs "github.com/Airgap-Castaways/deck/internal/logs"
)

func main() {
	if err := runMain(os.Args[1:]); err != nil {
		line := formatCLIEvent(ctrllogs.CLIEvent{Level: "error", Component: "cli", Event: "command_failed", Attrs: map[string]any{"error": err}})
		if _, writeErr := os.Stderr.WriteString(line + "\n"); writeErr != nil {
			fallback := ctrllogs.FormatCLIText(ctrllogs.CLIEvent{Level: "error", Component: "cli", Event: "stderr_write_failed", Attrs: map[string]any{"error": writeErr}})
			_, _ = os.Stderr.WriteString(fallback + "\n")
		}
		os.Exit(resolveExitCode(err))
	}
}

func runMain(args []string) error {
	root := newRootCommand()
	setCLIWriters(os.Stdout, os.Stderr)
	defer setCLIWriters(os.Stdout, os.Stderr)
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return err
}

func run(args []string) error {
	res := execute(args)
	if err := writeResult(res); err != nil {
		return err
	}
	return res.err
}

func execute(args []string) cliResult {
	root := newRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	setCLIWriters(&stdout, &stderr)
	defer setCLIWriters(os.Stdout, os.Stderr)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	if _, err := root.ExecuteC(); err != nil {
		res := errorResult(err)
		res.stdout = stdout.String() + res.stdout
		res.stderr = formatCLIError(stderr.String(), err)
		return res
	}
	return cliResult{stdout: stdout.String(), stderr: stderr.String()}
}

func formatCLIError(existing string, err error) string {
	formatted := strings.TrimRight(existing, "\n")
	message := formatCLIEvent(ctrllogs.CLIEvent{Level: "error", Component: "cli", Event: "command_failed", Attrs: map[string]any{"error": err}})
	if formatted == "" {
		return message + "\n"
	}
	if !strings.Contains(formatted, message) {
		formatted += "\n" + message
	}
	return formatted + "\n"
}
