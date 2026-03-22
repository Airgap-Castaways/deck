package workflowexec

import (
	"testing"

	"github.com/taedi90/deck/internal/workflowcontract"
)

func TestStepRegistryDelegatesToWorkflowContract(t *testing.T) {
	wantDefs := workflowcontract.StepDefinitions()
	gotDefs := StepDefinitions()
	if len(gotDefs) != len(wantDefs) {
		t.Fatalf("unexpected step definition count: got %d want %d", len(gotDefs), len(wantDefs))
	}
	for _, want := range wantDefs {
		got, ok := StepDefinitionForKey(StepTypeKey{APIVersion: want.APIVersion, Kind: want.Kind})
		if !ok {
			t.Fatalf("missing step definition for key %s/%s", want.APIVersion, want.Kind)
		}
		if got.SchemaFile != want.SchemaFile {
			t.Fatalf("schema file mismatch for %s: got %q want %q", want.Kind, got.SchemaFile, want.SchemaFile)
		}
		if got.Category != want.Category {
			t.Fatalf("category mismatch for %s: got %q want %q", want.Kind, got.Category, want.Category)
		}
		if file, ok := StepSchemaFileForKey(StepTypeKey{APIVersion: want.APIVersion, Kind: want.Kind}); !ok || file != want.SchemaFile {
			t.Fatalf("StepSchemaFileForKey mismatch for %s: got %q ok=%t want %q", want.Kind, file, ok, want.SchemaFile)
		}
	}
}

func TestRegisterableOutputsCoveredByContracts(t *testing.T) {
	for _, def := range StepDefinitions() {
		contract, ok := StepContractForKey(StepTypeKey{APIVersion: def.APIVersion, Kind: def.Kind})
		if !ok {
			t.Fatalf("missing keyed step contract for %s", def.Kind)
		}
		for _, output := range def.Outputs {
			if !contract.Outputs[output] {
				t.Fatalf("missing keyed top-level output %q for %s", output, def.Kind)
			}
		}
	}
}
