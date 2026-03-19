package schemadoc

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taedi90/deck/internal/workflowcontract"
	"github.com/taedi90/deck/schemas"
)

func TestRenderToolPageActionBasedUsesActionSectionsOnly(t *testing.T) {
	in := testToolPageInput(t, "Packages")
	rendered := string(RenderToolPage(in))

	if strings.Contains(rendered, "\n## Example\n") {
		t.Fatalf("action-based page should not render a page-level example:\n%s", rendered)
	}
	if strings.Contains(rendered, "\n## Fields\n") {
		t.Fatalf("action-based page should not render page-level fields:\n%s", rendered)
	}
	if !strings.Contains(rendered, "## Shared Step Fields") {
		t.Fatalf("expected shared step fields note:\n%s", rendered)
	}
	if !strings.Contains(rendered, "### `download`") || !strings.Contains(rendered, "### `install`") {
		t.Fatalf("expected action sections for Packages:\n%s", rendered)
	}
	if !strings.Contains(rendered, "`spec.repo.type`") || !strings.Contains(rendered, "`spec.backend.image`") {
		t.Fatalf("expected optional action-relevant download fields:\n%s", rendered)
	}
	if !strings.Contains(rendered, "`spec.source.type`") || !strings.Contains(rendered, "`spec.excludeRepos`") {
		t.Fatalf("expected install action fields:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[Workflow Schema](../workflow.md)") {
		t.Fatalf("expected workflow schema pointer:\n%s", rendered)
	}
	if strings.Contains(rendered, "- visibility:") || strings.Contains(rendered, "- category:") {
		t.Fatalf("action-based page should not render visibility/category:\n%s", rendered)
	}
}

func TestRenderToolPageNonActionUsesSingleExampleAndOptionalAPIVersion(t *testing.T) {
	in := testToolPageInput(t, "Checks")
	rendered := string(RenderToolPage(in))

	if !strings.Contains(rendered, "\n## Example\n") {
		t.Fatalf("expected single example section:\n%s", rendered)
	}
	if strings.Contains(rendered, "## Minimal Example") || strings.Contains(rendered, "## Realistic Example") {
		t.Fatalf("did not expect old example headings:\n%s", rendered)
	}
	if !strings.Contains(rendered, "| `apiVersion` | `string` | no |") {
		t.Fatalf("expected optional apiVersion row:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Optional step API version") {
		t.Fatalf("expected updated apiVersion description:\n%s", rendered)
	}
	if strings.Contains(rendered, "- visibility:") || strings.Contains(rendered, "- category:") {
		t.Fatalf("non-action page should not render visibility/category:\n%s", rendered)
	}
}

func TestRenderWorkflowPageUsesSingleExample(t *testing.T) {
	raw, err := schemas.WorkflowSchema()
	if err != nil {
		t.Fatalf("WorkflowSchema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal workflow schema: %v", err)
	}
	rendered := string(RenderWorkflowPage("schemas/deck-workflow.schema.json", schema, WorkflowMeta()))

	if !strings.Contains(rendered, "\n## Example\n") {
		t.Fatalf("expected single workflow example:\n%s", rendered)
	}
	if strings.Contains(rendered, "## Minimal Example") || strings.Contains(rendered, "## Realistic Example") {
		t.Fatalf("did not expect old workflow example headings:\n%s", rendered)
	}
}

func testToolPageInput(t *testing.T, kind string) PageInput {
	t.Helper()
	def, ok := workflowcontract.StepDefinitionForKind(kind)
	if !ok {
		t.Fatalf("missing step definition for %s", kind)
	}
	raw, err := schemas.ToolSchema(def.SchemaFile)
	if err != nil {
		t.Fatalf("ToolSchema(%q): %v", def.SchemaFile, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal tool schema %q: %v", def.SchemaFile, err)
	}
	properties, _ := schema["properties"].(map[string]any)
	spec, _ := properties["spec"].(map[string]any)
	actions := make([]string, 0, len(def.Actions))
	for _, action := range def.Actions {
		actions = append(actions, action.Name)
	}
	return PageInput{
		Kind:       kind,
		PageSlug:   strings.TrimSuffix(def.SchemaFile, ".schema.json"),
		Title:      kind,
		SchemaPath: filepath.ToSlash(filepath.Join("schemas", "tools", def.SchemaFile)),
		Schema:     schema,
		Meta:       ToolMeta(kind),
		Actions:    actions,
		Spec:       spec,
	}
}
