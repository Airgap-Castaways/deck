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

func TestRenderGroupPageListsKindsByGroup(t *testing.T) {
	page := testGroupPageInput(t, "files-archives")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "- group: `files-archives`") {
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
	page := testGroupPageInput(t, "container-runtime")
	rendered := string(RenderGroupPage(page))

	for _, want := range []string{"## Typical Flows", "### Configure containerd", "[Container Images](container-images.md)", "[Step Envelope Contract](../workflow-model.md#step-envelope-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in container runtime page:\n%s", want, rendered)
		}
	}
}

func TestRenderGroupPageUsesConcreteExamplesAndValidationRules(t *testing.T) {
	page := testGroupPageInput(t, "packages-repositories")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "## `DownloadPackage`") || !strings.Contains(rendered, "kind: DownloadPackage") {
		t.Fatalf("expected concrete DownloadPackage section:\n%s", rendered)
	}
	if !strings.Contains(rendered, "At least one of `spec.clean` or `spec.update` must be set.") {
		t.Fatalf("expected schema-derived branch rule in packages page:\n%s", rendered)
	}
	if strings.Contains(rendered, "schema: `../../../schemas/tools/") {
		t.Fatalf("did not expect raw per-kind schema path in public group page:\n%s", rendered)
	}
}

func TestRenderGroupPageEscapesMultilineExamples(t *testing.T) {
	page := testGroupPageInput(t, "services-systemd")
	rendered := string(RenderGroupPage(page))

	if !strings.Contains(rendered, "[Service]<br>Environment=NODE_IP={{ .vars.nodeIP }}") {
		t.Fatalf("expected multiline example to stay inside table cell:\n%s", rendered)
	}
}

func TestRenderGroupPagePreservesKindScopedNotes(t *testing.T) {
	rendered := string(RenderGroupPage(testGroupPageInput(t, "container-images")))
	loadSection := sectionForKind(rendered, "LoadImage")
	if strings.Contains(loadSection, "spec.auth") || strings.Contains(loadSection, "outputDir") {
		t.Fatalf("expected LoadImage notes to exclude DownloadImage-only guidance:\n%s", loadSection)
	}
	verifySection := sectionForKind(rendered, "VerifyImage")
	if !strings.Contains(verifySection, "Use this instead of `LoadImage`") {
		t.Fatalf("expected image verification guidance:\n%s", verifySection)
	}
}

func TestRenderTypedStepsPageListsGroups(t *testing.T) {
	page := RenderTypedStepsPage([]PageInput{testGroupPageInput(t, "host-prep"), testGroupPageInput(t, "kubernetes-lifecycle")})
	rendered := string(page)

	for _, want := range []string{"# Typed Steps", "## Apply", "## Common", "[Host Prep](typed-steps/apply/host-prep.md)", "[CheckHost](typed-steps/common/host-prep.md#checkhost)", "[Kubernetes Lifecycle](typed-steps/apply/kubernetes-lifecycle.md)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in typed steps index:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "groups/") {
		t.Fatalf("did not expect legacy groups links in typed steps index:\n%s", rendered)
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

func TestRoleFilteredKindsDeduplicatesKinds(t *testing.T) {
	got := roleFilteredKinds([]PageInput{{Variants: []VariantInput{
		{Kind: "DownloadFile", Roles: []string{"prepare"}},
		{Kind: "DownloadFile", Roles: []string{"prepare"}},
	}}}, "prepare", false)
	if len(got) != 1 || got[0] != "DownloadFile" {
		t.Fatalf("expected deduplicated prepare kinds, got %#v", got)
	}
}

func TestPhasePagesSeparatePhaseSpecificVariants(t *testing.T) {
	pages := PhasePages([]PageInput{testGroupPageInput(t, "packages-repositories"), testGroupPageInput(t, "host-prep")})
	preparePackages := phasePageByPhaseAndGroup(t, pages, "prepare", "packages-repositories")
	if got := pageKinds(preparePackages.Page.Variants); len(got) != 1 || got[0] != "DownloadPackage" {
		t.Fatalf("expected prepare packages page to contain only DownloadPackage, got %#v", got)
	}
	applyPackages := phasePageByPhaseAndGroup(t, pages, "apply", "packages-repositories")
	if got := pageKinds(applyPackages.Page.Variants); containsString(got, "DownloadPackage") || !containsString(got, "InstallPackage") {
		t.Fatalf("expected apply packages page to contain install/apply kinds only, got %#v", got)
	}
	commonHost := phasePageByPhaseAndGroup(t, pages, "common", "host-prep")
	if got := pageKinds(commonHost.Page.Variants); len(got) != 1 || got[0] != "CheckHost" {
		t.Fatalf("expected common host-prep page to contain only CheckHost, got %#v", got)
	}
}

func TestRenderTypedStepPhaseGroupPageUsesPhaseScopedLinks(t *testing.T) {
	pages := PhasePages([]PageInput{testGroupPageInput(t, "packages-repositories")})
	page := phasePageByPhaseAndGroup(t, pages, "prepare", "packages-repositories")
	rendered := string(RenderTypedStepPhaseGroupPage(page))

	for _, want := range []string{"- phase: `prepare`", "[DownloadPackage](#downloadpackage)", "[Typed Steps](../../typed-steps.md)", "[Step Envelope Contract](../../workflow-model.md#step-envelope-contract)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in phase group page:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "## `InstallPackage`") {
		t.Fatalf("did not expect apply-only InstallPackage in prepare page:\n%s", rendered)
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

func phasePageByPhaseAndGroup(t *testing.T, pages []PhasePage, phase string, group string) PhasePage {
	t.Helper()
	for _, page := range pages {
		if page.Phase.Key == phase && page.Page.Group == group {
			return page
		}
	}
	t.Fatalf("missing phase page phase=%s group=%s in %#v", phase, group, pages)
	return PhasePage{}
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
