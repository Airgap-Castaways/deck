package workflowcontract

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/stepmeta"
)

func TestStepDefFromMetaMissingRegistrationReturnsError(t *testing.T) {
	_, err := stepDefFromMeta("MissingKind", "missingkind")
	if err == nil {
		t.Fatal("expected error for missing step registration")
	}
	if !strings.Contains(err.Error(), "missing stepmeta registration for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepDefinitionsUseStepmetaCategoryProjection(t *testing.T) {
	defs, err := StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		entry, ok, err := stepmeta.LookupCatalogEntry(def.Kind)
		if err != nil {
			t.Fatalf("LookupCatalogEntry(%s): %v", def.Kind, err)
		}
		if !ok {
			t.Fatalf("expected stepmeta entry for %s", def.Kind)
		}
		if def.Category != stepmeta.CategoryForEntry(entry) {
			t.Fatalf("category mismatch for %s: def=%q stepmeta=%q", def.Kind, def.Category, stepmeta.CategoryForEntry(entry))
		}
	}
}

func TestGeneratorForKindMissingRegistrationReturnsError(t *testing.T) {
	_, err := generatorForKind("MissingKind")
	if err == nil {
		t.Fatal("expected error for missing step registration")
	}
	if !strings.Contains(err.Error(), "missing stepmeta registration for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
	}
}
