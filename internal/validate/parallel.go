package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowrefs"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func parallelApplyKindAllowed(kind string) bool {
	switch kind {
	case "Command", "CopyFile", "EnsureDirectory", "ExtractArchive", "WaitForCommand", "WaitForFile", "WaitForMissingFile", "WaitForService", "WaitForTCPPort", "WaitForMissingTCPPort", "WriteFile":
		return true
	default:
		return false
	}
}

func referencedRuntimeVars(step config.Step) []string {
	seen := map[string]bool{}
	refs, err := workflowrefs.WhenReferences(step.When)
	if err == nil {
		for _, ref := range refs {
			if ref.Namespace == workflowrefs.NamespaceRuntime {
				seen[ref.Root] = true
			}
		}
	}
	for _, ref := range workflowrefs.ValueTemplateReferences(step.Spec) {
		if ref.Namespace == workflowrefs.NamespaceRuntime {
			seen[ref.Root] = true
		}
	}
	vars := make([]string, 0, len(seen))
	for key := range seen {
		vars = append(vars, key)
	}
	sort.Strings(vars)
	return vars
}

func literalApplyTargetPath(step config.Step) string {
	if step.Kind == "WriteFile" || step.Kind == "CopyFile" || step.Kind == "EnsureDirectory" || step.Kind == "CreateSymlink" || step.Kind == "WriteContainerdConfig" || step.Kind == "WriteContainerdRegistryHosts" || step.Kind == "ConfigureRepository" || step.Kind == "EditTOML" || step.Kind == "EditYAML" || step.Kind == "EditJSON" {
		return stableLiteralPath(stringValue(step.Spec, "path"))
	}
	if step.Kind == "ExtractArchive" || step.Kind == "EditFile" || step.Kind == "WriteSystemdUnit" {
		if nested := mapValue(step.Spec, "output"); len(nested) > 0 {
			if path := stableLiteralPath(stringValue(nested, "path")); path != "" {
				return path
			}
		}
		return stableLiteralPath(stringValue(step.Spec, "path"))
	}
	return ""
}

func literalPrepareOutputRoot(step config.Step) string {
	switch step.Kind {
	case "DownloadPackage", "DownloadImage":
		return stableLiteralPath(stringValue(step.Spec, "outputDir"))
	case "DownloadFile":
		return stableLiteralPath(stringValue(step.Spec, "outputPath"))
	default:
		return ""
	}
}

func validatePrepareOutputRoot(step config.Step, output string) error {
	trimmed := strings.TrimSpace(output)
	switch step.Kind {
	case "DownloadFile":
		if workspacepaths.IsPreparedPathUnderRoot(trimmed, workspacepaths.PreparedFilesRoot) {
			return nil
		}
		return fmt.Errorf("E_PREPARE_OUTPUT_ROOT_INVALID: step %s (%s) outputPath must stay under %s/", step.ID, step.Kind, workspacepaths.PreparedFilesRoot)
	case "DownloadImage":
		if workspacepaths.IsPreparedPathUnderRoot(trimmed, workspacepaths.PreparedImagesRoot) {
			return nil
		}
		return fmt.Errorf("E_PREPARE_OUTPUT_ROOT_INVALID: step %s (%s) outputDir must stay under %s/", step.ID, step.Kind, workspacepaths.PreparedImagesRoot)
	case "DownloadPackage":
		if workspacepaths.IsPreparedPathUnderRoot(trimmed, workspacepaths.PreparedPackagesRoot) {
			return nil
		}
		return fmt.Errorf("E_PREPARE_OUTPUT_ROOT_INVALID: step %s (%s) outputDir must stay under %s/", step.ID, step.Kind, workspacepaths.PreparedPackagesRoot)
	default:
		return nil
	}
}
