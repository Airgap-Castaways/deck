package workflowcontract

import (
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
}

type StepTypeKey struct {
	APIVersion string
	Kind       string
}

func StepDefinitions() []StepDefinition {
	kinds := stepmeta.RegisteredKinds()
	defs := make([]StepDefinition, 0, len(kinds))
	for _, kind := range kinds {
		defs = append(defs, stepDefFromMeta(kind, generatorForKind(kind), categoryForKind(kind)))
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Kind < defs[j].Kind })
	return defs
}

func StepDefinitionForKey(key StepTypeKey) (StepDefinition, bool) {
	for _, def := range StepDefinitions() {
		if def.APIVersion == strings.TrimSpace(key.APIVersion) && def.Kind == strings.TrimSpace(key.Kind) {
			return def, true
		}
	}
	return StepDefinition{}, false
}

func stepDef(kind, family, familyTitle, group string, groupOrder int, docsPage string, docsOrder int, schemaFile, generator, visibility, category, summary, whenToUse string, roles, outputs []string) StepDefinition {
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
	}
	sort.Strings(def.Roles)
	sort.Strings(def.Outputs)
	return def
}

func stepDefFromMeta(kind string, generator string, category string) StepDefinition {
	entry, ok, err := stepmeta.LookupCatalogEntry(kind)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("missing stepmeta registration for " + kind)
	}
	projection := stepmeta.ProjectWorkflow(entry, category, generator)
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
	)
}

func generatorForKind(kind string) string {
	entry, ok, err := stepmeta.LookupCatalogEntry(kind)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("missing stepmeta registration for " + kind)
	}
	projection := stepmeta.ProjectWorkflow(entry, "", "")
	schemaFile := strings.TrimSpace(projection.SchemaFile)
	if strings.HasSuffix(schemaFile, ".schema.json") {
		return strings.TrimSuffix(schemaFile, ".schema.json")
	}
	return strings.ToLower(strings.TrimSpace(kind))
}

func categoryForKind(kind string) string {
	entry, ok, err := stepmeta.LookupCatalogEntry(kind)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("missing stepmeta registration for " + kind)
	}
	switch strings.TrimSpace(entry.Definition.Family) {
	case "command":
		return "advanced"
	case "containerd":
		return "runtime"
	case "directory", "file", "symlink":
		return "filesystem"
	case "image":
		return "containers"
	case "cluster-check", "kubeadm":
		return "kubernetes"
	case "package", "repository":
		return "packages"
	case "wait":
		return "control-flow"
	case "host-check", "kernel-module", "service", "swap", "sysctl", "systemd-unit":
		return "system"
	default:
		return "system"
	}
}
