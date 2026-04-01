package askcli

import (
	"strings"
	"testing"
)

func TestAskProgressFlushesStatus(t *testing.T) {
	writer := &flushCapture{}
	progress := askProgress{writer: writer, enabled: true}
	progress.status("planning authoring workflow")
	if writer.flushes == 0 {
		t.Fatalf("expected progress status to flush output")
	}
	if !strings.Contains(writer.String(), "ask status: planning authoring workflow") {
		t.Fatalf("expected progress output, got %q", writer.String())
	}
}
