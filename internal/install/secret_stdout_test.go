package install

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunTimedCommandMasksSecretStdoutAndStderr(t *testing.T) {
	ctx := withSecretValues(context.Background(), []string{"super-secret"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runTimedCommandSpecWithContext(ctx, []string{"sh", "-c", "printf 'stdout=super-secret'; printf 'stderr=super-secret\\n' >&2"}, nil, false, time.Second, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if strings.Contains(stdout.String(), "super-secret") || strings.Contains(stderr.String(), "super-secret") {
		t.Fatalf("secret leaked stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if stdout.String() != "stdout=***" {
		t.Fatalf("unexpected stdout %q", stdout.String())
	}
	if stderr.String() != "stderr=***\n" {
		t.Fatalf("unexpected stderr %q", stderr.String())
	}
}
