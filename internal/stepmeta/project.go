package stepmeta

import (
	"sort"
	"strings"
)

// StepCatalogEntry is the explicit source-of-truth entry consumed by workflow,
// schema, docs, and ask projections.
type StepCatalogEntry = Entry

func LookupCatalogEntry(kind string) (StepCatalogEntry, bool, error) {
	return Lookup(kind)
}

type WorkflowProjection struct {
	Kind        string
	Family      string
	FamilyTitle string
	Group       string
	GroupOrder  int
	DocsPage    string
	DocsOrder   int
	SchemaFile  string
	Visibility  string
	Category    string
	Summary     string
	WhenToUse   string
	Roles       []string
	Outputs     []string
	Generator   string
}

type ToolProjection struct {
	Kind      string
	Category  string
	Summary   string
	WhenToUse string
	Example   string
	Notes     []string
	FieldDocs map[string]FieldDoc
}

type SchemaProjection struct {
	SpecType any
	Patch    func(root map[string]any)
	Source   SourceRef
}

func ProjectWorkflow(entry Entry, generator string) WorkflowProjection {
	roles := append([]string(nil), entry.Definition.Roles...)
	outputs := append([]string(nil), entry.Definition.Outputs...)
	sort.Strings(roles)
	sort.Strings(outputs)
	category := CategoryForEntry(entry)
	return WorkflowProjection{
		Kind:        entry.Definition.Kind,
		Family:      entry.Definition.Family,
		FamilyTitle: entry.Definition.FamilyTitle,
		Group:       entry.Definition.Group,
		GroupOrder:  entry.Definition.GroupOrder,
		DocsPage:    entry.Definition.DocsPage,
		DocsOrder:   entry.Definition.DocsOrder,
		SchemaFile:  entry.Definition.SchemaFile,
		Visibility:  entry.Definition.Visibility,
		Category:    category,
		Summary:     entry.Docs.Summary,
		WhenToUse:   entry.Docs.WhenToUse,
		Roles:       roles,
		Outputs:     outputs,
		Generator:   generator,
	}
}

func ProjectTool(entry Entry) ToolProjection {
	fieldDocs := make(map[string]FieldDoc, len(entry.Docs.Fields))
	for _, field := range entry.Docs.Fields {
		fieldDocs[field.Path] = FieldDoc{Description: field.Description, Example: field.Example}
	}
	return ToolProjection{
		Kind:      entry.Definition.Kind,
		Category:  CategoryForEntry(entry),
		Summary:   entry.Docs.Summary,
		WhenToUse: entry.Docs.WhenToUse,
		Example:   entry.Docs.Example,
		Notes:     append([]string(nil), entry.Docs.Notes...),
		FieldDocs: fieldDocs,
	}
}

func CategoryForEntry(entry Entry) string {
	if category := strings.TrimSpace(entry.Definition.Category); category != "" {
		return category
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

func ProjectAsk(entry Entry) AskMetadata {
	return cloneDefinition(entry.Definition).Ask
}

func ProjectSchema(entry Entry) SchemaProjection {
	return SchemaProjection{SpecType: entry.Schema.SpecType, Patch: entry.Schema.Patch, Source: entry.Schema.Source}
}
