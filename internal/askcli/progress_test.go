package askcli

import (
	"strings"
	"testing"
)

func TestAskProgressFlushesStatus(t *testing.T) {
	writer := &flushCapture{}
	progress := newAskProgress(writer, "basic")
	progress.status("planning authoring workflow")
	if writer.flushes == 0 {
		t.Fatalf("expected progress status to flush output")
	}
	if !strings.Contains(writer.String(), "ask status: planning authoring workflow") {
		t.Fatalf("expected progress output, got %q", writer.String())
	}
}

func TestAskProgressDisabledWhenOff(t *testing.T) {
	writer := &flushCapture{}
	progress := newAskProgress(writer, "off")
	progress.status("planning authoring workflow")
	if writer.String() != "" || writer.flushes != 0 {
		t.Fatalf("expected no progress output, got %q with %d flushes", writer.String(), writer.flushes)
	}
}
