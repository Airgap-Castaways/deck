package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	ctrllogs "github.com/Airgap-Castaways/deck/internal/logs"
)

type cliEnv struct {
	stdout    io.Writer
	stderr    io.Writer
	verbosity int
	logFormat string
}

func newCLIEnv(stdout io.Writer, stderr io.Writer) *cliEnv {
	env := &cliEnv{logFormat: "text"}
	env.setWriters(stdout, stderr)
	env.setLogFormat("text")
	return env
}

type varFlag struct {
	values map[string]string
}

type stringSliceFlag struct {
	values []string
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

func (s *stringSliceFlag) Type() string {
	return "stringSlice"
}

func (s *stringSliceFlag) String() string {
	if s == nil || len(s.values) == 0 {
		return ""
	}
	return strings.Join(s.values, ",")
}

func (s *stringSliceFlag) Set(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("value must not be empty")
	}
	s.values = append(s.values, trimmed)
	return nil
}

func (s *stringSliceFlag) Values() []string {
	if s == nil || len(s.values) == 0 {
		return nil
	}
	return append([]string(nil), s.values...)
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

func resolveOutputFormat(output string) (string, error) {
	resolvedOutput := strings.ToLower(strings.TrimSpace(output))
	if resolvedOutput == "" {
		resolvedOutput = "text"
	}
	if resolvedOutput != "text" && resolvedOutput != "json" {
		return "", errors.New("--output must be text or json")
	}
	return resolvedOutput, nil
}

func resolveCLILogFormat(format string) (string, error) {
	resolved := strings.ToLower(strings.TrimSpace(format))
	if resolved == "" {
		resolved = "text"
	}
	if resolved != "text" && resolved != "json" {
		return "", errors.New("--log-format must be text or json")
	}
	return resolved, nil
}

func (e *cliEnv) setWriters(stdout io.Writer, stderr io.Writer) {
	if e == nil {
		return
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	e.stdout = stdout
	e.stderr = stderr
	e.refreshStyle()
}

func (e *cliEnv) setLogFormat(format string) {
	if e == nil {
		return
	}
	resolved, err := resolveCLILogFormat(format)
	if err != nil {
		resolved = "text"
	}
	e.logFormat = resolved
	ctrllogs.SetCLIFormat(resolved)
	e.refreshStyle()
}

func (e *cliEnv) refreshStyle() {
	if e == nil {
		return
	}
	ctrllogs.SetCLIColorEnabled(e.logFormat == "text" && (ctrllogs.WriterSupportsANSI(e.stderr) || ctrllogs.WriterSupportsANSI(e.stdout)))
}

func (e *cliEnv) stdoutWriter() io.Writer {
	if e == nil || e.stdout == nil {
		return os.Stdout
	}
	return e.stdout
}

func (e *cliEnv) stdoutJSONEncoder() *json.Encoder {
	return json.NewEncoder(e.stdoutWriter())
}

func (e *cliEnv) setVerbosity(level int) {
	if e == nil {
		return
	}
	if level < 0 {
		level = 0
	}
	e.verbosity = level
}

func formatCLIEvent(event ctrllogs.CLIEvent) string {
	return ctrllogs.RenderCLIOrFallback(event)
}

func (e *cliEnv) stderrCLIEvent(event ctrllogs.CLIEvent) error {
	if e == nil || e.stderr == nil {
		return ctrllogs.WriteCLIEvent(os.Stderr, event)
	}
	return ctrllogs.WriteCLIEvent(e.stderr, event)
}

func (e *cliEnv) stdoutCLIEvent(event ctrllogs.CLIEvent) error {
	return ctrllogs.WriteCLIEvent(e.stdoutWriter(), event)
}

func (e *cliEnv) verboseCLIEvent(level int, event ctrllogs.CLIEvent) error {
	if e == nil || e.verbosity < level {
		return nil
	}
	return e.stderrCLIEvent(event)
}

func (e *cliEnv) verbosef(level int, format string, args ...any) error {
	if e == nil || e.verbosity < level {
		return nil
	}
	_, err := fmt.Fprintf(e.stderr, format, args...)
	return err
}

func (e *cliEnv) stdoutPrintf(format string, args ...any) error {
	_, err := fmt.Fprintf(e.stdoutWriter(), format, args...)
	return err
}

func (e *cliEnv) stdoutPrintln(args ...any) error {
	_, err := fmt.Fprintln(e.stdoutWriter(), args...)
	return err
}

func (e *cliEnv) stderrPrintf(format string, args ...any) error {
	if e == nil || e.stderr == nil {
		_, err := fmt.Fprintf(os.Stderr, format, args...)
		return err
	}
	_, err := fmt.Fprintf(e.stderr, format, args...)
	return err
}

func closeSilently(closer io.Closer) {
	_ = closer.Close()
}
