package schemadoc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/schemas"
)

func TestRenderGroupPageListsKindsByGroup(t *testing.T) {
	page := testGroupPageInput(t, "filesystem-content")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "- group: `filesystem-content`") {
		t.Fatalf("expected group summary header:\n%s", rendered)
	}
	for _, kind := range []string{"EnsureDirectory", "WriteFile", "CopyFile", "EditFile", "CreateSymlink"} {
		if !strings.Contains(rendered, "`"+kind+"`") {
			t.Fatalf("expected kind %s in group page:\n%s", kind, rendered)
		}
	}
	if strings.Contains(rendered, "family:") {
		t.Fatalf("did not expect family label in public group page:\n%s", rendered)
	}
}

func TestRenderGroupPageIncludesTypicalFlowsAndSeeAlso(t *testing.T) {
	page := testGroupPageInput(t, "runtime-services")
	rendered := string(RenderGroupPage(page))

	for _, want := range []string{"## Typical Flows", "### Configure containerd", "[Waits and Polling](waits-polling.md)", "[Step Envelope Contract](../workflow-model.md#step-envelope-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in runtime services page:\n%s", want, rendered)
		}
	}
}

func TestRenderGroupPageUsesConcreteExamplesAndValidationRules(t *testing.T) {
	page := testGroupPageInput(t, "artifact-staging")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "## `DownloadPackage`") || !strings.Contains(rendered, "kind: DownloadPackage") {
		t.Fatalf("expected concrete DownloadPackage section:\n%s", rendered)
	}
	if !strings.Contains(rendered, "At least one of `spec.source` or `spec.items` must be set.") {
		t.Fatalf("expected schema-derived branch rule in artifact staging page:\n%s", rendered)
	}
	if strings.Contains(rendered, "schema: `../../../schemas/tools/") {
		t.Fatalf("did not expect raw per-kind schema path in public group page:\n%s", rendered)
	}
}

func TestRenderGroupPageEscapesMultilineExamples(t *testing.T) {
	page := testGroupPageInput(t, "runtime-services")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "[Service]<br>Environment=NODE_IP={{ .vars.nodeIP }}") {
		t.Fatalf("expected multiline example to stay inside table cell:\n%s", rendered)
	}
}

func TestRenderGroupPagePreservesKindScopedNotes(t *testing.T) {
	rendered := string(RenderGroupPage(testGroupPageInput(t, "kubernetes-lifecycle")))
	loadSection := sectionForKind(rendered, "LoadImage")
	if strings.Contains(loadSection, "spec.auth") || strings.Contains(loadSection, "outputDir") {
		t.Fatalf("expected LoadImage notes to exclude DownloadImage-only guidance:\n%s", loadSection)
	}
	verifySection := sectionForKind(rendered, "CheckKubernetesCluster")
	if !strings.Contains(verifySection, "Use this for typed bootstrap and upgrade verification") {
		t.Fatalf("expected Kubernetes cluster check guidance:\n%s", verifySection)
	}
}

func TestRenderTypedStepsPageListsGroups(t *testing.T) {
	page := RenderTypedStepsPage([]PageInput{testGroupPageInput(t, "host-prep"), testGroupPageInput(t, "kubernetes-lifecycle")})
	rendered := string(page)

	for _, want := range []string{"# Typed Steps", "## [Host Prep](groups/host-prep.md)", "## [Kubernetes Lifecycle](groups/kubernetes-lifecycle.md)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in typed steps index:\n%s", want, rendered)
		}
	}
}

func TestRenderTypedStepsPageLinksCoreContracts(t *testing.T) {
	rendered := string(RenderTypedStepsPage([]PageInput{testGroupPageInput(t, "host-prep")}))
	for _, want := range []string{"[Step Envelope Contract](workflow-model.md#step-envelope-contract)", "[Workflow Model](workflow-model.md#workflow-schema-contract)", "[Workspace Layout](workspace-layout.md#component-fragment-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in typed steps page:\n%s", want, rendered)
		}
	}
}

func TestRenderWorkflowSchemaPartialUsesNestedHeadings(t *testing.T) {
	raw, err := schemas.WorkflowSchema()
	if err != nil {
		t.Fatalf("WorkflowSchema: %v", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal workflow schema: %v", err)
	}
	rendered := string(RenderWorkflowSchemaPartial("../../schemas/deck-workflow.schema.json", schema, WorkflowMeta()))
	for _, want := range []string{"## Workflow Schema Contract", "### Example", "### Validation Rules", "- schema: `../../schemas/deck-workflow.schema.json`"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in workflow schema partial:\n%s", want, rendered)
		}
	}
}

func TestGroupMetadataIsStable(t *testing.T) {
	meta := MustGroupMeta("host-prep")
	if meta.Title != "Host Prep" {
		t.Fatalf("unexpected host-prep title: %q", meta.Title)
	}
	if got := DisplayFamilyTitle("host-check", ""); got != "Host Check" {
		t.Fatalf("unexpected family display title fallback: %q", got)
	}
}

func sectionForKind(page string, kind string) string {
	marker := "## `" + kind + "`"
	idx := strings.Index(page, marker)
	if idx == -1 {
		return ""
	}
	rest := page[idx:]
	if next := strings.Index(rest[len(marker):], "\n## `"); next >= 0 {
		return rest[:len(marker)+next]
	}
	return rest
}

func testGroupPageInput(t *testing.T, group string) PageInput {
	t.Helper()
	meta := MustGroupMeta(group)
	defs := workflowcontract.StepDefinitions()
	page := PageInput{
		Group:        group,
		PageSlug:     meta.Key,
		Title:        meta.Title,
		Summary:      meta.Summary,
		Description:  meta.Summary,
		WhenToUse:    meta.WhenToUse,
		TypicalFlows: meta.TypicalFlows,
		SeeAlso:      meta.SeeAlso,
	}
	for _, def := range defs {
		if def.Group != group || def.Visibility != "public" {
			continue
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
		page.Variants = append(page.Variants, VariantInput{
			Kind:        def.Kind,
			Title:       def.FamilyTitle,
			Description: def.Summary,
			Schema:      schema,
			Meta:        ToolMetaForDefinition(def),
			Required:    toRequiredStrings(spec["required"]),
			Spec:        spec,
			Outputs:     append([]string(nil), def.Outputs...),
			GroupOrder:  def.GroupOrder,
			DocsOrder:   def.DocsOrder,
		})
	}
	if len(page.Variants) == 0 {
		t.Fatalf("missing test page for group %s", group)
	}
	return page
}

func toRequiredStrings(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, _ := item.(string)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}
