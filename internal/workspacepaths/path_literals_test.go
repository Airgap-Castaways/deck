package workspacepaths

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestProductionCodeUsesWorkspacePathConstants(t *testing.T) {
	repoRoot := repoRoot(t)
	files := productionGoFiles(t, repoRoot)
	violations := make([]string, 0)
	for _, path := range files {
		violations = append(violations, fileLiteralViolations(t, repoRoot, path)...)
	}
	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	t.Fatalf("production Go files must reference canonical workflow paths through internal/workspacepaths, not string literals:\n%s", strings.Join(violations, "\n"))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func productionGoFiles(t *testing.T, repoRoot string) []string {
	t.Helper()
	paths := make([]string, 0)
	for _, dir := range []string{"cmd", "internal"} {
		root := filepath.Join(repoRoot, dir)
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if filepath.Clean(path) == filepath.Join(repoRoot, "internal", "workspacepaths") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return paths
}

func fileLiteralViolations(t *testing.T, repoRoot string, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	violations := make([]string, 0)
	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		for _, canonicalPath := range CanonicalWorkflowPaths() {
			if !strings.Contains(text, canonicalPath) {
				continue
			}
			pos := fset.Position(lit.Pos())
			rel, relErr := filepath.Rel(repoRoot, pos.Filename)
			if relErr != nil {
				rel = pos.Filename
			}
			violations = append(violations, fmt.Sprintf("- %s:%d contains %q", filepath.ToSlash(rel), pos.Line, canonicalPath))
		}
		return true
	})
	return violations
}
