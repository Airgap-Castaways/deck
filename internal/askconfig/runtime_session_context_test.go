package askconfig

import (
	"context"
	"errors"
	"testing"
)

func TestResolveRuntimeSessionWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, err := ResolveRuntimeSessionWithContext(ctx, "openai")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
