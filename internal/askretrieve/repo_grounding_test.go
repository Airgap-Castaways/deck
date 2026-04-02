package askretrieve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildStepspecSummaryParsesInlineAndHelperDefinitionsAsFacts(t *testing.T) {
	root := t.TempDir()
	stepspecDir := filepath.Join(root, "internal", "stepspec")
	if err := os.MkdirAll(stepspecDir, 0o755); err != nil {
		t.Fatalf("mkdir stepspec: %v", err)
	}
	content := `package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[Inline](stepmeta.Definition{
	Kind: "InlineKind",
	Ask: stepmeta.AskMetadata{
		Builders: []stepmeta.AuthoringBuilder{{ID: "apply.inline"}},
		ValidationHints: []stepmeta.ValidationHint{{ErrorContains: "bad", Fix: "fix"}},
	},
})

var _ = stepmeta.MustRegister[Helper](helperDefinition())

func helperDefinition() stepmeta.Definition {
	return stepmeta.Definition{
		Kind: "HelperKind",
		Ask: stepmeta.AskMetadata{
			Builders: []stepmeta.AuthoringBuilder{{ID: "prepare.helper"}, {ID: "apply.helper"}},
		},
	}
}
`
	if err := os.WriteFile(filepath.Join(stepspecDir, "sample_meta.go"), []byte(content), 0o600); err != nil {
		t.Fatalf("write sample meta: %v", err)
	}

	summary := buildStepspecSummary(root, strings.ToLower("Explain HelperKind InlineKind apply.helper"))
	for _, want := range []string{
		"- observed typed step kinds: 2",
		"- observed ask builders: 3",
		"- step fact: HelperKind builders=apply.helper",
		"- step fact: HelperKind builders=prepare.helper",
		"- step fact: InlineKind builders=apply.inline",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
	if strings.Contains(summary, "candidate step kind") {
		t.Fatalf("expected fact-only wording, got %q", summary)
	}
}

func TestBuildStepmetaSummaryUsesParsedBuilderAndValidationCounts(t *testing.T) {
	root := t.TempDir()
	stepspecDir := filepath.Join(root, "internal", "stepspec")
	if err := os.MkdirAll(stepspecDir, 0o755); err != nil {
		t.Fatalf("mkdir stepspec: %v", err)
	}
	content := `package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[Counted](stepmeta.Definition{
	Kind: "CountedKind",
	Ask: stepmeta.AskMetadata{
		Builders: []stepmeta.AuthoringBuilder{{ID: "apply.one"}, {ID: "apply.two"}},
		ValidationHints: []stepmeta.ValidationHint{{ErrorContains: "a", Fix: "x"}, {ErrorContains: "b", Fix: "y"}},
	},
})
`
	if err := os.WriteFile(filepath.Join(stepspecDir, "counted_meta.go"), []byte(content), 0o600); err != nil {
		t.Fatalf("write counted meta: %v", err)
	}

	summary := buildStepmetaSummary(root)
	for _, want := range []string{
		"- registered builder metadata blocks observed: 2",
		"- validation hints observed: 2",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in stepmeta summary, got %q", want, summary)
		}
	}
}

func TestBuildStepspecSummaryFallsBackToObservedFactsWithoutRankingLanguage(t *testing.T) {
	root := t.TempDir()
	stepspecDir := filepath.Join(root, "internal", "stepspec")
	if err := os.MkdirAll(stepspecDir, 0o755); err != nil {
		t.Fatalf("mkdir stepspec: %v", err)
	}
	content := `package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

var _ = stepmeta.MustRegister[Download](stepmeta.Definition{
	Kind: "DownloadPackage",
	Ask: stepmeta.AskMetadata{
		Builders: []stepmeta.AuthoringBuilder{{ID: "prepare.download-package"}},
	},
})
`
	if err := os.WriteFile(filepath.Join(stepspecDir, "download_package_meta.go"), []byte(content), 0o600); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	summary := buildStepspecSummary(root, strings.ToLower("Explain repository metadata"))
	if !strings.Contains(summary, "- observed step fact: DownloadPackage builders=prepare.download-package") {
		t.Fatalf("expected observed fact fallback, got %q", summary)
	}
	if strings.Contains(summary, "candidate step kind") {
		t.Fatalf("expected ranking language to be removed, got %q", summary)
	}
}
