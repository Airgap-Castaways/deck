package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	cliStdout    io.Writer = os.Stdout
	cliStderr    io.Writer = os.Stderr
	cliVerbosity int
)

type varFlag struct {
	values map[string]string
}

func (v *varFlag) Type() string {
	return "stringToString"
}

func (v *varFlag) String() string {
	if v == nil || len(v.values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(v.values))
	for key, value := range v.values {
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, ",")
}

func (v *varFlag) Set(raw string) error {
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return errors.New("--var must be key=value")
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return errors.New("--var must be key=value")
	}
	if v.values == nil {
		v.values = map[string]string{}
	}
	v.values[key] = parts[1]
	return nil
}

func (v *varFlag) AsMap() map[string]string {
	if v == nil || len(v.values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(v.values))
	for key, value := range v.values {
		cloned[key] = value
	}
	return cloned
}

func varsAsAnyMap(vars map[string]string) map[string]any {
	if len(vars) == 0 {
		return nil
	}
	converted := make(map[string]any, len(vars))
	for key, value := range vars {
		converted[key] = value
	}
	return converted
}

func setCLIWriters(stdout io.Writer, stderr io.Writer) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	cliStdout = stdout
	cliStderr = stderr
}

func stdoutWriter() io.Writer {
	return cliStdout
}

func stdoutJSONEncoder() *json.Encoder {
	return json.NewEncoder(stdoutWriter())
}

func setCLIVerbosity(level int) {
	if level < 0 {
		level = 0
	}
	cliVerbosity = level
}

func verbosef(level int, format string, args ...any) error {
	if cliVerbosity < level {
		return nil
	}
	_, err := fmt.Fprintf(cliStderr, format, args...)
	return err
}

func stdoutPrintf(format string, args ...any) error {
	_, err := fmt.Fprintf(stdoutWriter(), format, args...)
	return err
}

func stdoutPrintln(args ...any) error {
	_, err := fmt.Fprintln(stdoutWriter(), args...)
	return err
}

func stderrPrintf(format string, args ...any) error {
	_, err := fmt.Fprintf(cliStderr, format, args...)
	return err
}

func closeSilently(closer io.Closer) {
	_ = closer.Close()
}
