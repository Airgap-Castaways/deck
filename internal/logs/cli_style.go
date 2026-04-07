package logs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	ansiReset      = "\x1b[0m"
	ansiBold       = "1"
	ansiDim        = "2"
	ansiRed        = "31"
	ansiGreen      = "32"
	ansiYellow     = "33"
	ansiBlue       = "34"
	ansiMagenta    = "35"
	ansiCyan       = "36"
	ansiBrightGray = "90"
)

var ansiResetBytes = []byte(ansiReset)

type ansiAwareWriter interface {
	SupportsANSI() bool
}

type subprocessPrefixWriter struct {
	mu          sync.Mutex
	writer      io.Writer
	prefix      string
	atLineStart bool
}

func SetCLIColorEnabled(enabled bool) {
	cliFormatMu.Lock()
	defer cliFormatMu.Unlock()
	defaultCLIColorEnabled = enabled
}

func CLIColorEnabled() bool {
	cliFormatMu.RLock()
	defer cliFormatMu.RUnlock()
	return defaultCLIColorEnabled
}

func WriterSupportsANSI(w io.Writer) bool {
	if w == nil {
		return false
	}
	if aware, ok := w.(ansiAwareWriter); ok {
		return aware.SupportsANSI()
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	term := strings.TrimSpace(os.Getenv("TERM"))
	if term == "" || strings.EqualFold(term, "dumb") {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func WrapCLISubprocessWriters(command string, stdout io.Writer, stderr io.Writer) (io.Writer, io.Writer) {
	return WrapCLISubprocessWriter(command, stdout), WrapCLISubprocessWriter(command, stderr)
}

func WrapCLISubprocessWriter(command string, writer io.Writer) io.Writer {
	if writer == nil || !CLIColorEnabled() || !WriterSupportsANSI(writer) {
		return writer
	}
	label := subprocessLabel(command)
	if label == "" {
		return writer
	}
	return &subprocessPrefixWriter{writer: writer, prefix: "[" + label + "] ", atLineStart: true}
}

func (w *subprocessPrefixWriter) Write(p []byte) (int, error) {
	if w == nil || w.writer == nil || len(p) == 0 {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	var out bytes.Buffer
	start := 0
	for start < len(p) {
		idx := bytes.IndexByte(p[start:], '\n')
		if idx < 0 {
			w.writeLine(&out, p[start:])
			break
		}
		chunkEnd := start + idx
		w.writeLine(&out, p[start:chunkEnd])
		out.WriteByte('\n')
		w.atLineStart = true
		start = chunkEnd + 1
	}
	if _, err := w.writer.Write(out.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *subprocessPrefixWriter) writeLine(out *bytes.Buffer, line []byte) {
	if w == nil || out == nil {
		return
	}
	if w.atLineStart {
		writeColorizedCLIBytes(out, []byte(w.prefix), ansiBrightGray)
		w.atLineStart = false
	}
	writeColorizedCLIBytes(out, line, ansiBrightGray)
}

func colorizeCLIText(raw string, code string) string {
	if raw == "" || code == "" || !CLIColorEnabled() {
		return raw
	}
	if strings.HasSuffix(raw, "\n") {
		trimmed := strings.TrimSuffix(raw, "\n")
		return colorizeCLIText(trimmed, code) + "\n"
	}
	return "\x1b[" + code + "m" + raw + ansiReset
}

func writeColorizedCLIBytes(out *bytes.Buffer, raw []byte, code string) {
	if out == nil || len(raw) == 0 {
		return
	}
	if code == "" || !CLIColorEnabled() {
		out.Write(raw)
		return
	}
	prefix := ansiCodePrefix(code)
	out.WriteString(prefix)
	remaining := raw
	for len(remaining) > 0 {
		idx := bytes.Index(remaining, ansiResetBytes)
		if idx < 0 {
			out.Write(remaining)
			break
		}
		out.Write(remaining[:idx])
		out.Write(ansiResetBytes)
		out.WriteString(prefix)
		remaining = remaining[idx+len(ansiResetBytes):]
	}
	out.Write(ansiResetBytes)
}

func ansiCodePrefix(code string) string {
	if code == "" {
		return ""
	}
	return "\x1b[" + code + "m"
}

func colorizeCLIField(key string, value any, keyCode string, valueCode string) string {
	formattedKey := key
	formattedValue := formatCLIValue(value)
	if CLIColorEnabled() {
		formattedKey = colorizeCLIText(key, keyCode)
		formattedValue = colorizeCLIText(formattedValue, valueCode)
	}
	return formattedKey + "=" + formattedValue
}

func cliFieldCodes(key string, value any) (string, string) {
	trimmedKey := strings.TrimSpace(key)
	textValue := strings.TrimSpace(valueAsString(value))
	keyCode := ansiBrightGray
	valueCode := ""
	if trimmedKey == "" {
		return keyCode, valueCode
	}
	if strings.EqualFold(trimmedKey, "ts") {
		return keyCode, ansiBrightGray
	}
	if strings.EqualFold(trimmedKey, "level") {
		return keyCode, cliLevelColor(textValue)
	}
	if strings.EqualFold(trimmedKey, "component") {
		return keyCode, ansiBlue
	}
	if strings.EqualFold(trimmedKey, "event") {
		return keyCode, ansiCyan
	}
	if strings.EqualFold(trimmedKey, "message") {
		return keyCode, ansiBold
	}
	return keyCode, cliAttrValueColor(trimmedKey, textValue)
}

func cliLevelColor(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return ansiRed
	case "warn", "warning":
		return ansiYellow
	case "debug":
		return ansiCyan
	default:
		return ansiGreen
	}
}

func cliAttrValueColor(key, value string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "status":
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "failed":
			return ansiRed
		case "succeeded", "ok", "completed":
			return ansiGreen
		case "started", "running":
			return ansiCyan
		case "skipped":
			return ansiBlue
		}
	case "phase":
		return ansiMagenta
	case "kind":
		return ansiCyan
	case "error", "failed_step":
		return ansiRed
	case "reason":
		return ansiYellow
	case "step":
		return ansiBold
	}
	return ""
}

func subprocessLabel(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	base := strings.ToLower(strings.TrimSpace(filepath.Base(trimmed)))
	if base == "" || base == "." {
		return ""
	}
	switch base {
	case "apt", "apt-get", "apk", "dnf", "dpkg", "rpm", "yum", "zypper":
		return "pkg"
	case "kubeadm":
		return "kubeadm"
	default:
		return "cmd"
	}
}
