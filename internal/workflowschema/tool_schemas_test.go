package workflowschema

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestSchemaMetadataForDefinitionMissingEntryReturnsError(t *testing.T) {
	_, err := SchemaMetadataForDefinition(workflowexec.StepDefinition{Kind: "MissingKind"})
	if err == nil {
		t.Fatal("expected error for missing step metadata")
	}
	if !strings.Contains(err.Error(), "missing stepmeta entry for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
	}
}
