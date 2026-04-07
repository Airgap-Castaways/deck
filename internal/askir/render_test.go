package askir

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestRenderDocumentQuotesWholeValueTemplateScalars(t *testing.T) {
	content, err := renderDocument("workflows/scenarios/apply.yaml", askcontract.GeneratedDocument{
		Path: "workflows/scenarios/apply.yaml",
		Kind: "workflow",
		Workflow: &askcontract.WorkflowDocument{
			Version: "v1alpha1",
			Steps: []askcontract.WorkflowStep{{
				ID:   "init",
				Kind: "InitKubeadm",
				Spec: map[string]any{"kubernetesVersion": "{{ .vars.kubernetesVersion }}"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("render workflow with template scalar: %v", err)
	}
	if !strings.Contains(content, `kubernetesVersion: "{{ .vars.kubernetesVersion }}"`) {
		t.Fatalf("expected quoted whole-value template scalar, got %q", content)
	}
	if !strings.Contains(content, "\n  - id: init\n") {
		t.Fatalf("expected workflow output to use 2-space list indentation, got %q", content)
	}
}

func TestRenderDocumentPreservesBlockScalarTemplateLines(t *testing.T) {
	content, err := renderDocument("workflows/scenarios/apply.yaml", askcontract.GeneratedDocument{
		Path: "workflows/scenarios/apply.yaml",
		Kind: "workflow",
		Workflow: &askcontract.WorkflowDocument{
			Version: "v1alpha1",
			Steps: []askcontract.WorkflowStep{{
				ID:   "run",
				Kind: "Command",
				Spec: map[string]any{"command": []any{"bash", "-lc", "cat <<'EOF'\n{{ .vars.script }}\nEOF"}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("render workflow with block scalar command: %v", err)
	}
	if !strings.Contains(content, "{{ .vars.script }}") {
		t.Fatalf("expected block scalar content to be preserved, got %q", content)
	}
	if strings.Contains(content, `"{{ .vars.script }}"`) {
		t.Fatalf("did not expect block scalar line to be force-quoted, got %q", content)
	}
}

func TestNormalizeTemplateAliasesSupportsBracketIndexes(t *testing.T) {
	normalized := normalizeTemplateAliases("${{ vars.nodes[0].ip }}")
	if normalized != "{{ .vars.nodes[0].ip }}" {
		t.Fatalf("expected bracket-index alias normalization, got %q", normalized)
	}
}

func TestNormalizeTemplateAliasesNormalizesSimplePaths(t *testing.T) {
	normalized := normalizeTemplateAliases("{{ vars.name }} {{ runtime.host.os.family }}")
	if normalized != "{{ .vars.name }} {{ .runtime.host.os.family }}" {
		t.Fatalf("expected simple alias normalization, got %q", normalized)
	}
}

func TestNormalizeTemplateAliasesPreservesUnrelatedExpressions(t *testing.T) {
	input := `{{ printf "%s" vars.name }}`
	normalized := normalizeTemplateAliases(input)
	if normalized != input {
		t.Fatalf("expected unrelated expression to be preserved, got %q", normalized)
	}
}

func TestNormalizeTemplateAliasesPreservesControlActionsWhileNormalizingSimpleRefs(t *testing.T) {
	input := `{{ if vars.enabled }}{{ vars.name }}{{ end }}`
	normalized := normalizeTemplateAliases(input)
	if normalized != `{{ if vars.enabled }}{{ .vars.name }}{{ end }}` {
		t.Fatalf("expected simple ref normalization inside control flow, got %q", normalized)
	}
}
