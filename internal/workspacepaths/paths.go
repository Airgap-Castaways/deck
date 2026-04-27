package workspacepaths

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	WorkflowRootDir             = "workflows"
	WorkflowScenariosDir        = "scenarios"
	WorkflowComponentsDir       = "components"
	WorkflowIndexRel            = "index.json"
	WorkflowIndexPathRel        = WorkflowRootDir + "/" + WorkflowIndexRel
	CanonicalScenariosDir       = WorkflowRootDir + "/" + WorkflowScenariosDir
	CanonicalComponentsDir      = WorkflowRootDir + "/" + WorkflowComponentsDir
	CanonicalScenariosPrefix    = CanonicalScenariosDir + "/"
	CanonicalComponentsPrefix   = CanonicalComponentsDir + "/"
	CanonicalPrepareWorkflowRel = "prepare.yaml"
	CanonicalApplyWorkflowRel   = "scenarios/apply.yaml"
	WorkflowVarsRel             = "vars.yaml"
	CanonicalPrepareWorkflow    = WorkflowRootDir + "/" + CanonicalPrepareWorkflowRel
	CanonicalApplyWorkflow      = WorkflowRootDir + "/" + CanonicalApplyWorkflowRel
	CanonicalVarsWorkflow       = WorkflowRootDir + "/" + WorkflowVarsRel
	PreparedDirRel              = "outputs"
	PreparedFilesRoot           = "files"
	PreparedPackagesRoot        = "packages"
	PreparedImagesRoot          = "images"
	PreparedBinRoot             = "bin"
)

var canonicalPreparedRoots = []string{
	PreparedFilesRoot,
	PreparedPackagesRoot,
	PreparedImagesRoot,
	PreparedBinRoot,
}

var canonicalWorkflowPaths = []string{
	CanonicalPrepareWorkflow,
	CanonicalApplyWorkflow,
	CanonicalVarsWorkflow,
}

var allowedAuthoringPaths = []string{
	CanonicalPrepareWorkflow,
	WorkflowRootDir + "/" + WorkflowScenariosDir + "/*.yaml",
	WorkflowRootDir + "/" + WorkflowComponentsDir + "/*.yaml",
	CanonicalVarsWorkflow,
}

func WorkflowPath(root string, rel string) string {
	parts := append([]string{root, WorkflowRootDir}, strings.Split(filepath.ToSlash(rel), "/")...)
	return filepath.Join(parts...)
}

func WorkflowRootPath(root string) string {
	return filepath.Join(root, WorkflowRootDir)
}

func WorkflowScenariosPath(root string) string {
	return filepath.Join(WorkflowRootPath(root), WorkflowScenariosDir)
}

func WorkflowComponentsPath(root string) string {
	return filepath.Join(WorkflowRootPath(root), WorkflowComponentsDir)
}

func CanonicalPrepareWorkflowPath(root string) string {
	return WorkflowPath(root, CanonicalPrepareWorkflowRel)
}

func CanonicalApplyWorkflowPath(root string) string {
	return WorkflowPath(root, CanonicalApplyWorkflowRel)
}

func CanonicalVarsPath(root string) string {
	return WorkflowPath(root, WorkflowVarsRel)
}

func DefaultPreparedRoot(root string) string {
	return filepath.Join(root, PreparedDirRel)
}

func CanonicalPreparedRoots() []string {
	return append([]string(nil), canonicalPreparedRoots...)
}

func CanonicalWorkflowPaths() []string {
	return append([]string(nil), canonicalWorkflowPaths...)
}

func AllowedAuthoringPathPatterns() []string {
	return append([]string(nil), allowedAuthoringPaths...)
}

func IsAllowedAuthoringPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" || strings.Contains(clean, "..") {
		return false
	}
	return IsCanonicalPrepareWorkflowPath(clean) ||
		IsCanonicalVarsWorkflowPath(clean) ||
		IsScenarioAuthoringPath(clean) ||
		IsComponentAuthoringPath(clean)
}

func IsCanonicalPrepareWorkflowPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return clean == CanonicalPrepareWorkflow || strings.HasSuffix(clean, "/"+CanonicalPrepareWorkflow)
}

func IsCanonicalVarsWorkflowPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return clean == CanonicalVarsWorkflow || strings.HasSuffix(clean, "/"+CanonicalVarsWorkflow)
}

func IsScenarioAuthoringPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return strings.HasPrefix(clean, CanonicalScenariosPrefix) || strings.Contains(clean, "/"+CanonicalScenariosPrefix)
}

func IsWorkflowAuthoringPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return strings.HasPrefix(clean, WorkflowRootDir+"/") || strings.Contains(clean, "/"+WorkflowRootDir+"/")
}

func IsComponentAuthoringPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return strings.HasPrefix(clean, CanonicalComponentsPrefix) || strings.Contains(clean, "/"+CanonicalComponentsPrefix)
}

func IsCanonicalPreparedPath(rel string) bool {
	trimmed := filepath.ToSlash(strings.TrimSpace(rel))
	if trimmed == "" {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if cleaned == "." {
		return false
	}
	for _, root := range canonicalPreparedRoots {
		if cleaned == root || strings.HasPrefix(cleaned, root+"/") {
			return true
		}
	}
	return false
}

func IsPreparedPathUnderRoot(rel string, root string) bool {
	trimmedRoot := filepath.ToSlash(strings.TrimSpace(root))
	if trimmedRoot == "" {
		return false
	}
	cleanedRoot := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmedRoot)))
	if cleanedRoot == "." {
		return false
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel))))
	if cleaned == "." {
		return false
	}
	return cleaned == cleanedRoot || strings.HasPrefix(cleaned, cleanedRoot+"/")
}

func PreparedRootPath(root string) string {
	return filepath.ToSlash(filepath.Join(PreparedDirRel, strings.TrimSpace(root)))
}

func IsPreparedFilePath(rel string) bool {
	return IsPreparedManifestPathUnderRoot(rel, PreparedFilesRoot)
}

func IsPreparedImagePath(rel string) bool {
	return IsPreparedManifestPathUnderRoot(rel, PreparedImagesRoot)
}

func IsPreparedPackagePath(rel string) bool {
	return IsPreparedManifestPathUnderRoot(rel, PreparedPackagesRoot)
}

func IsPreparedManifestPathUnderRoot(rel string, root string) bool {
	trimmed := filepath.ToSlash(strings.TrimSpace(rel))
	if trimmed == "" {
		return false
	}
	return IsPreparedPathUnderRoot(trimmed, root) || IsPreparedPathUnderRoot(trimmed, PreparedRootPath(root))
}

func LocateWorkflowTreeRoot(workflowPath string) (string, error) {
	resolved, err := filepath.Abs(strings.TrimSpace(workflowPath))
	if err != nil {
		return "", fmt.Errorf("resolve workflow path: %w", err)
	}
	dir := filepath.Dir(resolved)
	for {
		if filepath.Base(dir) == WorkflowRootDir {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("workflow path is not under %s/: %s", WorkflowRootDir, resolved)
}

func IsPathUnderWorkflowSubdir(path string, subdir string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return false
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	marker := string(filepath.Separator) + WorkflowRootDir + string(filepath.Separator) + strings.Trim(subdir, string(filepath.Separator)) + string(filepath.Separator)
	return strings.Contains(resolved, marker)
}

func IsScenarioWorkflowPath(path string) bool {
	return IsPathUnderWorkflowSubdir(path, WorkflowScenariosDir)
}

func IsComponentWorkflowPath(path string) bool {
	return IsPathUnderWorkflowSubdir(path, WorkflowComponentsDir)
}
