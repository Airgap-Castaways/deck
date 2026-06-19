package doccheck

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gitHashObject returns the git blob SHA-1 for a file, computed in pure Go.
// This matches `git hash-object <path>` (blob header + content) without
// spawning an external git process — faster, portable, and one fewer external
// dependency in the test environment. The translator (translate-docs.mjs)
// records the same hash via `git hash-object`, so the two stay in sync.
func gitHashObject(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file for hash %s: %v", path, err)
	}
	// git blob IDs are SHA-1 by definition; this is a content address, not a
	// security control. hash.Hash.Write never returns an error.
	h := sha1.New()
	_, _ = io.WriteString(h, fmt.Sprintf("blob %d\x00", len(data)))
	_, _ = h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// readSourceHash extracts `source:` and `source_hash:` from a translated file's
// YAML frontmatter. Returns ("","") when there is no frontmatter.
func readSourceHash(b []byte) (source string, hash string) {
	s := string(b)
	if !strings.HasPrefix(s, "---") {
		return "", ""
	}
	end := strings.Index(s[3:], "---")
	if end < 0 {
		return "", ""
	}
	for _, line := range strings.Split(s[3:3+end], "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "source:"); ok {
			source = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "source_hash:"); ok {
			hash = strings.TrimSpace(v)
		}
	}
	return source, hash
}

func TestReadSourceHashParsesFrontmatter(t *testing.T) {
	src, hash := readSourceHash([]byte("---\nsource: docs/x.md\nsource_hash: abc123\n---\n# 제목\n"))
	if src != "docs/x.md" || hash != "abc123" {
		t.Fatalf("got source=%q hash=%q", src, hash)
	}
	if s, h := readSourceHash([]byte("# no frontmatter\n")); s != "" || h != "" {
		t.Fatalf("expected empty for no-frontmatter, got %q %q", s, h)
	}
}

const koDocsDir = "website/i18n/ko/docusaurus-plugin-content-docs/current"

func TestI18nKoDrift(t *testing.T) {
	root := repoRoot(t)
	koRoot := filepath.Join(root, koDocsDir)
	if _, err := os.Stat(koRoot); err != nil {
		t.Skip("no Korean translations yet; nothing to check")
	}
	var stale, orphan []string
	if walkErr := filepath.Walk(koRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		source, recorded := readSourceHash(b)
		if source == "" || recorded == "" {
			orphan = append(orphan, rel+" (missing source/source_hash frontmatter)")
			return nil
		}
		srcPath := filepath.Join(root, source)
		if _, err2 := os.Stat(srcPath); err2 != nil {
			stale = append(stale, rel+" -> source missing: "+source)
			return nil //nolint:nilerr // source-missing is a test failure, not a walk error
		}
		if got := gitHashObject(t, srcPath); got != recorded {
			stale = append(stale, rel+" -> source changed (recorded "+recorded[:min(8, len(recorded))]+", now "+got[:8]+"); re-run make docs-translate")
		}
		return nil
	}); walkErr != nil {
		t.Errorf("walk %s: %v", koDocsDir, walkErr)
	}
	if len(orphan) > 0 {
		t.Errorf("ko files without proper frontmatter:\n%s", strings.Join(orphan, "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("stale Korean translations (source changed since translation):\n%s", strings.Join(stale, "\n"))
	}
}
