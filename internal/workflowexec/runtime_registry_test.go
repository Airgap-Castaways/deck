package workflowexec

import (
	"strings"
	"testing"
)

func TestStepRoleHandlersRequireCompleteRoleCoverage(t *testing.T) {
	handlers := handlersForRole(t, "apply")
	delete(handlers, "Command")

	_, err := StepRoleHandlers("apply", handlers)
	if err == nil {
		t.Fatal("expected missing handler error")
	}
	if !strings.Contains(err.Error(), "missing step handlers for role \"apply\"") || !strings.Contains(err.Error(), "Command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepRoleHandlersRejectWrongRole(t *testing.T) {
	handlers := handlersForRole(t, "apply")
	handlers["DownloadFile"] = "DownloadFile"

	_, err := StepRoleHandlers("apply", handlers)
	if err == nil {
		t.Fatal("expected wrong role error")
	}
	if !strings.Contains(err.Error(), "DownloadFile") || !strings.Contains(err.Error(), "metadata roles are prepare") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepRoleHandlersAggregateRolesForDuplicateKinds(t *testing.T) {
	registered, err := stepRoleHandlersForDefinitions("apply", map[string]string{"Shared": "handler"}, []StepDefinition{
		{Kind: "Shared ", Roles: []string{"apply"}},
		{Kind: "Shared", Roles: []string{"prepare"}},
	})
	if err != nil {
		t.Fatalf("StepRoleHandlers: %v", err)
	}
	if got := registered["Shared"]; got != "handler" {
		t.Fatalf("handler mismatch: got %q", got)
	}
}

func TestStepRoleHandlerForKeyRejectsUnsupportedRole(t *testing.T) {
	handlers := MustStepRoleHandlers("apply", handlersForRole(t, "apply"))

	_, ok, err := StepRoleHandlerForKey("apply", handlers, StepTypeKey{APIVersion: "deck/v1", Kind: "DownloadFile"})
	if err != nil {
		t.Fatalf("StepRoleHandlerForKey: %v", err)
	}
	if ok {
		t.Fatal("expected prepare-only kind to be unsupported for apply")
	}
}

func handlersForRole(t *testing.T, role string) map[string]string {
	t.Helper()
	defs, err := StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	handlers := map[string]string{}
	for _, def := range defs {
		if containsString(def.Roles, role) {
			handlers[def.Kind] = def.Kind
		}
	}
	return handlers
}
