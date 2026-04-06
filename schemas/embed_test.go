package schemas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestGeneratedGroupPagesExist(t *testing.T) {
	seenGroups := map[string]bool{}
	defs, err := workflowcontract.StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		if def.Visibility != "public" {
			continue
		}
		if seenGroups[def.Group] {
			continue
		}
		seenGroups[def.Group] = true
		page := filepath.Join("..", "docs", "reference", "groups", def.Group+".md")
		if _, err := os.Stat(page); err != nil {
			t.Fatalf("group page missing for %s: %v", def.Kind, err)
		}
	}
}

func TestToolSchemasCoverStepContracts(t *testing.T) {
	for _, kind := range workflowexec.StepKinds() {
		def, ok, err := workflowexec.StepDefinitionForKey(workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: kind})
		if err != nil {
			t.Fatalf("StepDefinitionForKey(%s): %v", kind, err)
		}
		if !ok {
			t.Fatalf("missing definition for %s", kind)
		}
		file, ok := workflowexec.StepSchemaFileForKey(workflowexec.StepTypeKey{APIVersion: def.APIVersion, Kind: def.Kind})
		if !ok {
			t.Fatalf("missing schema file for kind %s", kind)
		}
		raw, err := ToolSchema(file)
		if err != nil {
			t.Fatalf("ToolSchema(%q): %v", file, err)
		}
		if len(raw) == 0 {
			t.Fatalf("expected schema content for kind %s", kind)
		}
	}
}

func TestWorkflowSchemaCoversStepKinds(t *testing.T) {
	raw, err := WorkflowSchema()
	if err != nil {
		t.Fatalf("WorkflowSchema: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal workflow schema: %v", err)
	}
	properties, _ := doc["properties"].(map[string]any)
	steps, _ := properties["steps"].(map[string]any)
	items, _ := steps["items"].(map[string]any)
	itemProps, _ := items["properties"].(map[string]any)
	kindField, _ := itemProps["kind"].(map[string]any)
	enum, _ := kindField["enum"].([]any)
	seen := map[string]bool{}
	for _, rawValue := range enum {
		value, _ := rawValue.(string)
		seen[value] = true
	}
	for _, kind := range workflowexec.StepKinds() {
		if !seen[kind] {
			t.Fatalf("workflow schema kind enum missing %s", kind)
		}
	}
}
