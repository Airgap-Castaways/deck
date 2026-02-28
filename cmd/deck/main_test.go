package main

import (
	"archive/tar"
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

func TestRunBundleImport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		root := t.TempDir()
		archive := filepath.Join(root, "bundle.tar")
		dest := filepath.Join(root, "imported")

		if err := writeTarForMainTest(archive, []tarTestEntry{{name: "files/a.txt", body: []byte("hello")}}); err != nil {
			t.Fatalf("write tar: %v", err)
		}

		if err := run([]string{"bundle", "import", "--file", archive, "--dest", dest}); err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		raw, err := os.ReadFile(filepath.Join(dest, "files", "a.txt"))
		if err != nil {
			t.Fatalf("read imported file: %v", err)
		}
		if string(raw) != "hello" {
			t.Fatalf("unexpected imported content: %q", string(raw))
		}
	})

	t.Run("path traversal failure", func(t *testing.T) {
		root := t.TempDir()
		archive := filepath.Join(root, "bundle.tar")
		dest := filepath.Join(root, "imported")

		if err := writeTarForMainTest(archive, []tarTestEntry{{name: "../evil.txt", body: []byte("x")}}); err != nil {
			t.Fatalf("write tar: %v", err)
		}

		err := run([]string{"bundle", "import", "--file", archive, "--dest", dest})
		if err == nil {
			t.Fatalf("expected failure")
		}
		if !strings.Contains(err.Error(), "E_BUNDLE_IMPORT_PATH_TRAVERSAL") {
			t.Fatalf("expected traversal error code, got %v", err)
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

type tarTestEntry struct {
	name string
	body []byte
}

func writeTarForMainTest(path string, entries []tarTestEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err := tw.Write(e.body); err != nil {
			return err
		}
	}
	return nil
}
