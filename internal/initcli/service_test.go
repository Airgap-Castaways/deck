package initcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func TestEnsureWorkspaceScaffoldCreatesMinimalLayoutOnly(t *testing.T) {
	root := t.TempDir()
	result, err := EnsureWorkspaceScaffold(root, ".deck")
	if err != nil {
		t.Fatalf("EnsureWorkspaceScaffold: %v", err)
	}
	if len(result.Directories) == 0 || len(result.Files) == 0 {
		t.Fatalf("expected scaffold result to include directories and files: %+v", result)
	}
	for _, rel := range []string{
		".deck",
		workspacepaths.WorkflowRootDir,
		filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowScenariosDir),
		filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir),
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedFilesRoot),
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedImagesRoot),
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedPackagesRoot),
		".gitignore",
		".deckignore",
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedFilesRoot, ".keep"),
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedImagesRoot, ".keep"),
		filepath.Join(workspacepaths.PreparedDirRel, workspacepaths.PreparedPackagesRoot, ".keep"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected scaffold path %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		workspacepaths.CanonicalPrepareWorkflow,
		workspacepaths.CanonicalApplyWorkflow,
		workspacepaths.CanonicalVarsWorkflow,
		filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir, "example-apply.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("minimal scaffold must not create starter workflow file %s, stat err=%v", rel, err)
		}
	}
}
