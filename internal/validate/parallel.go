package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowrefs"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func referencedRuntimeVars(step config.Step) ([]string, error) {
	seen := map[string]bool{}
	refs, err := workflowrefs.WhenNamespaceRoots(step.When, workflowrefs.NamespaceRuntime)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		seen[ref] = true
	}
	templateRefs, err := workflowrefs.ValueNamespaceRoots(step.Spec, workflowrefs.NamespaceRuntime)
	if err != nil {
		return nil, err
	}
	for _, ref := range templateRefs {
		seen[ref] = true
	}
	vars := make([]string, 0, len(seen))
	for key := range seen {
		vars = append(vars, key)
	}
	sort.Strings(vars)
	return vars, nil
}

func parallelMetadata(workflowVersion string, step config.Step) (workflowcontract.ParallelMetadata, bool, error) {
	key, err := effectiveStepTypeKey(workflowVersion, step)
	if err != nil {
		return workflowcontract.ParallelMetadata{}, false, err
	}
	def, ok, err := workflowcontract.StepDefinitionForKey(workflowcontract.StepTypeKey(key))
	if err != nil {
		return workflowcontract.ParallelMetadata{}, false, err
	}
	if !ok {
		return workflowcontract.ParallelMetadata{}, false, nil
	}
	return def.Parallel, true, nil
}

func literalApplyTargetPath(step config.Step, meta workflowcontract.ParallelMetadata) string {
	for _, path := range meta.ApplyTargetPaths {
		if value := stableLiteralPath(specStringValue(step.Spec, path)); value != "" {
			return value
		}
	}
	return ""
}

func literalPrepareOutputRoot(step config.Step, meta workflowcontract.ParallelMetadata) (string, workflowcontract.OutputRootConstraint) {
	constraint := meta.PrepareOutput
	if strings.TrimSpace(constraint.Path) == "" {
		return "", workflowcontract.OutputRootConstraint{}
	}
	return stableLiteralPath(specStringValue(step.Spec, constraint.Path)), constraint
}

func validatePrepareOutputRoot(step config.Step, output string, constraint workflowcontract.OutputRootConstraint) error {
	trimmed := strings.TrimSpace(output)
	root := strings.TrimSpace(constraint.Root)
	if root == "" || workspacepaths.IsPreparedPathUnderRoot(trimmed, root) {
		return nil
	}
	field := specFieldName(constraint.Path)
	example := strings.TrimSpace(constraint.Example)
	if example == "" {
		example = root + "/..."
	}
	return fmt.Errorf("E_PREPARE_OUTPUT_ROOT_INVALID: step %s (%s) %s must stay under %s/ (e.g. %s); omit %s to use the default", step.ID, step.Kind, field, root, example, field)
}

func specStringValue(spec map[string]any, path string) string {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) < 2 || parts[0] != "spec" {
		return ""
	}
	current := spec
	for _, part := range parts[1 : len(parts)-1] {
		current = mapValue(current, part)
		if len(current) == 0 {
			return ""
		}
	}
	return stringValue(current, parts[len(parts)-1])
}

func specFieldName(path string) string {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
