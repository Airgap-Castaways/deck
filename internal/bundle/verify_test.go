package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyManifest(t *testing.T) {
	t.Run("passes for valid manifest", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
			t.Fatalf("mkdir files: %v", err)
		}
		content := []byte("ok")
		if err := os.WriteFile(filepath.Join(root, "files", "a.txt"), content, 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := writeManifest(root, "files/a.txt", content); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		if err := VerifyManifest(root); err != nil {
			t.Fatalf("expected valid manifest, got %v", err)
		}
	})

	t.Run("fails on hash mismatch", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
			t.Fatalf("mkdir files: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "files", "a.txt"), []byte("ok"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if err := writeManifest(root, "files/a.txt", []byte("different")); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		err := VerifyManifest(root)
		if err == nil {
			t.Fatalf("expected integrity error")
		}
		if !strings.Contains(err.Error(), "E_BUNDLE_INTEGRITY") {
			t.Fatalf("expected E_BUNDLE_INTEGRITY, got %v", err)
		}
	})
}

func writeManifest(root, rel string, content []byte) error {
	sum := sha256.Sum256(content)
	mf := ManifestFile{Entries: []ManifestEntry{{
		Path:   rel,
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(content)),
	}}}
	raw, err := json.Marshal(mf)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0o644)
}
