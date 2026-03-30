package askir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestMaterializeWorkflowDocumentRendersYAML(t *testing.T) {
	root := t.TempDir()
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path: "workflows/scenarios/apply.yaml",
			Kind: "workflow",
			Workflow: &askcontract.WorkflowDocument{
				Version: "v1alpha1",
				Steps: []askcontract.WorkflowStep{{
					ID:   "run",
					Kind: "Command",
					Spec: map[string]any{"command": []any{"true"}},
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("materialize workflow document: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one materialized file, got %#v", files)
	}
	for _, want := range []string{"version: v1alpha1", "kind: Command", "spec:"} {
		if !strings.Contains(files[0].Content, want) {
			t.Fatalf("expected %q in rendered workflow, got %q", want, files[0].Content)
		}
	}
}

func TestMaterializeRefineEditsAppliesStructuredChanges(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path:   "workflows/scenarios/apply.yaml",
			Action: "edit",
			Edits:  []askcontract.StructuredEditAction{{Op: "set", RawPath: "steps.0.timeout", Value: "5m"}},
		}},
	})
	if err != nil {
		t.Fatalf("materialize refine edit: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one edited file, got %#v", files)
	}
	if !strings.Contains(files[0].Content, "timeout: 5m") {
		t.Fatalf("expected structured edit to be applied, got %q", files[0].Content)
	}
}

func TestMaterializeRefineBracketPathEdit(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	content := "version: v1alpha1\nsteps:\n  - id: first\n    kind: Command\n    spec:\n      command: [true]\n  - id: second\n    kind: Command\n    spec:\n      command: [true]\n  - id: third\n    kind: Command\n    spec:\n      command: [true]\n  - id: fourth\n    kind: Command\n    spec:\n      command: [true]\n"
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	files, err := Materialize(root, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path:   "workflows/scenarios/apply.yaml",
			Action: "edit",
			Edits:  []askcontract.StructuredEditAction{{Op: "set", RawPath: "steps[3].timeout", Value: "10m"}},
		}},
	})
	if err != nil {
		t.Fatalf("materialize bracket path edit: %v", err)
	}
	if !strings.Contains(files[0].Content, "id: fourth") || !strings.Contains(files[0].Content, "timeout: 10m") {
		t.Fatalf("expected bracket path edit to update fourth step, got %q", files[0].Content)
	}
}

func TestMaterializeDeleteDocument(t *testing.T) {
	files, err := Materialize(t.TempDir(), askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{Path: "workflows/components/old.yaml", Action: "delete"}},
	})
	if err != nil {
		t.Fatalf("materialize delete: %v", err)
	}
	if len(files) != 1 || !files[0].Delete {
		t.Fatalf("expected delete file, got %#v", files)
	}
}

func TestMaterializeWithBasePreservesUntouchedFiles(t *testing.T) {
	base := []askcontract.GeneratedFile{{Path: "workflows/vars.yaml", Content: "role: control-plane\n"}}
	files, err := MaterializeWithBase(t.TempDir(), base, askcontract.GenerationResponse{
		Documents: []askcontract.GeneratedDocument{{
			Path: "workflows/scenarios/apply.yaml",
			Kind: "workflow",
			Workflow: &askcontract.WorkflowDocument{
				Version: "v1alpha1",
				Steps:   []askcontract.WorkflowStep{{ID: "run", Kind: "Command", Spec: map[string]any{"command": []any{"true"}}}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("materialize with base: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected untouched base file plus new file, got %#v", files)
	}
	if files[0].Path != "workflows/vars.yaml" || files[0].Content != base[0].Content {
		t.Fatalf("expected untouched base content to be preserved, got %#v", files)
	}
}

func TestParseDocumentWorkflow(t *testing.T) {
	doc, err := ParseDocument("workflows/scenarios/apply.yaml", []byte("version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [true]\n"))
	if err != nil {
		t.Fatalf("parse workflow document: %v", err)
	}
	if doc.Workflow == nil || doc.Workflow.Version != "v1alpha1" || len(doc.Workflow.Steps) != 1 {
		t.Fatalf("unexpected parsed workflow: %#v", doc)
	}
}

func TestParseDocumentVars(t *testing.T) {
	doc, err := ParseDocument("workflows/vars.yaml", []byte("role: control-plane\nversion: v1.30.1\n"))
	if err != nil {
		t.Fatalf("parse vars document: %v", err)
	}
	if doc.Vars == nil || doc.Vars["role"] != "control-plane" {
		t.Fatalf("unexpected vars document: %#v", doc)
	}
}

func TestResolveStructuredEditPathFindsPhaseStepByID(t *testing.T) {
	doc := askcontract.GeneratedDocument{Path: "workflows/scenarios/apply.yaml", Kind: "workflow", Workflow: &askcontract.WorkflowDocument{Version: "v1alpha1", Phases: []askcontract.WorkflowPhase{{Name: "apply", Steps: []askcontract.WorkflowStep{{ID: "apply-runtime-ready", Kind: "WaitForService", Spec: map[string]any{"timeout": "5m"}}}}}}}
	path := resolveStructuredEditPath("steps.apply-runtime-ready.spec.timeout", doc)
	if path != "/phases/0/steps/0/spec/timeout" {
		t.Fatalf("unexpected resolved path: %q", path)
	}
}
