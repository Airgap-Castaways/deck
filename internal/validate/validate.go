package validate

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"

	"github.com/taedi90/deck/internal/config"
	"github.com/taedi90/deck/internal/workflowexec"
)

var (
	runtimeVarNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	singleBraceTemplatePattern = regexp.MustCompile(`(^|[^\{])(\{\s*\.(vars|runtime)\.[^{}]+\})([^\}]|$)`)
)

//go:embed schemas/deck-workflow.schema.json schemas/tools/*.schema.json
var schemaFS embed.FS

var toolSchemaByKind = map[string]string{
	"CheckHost":           "check-host.schema.json",
	"DownloadPackages":    "download-packages.schema.json",
	"DownloadK8sPackages": "download-k8s-packages.schema.json",
	"DownloadImages":      "download-images.schema.json",
	"DownloadFile":        "download-file.schema.json",
	"InstallArtifacts":    "install-artifacts.schema.json",
	"InstallPackages":     "install-packages.schema.json",
	"WriteFile":           "write-file.schema.json",
	"EditFile":            "edit-file.schema.json",
	"CopyFile":            "copy-file.schema.json",
	"Sysctl":              "sysctl.schema.json",
	"Modprobe":            "modprobe.schema.json",
	"Service":             "service.schema.json",
	"EnsureDir":           "ensure-dir.schema.json",
	"Symlink":             "symlink.schema.json",
	"InstallFile":         "install-file.schema.json",
	"TemplateFile":        "template-file.schema.json",
	"SystemdUnit":         "systemd-unit.schema.json",
	"RepoConfig":          "repo-config.schema.json",
	"PackageCache":        "package-cache.schema.json",
	"ContainerdConfig":    "containerd-config.schema.json",
	"Swap":                "swap.schema.json",
	"KernelModule":        "kernel-module.schema.json",
	"SysctlApply":         "sysctl-apply.schema.json",
	"RunCommand":          "run-command.schema.json",
	"WaitPath":            "wait-path.schema.json",
	"VerifyImages":        "verify-images.schema.json",
	"KubeadmInit":         "kubeadm-init.schema.json",
	"KubeadmJoin":         "kubeadm-join.schema.json",
	"KubeadmReset":        "kubeadm-reset.schema.json",
}

var registerOutputContract = map[string][]string{
	"CheckHost":           {"passed", "failedChecks"},
	"DownloadFile":        {"path", "artifacts"},
	"DownloadPackages":    {"artifacts"},
	"DownloadK8sPackages": {"artifacts"},
	"DownloadImages":      {"artifacts"},
	"WriteFile":           {"path"},
	"CopyFile":            {"dest"},
	"Service":             {"name"},
	"EnsureDir":           {"path"},
	"Symlink":             {"path"},
	"InstallFile":         {"path"},
	"TemplateFile":        {"path"},
	"SystemdUnit":         {"path"},
	"RepoConfig":          {"path"},
	"ContainerdConfig":    {"path"},
	"KernelModule":        {"name"},
	"KubeadmInit":         {"joinFile"},
}

// File validates workflow structure and semantic rules.
func File(path string) error {
	if path == "" {
		return fmt.Errorf("file path is empty")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read workflow file: %w", err)
	}
	return Bytes(path, content)
}

func Workspace(root string) ([]string, error) {
	resolvedRoot := strings.TrimSpace(root)
	if resolvedRoot == "" {
		resolvedRoot = "."
	}
	workflowRoot := filepath.Join(resolvedRoot, "workflows")
	info, err := os.Stat(workflowRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("workflow directory not found: %s", workflowRoot)
		}
		return nil, fmt.Errorf("stat workflow directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workflow directory is not a directory: %s", workflowRoot)
	}

	required := []string{
		filepath.Join(workflowRoot, "scenarios", "prepare.yaml"),
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("required workflow file not found: %s", path)
		}
	}

	scenarioRoot := filepath.Join(workflowRoot, "scenarios")
	if info, err := os.Stat(scenarioRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("workflow scenarios directory not found: %s", scenarioRoot)
	}

	files := make([]string, 0)
	err = filepath.WalkDir(scenarioRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read workflow directory: %w", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no scenario workflow files found under: %s", scenarioRoot)
	}

	validated := make([]string, 0)
	visited := map[string]bool{}
	for _, path := range files {
		validatedFiles, err := lintLocalEntrypoint(path, nil, visited)
		if err != nil {
			return nil, err
		}
		validated = append(validated, validatedFiles...)
	}
	return dedupeAndSort(validated), nil
}

func Entrypoint(path string) ([]string, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("resolve workflow path: %w", err)
	}
	return lintLocalEntrypoint(absPath, nil, map[string]bool{})
}

func Workflow(name string, wf *config.Workflow) error {
	if strings.TrimSpace(name) == "" {
		name = "<workflow>"
	}
	if wf == nil {
		return withWorkflowName(name, fmt.Errorf("workflow is nil"))
	}
	if len(wf.Phases) > 0 && len(wf.Steps) > 0 {
		return withWorkflowName(name, fmt.Errorf("workflow cannot set both phases and steps"))
	}
	if wf.Role == "" {
		return withWorkflowName(name, fmt.Errorf("role is required"))
	}
	if strings.TrimSpace(wf.Role) != "prepare" && strings.TrimSpace(wf.Role) != "apply" {
		return withWorkflowName(name, fmt.Errorf("unsupported role: %s (supported: prepare, apply)", wf.Role))
	}
	if wf.Version == "" {
		return withWorkflowName(name, fmt.Errorf("version is required"))
	}
	if strings.TrimSpace(wf.Version) != "v1alpha1" {
		return withWorkflowName(name, fmt.Errorf("unsupported version: %s (supported: v1alpha1)", wf.Version))
	}
	if err := validateToolSchemas(wf, true); err != nil {
		return withWorkflowName(name, err)
	}
	if err := validateSemantics(wf); err != nil {
		return withWorkflowName(name, err)
	}
	return nil
}

func Bytes(name string, content []byte) error {
	if strings.TrimSpace(name) == "" {
		name = "<workflow>"
	}

	if err := validateSingleBraceTemplates(name, content); err != nil {
		return withWorkflowName(name, err)
	}

	var wf config.Workflow
	if err := yaml.Unmarshal(content, &wf); err != nil {
		return withWorkflowName(name, fmt.Errorf("parse yaml: %w", err))
	}
	if err := validateSchema(name, content); err != nil {
		return withWorkflowName(name, err)
	}
	if err := validateToolSchemas(&wf, false); err != nil {
		return withWorkflowName(name, err)
	}
	if err := validateSemantics(&wf); err != nil {
		return withWorkflowName(name, err)
	}
	return nil
}

func withWorkflowName(name string, err error) error {
	if err == nil {
		return nil
	}
	prefix := strings.TrimSpace(name) + ": "
	if strings.HasPrefix(err.Error(), prefix) {
		return err
	}
	return fmt.Errorf("%s%w", prefix, err)
}

func lintLocalEntrypoint(path string, inheritedVars map[string]any, visiting map[string]bool) ([]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve workflow path: %w", err)
	}
	if visiting[absPath] {
		return nil, nil
	}
	visiting[absPath] = true

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, withWorkflowName(absPath, fmt.Errorf("read workflow file: %w", err))
	}
	if err := validateSingleBraceTemplates(absPath, content); err != nil {
		return nil, withWorkflowName(absPath, err)
	}
	if err := validateSchema(absPath, content); err != nil {
		return nil, withWorkflowName(absPath, err)
	}

	var raw config.Workflow
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, withWorkflowName(absPath, fmt.Errorf("parse yaml: %w", err))
	}

	loadOpts := config.LoadOptions{VarOverrides: cloneAnyMap(inheritedVars)}
	wf, err := config.LoadWithOptions(context.Background(), absPath, loadOpts)
	if err != nil {
		return nil, withWorkflowName(absPath, err)
	}

	validated := []string{absPath}
	for _, ref := range append([]string{}, raw.Imports...) {
		child, err := resolveLocalWorkflowImport(absPath, ref)
		if err != nil {
			return nil, withWorkflowName(absPath, err)
		}
		files, err := lintLocalEntrypoint(child, wf.Vars, visiting)
		if err != nil {
			return nil, err
		}
		validated = append(validated, files...)
	}
	for _, phase := range raw.Phases {
		for _, phaseImport := range phase.Imports {
			child, err := resolveLocalWorkflowImport(absPath, phaseImport.Path)
			if err != nil {
				return nil, withWorkflowName(absPath, err)
			}
			files, err := lintLocalEntrypoint(child, wf.Vars, visiting)
			if err != nil {
				return nil, err
			}
			validated = append(validated, files...)
		}
	}
	if err := Workflow(absPath, wf); err != nil {
		return nil, err
	}
	return dedupeAndSort(validated), nil
}

func resolveLocalWorkflowImport(originPath string, importRef string) (string, error) {
	ref := strings.TrimSpace(importRef)
	if ref == "" {
		return "", fmt.Errorf("workflow import path is empty")
	}
	if strings.Contains(ref, "://") {
		return "", fmt.Errorf("remote imports are not supported during workspace lint: %s", ref)
	}
	joined := filepath.Clean(filepath.Join(filepath.Dir(originPath), ref))
	return filepath.Abs(joined)
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func dedupeAndSort(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func validateToolSchemas(wf *config.Workflow, renderTemplates bool) error {
	for _, step := range workflowSteps(wf) {
		schemaFile, ok := toolSchemaByKind[step.Kind]
		if !ok {
			continue
		}
		toolSchemaRaw, err := schemaFS.ReadFile("schemas/tools/" + schemaFile)
		if err != nil {
			return fmt.Errorf("E_SCHEMA_INVALID: tool schema not found for kind %s", step.Kind)
		}

		stepForSchema := step
		if strings.TrimSpace(stepForSchema.APIVersion) == "" {
			stepForSchema.APIVersion = "deck/v1alpha1"
		}
		if renderTemplates {
			renderedSpec, err := workflowexec.RenderSpec(stepForSchema.Spec, wf, nil, nil)
			if err != nil {
				return fmt.Errorf("E_TEMPLATE_RENDER: step %s (%s): %w", step.ID, step.Kind, err)
			}
			stepForSchema.Spec = renderedSpec
		}

		raw, err := json.Marshal(stepForSchema)
		if err != nil {
			return fmt.Errorf("marshal step for schema validation: %w", err)
		}

		result, err := gojsonschema.Validate(
			gojsonschema.NewBytesLoader(toolSchemaRaw),
			gojsonschema.NewBytesLoader(raw),
		)
		if err != nil {
			return fmt.Errorf("run tool schema validation: %w", err)
		}
		if result.Valid() {
			continue
		}

		msgs := make([]string, 0, len(result.Errors()))
		for _, e := range result.Errors() {
			msgs = append(msgs, e.String())
		}
		return fmt.Errorf("E_SCHEMA_INVALID: step %s (%s): %s", step.ID, step.Kind, strings.Join(msgs, "; "))
	}

	return nil
}

func validateSchema(name string, content []byte) error {
	schemaRaw, err := schemaFS.ReadFile("schemas/deck-workflow.schema.json")
	if err != nil {
		return fmt.Errorf("workflow schema not found: docs/schemas/deck-workflow.schema.json")
	}

	var doc any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	normalized, err := normalizeYAMLForJSON(doc)
	if err != nil {
		return fmt.Errorf("normalize workflow for schema validation: %w", err)
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal workflow for schema validation: %w", err)
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schemaRaw),
		gojsonschema.NewBytesLoader(raw),
	)
	if err != nil {
		return fmt.Errorf("run schema validation: %w", err)
	}

	if result.Valid() {
		return nil
	}

	msgs := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		msgs = append(msgs, e.String())
	}
	return fmt.Errorf("E_SCHEMA_INVALID: %s", strings.Join(msgs, "; "))
}

func validateSemantics(wf *config.Workflow) error {
	seenStepID := map[string]bool{}
	assignedRuntime := map[string]string{}

	for _, step := range workflowSteps(wf) {
		if step.ID == "" {
			continue
		}
		if seenStepID[step.ID] {
			return fmt.Errorf("E_DUPLICATE_STEP_ID: %s", step.ID)
		}
		seenStepID[step.ID] = true

		for runtimeVar, outputKey := range step.Register {
			if !runtimeVarNamePattern.MatchString(runtimeVar) {
				return fmt.Errorf("E_REGISTER_VAR_INVALID: %s", runtimeVar)
			}
			if isReservedRuntimeVar(runtimeVar) {
				return fmt.Errorf("E_RUNTIME_VAR_RESERVED: %s", runtimeVar)
			}
			if strings.TrimSpace(outputKey) == "" {
				return fmt.Errorf("E_REGISTER_OUTPUT_NOT_FOUND: empty output key in step %s", step.ID)
			}
			if !isValidOutputKey(step.Kind, outputKey) {
				return fmt.Errorf("E_REGISTER_OUTPUT_NOT_FOUND: step %s (%s) has no output key %s", step.ID, step.Kind, outputKey)
			}
			if previous, exists := assignedRuntime[runtimeVar]; exists {
				return fmt.Errorf("E_RUNTIME_VAR_REDEFINED: %s (previous step: %s)", runtimeVar, previous)
			}
			assignedRuntime[runtimeVar] = step.ID
		}
	}

	return nil
}

func isReservedRuntimeVar(runtimeVar string) bool {
	trimmed := strings.TrimSpace(runtimeVar)
	return trimmed == "host" || strings.HasPrefix(trimmed, "host.")
}

func isValidOutputKey(kind, outputKey string) bool {
	allowed, ok := registerOutputContract[kind]
	if !ok {
		return false
	}
	for _, v := range allowed {
		if v == outputKey {
			return true
		}
	}
	return false
}

func workflowSteps(wf *config.Workflow) []config.Step {
	if len(wf.Phases) > 0 {
		steps := make([]config.Step, 0)
		for _, phase := range wf.Phases {
			steps = append(steps, phase.Steps...)
		}
		return steps
	}

	return wf.Steps
}

func validateSingleBraceTemplates(name string, content []byte) error {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	for idx, line := range lines {
		matches := singleBraceTemplatePattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		return fmt.Errorf("E_TEMPLATE_SINGLE_BRACE: %s:%d: unsupported single-brace template %s", name, idx+1, strings.TrimSpace(matches[2]))
	}

	return nil
}

func normalizeYAMLForJSON(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			normalized, err := normalizeYAMLForJSON(v)
			if err != nil {
				return nil, err
			}
			out[k] = normalized
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string map key type %T", k)
			}
			normalized, err := normalizeYAMLForJSON(v)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			normalized, err := normalizeYAMLForJSON(typed[i])
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		return typed, nil
	}
}
