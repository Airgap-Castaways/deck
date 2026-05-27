package operatorio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

type Interface interface {
	Message(level, message, stream string) error
	Confirm(ctx context.Context, message string, defaultValue *bool) (bool, error)
	Input(ctx context.Context, message string, opts InputOptions) (string, error)
}

type InputOptions struct {
	Default    string
	HasDefault bool
	Required   bool
	Secret     bool
}

type Terminal struct {
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	NonInteractive bool
	mu             sync.Mutex
	reader         *bufio.Reader
}

func Default() Interface {
	return &Terminal{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
}

func (t *Terminal) Message(level, message, stream string) error {
	writer := t.stdout()
	if strings.TrimSpace(stream) == "stderr" {
		writer = t.stderr()
	}
	text := strings.TrimRight(message, "\n")
	prefix := messagePrefix(level)
	if prefix != "" && text != "" {
		text = prefix + text
	}
	_, err := fmt.Fprintln(writer, text)
	return err
}

func (t *Terminal) Confirm(ctx context.Context, message string, defaultValue *bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if t.NonInteractive {
		if defaultValue == nil {
			return false, fmt.Errorf("non-interactive confirm requires spec.default")
		}
		return *defaultValue, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	reader := t.bufferedReader()
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if _, err := fmt.Fprint(t.stdout(), strings.TrimSpace(message), confirmSuffix(defaultValue), " "); err != nil {
			return false, err
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "" && defaultValue != nil {
			return *defaultValue, nil
		}
		switch answer {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		if err == io.EOF {
			return false, fmt.Errorf("confirm input ended before a valid answer")
		}
	}
}

func (t *Terminal) Input(ctx context.Context, message string, opts InputOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t.NonInteractive {
		if opts.Secret {
			return "", fmt.Errorf("non-interactive secret input requires an explicit secret provider")
		}
		if opts.HasDefault {
			return opts.Default, nil
		}
		if !opts.Required {
			return "", nil
		}
		return "", fmt.Errorf("non-interactive input requires spec.default or required=false")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	reader := t.bufferedReader()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		line, err := t.readInputLine(reader, message, opts)
		if err != nil && err != io.EOF {
			return "", err
		}
		value := strings.TrimRight(line, "\r\n")
		if value == "" && opts.HasDefault {
			return opts.Default, nil
		}
		if value != "" || !opts.Required {
			return value, nil
		}
		if err == io.EOF {
			return "", fmt.Errorf("input ended before a required value was provided")
		}
	}
}

func (t *Terminal) readInputLine(reader *bufio.Reader, message string, opts InputOptions) (string, error) {
	if _, err := fmt.Fprint(t.stdout(), strings.TrimSpace(message), inputSuffix(opts), " "); err != nil {
		return "", err
	}
	if !opts.Secret {
		return reader.ReadString('\n')
	}
	file, ok := t.stdin().(*os.File)
	if !ok {
		return reader.ReadString('\n')
	}
	fd, ok := terminalFD(file)
	if !ok || !term.IsTerminal(fd) {
		return reader.ReadString('\n')
	}
	password, err := term.ReadPassword(fd)
	if err != nil {
		return reader.ReadString('\n')
	}
	_, _ = fmt.Fprintln(t.stdout())
	return string(password) + "\n", nil
}

func (t *Terminal) stdin() io.Reader {
	if t.Stdin != nil {
		return t.Stdin
	}
	return os.Stdin
}

func (t *Terminal) stdout() io.Writer {
	if t.Stdout != nil {
		return t.Stdout
	}
	return os.Stdout
}

func (t *Terminal) stderr() io.Writer {
	if t.Stderr != nil {
		return t.Stderr
	}
	return os.Stderr
}

func (t *Terminal) bufferedReader() *bufio.Reader {
	if t.reader == nil {
		t.reader = bufio.NewReader(t.stdin())
	}
	return t.reader
}

func terminalFD(file *os.File) (int, bool) {
	fdRaw := file.Fd()
	if fdRaw > math.MaxInt {
		return 0, false
	}
	return int(fdRaw), true
}

func messagePrefix(level string) string {
	switch strings.TrimSpace(level) {
	case "warn":
		return "WARN: "
	case "error":
		return "ERROR: "
	default:
		return ""
	}
}

func confirmSuffix(defaultValue *bool) string {
	if defaultValue == nil {
		return " [y/n]"
	}
	if *defaultValue {
		return " [Y/n]"
	}
	return " [y/N]"
}

func inputSuffix(opts InputOptions) string {
	if opts.HasDefault {
		return fmt.Sprintf(" [%s]", opts.Default)
	}
	return ""
}
