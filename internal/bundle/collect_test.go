package bundle

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectArchive(t *testing.T) {
	t.Run("creates archive with bundle files", func(t *testing.T) {
		root := t.TempDir()
		bundleRoot := filepath.Join(root, "bundle")
		if err := os.MkdirAll(filepath.Join(bundleRoot, "files"), 0o755); err != nil {
			t.Fatalf("mkdir bundle files: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bundleRoot, "manifest.json"), []byte(`{"entries":[]}`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bundleRoot, "files", "a.txt"), []byte("hello"), 0o644); err != nil {
			t.Fatalf("write bundle file: %v", err)
		}

		out := filepath.Join(root, "bundle.tar")
		if err := CollectArchive(bundleRoot, out); err != nil {
			t.Fatalf("collect archive: %v", err)
		}

		names, err := tarEntryNames(out)
		if err != nil {
			t.Fatalf("read tar entries: %v", err)
		}
		if !containsName(names, "manifest.json") {
			t.Fatalf("expected manifest.json in archive, got %#v", names)
		}
		if !containsName(names, "files/a.txt") {
			t.Fatalf("expected files/a.txt in archive, got %#v", names)
		}
	})

	t.Run("excludes output when output inside bundle root", func(t *testing.T) {
		root := t.TempDir()
		bundleRoot := filepath.Join(root, "bundle")
		if err := os.MkdirAll(bundleRoot, 0o755); err != nil {
			t.Fatalf("mkdir bundle root: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bundleRoot, "manifest.json"), []byte(`{"entries":[]}`), 0o644); err != nil {
			t.Fatalf("write manifest: %v", err)
		}

		out := filepath.Join(bundleRoot, "bundle.tar")
		if err := CollectArchive(bundleRoot, out); err != nil {
			t.Fatalf("collect archive: %v", err)
		}

		names, err := tarEntryNames(out)
		if err != nil {
			t.Fatalf("read tar entries: %v", err)
		}
		for _, n := range names {
			if strings.Contains(n, "bundle.tar") {
				t.Fatalf("archive should not contain itself: %#v", names)
			}
		}
	})
}

func tarEntryNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		names = append(names, hdr.Name)
	}
	return names, nil
}

func containsName(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
