package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeFilesWarnsOnOpaqueCommand(t *testing.T) {
	path := writeAnalysisWorkflow(t, `version: v1alpha1
steps:
  - id: run
    kind: Command
    spec:
      command: [true]
`)

	findings, err := AnalyzeFiles([]string{path})
	if err != nil {
		t.Fatalf("AnalyzeFiles failed: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %#v", findings)
	}
	if findings[0].Code != "W_COMMAND_OPAQUE" {
		t.Fatalf("expected opaque command warning, got %#v", findings[0])
	}
}

func TestAnalyzeFilesSuggestsTypedReplacementForServiceCommand(t *testing.T) {
	path := writeAnalysisWorkflow(t, `version: v1alpha1
steps:
  - id: restart-containerd
    kind: Command
    spec:
      command: [systemctl, restart, containerd]
`)

	findings, err := AnalyzeFiles([]string{path})
	if err != nil {
		t.Fatalf("AnalyzeFiles failed: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %#v", findings)
	}
	if findings[0].Code != "W_COMMAND_OPAQUE" {
		t.Fatalf("expected opaque command warning first, got %#v", findings[0])
	}
	if findings[1].Code != "W_COMMAND_TYPED_PREFERRED" {
		t.Fatalf("expected typed preference warning second, got %#v", findings[1])
	}
	if !strings.Contains(findings[1].Hint, "ManageService") {
		t.Fatalf("expected ManageService hint, got %#v", findings[1])
	}
}

func TestAnalyzeFilesDoesNotSuggestTypedReplacementForShellWrappedCommand(t *testing.T) {
	path := writeAnalysisWorkflow(t, `version: v1alpha1
steps:
  - id: restart-containerd
    kind: Command
    spec:
      command: [bash, -lc, "systemctl restart containerd"]
`)

	findings, err := AnalyzeFiles([]string{path})
	if err != nil {
		t.Fatalf("AnalyzeFiles failed: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %#v", findings)
	}
	if findings[0].Code != "W_COMMAND_OPAQUE" {
		t.Fatalf("expected opaque command warning, got %#v", findings[0])
	}
}

func writeAnalysisWorkflow(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workflow.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	return path
}
