package askretrieve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildStepspecSummaryParsesInlineAndHelperDefinitions(t *testing.T) {
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

	summary := buildStepspecSummary(root, "helper apply")
	for _, want := range []string{
		"- directory: internal/stepspec/*_meta.go",
		"- candidate step kind: HelperKind builder=apply.helper",
		"- candidate step kind: HelperKind builder=prepare.helper",
		"- candidate step kind: InlineKind builder=apply.inline",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary, got %q", want, summary)
		}
	}
	if strings.Index(summary, "HelperKind") > strings.Index(summary, "InlineKind") {
		t.Fatalf("expected prompt-matching helper entries to rank before inline entry, got %q", summary)
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
