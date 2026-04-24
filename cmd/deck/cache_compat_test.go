package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/userdirs"
)

func TestResolveLegacyDeckCacheRoot(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		legacyRoot, err := userdirs.LegacyCacheRoot()
		if err != nil {
			t.Fatalf("LegacyCacheRoot failed: %v", err)
		}
		if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
			t.Fatalf("mkdir legacy cache root: %v", err)
		}

		resolved, found, err := resolveLegacyDeckCacheRoot()
		if err != nil {
			t.Fatalf("resolveLegacyDeckCacheRoot failed: %v", err)
		}
		if !found {
			t.Fatal("expected legacy cache root to be found")
		}
		if resolved != legacyRoot {
			t.Fatalf("unexpected legacy cache root: got %q want %q", resolved, legacyRoot)
		}
	})

	t.Run("missing", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		resolved, found, err := resolveLegacyDeckCacheRoot()
		if err != nil {
			t.Fatalf("resolveLegacyDeckCacheRoot failed: %v", err)
		}
		if found {
			t.Fatal("expected missing legacy cache root")
		}
		if resolved != "" {
			t.Fatalf("expected empty legacy cache root, got %q", resolved)
		}
	})

	t.Run("stat error", func(t *testing.T) {
		homeParent := t.TempDir()
		homeFile := filepath.Join(homeParent, "home-file")
		if err := os.WriteFile(homeFile, []byte("x"), 0o644); err != nil {
			t.Fatalf("write fake home file: %v", err)
		}
		t.Setenv("HOME", homeFile)

		resolved, found, err := resolveLegacyDeckCacheRoot()
		if err == nil {
			t.Fatal("expected stat error")
		}
		if found {
			t.Fatal("did not expect found on stat error")
		}
		if resolved != "" {
			t.Fatalf("expected empty legacy cache root on error, got %q", resolved)
		}
		if !strings.Contains(err.Error(), "stat legacy cache root") {
			t.Fatalf("expected stat error context, got %v", err)
		}
	})
}
