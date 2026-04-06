package prepare

import (
	"context"
	"testing"
)

func TestDetachedContextStripsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	detached := detachedContext(ctx)
	if detached == nil {
		t.Fatalf("expected detached context")
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("expected detached context to ignore cancellation, got %v", err)
	}
}
