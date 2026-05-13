package initcli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Options struct {
	Output       string
	DeckWorkDir  string
	StdoutPrintf func(format string, args ...any) error
}

type ScaffoldResult struct {
	Directories []string
	Files       []string
}

func Run(opts Options) error {
	resolvedOutput := strings.TrimSpace(opts.Output)
	if resolvedOutput == "" {
		resolvedOutput = "."
	}
	deckWorkDir := strings.TrimSpace(opts.DeckWorkDir)
	if deckWorkDir == "" {
		deckWorkDir = ".deck"
	}
	templates := templateFiles(resolvedOutput)
	overwriteTargets := make([]string, 0, len(templates))
	for path := range templates {
		if _, err := os.Stat(path); err == nil {
			overwriteTargets = append(overwriteTargets, path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("init: stat target path %s: %w", path, err)
		}
	}
	if len(overwriteTargets) > 0 {
		sort.Strings(overwriteTargets)
		return fmt.Errorf("init: starter layout already contains target paths; refusing to overwrite: %s (choose another --out or remove these files)", strings.Join(overwriteTargets, ", "))
	}

	scaffold, err := EnsureWorkspaceScaffold(resolvedOutput, deckWorkDir)
	if err != nil {
		return err
	}
	for path, body := range templates {
		if err := filemode.WriteArtifactFile(path, []byte(body)); err != nil {
			return fmt.Errorf("init: write %s: %w", path, err)
		}
	}
	created := make([]string, 0, len(templates)+len(scaffold.Directories)+len(scaffold.Files))
	created = append(created, scaffold.Directories...)
	created = append(created, scaffold.Files...)
	for path := range templates {
		created = append(created, path)
	}
	sort.Strings(created)
	if opts.StdoutPrintf == nil {
		return nil
	}
	return opts.StdoutPrintf("init: wrote %s\n", strings.Join(created, ", "))
}

func EnsureWorkspaceScaffold(root string, deckWorkDir string) (ScaffoldResult, error) {
	resolvedRoot := strings.TrimSpace(root)
	if resolvedRoot == "" {
		resolvedRoot = "."
	}
	resolvedDeckWorkDir := strings.TrimSpace(deckWorkDir)
	if resolvedDeckWorkDir == "" {
		resolvedDeckWorkDir = ".deck"
	}
	result := ScaffoldResult{
		Directories: scaffoldDirs(resolvedRoot, resolvedDeckWorkDir),
		Files:       scaffoldFiles(resolvedRoot),
	}
	for _, dir := range result.Directories {
		if err := filemode.EnsureArtifactDir(dir); err != nil {
			return ScaffoldResult{}, fmt.Errorf("init: create directory %s: %w", dir, err)
		}
	}
	for path, content := range scaffoldFileDefaults(resolvedRoot) {
		if err := ensureFileWithDefault(path, content); err != nil {
			return ScaffoldResult{}, err
		}
	}
	sort.Strings(result.Directories)
	return result, nil
}

func scaffoldDirs(root string, deckWorkDir string) []string {
	return []string{
		filepath.Join(root, deckWorkDir),
		filepath.Join(root, workspacepaths.WorkflowRootDir),
		filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowScenariosDir),
		filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir),
		filepath.Join(root, workspacepaths.PreparedDirRel),
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedFilesRoot),
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedImagesRoot),
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedPackagesRoot),
	}
}

func scaffoldFiles(root string) []string {
	defaults := scaffoldFileDefaults(root)
	files := make([]string, 0, len(defaults))
	for path := range defaults {
		files = append(files, path)
	}
	sort.Strings(files)
	return files
}

func scaffoldFileDefaults(root string) map[string]string {
	return map[string]string{
		filepath.Join(root, ".gitignore"):  defaultGitignoreContent(),
		filepath.Join(root, ".deckignore"): defaultDeckignoreContent(),
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedFilesRoot, ".keep"):    "",
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedImagesRoot, ".keep"):   "",
		filepath.Join(root, workspacepaths.PreparedDirRel, workspacepaths.PreparedPackagesRoot, ".keep"): "",
	}
}

func templateFiles(root string) map[string]string {
	applyComponentContent := strings.Join([]string{"steps: []", ""}, "\n")
	prepareScenarioContent := strings.Join([]string{"version: v1alpha1", "phases:", "  - name: prepare", "    steps: []", ""}, "\n")
	applyScenarioContent := strings.Join([]string{"version: v1alpha1", "phases:", "  - name: install", "    imports:", "      - path: example-apply.yaml", ""}, "\n")

	return map[string]string{
		workspacepaths.CanonicalVarsPath(root):            "{}\n",
		workspacepaths.CanonicalPrepareWorkflowPath(root): prepareScenarioContent,
		workspacepaths.CanonicalApplyWorkflowPath(root):   applyScenarioContent,
		filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir, "example-apply.yaml"): applyComponentContent,
	}
}

func ensureFileWithDefault(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("init: stat target path %s: %w", path, err)
	}
	if err := filemode.WriteArtifactFile(path, []byte(content)); err != nil {
		return fmt.Errorf("init: write %s: %w", path, err)
	}
	return nil
}

func defaultGitignoreContent() string {
	return strings.Join([]string{"/.deck/", "/deck", "/outputs/", "*.tar", ""}, "\n")
}

func defaultDeckignoreContent() string {
	return strings.Join([]string{".git/", ".gitignore", ".deckignore", "/*.tar", ""}, "\n")
}
