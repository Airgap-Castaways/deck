package schemadoc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowcontext"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
	"github.com/Airgap-Castaways/deck/schemas"
)

func TestRenderStepKindsPageListsPhaseGroups(t *testing.T) {
	page := RenderStepKindsPage([]PageInput{testGroupPageInput(t, "host-prep"), testGroupPageInput(t, "kubernetes-lifecycle")})
	rendered := string(page)

	for _, want := range []string{"# Step Kinds", "## Apply", "## Common", "### Host Prep", "[CheckHost](step-kinds/check-host.md)", "### Kubernetes Lifecycle", "[InitKubeadm](step-kinds/init-kubeadm.md)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in step kinds index:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "groups/") || strings.Contains(rendered, "typed-steps/") {
		t.Fatalf("did not expect legacy groups or typed-steps links in step kinds index:\n%s", rendered)
	}
}

func TestRenderStepKindsPageLinksCoreContracts(t *testing.T) {
	rendered := string(RenderStepKindsPage([]PageInput{testGroupPageInput(t, "host-prep")}))
	for _, want := range []string{"[Step Envelope Contract](workflow-model.md#step-envelope-contract)", "[Workflow Model](workflow-model.md#workflow-schema-contract)", "[Workspace Layout](workspace-layout.md#component-fragment-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in step kinds page:\n%s", want, rendered)
		}
	}
}

func TestStepKindPagesAssignPhases(t *testing.T) {
	pages := StepKindPages([]PageInput{testGroupPageInput(t, "packages-repositories"), testGroupPageInput(t, "host-prep")})
	if got := stepKindPageByKind(t, pages, "DownloadPackage").Phase.Key; got != "prepare" {
		t.Fatalf("expected DownloadPackage to be prepare, got %q", got)
	}
	if got := stepKindPageByKind(t, pages, "InstallPackage").Phase.Key; got != "apply" {
		t.Fatalf("expected InstallPackage to be apply, got %q", got)
	}
	if got := stepKindPageByKind(t, pages, "CheckHost").Phase.Key; got != "common" {
		t.Fatalf("expected CheckHost to be common, got %q", got)
	}
}

func TestRenderStepKindPageUsesKindScopedLinks(t *testing.T) {
	pages := StepKindPages([]PageInput{testGroupPageInput(t, "packages-repositories")})
	page := stepKindPageByKind(t, pages, "DownloadPackage")
	rendered := string(RenderStepKindPage(page))

	for _, want := range []string{"# DownloadPackage", "- phase: `prepare`", "- group: Packages and Repositories", "[Step Kinds](../step-kinds.md)", "[Step Envelope Contract](../workflow-model.md#step-envelope-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in step kind page:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "## `InstallPackage`") || strings.Contains(rendered, "typed-steps") {
		t.Fatalf("did not expect other step detail or legacy typed-steps link in kind page:\n%s", rendered)
	}
}

func TestRenderStepKindPageUsesConcreteExamplesAndValidationRules(t *testing.T) {
	pages := StepKindPages([]PageInput{testGroupPageInput(t, "packages-repositories")})
	page := stepKindPageByKind(t, pages, "RefreshRepository")
	rendered := string(RenderStepKindPage(page))

	if !strings.Contains(rendered, "kind: RefreshRepository") {
		t.Fatalf("expected concrete RefreshRepository example:\n%s", rendered)
	}
	if !strings.Contains(rendered, "At least one of `spec.clean` or `spec.update` must be set.") {
		t.Fatalf("expected schema-derived branch rule in step kind page:\n%s", rendered)
	}
	if strings.Contains(rendered, "schema: `../../../schemas/tools/") {
		t.Fatalf("did not expect raw per-kind schema path in public step kind page:\n%s", rendered)
	}
}

func TestRenderStepKindPagePreservesKindScopedNotes(t *testing.T) {
	pages := StepKindPages([]PageInput{testGroupPageInput(t, "container-images")})
	loadPage := string(RenderStepKindPage(stepKindPageByKind(t, pages, "LoadImage")))
	if strings.Contains(loadPage, "spec.auth") || strings.Contains(loadPage, "outputDir") {
		t.Fatalf("expected LoadImage notes to exclude DownloadImage-only guidance:\n%s", loadPage)
	}
	verifyPage := string(RenderStepKindPage(stepKindPageByKind(t, pages, "VerifyImage")))
	if !strings.Contains(verifyPage, "Use this instead of `LoadImage`") {
		t.Fatalf("expected image verification guidance:\n%s", verifyPage)
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

func TestRenderSystemVariablesPartialUsesFieldDefinitions(t *testing.T) {
	rendered := string(RenderSystemVariablesPartial())
	for _, def := range workflowcontext.FieldDefinitions() {
		if !strings.Contains(rendered, "`"+def.Path+"`") {
			t.Fatalf("expected context field %s in system variable docs:\n%s", def.Path, rendered)
		}
	}
	for _, def := range workflowexec.RuntimeHostFieldDefinitions() {
		if !strings.Contains(rendered, "`"+def.Path+"`") {
			t.Fatalf("expected runtime field %s in system variable docs:\n%s", def.Path, rendered)
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

func stepKindPageByKind(t *testing.T, pages []StepKindPage, kind string) StepKindPage {
	t.Helper()
	for _, page := range pages {
		if page.Variant.Kind == kind {
			return page
		}
	}
	t.Fatalf("missing step kind page kind=%s in %#v", kind, pages)
	return StepKindPage{}
}

func testGroupPageInput(t *testing.T, group string) PageInput {
	t.Helper()
	meta := MustGroupMeta(group)
	defs, err := workflowcontract.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
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
			Roles:       append([]string(nil), def.Roles...),
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
