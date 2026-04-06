package schemadoc

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestToolMetadataCoversStepKinds(t *testing.T) {
	kinds, err := workflowexec.StepKinds()
	if err != nil {
		t.Fatalf("StepKinds: %v", err)
	}
	for _, kind := range kinds {
		def, ok, err := workflowcontract.StepDefinitionForKey(workflowcontract.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: kind})
		if err != nil {
			t.Fatalf("StepDefinitionForKey(%s): %v", kind, err)
		}
		if !ok {
			t.Fatalf("missing step definition for %s", kind)
		}
		meta := ToolMetaForDefinition(def)
		if meta.Kind != kind {
			t.Fatalf("unexpected normalized kind for %s: %q", kind, meta.Kind)
		}
		if strings.TrimSpace(meta.Summary) == "" {
			t.Fatalf("missing tool metadata summary for kind %s", kind)
		}
	}
}

func TestSharedRegisterExamplesUseGenericOutputs(t *testing.T) {
	for name, example := range map[string]string{
		"common register":   commonFieldDocs["register"].Example,
		"workflow register": WorkflowMeta().FieldDocs["steps[].register"].Example,
	} {
		if strings.Contains(example, "joinCommand") || strings.Contains(example, "joinCmd") {
			t.Fatalf("%s example should not reference kubeadm-specific outputs: %q", name, example)
		}
	}
}

func TestSharedContractTextUsesWorkflowContract(t *testing.T) {
	if commonFieldDocs["when"].Description != workflowcontract.WhenDescription() {
		t.Fatalf("unexpected common when description: %q", commonFieldDocs["when"].Description)
	}
	if commonFieldDocs["when"].Example != workflowcontract.WhenExample() {
		t.Fatalf("unexpected common when example: %q", commonFieldDocs["when"].Example)
	}
	if commonFieldDocs["register"].Description != workflowcontract.RegisterDescription() {
		t.Fatalf("unexpected common register description: %q", commonFieldDocs["register"].Description)
	}
	if commonFieldDocs["register"].Example != workflowcontract.RegisterExample() {
		t.Fatalf("unexpected common register example: %q", commonFieldDocs["register"].Example)
	}

	workflowMeta := WorkflowMeta()
	if workflowMeta.FieldDocs["steps[].when"].Description != workflowcontract.WhenDescription() {
		t.Fatalf("unexpected workflow when description: %q", workflowMeta.FieldDocs["steps[].when"].Description)
	}
	if workflowMeta.FieldDocs["steps[].register"].Description != workflowcontract.RegisterDescription() {
		t.Fatalf("unexpected workflow register description: %q", workflowMeta.FieldDocs["steps[].register"].Description)
	}
}

func TestRemovedFieldsStayOutOfPublicMetadata(t *testing.T) {
	checks := []struct {
		kind  string
		field string
	}{
		{kind: "DownloadFile", field: "spec.owner"},
		{kind: "DownloadFile", field: "spec.group"},
		{kind: "WaitForService", field: "spec.state"},
	}
	for _, tc := range checks {
		def, ok, err := workflowcontract.StepDefinitionForKey(workflowcontract.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: tc.kind})
		if err != nil {
			t.Fatalf("StepDefinitionForKey(%s): %v", tc.kind, err)
		}
		if !ok {
			t.Fatalf("missing step definition for %s", tc.kind)
		}
		meta := ToolMetaForDefinition(def)
		if _, exists := meta.FieldDocs[tc.field]; exists {
			t.Fatalf("field %s should not appear in %s metadata", tc.field, tc.kind)
		}
	}
}

func TestToolMetadataCategoryMatchesRegistry(t *testing.T) {
	defs, err := workflowexec.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		meta := ToolMetaForDefinition(def)
		if meta.Category != def.Category {
			t.Fatalf("category mismatch for %s: metadata=%q registry=%q", def.Kind, meta.Category, def.Category)
		}
		if meta.Summary != def.Summary {
			t.Fatalf("summary mismatch for %s: metadata=%q registry=%q", def.Kind, meta.Summary, def.Summary)
		}
		if meta.WhenToUse != def.WhenToUse {
			t.Fatalf("whenToUse mismatch for %s: metadata=%q registry=%q", def.Kind, meta.WhenToUse, def.WhenToUse)
		}
	}
}

func TestPublicStepDefinitionsMapToKnownGroups(t *testing.T) {
	defs, err := workflowexec.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		if def.Visibility != "public" {
			continue
		}
		if _, ok := GroupMeta(def.Group); !ok {
			t.Fatalf("missing group metadata for %s group %q", def.Kind, def.Group)
		}
		if def.GroupOrder <= 0 {
			t.Fatalf("expected positive group order for %s", def.Kind)
		}
	}
}

func TestToolMetadataRemovesLegacyActionFieldDocs(t *testing.T) {
	defs, err := workflowexec.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		meta := ToolMetaForDefinition(def)
		if _, ok := meta.FieldDocs["spec.action"]; ok {
			t.Fatalf("legacy spec.action field doc should not be exposed for %s", def.Kind)
		}
	}
}

func TestRepresentativeToolMetadataStaysDetailed(t *testing.T) {
	tests := []struct {
		kind       string
		fieldPaths []string
	}{
		{kind: "CheckHost", fieldPaths: []string{"spec.checks", "spec.failFast"}},
		{kind: "DownloadImage", fieldPaths: []string{"spec.images", "spec.outputDir"}},
		{kind: "LoadImage", fieldPaths: []string{"spec.sourceDir", "spec.runtime"}},
		{kind: "DownloadPackage", fieldPaths: []string{"spec.packages", "spec.distro.family", "spec.backend.image"}},
		{kind: "InstallPackage", fieldPaths: []string{"spec.packages", "spec.source.path"}},
		{kind: "JoinKubeadm", fieldPaths: []string{"spec.joinFile", "spec.configFile"}},
	}
	for _, tc := range tests {
		def, ok, err := workflowcontract.StepDefinitionForKey(workflowcontract.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: tc.kind})
		if err != nil {
			t.Fatalf("StepDefinitionForKey(%s): %v", tc.kind, err)
		}
		if !ok {
			t.Fatalf("missing step definition for %s", tc.kind)
		}
		meta := ToolMetaForDefinition(def)
		if strings.TrimSpace(meta.Example) == "" {
			t.Fatalf("expected example for %s", tc.kind)
		}
		if len(meta.Notes) == 0 {
			t.Fatalf("expected operational notes for %s", tc.kind)
		}
		for _, fieldPath := range tc.fieldPaths {
			field, ok := meta.FieldDocs[fieldPath]
			if !ok {
				t.Fatalf("expected field doc %s for %s", fieldPath, tc.kind)
			}
			if strings.TrimSpace(field.Description) == "" {
				t.Fatalf("expected description for %s on %s", fieldPath, tc.kind)
			}
		}
	}
}
