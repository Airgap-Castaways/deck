package main

import (
	"testing"

	"github.com/taedi90/deck/internal/workflowexec"
)

func TestToolSchemaGeneratorsCoverStepDefinitions(t *testing.T) {
	generators := toolSchemaGenerators()
	for _, def := range workflowexec.StepDefinitions() {
		if _, ok := generators[def.Kind]; !ok {
			t.Fatalf("missing generator for %s", def.Kind)
		}
	}
	for kind := range generators {
		if _, ok := workflowexec.StepContractForKind(kind); !ok {
			t.Fatalf("generator registered for unknown kind %s", kind)
		}
	}
}

func TestToolSchemaDefinitionsUseRegistrySchemaFiles(t *testing.T) {
	defs, err := toolSchemaDefinitions()
	if err != nil {
		t.Fatalf("toolSchemaDefinitions: %v", err)
	}
	for _, def := range workflowexec.StepDefinitions() {
		if _, ok := defs[def.SchemaFile]; !ok {
			t.Fatalf("generated schemas missing %s for %s", def.SchemaFile, def.Kind)
		}
	}
	if len(defs) != len(workflowexec.StepDefinitions()) {
		t.Fatalf("expected %d generated tool schemas, got %d", len(workflowexec.StepDefinitions()), len(defs))
	}
}

func TestActionScopedDefinitionsDeclareFieldOwnership(t *testing.T) {
	for _, def := range workflowexec.StepDefinitions() {
		if len(def.Actions) == 0 {
			continue
		}
		for _, action := range def.Actions {
			if len(action.Fields) == 0 {
				t.Fatalf("missing field ownership for %s.%s", def.Kind, action.Name)
			}
		}
	}
}
