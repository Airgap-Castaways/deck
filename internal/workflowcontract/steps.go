package workflowcontract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	_ "github.com/Airgap-Castaways/deck/internal/stepspec"
)

type StepDefinition struct {
	APIVersion          string
	Kind                string
	Family              string
	FamilyTitle         string
	Group               string
	GroupOrder          int
	DocsPage            string
	DocsOrder           int
	SchemaFile          string
	ToolSchemaGenerator string
	Visibility          string
	Category            string
	Summary             string
	WhenToUse           string
	Roles               []string
	Outputs             []string
	Parallel            ParallelMetadata
}

type ParallelMetadata struct {
	ApplySafe        bool
	ApplyTargetPaths []string
	PrepareOutput    OutputRootConstraint
}

type OutputRootConstraint struct {
	Path    string
	Root    string
	Example string
}

type StepTypeKey struct {
	APIVersion string
	Kind       string
}

func StepDefinitions() ([]StepDefinition, error) {
	kinds := stepmeta.RegisteredKinds()
	defs := make([]StepDefinition, 0, len(kinds))
	for _, kind := range kinds {
		generator, err := generatorForKind(kind)
		if err != nil {
			return nil, err
		}
		def, err := stepDefFromMeta(kind, generator)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Kind < defs[j].Kind })
	return defs, nil
}

func StepDefinitionForKey(key StepTypeKey) (StepDefinition, bool, error) {
	defs, err := StepDefinitions()
	if err != nil {
		return StepDefinition{}, false, err
	}
	for _, def := range defs {
		if def.APIVersion == strings.TrimSpace(key.APIVersion) && def.Kind == strings.TrimSpace(key.Kind) {
			return def, true, nil
		}
	}
	return StepDefinition{}, false, nil
}

func stepDef(kind, family, familyTitle, group string, groupOrder int, docsPage string, docsOrder int, schemaFile, generator, visibility, category, summary, whenToUse string, roles, outputs []string, parallel ParallelMetadata) StepDefinition {
	def := StepDefinition{
		APIVersion:          BuiltInStepAPIVersion,
		Kind:                kind,
		Family:              family,
		FamilyTitle:         familyTitle,
		Group:               group,
		GroupOrder:          groupOrder,
		DocsPage:            docsPage,
		DocsOrder:           docsOrder,
		SchemaFile:          schemaFile,
		ToolSchemaGenerator: generator,
		Visibility:          visibility,
		Category:            category,
		Summary:             summary,
		WhenToUse:           whenToUse,
		Roles:               append([]string(nil), roles...),
		Outputs:             append([]string(nil), outputs...),
		Parallel:            parallel,
	}
	def.Parallel.ApplyTargetPaths = append([]string(nil), parallel.ApplyTargetPaths...)
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	return def
}

func stepDefFromMeta(kind string, generator string) (StepDefinition, error) {
	entry, ok, err := stepmeta.LookupCatalogEntry(kind)
	if err != nil {
		return StepDefinition{}, fmt.Errorf("lookup step metadata for %s: %w", kind, err)
	}
	if !ok {
		return StepDefinition{}, fmt.Errorf("missing stepmeta registration for %s", kind)
	}
	projection := stepmeta.ProjectWorkflow(entry, generator)
	return stepDef(
		projection.Kind,
		projection.Family,
		projection.FamilyTitle,
		projection.Group,
		projection.GroupOrder,
		projection.DocsPage,
		projection.DocsOrder,
		projection.SchemaFile,
		projection.Generator,
		projection.Visibility,
		projection.Category,
		projection.Summary,
		projection.WhenToUse,
		projection.Roles,
		projection.Outputs,
		ParallelMetadata{
			ApplySafe:        projection.Parallel.ApplySafe,
			ApplyTargetPaths: append([]string(nil), projection.Parallel.ApplyTargetPaths...),
			PrepareOutput: OutputRootConstraint{
				Path:    projection.Parallel.PrepareOutput.Path,
				Root:    projection.Parallel.PrepareOutput.Root,
				Example: projection.Parallel.PrepareOutput.Example,
			},
		},
	), nil
}

func generatorForKind(kind string) (string, error) {
	entry, ok, err := stepmeta.LookupCatalogEntry(kind)
	if err != nil {
		return "", fmt.Errorf("lookup step metadata for %s: %w", kind, err)
	}
	if !ok {
		return "", fmt.Errorf("missing stepmeta registration for %s", kind)
	}
	projection := stepmeta.ProjectWorkflow(entry, "")
	schemaFile := strings.TrimSpace(projection.SchemaFile)
	if strings.HasSuffix(schemaFile, ".schema.json") {
		return strings.TrimSuffix(schemaFile, ".schema.json"), nil
	}
	return strings.ToLower(strings.TrimSpace(kind)), nil
}
