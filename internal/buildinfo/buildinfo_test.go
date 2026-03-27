package buildinfo

import "testing"

func TestCurrentUsesFallbacksAndParsesDirty(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalDirty := Dirty
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
		Dirty = originalDirty
	})

	Version = ""
	Commit = ""
	Date = ""
	Dirty = "true"

	info := Current()
	if info.Name != Name {
		t.Fatalf("unexpected name: %q", info.Name)
	}
	if info.Version != "dev" {
		t.Fatalf("unexpected version: %q", info.Version)
	}
	if info.Commit != "unknown" {
		t.Fatalf("unexpected commit: %q", info.Commit)
	}
	if info.Date != "unknown" {
		t.Fatalf("unexpected date: %q", info.Date)
	}
	if !info.Dirty {
		t.Fatalf("expected dirty build info")
	}
	if Summary() != "deck dev" {
		t.Fatalf("unexpected summary: %q", Summary())
	}
}
