package secretmask

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"sync"
)

const minSecretLength = 4

type Writer struct {
	mu      sync.Mutex
	writer  io.Writer
	secrets []string
	buffer  []byte
}

func NewWriter(writer io.Writer, secrets []string) io.Writer {
	filtered := normalizeSecrets(secrets)
	if writer == nil || len(filtered) == 0 {
		return writer
	}
	return &Writer{writer: writer, secrets: filtered}
}

func (w *Writer) Write(p []byte) (int, error) {
	if w == nil || w.writer == nil || len(p) == 0 {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer = append(w.buffer, p...)
	if err := w.flushCompleteLines(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *Writer) Flush() error {
	if w == nil || w.writer == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushAll()
}

func (w *Writer) flushCompleteLines() error {
	lastNewline := bytes.LastIndexByte(w.buffer, '\n')
	if lastNewline < 0 {
		return nil
	}
	return w.flushPrefix(lastNewline + 1)
}

func (w *Writer) flushAll() error {
	return w.flushPrefix(len(w.buffer))
}

func (w *Writer) flushPrefix(n int) error {
	if n <= 0 {
		return nil
	}
	masked := maskString(string(w.buffer[:n]), w.secrets)
	if _, err := io.WriteString(w.writer, masked); err != nil {
		return err
	}
	w.buffer = append([]byte(nil), w.buffer[n:]...)
	return nil
}

func normalizeSecrets(secrets []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if len(strings.TrimSpace(secret)) < minSecretLength || seen[secret] {
			continue
		}
		seen[secret] = true
		out = append(out, secret)
	}
	slices.SortFunc(out, func(a, b string) int { return len(b) - len(a) })
	return out
}

func maskString(text string, secrets []string) string {
	masked := text
	for _, secret := range secrets {
		masked = strings.ReplaceAll(masked, secret, "***")
	}
	return masked
}
