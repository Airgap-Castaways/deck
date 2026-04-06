package askknowledge

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
)

type Bundle struct {
	Workflow    WorkflowKnowledge
	Topology    TopologyKnowledge
	Components  ComponentKnowledge
	Vars        VarsKnowledge
	Policy      PolicyKnowledge
	Steps       []StepKnowledge
	Constraints []ConstraintKnowledge
}

type WorkflowKnowledge struct {
	SupportedRoles   []string
	SupportedVersion string
	TopLevelModes    []string
	RequiredFields   []string
	Notes            []string
	PhaseExample     string
	StepsExample     string
}

type TopologyKnowledge struct {
	WorkflowRoot     string
	ScenarioDir      string
	ComponentDir     string
	VarsPath         string
	CanonicalPrepare string
	CanonicalApply   string
	AllowedPaths     []string
}

type ComponentKnowledge struct {
	ImportRule      string
	FragmentRule    string
	ImportExample   string
	FragmentExample string
	AllowedRootKeys []string
}

type VarsKnowledge struct {
	Path        string
	Summary     string
	PreferFor   []string
	AvoidFor    []string
	ExampleKeys []string
}

type PolicyKnowledge struct {
	AssumeOfflineByDefault bool
	PrepareArtifactKinds   []string
	ForbiddenApplyActions  []string
	VarsAdvisory           []string
	ComponentAdvisory      []string
}

type StepKnowledge struct {
	Kind                     string
	Category                 string
	Summary                  string
	WhenToUse                string
	SchemaFile               string
	AllowedRoles             []string
	Outputs                  []string
	Example                  string
	KeyFields                []askcontext.StepFieldContext
	SchemaRuleSummaries      []string
	ConstrainedLiteralFields []askcontext.ConstrainedFieldHint
}

type ConstraintKnowledge struct {
	StepKind      string
	Path          string
	AllowedValues []string
	Guidance      string
	SourceRef     string
}

var (
	bundleOnce sync.Once
	bundleData Bundle
)

func Current() Bundle {
	bundleOnce.Do(func() {
		bundleData = buildBundle()
	})
	return bundleData
}

func buildBundle() Bundle {
	catalog := askcatalog.Current()
	bundle := Bundle{
		Workflow: WorkflowKnowledge{
			SupportedRoles:   append([]string(nil), catalog.Workflow.SupportedModes...),
			SupportedVersion: catalog.Workflow.SupportedVersion,
			TopLevelModes:    append([]string(nil), catalog.Workflow.TopLevelModes...),
			RequiredFields:   append([]string(nil), catalog.Workflow.RequiredFields...),
			Notes:            append([]string(nil), catalog.Workflow.InvariantNotes...),
			PhaseExample:     strings.TrimSpace(catalog.Workflow.PhaseExample),
			StepsExample:     strings.TrimSpace(catalog.Workflow.StepsExample),
		},
		Topology: TopologyKnowledge{
			WorkflowRoot:     catalog.Workspace.WorkflowRoot,
			ScenarioDir:      catalog.Workspace.ScenarioDir,
			ComponentDir:     catalog.Workspace.ComponentDir,
			VarsPath:         catalog.Workspace.VarsPath,
			CanonicalPrepare: catalog.Workspace.CanonicalPrepare,
			CanonicalApply:   catalog.Workspace.CanonicalApply,
			AllowedPaths:     append([]string(nil), catalog.Workspace.AllowedPaths...),
		},
		Components: ComponentKnowledge{
			ImportRule:      catalog.Components.ImportRule,
			FragmentRule:    catalog.Components.FragmentRule,
			ImportExample:   strings.TrimSpace(catalog.Components.ImportExample),
			FragmentExample: strings.TrimSpace(catalog.Components.FragmentExample),
			AllowedRootKeys: append([]string(nil), catalog.Components.AllowedRootKeys...),
		},
		Vars: VarsKnowledge{
			Path:        catalog.Vars.Path,
			Summary:     catalog.Vars.Summary,
			PreferFor:   append([]string(nil), catalog.Vars.PreferFor...),
			AvoidFor:    append([]string(nil), catalog.Vars.AvoidFor...),
			ExampleKeys: append([]string(nil), catalog.Vars.ExampleKeys...),
		},
		Policy: PolicyKnowledge{
			AssumeOfflineByDefault: catalog.Policy.AssumeOfflineByDefault,
			PrepareArtifactKinds:   append([]string(nil), catalog.Policy.PrepareArtifactKinds...),
			ForbiddenApplyActions:  append([]string(nil), catalog.Policy.ForbiddenApplyActions...),
			VarsAdvisory:           append([]string(nil), catalog.Policy.VarsAdvisory...),
			ComponentAdvisory:      append([]string(nil), catalog.Policy.ComponentAdvisory...),
		},
	}
	for _, step := range catalog.StepKinds() {
		bundle.Steps = append(bundle.Steps, StepKnowledge{
			Kind:                     step.Kind,
			Category:                 step.Category,
			Summary:                  step.Summary,
			WhenToUse:                step.WhenToUse,
			SchemaFile:               step.SchemaFile,
			AllowedRoles:             append([]string(nil), step.AllowedRoles...),
			Outputs:                  append([]string(nil), step.Outputs...),
			KeyFields:                stepFieldContexts(step),
			SchemaRuleSummaries:      append([]string(nil), step.RuleSummaries...),
			ConstrainedLiteralFields: constrainedFieldHints(step.ConstrainedLiteralFields),
		})
		for _, field := range step.ConstrainedLiteralFields {
			bundle.Constraints = append(bundle.Constraints, ConstraintKnowledge{
				StepKind:      step.Kind,
				Path:          field.Path,
				AllowedValues: append([]string(nil), field.AllowedValues...),
				Guidance:      field.Guidance,
				SourceRef:     step.SchemaFile,
			})
		}
	}
	sort.Slice(bundle.Steps, func(i, j int) bool { return bundle.Steps[i].Kind < bundle.Steps[j].Kind })
	sort.Slice(bundle.Constraints, func(i, j int) bool {
		if bundle.Constraints[i].StepKind == bundle.Constraints[j].StepKind {
			return bundle.Constraints[i].Path < bundle.Constraints[j].Path
		}
		return bundle.Constraints[i].StepKind < bundle.Constraints[j].StepKind
	})
	return bundle
}

func stepFieldContexts(step askcatalog.Step) []askcontext.StepFieldContext {
	keys := append([]string(nil), step.KeyFields...)
	if len(keys) == 0 {
		for path, field := range step.Fields {
			if field.Requirement == "required" {
				keys = append(keys, path)
			}
		}
		sort.Strings(keys)
		if len(keys) > 5 {
			keys = keys[:5]
		}
	}
	out := make([]askcontext.StepFieldContext, 0, len(keys))
	for _, key := range keys {
		field, ok := step.Fields[key]
		if !ok {
			continue
		}
		out = append(out, askcontext.StepFieldContext{Path: key, Description: field.Description, Example: field.Example, Requirement: string(field.Requirement)})
	}
	return out
}

func constrainedFieldHints(items []stepmeta.ConstrainedLiteralField) []askcontext.ConstrainedFieldHint {
	out := make([]askcontext.ConstrainedFieldHint, 0, len(items))
	for _, item := range items {
		out = append(out, askcontext.ConstrainedFieldHint{Path: item.Path, AllowedValues: append([]string(nil), item.AllowedValues...), Guidance: item.Guidance})
	}
	return out
}

func (b Bundle) WorkflowPromptBlock() string {
	lines := []string{
		"Workflow source-of-truth:",
		fmt.Sprintf("- supported roles: %s", strings.Join(b.Workflow.SupportedRoles, ", ")),
		fmt.Sprintf("- supported workflow version: %s", b.Workflow.SupportedVersion),
		fmt.Sprintf("- top-level workflow modes: %s", strings.Join(b.Workflow.TopLevelModes, ", ")),
		fmt.Sprintf("- allowed generated paths: %s", strings.Join(b.Topology.AllowedPaths, ", ")),
	}
	if len(b.Workflow.RequiredFields) > 0 {
		lines = append(lines, fmt.Sprintf("- required workflow fields: %s", strings.Join(b.Workflow.RequiredFields, ", ")))
	}
	for _, note := range b.Workflow.Notes {
		lines = append(lines, "- "+strings.TrimSpace(note))
	}
	return strings.Join(lines, "\n")
}

func (b Bundle) PolicyPromptBlock() string {
	lines := []string{"Authoring policy from deck metadata:"}
	if b.Policy.AssumeOfflineByDefault {
		lines = append(lines, "- assume offline unless the request explicitly says online")
	}
	lines = append(lines,
		"- use prepare only when packages, images, binaries, archives, bundles, or repository mirrors must be staged",
		"- keep typed schema fields as native YAML values instead of stringifying arrays or objects",
		"- keep vars and components advisory unless the request or plan requires them",
	)
	for _, rule := range b.Policy.ForbiddenApplyActions {
		lines = append(lines, "- apply should avoid: "+strings.TrimSpace(rule))
	}
	return strings.Join(lines, "\n")
}

func (b Bundle) ComponentPromptBlock() string {
	lines := []string{
		"Component and import source-of-truth:",
		"- " + strings.TrimSpace(b.Components.ImportRule),
		"- " + strings.TrimSpace(b.Components.FragmentRule),
		fmt.Sprintf("- component fragment root keys: %s", strings.Join(b.Components.AllowedRootKeys, ", ")),
	}
	if b.Components.ImportExample != "" {
		lines = append(lines, "- import example:")
		for _, line := range strings.Split(b.Components.ImportExample, "\n") {
			lines = append(lines, "  "+line)
		}
	}
	if b.Components.FragmentExample != "" {
		lines = append(lines, "- component fragment example:")
		for _, line := range strings.Split(b.Components.FragmentExample, "\n") {
			lines = append(lines, "  "+line)
		}
	}
	return strings.Join(lines, "\n")
}

func (b Bundle) VarsPromptBlock() string {
	lines := []string{
		"Vars source-of-truth:",
		"- path: " + b.Vars.Path,
		"- " + strings.TrimSpace(b.Vars.Summary),
		"- prefer vars for: " + strings.Join(b.Vars.PreferFor, ", "),
		"- avoid vars for: " + strings.Join(b.Vars.AvoidFor, ", "),
	}
	if len(b.Vars.ExampleKeys) > 0 {
		lines = append(lines, "- example vars keys: "+strings.Join(b.Vars.ExampleKeys, ", "))
	}
	return strings.Join(lines, "\n")
}

func (b Bundle) ConstraintPromptBlock(stepKinds []string) string {
	allowed := map[string]bool{}
	for _, kind := range stepKinds {
		allowed[strings.TrimSpace(kind)] = true
	}
	lines := []string{"Schema-constrained literal fields:"}
	count := 0
	for _, item := range b.Constraints {
		if len(allowed) > 0 && !allowed[item.StepKind] {
			continue
		}
		line := fmt.Sprintf("- %s %s", item.StepKind, item.Path)
		if len(item.AllowedValues) > 0 {
			line += fmt.Sprintf(" allowed=%s", strings.Join(item.AllowedValues, ", "))
		}
		if strings.TrimSpace(item.Guidance) != "" {
			line += " guidance=" + strings.TrimSpace(item.Guidance)
		}
		lines = append(lines, line)
		count++
	}
	if count == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
