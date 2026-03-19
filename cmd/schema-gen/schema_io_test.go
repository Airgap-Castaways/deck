package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRegistrySchemaFilesRejectsUnknownSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.schema.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	err = ensureRegistrySchemaFiles(dir, entries)
	if err == nil {
		t.Fatalf("expected unknown schema error")
	}
	if !strings.Contains(err.Error(), "unknown tool schema file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
