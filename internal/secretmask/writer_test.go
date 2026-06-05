package secretmask

import (
	"bytes"
	"testing"
)

func TestWriterMasksSecretAcrossChunkBoundary(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, []string{"super-secret"})
	if _, err := w.Write([]byte("token=super-")); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	if _, err := w.Write([]byte("secret\n")); err != nil {
		t.Fatalf("write second chunk: %v", err)
	}
	flusher, ok := w.(interface{ Flush() error })
	if !ok {
		t.Fatalf("writer does not expose Flush")
	}
	if err := flusher.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := out.String(); got != "token=***\n" {
		t.Fatalf("unexpected masked output %q", got)
	}
}

func TestWriterFlushMasksUnterminatedLine(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, []string{"super-secret"})
	if _, err := w.Write([]byte("token=super-secret")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.String() != "" {
		t.Fatalf("unterminated line should remain buffered, got %q", out.String())
	}
	if err := w.(interface{ Flush() error }).Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := out.String(); got != "token=***" {
		t.Fatalf("unexpected masked output %q", got)
	}
}

func TestWriterPreservesWhitespaceInSecretValue(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, []string{" token "})
	if _, err := w.Write([]byte("value= token \n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.(interface{ Flush() error }).Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := out.String(); got != "value=***\n" {
		t.Fatalf("unexpected masked output %q", got)
	}
}
