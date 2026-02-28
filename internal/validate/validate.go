package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/taedi90/deck/internal/config"
	"github.com/xeipuuv/gojsonschema"
)

var runtimeVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var toolSchemaByKind = map[string]string{
	"CheckHost":           "check-host.schema.json",
	"DownloadPackages":    "download-packages.schema.json",
	"DownloadK8sPackages": "download-k8s-packages.schema.json",
	"DownloadImages":      "download-images.schema.json",
	"DownloadFile":        "download-file.schema.json",
	"InstallPackages":     "install-packages.schema.json",
	"WriteFile":           "write-file.schema.json",
	"EditFile":            "edit-file.schema.json",
	"CopyFile":            "copy-file.schema.json",
	"Sysctl":              "sysctl.schema.json",
	"Modprobe":            "modprobe.schema.json",
	"RunCommand":          "run-command.schema.json",
	"VerifyImages":        "verify-images.schema.json",
	"KubeadmInit":         "kubeadm-init.schema.json",
	"KubeadmJoin":         "kubeadm-join.schema.json",
}

var registerOutputContract = map[string][]string{
	"DownloadFile":        {"path", "artifacts"},
	"DownloadPackages":    {"artifacts"},
	"DownloadK8sPackages": {"artifacts"},
	"DownloadImages":      {"artifacts"},
	"WriteFile":           {"path"},
	"CopyFile":            {"dest"},
	"KubeadmInit":         {"joinFile"},
}

// File validates workflow structure and semantic rules.
func File(path string) error {
	if path == "" {
		return fmt.Errorf("file path is empty")
	}

	wf, err := config.Load(path)
	if err != nil {
		return err
	}

	if wf.Version == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(wf.Version) != "v1" {
		return fmt.Errorf("unsupported version: %s", wf.Version)
	}

	if err := validateSchema(wf); err != nil {
		return err
	}
	if err := validateToolSchemas(wf); err != nil {
		return err
	}
	if err := validateSemantics(wf); err != nil {
		return err
	}

	return nil
}

func validateToolSchemas(wf *config.Workflow) error {
	schemaPath, err := workflowSchemaPath()
	if err != nil {
		return err
	}
	toolsDir := filepath.Join(filepath.Dir(schemaPath), "tools")

	for _, phase := range wf.Phases {
		for _, step := range phase.Steps {
			schemaFile, ok := toolSchemaByKind[step.Kind]
			if !ok {
				continue
			}
			toolSchemaPath := filepath.Join(toolsDir, schemaFile)
			if _, err := os.Stat(toolSchemaPath); err != nil {
				return fmt.Errorf("E_SCHEMA_INVALID: tool schema not found for kind %s", step.Kind)
			}

			raw, err := json.Marshal(step)
			if err != nil {
				return fmt.Errorf("marshal step for schema validation: %w", err)
			}

			result, err := gojsonschema.Validate(
				gojsonschema.NewReferenceLoader("file://"+toolSchemaPath),
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
	}

	return nil
}

func validateSchema(wf *config.Workflow) error {
	schemaPath, err := workflowSchemaPath()
	if err != nil {
		return err
	}

	raw, err := json.Marshal(wf)
	if err != nil {
		return fmt.Errorf("marshal workflow for schema validation: %w", err)
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewReferenceLoader("file://"+schemaPath),
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

func workflowSchemaPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	cur := wd
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(cur, "docs", "schemas", "deck-workflow.schema.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}

	return "", fmt.Errorf("workflow schema not found: docs/schemas/deck-workflow.schema.json")
}

func validateSemantics(wf *config.Workflow) error {
	seenStepID := map[string]bool{}
	assignedRuntime := map[string]string{}

	for _, phase := range wf.Phases {
		for _, step := range phase.Steps {
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
	}

	return nil
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
