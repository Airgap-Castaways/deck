package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserFacingCommandFlowDocsDoNotMentionDoctor(t *testing.T) {
	for _, rel := range []string{
		"README.md",
		"docs/reference/cli.md",
		"docs/guides/quick-start.md",
		"test/vagrant/README.md",
	} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			content := readRepoDoc(t, rel)
			if containsRemovedDoctorCommandReference(content) {
				t.Fatalf("%s must not mention removed doctor command", rel)
			}
		})
	}
}

func TestQuickStartDocsIncludeCurrentLifecycleCommands(t *testing.T) {
	for _, rel := range []string{"README.md", "README.ko.md", "docs/guides/quick-start.md"} {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			content := readRepoDoc(t, rel)
			for _, want := range []string{"deck init", "deck lint", "deck prepare", "deck bundle build", "deck apply"} {
				if !strings.Contains(content, want) {
					t.Fatalf("%s must include %q", rel, want)
				}
			}
		})
	}
}

func TestLocalizedReadmeKeepsGoRequirementInSync(t *testing.T) {
	english := readRepoDoc(t, "README.md")
	korean := readRepoDoc(t, "README.ko.md")
	if !strings.Contains(english, "Go 1.25.9") {
		t.Fatalf("README.md must include the canonical Go requirement")
	}
	if !strings.Contains(korean, "Go 1.25.9") {
		t.Fatalf("README.ko.md must include the canonical Go requirement")
	}
}

func TestLegacyCompatibilityDocsListCurrentCompatTests(t *testing.T) {
	content := readRepoDoc(t, "docs/contributing/legacy-compatibility.md")
	for _, rel := range []string{
		"cmd/deck/source_defaults_compat_test.go",
		"cmd/deck/cache_compat_test.go",
		"internal/install/state_compat_test.go",
		"internal/prepare/cache_compat_test.go",
	} {
		if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected compatibility test file %s: %v", rel, err)
		}
		if !strings.Contains(content, rel) {
			t.Fatalf("legacy compatibility docs must list %s", rel)
		}
	}
}

func readRepoDoc(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func containsRemovedDoctorCommandReference(content string) bool {
	lower := strings.ToLower(content)
	for _, pattern := range []string{
		"deck doctor",
		"-> doctor",
		"doctor ->",
		"- `doctor`",
		"* `doctor`",
		"## doctor",
		"### doctor",
	} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
