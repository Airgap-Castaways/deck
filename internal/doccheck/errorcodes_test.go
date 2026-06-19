package doccheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// errCodeLiteral matches E_* codes in both the standalone string-literal form ("E_FOO")
// and the inline fmt.Errorf form ("E_FOO: message"). The trailing [":]  matches either
// a closing quote or a colon so both forms are captured by the same expression.
var errCodeLiteral = regexp.MustCompile(`"(E_[A-Z0-9_]+)[":]`)

// collectSourceErrorCodes scans internal/ and cmd/ .go files (excluding tests) for E_* string literals.
func collectSourceErrorCodes(t *testing.T, root string) map[string]bool {
	t.Helper()
	codes := map[string]bool{}
	for _, base := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, base), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range errCodeLiteral.FindAllStringSubmatch(string(b), -1) {
				codes[m[1]] = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
	return codes
}

func TestAllErrorCodesDocumented(t *testing.T) {
	root := repoRoot(t)
	codes := collectSourceErrorCodes(t, root)
	if len(codes) == 0 {
		t.Fatal("expected to find E_* codes in source")
	}

	catalog, err := os.ReadFile(filepath.Join(root, "docs", "diagnostics", "error-codes.md"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	text := string(catalog)

	var missing []string
	for code := range codes {
		if !strings.Contains(text, "`"+code+"`") {
			missing = append(missing, code)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("error codes missing from docs/diagnostics/error-codes.md:\n%s", strings.Join(missing, "\n"))
	}
}
