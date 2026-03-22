package workflowcontract

import "testing"

func TestResolveStepAPIVersion(t *testing.T) {
	t.Run("defaults from workflow version", func(t *testing.T) {
		got, err := ResolveStepAPIVersion(SupportedWorkflowVersion, "")
		if err != nil {
			t.Fatalf("ResolveStepAPIVersion returned error: %v", err)
		}
		if got != BuiltInStepAPIVersion {
			t.Fatalf("unexpected default step apiVersion: got %q want %q", got, BuiltInStepAPIVersion)
		}
	})

	t.Run("accepts supported explicit apiVersion", func(t *testing.T) {
		got, err := ResolveStepAPIVersion(SupportedWorkflowVersion, BuiltInStepAPIVersion)
		if err != nil {
			t.Fatalf("ResolveStepAPIVersion returned error: %v", err)
		}
		if got != BuiltInStepAPIVersion {
			t.Fatalf("unexpected explicit step apiVersion: got %q want %q", got, BuiltInStepAPIVersion)
		}
	})

	t.Run("rejects unsupported workflow version", func(t *testing.T) {
		if _, err := ResolveStepAPIVersion("v9", ""); err == nil {
			t.Fatalf("expected error for unsupported workflow version")
		}
	})

	t.Run("rejects unsupported explicit apiVersion", func(t *testing.T) {
		if _, err := ResolveStepAPIVersion(SupportedWorkflowVersion, "deck/v9"); err == nil {
			t.Fatalf("expected error for unsupported explicit step apiVersion")
		}
	})
}
