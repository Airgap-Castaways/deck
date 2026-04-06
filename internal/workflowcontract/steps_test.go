package workflowcontract

import (
	"strings"
	"testing"
)

func TestStepDefFromMetaMissingRegistrationReturnsError(t *testing.T) {
	_, err := stepDefFromMeta("MissingKind", "missingkind", "system")
	if err == nil {
		t.Fatal("expected error for missing step registration")
	}
	if !strings.Contains(err.Error(), "missing stepmeta registration for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
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
