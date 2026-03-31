package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := runMain(os.Args[1:]); err != nil {
		if _, writeErr := fmt.Fprintf(os.Stderr, "Error: %v\n", err); writeErr != nil {
			fmt.Fprintf(os.Stderr, "deck: %v\n", writeErr)
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
	message := fmt.Sprintf("Error: %v", err)
	if formatted == "" {
		return message + "\n"
	}
	if !strings.Contains(formatted, message) {
		formatted += "\n" + message
	}
	return formatted + "\n"
}
