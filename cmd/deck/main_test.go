package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBundleVerify(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		bundle := t.TempDir()
		if err := os.MkdirAll(filepath.Join(bundle, "files"), 0o755); err != nil {
			t.Fatalf("mkdir files: %v", err)
		}
		content := []byte("ok")
		if err := os.WriteFile(filepath.Join(bundle, "files", "a.txt"), content, 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		if err := writeManifestForMainTest(bundle, "files/a.txt", content); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if err := run([]string{"bundle", "verify", "--bundle", bundle}); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	t.Run("integrity failure", func(t *testing.T) {
		bundle := t.TempDir()
		if err := os.MkdirAll(filepath.Join(bundle, "files"), 0o755); err != nil {
			t.Fatalf("mkdir files: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bundle, "files", "a.txt"), []byte("ok"), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		if err := writeManifestForMainTest(bundle, "files/a.txt", []byte("different")); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		err := run([]string{"bundle", "verify", "--bundle", bundle})
		if err == nil {
			t.Fatalf("expected failure")
		}
		if !strings.Contains(err.Error(), "E_BUNDLE_INTEGRITY") {
			t.Fatalf("expected E_BUNDLE_INTEGRITY, got %v", err)
		}
	})
}

func writeManifestForMainTest(bundleRoot, rel string, content []byte) error {
	sum := sha256.Sum256(content)
	manifest := map[string]any{
		"entries": []any{map[string]any{
			"path":   rel,
			"sha256": hex.EncodeToString(sum[:]),
			"size":   len(content),
		}},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bundleRoot, "manifest.json"), raw, 0o644)
}
