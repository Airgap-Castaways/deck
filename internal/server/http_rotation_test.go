package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRotation(t *testing.T) {
	root := t.TempDir()
	logger, err := newAuditLogger(root, auditLoggerOptions{maxSizeBytes: 64, maxFiles: 2})
	if err != nil {
		t.Fatalf("newAuditLogger: %v", err)
	}

	first := "first-" + strings.Repeat("a", 120)
	second := "second-" + strings.Repeat("b", 120)
	third := "third-" + strings.Repeat("c", 120)

	logger.Write(map[string]any{"message": first})
	logger.Write(map[string]any{"message": second})
	logger.Write(map[string]any{"message": third})

	auditPath := filepath.Join(root, ".deck", "logs", "server-audit.log")
	current := mustReadFile(t, auditPath)
	rot1 := mustReadFile(t, auditPath+".1")
	rot2 := mustReadFile(t, auditPath+".2")

	if !strings.Contains(current, third) || strings.Contains(current, second) || strings.Contains(current, first) {
		t.Fatalf("unexpected current audit file contents: %q", current)
	}
	if !strings.Contains(rot1, second) || strings.Contains(rot1, first) {
		t.Fatalf("unexpected .1 audit file contents: %q", rot1)
	}
	if !strings.Contains(rot2, first) {
		t.Fatalf("unexpected .2 audit file contents: %q", rot2)
	}
	if _, err := os.Stat(auditPath + ".3"); !os.IsNotExist(err) {
		t.Fatalf("expected .3 to be removed, stat err=%v", err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}
