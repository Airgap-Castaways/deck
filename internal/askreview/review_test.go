package askreview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func TestWorkspaceReviewsScenarioFilesBeyondCanonicalApply(t *testing.T) {
	root := t.TempDir()
	writeReviewFile(t, root, filepath.Join(workspacepaths.CanonicalScenariosDir, "worker.yaml"), `version: v1alpha1
steps:
  - id: one
    kind: Command
    spec:
      command: ["true"]
  - id: two
    kind: Command
    spec:
      command: ["true"]
  - id: three
    kind: Command
    spec:
      command: ["true"]
`)

	findings := Workspace(root)
	if !hasFinding(findings, "workspace uses 3 Command steps") {
		t.Fatalf("expected non-canonical scenario to contribute command finding, got %#v", findings)
	}
	if !hasFinding(findings, "Command step relies on opaque shell behavior") {
		t.Fatalf("expected workflow analysis warning, got %#v", findings)
	}
}

func TestWorkspaceReportsValidationFindingsFromScenarioImports(t *testing.T) {
	root := t.TempDir()
	writeReviewFile(t, root, filepath.Join(workspacepaths.CanonicalScenariosDir, "apply.yaml"), `version: v1alpha1
phases:
  - name: apply
    imports:
      - path: bad.yaml
`)
	writeReviewFile(t, root, filepath.Join(workspacepaths.CanonicalComponentsDir, "bad.yaml"), `steps:
  - id: dup
    kind: Command
    spec:
      command: ["true"]
  - id: dup
    kind: Command
    spec:
      command: ["true"]
`)

	findings := Workspace(root)
	if !hasFindingWithSeverity(findings, "blocking", "duplicate_step_id") {
		t.Fatalf("expected imported component validation finding, got %#v", findings)
	}
}

func TestWorkspaceReportsValidationFindingsFromPrepareImports(t *testing.T) {
	root := t.TempDir()
	writeReviewFile(t, root, workspacepaths.CanonicalPrepareWorkflow, `version: v1alpha1
phases:
  - name: prepare
    imports:
      - path: bad-prepare.yaml
`)
	writeReviewFile(t, root, filepath.Join(workspacepaths.CanonicalComponentsDir, "bad-prepare.yaml"), `steps:
  - id: install
    kind: InstallPackage
    spec:
      packages: kubeadm
`)

	findings := Workspace(root)
	if !hasFindingWithSeverity(findings, "blocking", "Invalid type. Expected: array") {
		t.Fatalf("expected prepare import validation finding, got %#v", findings)
	}
}

func TestWorkspaceDoesNotValidateComponentsWithoutScenarioVars(t *testing.T) {
	root := t.TempDir()
	writeReviewFile(t, root, filepath.Join(workspacepaths.CanonicalComponentsDir, "templated.yaml"), `steps:
  - id: install
    kind: InstallPackage
    spec:
      packages: "{{ .vars.runtimePackages }}"
`)

	findings := Workspace(root)
	if hasFindingWithSeverity(findings, "blocking", "Invalid type. Expected: array") {
		t.Fatalf("expected component templates to avoid context-free validation blockers, got %#v", findings)
	}
}

func TestYAMLFilesUnderReturnsPartialPathsAfterWalkError(t *testing.T) {
	root := t.TempDir()
	writeReviewFile(t, root, "ok.yaml", "version: v1alpha1\nsteps: []\n")
	blocked := filepath.Join(root, "zz-blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "hidden.yaml"), []byte("version: v1alpha1\nsteps: []\n"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatalf("chmod blocked: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	paths := yamlFilesUnder(root)
	if len(paths) == 0 || !hasPath(paths, filepath.Join(root, "ok.yaml")) {
		t.Fatalf("expected partial paths after walk error, got %#v", paths)
	}
}

func TestCandidateReportsValidationFindingsForGeneratedWorkflows(t *testing.T) {
	findings := Candidate(map[string]string{
		workspacepaths.CanonicalApplyWorkflow: `version: v1alpha1
steps:
  - id: dup
    kind: Command
    spec:
      command: ["true"]
  - id: dup
    kind: Command
    spec:
      command: ["true"]
`,
		workspacepaths.CanonicalVarsWorkflow: "not: a workflow\n",
	})

	if !hasFindingWithSeverity(findings, "blocking", "duplicate_step_id") {
		t.Fatalf("expected candidate validation finding, got %#v", findings)
	}
	if hasFinding(findings, workspacepaths.CanonicalVarsWorkflow+" validation") {
		t.Fatalf("expected vars file to be ignored by workflow review, got %#v", findings)
	}
}

func writeReviewFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func hasFinding(findings []Finding, needle string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.Message, needle) {
			return true
		}
	}
	return false
}

func hasPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

func hasFindingWithSeverity(findings []Finding, severity string, needle string) bool {
	for _, finding := range findings {
		if finding.Severity == severity && strings.Contains(finding.Message, needle) {
			return true
		}
	}
	return false
}
