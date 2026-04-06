package askdiagnostic

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func TestFromValidationErrorDetectsComponentFragmentShape(t *testing.T) {
	diags := FromValidationError(nil, "workflows/components/bootstrap.yaml: additional property version is not allowed", askknowledge.Current())
	joined := JSON(diags)
	if !strings.Contains(joined, "component_fragment_shape") {
		t.Fatalf("expected component fragment diagnostic, got %s", joined)
	}
}

func TestRepairPromptBlockIncludesStructuredJSON(t *testing.T) {
	text := RepairPromptBlock([]Diagnostic{{Code: "schema_invalid", Severity: "blocking", Message: "bad shape", SuggestedFix: "fix it"}})
	for _, want := range []string{"Structured diagnostics JSON:", "schema_invalid", "fix it"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in repair prompt, got %q", want, text)
		}
	}
}

func TestFromValidationErrorSuggestsKnownStepFields(t *testing.T) {
	diags := FromValidationError(nil, "E_SCHEMA_INVALID: step install-packages (InstallPackage): spec: Additional property sourceDir is not allowed", askknowledge.Current())
	joined := JSON(diags)
	for _, want := range []string{"unknown_step_field", "spec.source", "InstallPackage"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in diagnostics, got %s", want, joined)
		}
	}
}

func TestFromValidationErrorSuggestsRequiredInitKubeadmField(t *testing.T) {
	diags := FromValidationError(nil, "E_SCHEMA_INVALID: step init-cluster (InitKubeadm): spec: outputJoinFile is required", askknowledge.Current())
	joined := JSON(diags)
	for _, want := range []string{"missing_step_field", "spec.outputJoinFile", "InitKubeadm"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in diagnostics, got %s", want, joined)
		}
	}
}

func TestFromValidationErrorSuggestsUniqueDuplicateStepIDs(t *testing.T) {
	diags := FromValidationError(nil, "workflows/scenarios/apply.yaml: E_DUPLICATE_STEP_ID: preflight-host", askknowledge.Current())
	joined := JSON(diags)
	for _, want := range []string{string(workflowissues.CodeDuplicateStepID), "Every step id must be unique across top-level steps and steps nested under phases.", "control-plane-preflight-host", "workflows/scenarios/apply.yaml"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in diagnostics, got %s", want, joined)
		}
	}
}

func TestFromValidationErrorPrefersStructuredIssues(t *testing.T) {
	err := validationErrorWithIssues()
	diags := FromValidationError(err, err.Error(), askknowledge.Current())
	joined := JSON(diags)
	for _, want := range []string{"missing_step_field", "spec.backend.image", "DownloadPackage", "package.download.schema.json"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in diagnostics, got %s", want, joined)
		}
	}
}

func TestFromValidationErrorSuggestsSupportedDraftBuilders(t *testing.T) {
	diags := FromValidationError(nil, `unsupported draft builder "prepare.check-host" for workflows/prepare.yaml`, askknowledge.Current())
	joined := JSON(diags)
	for _, want := range []string{"unsupported_draft_builder", "prepare.download-package", "prepare.download-image", "workflows/prepare.yaml"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in diagnostics, got %s", want, joined)
		}
	}
}

func validationErrorWithIssues() error {
	return &validate.ValidationError{Message: "workflows/prepare.yaml: E_SCHEMA_INVALID: step prepare-stage-packages (DownloadPackage): spec.backend: image is required", Issues: []validate.Issue{{Code: "missing_step_field", Severity: "blocking", File: "workflows/prepare.yaml", Path: "spec.backend.image", StepID: "prepare-stage-packages", StepKind: "DownloadPackage", Message: "DownloadPackage requires spec.backend.image", SourceRef: "package.download.schema.json"}}}
}
