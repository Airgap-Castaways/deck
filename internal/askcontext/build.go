package askcontext

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

var (
	manifestOnce sync.Once
	manifestData Manifest
)

func Current() Manifest {
	manifestOnce.Do(func() {
		manifestData = buildManifest()
	})
	return manifestData
}

func buildManifest() Manifest {
	catalog := askcatalog.Current()
	cli := AskCommandMeta()
	return Manifest{
		CLI: CLIContext{
			Command:             "deck ask",
			PlanSubcommand:      "deck ask plan",
			ConfigSubcommand:    "deck ask config",
			TopLevelDescription: cli.Short,
			ImportantFlags:      append([]CLIFlag(nil), cli.Flags...),
			Examples: []string{
				fmt.Sprintf(`deck ask "explain what %s does"`, workspacepaths.CanonicalApplyWorkflow),
				`deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"`,
				`deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"`,
			},
		},
		Topology: WorkspaceTopology{
			WorkflowRoot:      catalog.Workspace.WorkflowRoot,
			ScenarioDir:       catalog.Workspace.ScenarioDir,
			ComponentDir:      catalog.Workspace.ComponentDir,
			VarsPath:          catalog.Workspace.VarsPath,
			AllowedPaths:      append([]string(nil), catalog.Workspace.AllowedPaths...),
			CanonicalPrepare:  catalog.Workspace.CanonicalPrepare,
			CanonicalApply:    catalog.Workspace.CanonicalApply,
			GeneratedPathNote: catalog.Workspace.GeneratedPathNote,
		},
		Workflow: WorkflowRules{
			Summary:          catalog.Workflow.Summary,
			TopLevelModes:    append([]string(nil), catalog.Workflow.TopLevelModes...),
			SupportedModes:   append([]string(nil), catalog.Workflow.SupportedModes...),
			SupportedVersion: catalog.Workflow.SupportedVersion,
			ImportRule:       catalog.Workflow.ImportRule,
			RequiredFields:   append([]string(nil), catalog.Workflow.RequiredFields...),
			PhaseRules:       append([]string(nil), catalog.Workflow.PhaseRules...),
			StepRules:        append([]string(nil), catalog.Workflow.StepRules...),
			PhaseExample:     catalog.Workflow.PhaseExample,
			StepsExample:     catalog.Workflow.StepsExample,
			Notes:            append([]string(nil), catalog.Workflow.InvariantNotes...),
		},
		Policy: AuthoringPolicy{
			AssumeOfflineByDefault: catalog.Policy.AssumeOfflineByDefault,
			PrepareArtifactKinds:   append([]string(nil), catalog.Policy.PrepareArtifactKinds...),
			ForbiddenApplyActions:  append([]string(nil), catalog.Policy.ForbiddenApplyActions...),
			VarsAdvisory:           append([]string(nil), catalog.Policy.VarsAdvisory...),
			ComponentAdvisory:      append([]string(nil), catalog.Policy.ComponentAdvisory...),
		},
		Modes:      modeGuidance(catalog.Modes),
		Components: componentGuidance(catalog.Components),
		Vars:       varsGuidance(catalog.Vars),
		StepKinds:  buildStepKinds(),
	}
}

func AllowedGeneratedPathPatterns() []string {
	return askcatalog.AllowedGeneratedPathPatterns()
}

func AllowedGeneratedPath(path string) bool {
	return askcatalog.AllowedGeneratedPath(path)
}

func buildStepKinds() []StepKindContext {
	catalog := askcatalog.Current()
	out := make([]StepKindContext, 0, len(catalog.StepKinds()))
	for _, step := range catalog.StepKinds() {
		ctx := StepKindContext{
			Kind:                     step.Kind,
			Category:                 step.Category,
			Summary:                  step.Summary,
			WhenToUse:                step.WhenToUse,
			SchemaFile:               step.SchemaFile,
			AllowedRoles:             append([]string(nil), step.AllowedRoles...),
			Outputs:                  append([]string(nil), step.Outputs...),
			KeyFields:                stepFieldContexts(step),
			SchemaRuleSummaries:      append([]string(nil), step.RuleSummaries...),
			ValidationHints:          validationHints(step.ValidationHints),
			Capabilities:             append([]string(nil), step.Capabilities...),
			ProducesArtifacts:        append([]string(nil), step.Contract.ProducesArtifacts...),
			ConsumesArtifacts:        append([]string(nil), step.Contract.ConsumesArtifacts...),
			PublishesState:           append([]string(nil), step.Contract.PublishesState...),
			ConsumesState:            append([]string(nil), step.Contract.ConsumesState...),
			RoleSensitive:            step.Contract.RoleSensitive,
			VerificationRelated:      step.Contract.VerificationRelated,
			MatchSignals:             append([]string(nil), step.MatchSignals...),
			AntiSignals:              append([]string(nil), step.AntiSignals...),
			QualityRules:             qualityRules(step.QualityRules),
			ConstrainedLiteralFields: constrainedLiteralFields(step.ConstrainedLiteralFields),
		}
		out = append(out, ctx)
	}
	return out
}

func stepFieldContexts(step askcatalog.Step) []StepFieldContext {
	keys := append([]string(nil), step.KeyFields...)
	if len(keys) == 0 {
		for path, field := range step.Fields {
			if field.Requirement == "required" {
				keys = append(keys, path)
			}
		}
		sortStrings(keys)
		if len(keys) > 5 {
			keys = keys[:5]
		}
	}
	out := make([]StepFieldContext, 0, len(keys))
	for _, key := range keys {
		field, ok := step.Fields[key]
		if !ok {
			continue
		}
		out = append(out, StepFieldContext{Path: key, Description: field.Description, Example: field.Example, Requirement: string(field.Requirement)})
	}
	return out
}

func validationHints(items []stepmeta.ValidationHint) []ValidationHint {
	out := make([]ValidationHint, 0, len(items))
	for _, item := range items {
		out = append(out, ValidationHint{ErrorContains: item.ErrorContains, Fix: item.Fix})
	}
	return out
}

func qualityRules(items []stepmeta.QualityRule) []QualityRule {
	out := make([]QualityRule, 0, len(items))
	for _, item := range items {
		out = append(out, QualityRule{Trigger: item.Trigger, Message: item.Message, Level: item.Level})
	}
	return out
}

func constrainedLiteralFields(items []stepmeta.ConstrainedLiteralField) []ConstrainedFieldHint {
	out := make([]ConstrainedFieldHint, 0, len(items))
	for _, item := range items {
		out = append(out, ConstrainedFieldHint{Path: item.Path, AllowedValues: append([]string(nil), item.AllowedValues...), Guidance: item.Guidance})
	}
	return out
}

func modeGuidance(items []askcatalog.ModeRules) []ModeGuidance {
	out := make([]ModeGuidance, 0, len(items))
	for _, item := range items {
		out = append(out, ModeGuidance{
			Mode:        item.Mode,
			Summary:     item.Summary,
			WhenToUse:   item.WhenToUse,
			Prefer:      append([]string(nil), item.Prefer...),
			Avoid:       append([]string(nil), item.Avoid...),
			OutputFiles: append([]string(nil), item.OutputFiles...),
		})
	}
	return out
}

func componentGuidance(item askcatalog.ComponentRules) ComponentGuidance {
	return ComponentGuidance{
		Summary:         item.Summary,
		ImportRule:      item.ImportRule,
		ReuseRule:       item.ReuseRule,
		LocationRule:    item.LocationRule,
		FragmentRule:    item.FragmentRule,
		ImportExample:   item.ImportExample,
		FragmentExample: item.FragmentExample,
	}
}

func varsGuidance(item askcatalog.VarsRules) VarsGuidance {
	return VarsGuidance{
		Path:        item.Path,
		Summary:     item.Summary,
		PreferFor:   append([]string(nil), item.PreferFor...),
		AvoidFor:    append([]string(nil), item.AvoidFor...),
		ExampleKeys: append([]string(nil), item.ExampleKeys...),
	}
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.TrimSpace(values[i]) < strings.TrimSpace(values[j])
	})
}
