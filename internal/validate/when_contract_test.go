package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowRejectsLegacyWhenOperators(t *testing.T) {
	wf := []byte(`version: v1alpha1
steps:
  - id: bad-when
    apiVersion: deck/v1alpha1
    kind: EnsureDirectory
    when: vars.enabled and runtime.ready
    spec:
      path: /var/lib/deck
`)

	err := Bytes("legacy-when.yaml", wf)
	if err == nil {
		t.Fatalf("expected legacy when operator to fail")
	}
	if !strings.Contains(err.Error(), "invalid when expression") {
		t.Fatalf("expected invalid when expression error, got %v", err)
	}
	if !strings.Contains(err.Error(), "use && instead of and") {
		t.Fatalf("expected migration hint, got %v", err)
	}
}

func TestDocsExamplesValidate(t *testing.T) {
	// The documented example is the offline-kubernetes workspace. Validate it as
	// a workspace so scenarios and component fragments resolve against shared
	// vars and imports, rather than as standalone files.
	root := filepath.Join("..", "..", "docs", "examples", "offline-kubernetes")
	paths, err := Workspace(root)
	if err != nil {
		t.Fatalf("validate offline-kubernetes workspace: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no docs example workflows found in %s", root)
	}
}

func TestScenarioWorkspaceValidates(t *testing.T) {
	_, err := Workspace(filepath.Join("..", "..", "test"))
	if err != nil {
		t.Fatalf("validate scenario workspace: %v", err)
	}
	if err := File(filepath.Join("..", "..", "test", "workflows", "prepare.yaml")); err != nil {
		t.Fatalf("validate canonical test prepare workflow: %v", err)
	}
}

func TestWorkflowModelDocsDescribeRuntimeRegisterNamespace(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "workflow-model.md"))
	if err != nil {
		t.Fatalf("read workflow model docs: %v", err)
	}
	content := string(raw)
	if strings.Contains(content, "available to later steps via `vars.`") {
		t.Fatalf("workflow model docs must not describe register outputs as vars")
	}
	if !strings.Contains(content, "available to later steps via `runtime.`") {
		t.Fatalf("workflow model docs must describe runtime register namespace")
	}
}
