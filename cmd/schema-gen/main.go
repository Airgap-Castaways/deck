package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/schemadoc"
)

type schemaDoc struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Visibility  string         `json:"x-deck-visibility"`
	Required    []string       `json:"required"`
	Properties  map[string]any `json:"properties"`
}

type toolSchemaDoc struct {
	File        string
	Kind        string
	Title       string
	Description string
	Visibility  string
	SpecFields  []string
	Required    []string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	workflowSchemaPath := filepath.Join(root, "schemas", "deck-workflow.schema.json")
	workflowSchema, err := generateWorkflowSchema()
	if err != nil {
		return err
	}
	if err := writeJSONFile(workflowSchemaPath, workflowSchema); err != nil {
		return err
	}
	componentFragmentSchemaPath := filepath.Join(root, "schemas", "deck-component-fragment.schema.json")
	componentFragmentSchema, err := generateComponentFragmentSchema()
	if err != nil {
		return err
	}
	if err := writeJSONFile(componentFragmentSchemaPath, componentFragmentSchema); err != nil {
		return err
	}
	if err := writeToolSchemas(root); err != nil {
		return err
	}

	toolDefinitionSchemaPath := filepath.Join(root, "schemas", "deck-tooldefinition.schema.json")
	toolDefinitionSchema, err := generateToolDefinitionSchema()
	if err != nil {
		return err
	}
	if err := writeJSONFile(toolDefinitionSchemaPath, toolDefinitionSchema); err != nil {
		return err
	}

	workflowSchemaMap, err := readSchemaMap(workflowSchemaPath)
	if err != nil {
		return err
	}
	componentFragmentSchemaMap, err := readSchemaMap(componentFragmentSchemaPath)
	if err != nil {
		return err
	}
	toolDefinitionSchemaMap, err := readSchemaMap(toolDefinitionSchemaPath)
	if err != nil {
		return err
	}
	tools, err := loadToolSchemas(filepath.Join(root, "schemas", "tools"))
	if err != nil {
		return err
	}
	groupPages, err := loadGroupPageInputs(filepath.Join(root, "schemas", "tools"))
	if err != nil {
		return err
	}

	if err := writeGeneratedSchemaDocs(root, workflowSchemaMap, componentFragmentSchemaMap, toolDefinitionSchemaMap, groupPages); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "schemas", "README.md"), renderSchemasReadme(tools)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "schemas", "tools", "README.md"), renderToolSchemasReadme(tools)); err != nil {
		return err
	}
	return nil
}

func writeGeneratedSchemaDocs(root string, workflowSchema, componentFragmentSchema, toolDefinitionSchema map[string]any, groupPages []schemadoc.PageInput) error {
	if err := removeDirIfExists(filepath.Join(root, "docs", "reference", "schema")); err != nil {
		return err
	}
	if err := removeDirIfExists(filepath.Join(root, "docs", "_generated")); err != nil {
		return err
	}
	if err := removeDirIfExists(filepath.Join(root, "docs", "reference", "groups")); err != nil {
		return err
	}
	if err := removeDirIfExists(filepath.Join(root, "docs", "reference", "typed-steps")); err != nil {
		return err
	}
	if err := removeDirIfExists(filepath.Join(root, "docs", "reference", "typed-steps.md")); err != nil {
		return err
	}
	if err := removeDirIfExists(filepath.Join(root, "docs", "reference", "step-kinds")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(root, "docs", "reference", "step-kinds.md"), schemadoc.RenderStepKindsPage(groupPages)); err != nil {
		return err
	}
	if err := syncGeneratedBlock(filepath.Join(root, "docs", "reference", "workflow-model.md"), workflowSchemaBlockBegin, workflowSchemaBlockEnd, schemadoc.RenderWorkflowSchemaPartial("../../schemas/deck-workflow.schema.json", workflowSchema, schemadoc.WorkflowMeta())); err != nil {
		return err
	}
	if err := syncGeneratedBlock(filepath.Join(root, "docs", "reference", "workflow-model.md"), systemVariablesBlockBegin, systemVariablesBlockEnd, schemadoc.RenderSystemVariablesPartial()); err != nil {
		return err
	}
	if err := syncGeneratedBlock(filepath.Join(root, "docs", "reference", "workspace-layout.md"), componentFragmentBlockBegin, componentFragmentBlockEnd, schemadoc.RenderComponentFragmentSchemaPartial("../../schemas/deck-component-fragment.schema.json", componentFragmentSchema, schemadoc.ComponentFragmentMeta())); err != nil {
		return err
	}
	if err := syncGeneratedBlock(filepath.Join(root, "docs", "contributing", "tool-definition-schema.md"), toolDefinitionBlockBegin, toolDefinitionBlockEnd, schemadoc.RenderToolDefinitionSchemaPartial("../../schemas/deck-tooldefinition.schema.json", toolDefinitionSchema, schemadoc.ToolDefinitionMeta())); err != nil {
		return err
	}
	for _, page := range schemadoc.StepKindPages(groupPages) {
		if err := writeFile(filepath.Join(root, "docs", "reference", "step-kinds", schemadoc.StepKindSlug(page.Variant.Kind)+".md"), schemadoc.RenderStepKindPage(page)); err != nil {
			return err
		}
	}
	return nil
}

func removeDirIfExists(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(dir)
}

func writeToolSchemas(root string) error {
	definitions, err := toolSchemaDefinitions()
	if err != nil {
		return err
	}
	if err := removeStaleGeneratedSchemas(filepath.Join(root, "schemas", "tools"), definitions); err != nil {
		return err
	}
	for name, schema := range definitions {
		if err := writeJSONFile(filepath.Join(root, "schemas", "tools", name), schema); err != nil {
			return err
		}
	}
	return nil
}

func removeStaleGeneratedSchemas(dir string, definitions map[string]map[string]any) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		if _, ok := definitions[entry.Name()]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
