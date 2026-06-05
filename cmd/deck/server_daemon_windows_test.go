//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestIsDetachedServerProcessMissingOnWindows(t *testing.T) {
	if !isDetachedServerProcessMissing(os.ErrProcessDone) {
		t.Fatalf("expected os.ErrProcessDone to be treated as missing")
	}
	if !isDetachedServerProcessMissing(fmt.Errorf("kill process: %w", windowsErrorInvalidParameter)) {
		t.Fatalf("expected invalid parameter errno to be treated as missing")
	}
	if isDetachedServerProcessMissing(syscall.ERROR_ACCESS_DENIED) {
		t.Fatalf("expected access denied to remain a hard error")
	}
}
