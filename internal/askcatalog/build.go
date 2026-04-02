package askcatalog

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/Airgap-Castaways/deck/internal/schemadoc"
	"github.com/Airgap-Castaways/deck/internal/schemafacts"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
	deckschemas "github.com/Airgap-Castaways/deck/schemas"
)

var (
	once sync.Once
	data Catalog
)

func Current() Catalog {
	once.Do(func() {
		data = build()
	})
	return data
}

func AllowedGeneratedPath(path string) bool {
	return workspacepaths.IsAllowedAuthoringPath(path)
}

func AllowedGeneratedPathPatterns() []string {
	return workspacepaths.AllowedAuthoringPathPatterns()
}

func build() Catalog {
	defs := workflowexec.BuiltInTypeDefinitions()
	steps := map[string]Step{}
	ordered := make([]string, 0, len(defs))
	for _, def := range defs {
		entry, ok, err := stepmeta.LookupCatalogEntry(def.Step.Kind)
		if err != nil || !ok {
			continue
		}
		workflowMeta := stepmeta.ProjectWorkflow(entry, def.Step.Category, def.Step.ToolSchemaGenerator)
		toolMeta := schemadoc.ToolMetaForDefinition(def.Step)
		askMeta := stepmeta.ProjectAsk(entry)
		facts := schemaFactsForSchemaFile(workflowMeta.SchemaFile)
		steps[workflowMeta.Kind] = Step{
			Kind:                     workflowMeta.Kind,
			Category:                 workflowMeta.Category,
			Summary:                  toolMeta.Summary,
			WhenToUse:                toolMeta.WhenToUse,
			SchemaFile:               workflowMeta.SchemaFile,
			AllowedRoles:             append([]string(nil), workflowMeta.Roles...),
			Outputs:                  append([]string(nil), workflowMeta.Outputs...),
			Capabilities:             append([]string(nil), askMeta.Capabilities...),
			Contract:                 projectContract(askMeta.ContractHints),
			MatchSignals:             append([]string(nil), askMeta.MatchSignals...),
			AntiSignals:              append([]string(nil), askMeta.AntiSignals...),
			ValidationHints:          append([]stepmeta.ValidationHint(nil), askMeta.ValidationHints...),
			ConstrainedLiteralFields: append([]stepmeta.ConstrainedLiteralField(nil), askMeta.ConstrainedLiteralFields...),
			QualityRules:             append([]stepmeta.QualityRule(nil), askMeta.QualityRules...),
			KeyFields:                append([]string(nil), askMeta.KeyFields...),
			Fields:                   projectFields(facts, toolMeta, askMeta),
			RuleSummaries:            append([]string(nil), facts.RuleSummaries...),
			Builders:                 projectBuilders(workflowMeta.Kind, askMeta.Builders),
		}
		ordered = append(ordered, workflowMeta.Kind)
	}
	sort.Strings(ordered)
	return Catalog{
		Workflow: WorkflowRules{
			SupportedVersion: validate.SupportedWorkflowVersion(),
			TopLevelModes:    validate.WorkflowTopLevelModes(),
			SupportedModes:   validate.SupportedWorkflowRoles(),
			RequiredFields:   workflowRequiredFields(),
			ImportRule:       validate.WorkflowImportRule(),
			InvariantNotes:   append([]string(nil), validate.WorkflowInvariantNotes()...),
		},
		Workspace: WorkspaceRules{
			WorkflowRoot:     workspacepaths.WorkflowRootDir,
			ScenarioDir:      workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowScenariosDir,
			ComponentDir:     workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowComponentsDir,
			VarsPath:         workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowVarsRel,
			AllowedPaths:     AllowedGeneratedPathPatterns(),
			CanonicalPrepare: workspacepaths.WorkflowRootDir + "/" + workspacepaths.CanonicalPrepareWorkflowRel,
			CanonicalApply:   workspacepaths.WorkflowRootDir + "/" + workspacepaths.CanonicalApplyWorkflowRel,
		},
		Steps:   steps,
		ordered: ordered,
	}
}

func projectContract(hints stepmeta.ContractHints) ContractBindings {
	return ContractBindings{
		ProducesArtifacts:   append([]string(nil), hints.ProducesArtifacts...),
		ConsumesArtifacts:   append([]string(nil), hints.ConsumesArtifacts...),
		PublishesState:      append([]string(nil), hints.PublishesState...),
		ConsumesState:       append([]string(nil), hints.ConsumesState...),
		RoleSensitive:       hints.RoleSensitive,
		VerificationRelated: hints.VerificationRelated,
	}
}

func projectFields(facts schemafacts.DocumentFacts, toolMeta schemadoc.ToolMetadata, ask stepmeta.AskMetadata) map[string]Field {
	constrained := map[string]stepmeta.ConstrainedLiteralField{}
	for _, field := range ask.ConstrainedLiteralFields {
		constrained[strings.TrimSpace(field.Path)] = field
	}
	out := map[string]Field{}
	for _, fact := range facts.Fields {
		if !strings.HasPrefix(fact.Path, "spec") {
			continue
		}
		field := Field{
			Path:        strings.TrimSpace(fact.Path),
			Type:        strings.TrimSpace(fact.Type),
			Requirement: fact.Requirement,
			Default:     strings.TrimSpace(fact.Default),
			Pattern:     strings.TrimSpace(fact.Pattern),
			Enum:        append([]string(nil), fact.Enum...),
			Description: strings.TrimSpace(fact.Description),
			Example:     strings.TrimSpace(fact.Example),
		}
		if doc, ok := toolMeta.FieldDocs[field.Path]; ok {
			if field.Description == "" {
				field.Description = strings.TrimSpace(doc.Description)
			}
			if field.Example == "" {
				field.Example = strings.TrimSpace(doc.Example)
			}
		}
		if hint, ok := constrained[field.Path]; ok {
			field.ConstrainedLiteral = true
			field.Guidance = strings.TrimSpace(hint.Guidance)
			if len(field.Enum) == 0 && len(hint.AllowedValues) > 0 {
				field.Enum = append([]string(nil), hint.AllowedValues...)
			}
		}
		out[field.Path] = field
	}
	return out
}

func projectBuilders(kind string, builders []stepmeta.AuthoringBuilder) []Builder {
	out := make([]Builder, 0, len(builders))
	for _, builder := range builders {
		projected := Builder{
			ID:                   strings.TrimSpace(builder.ID),
			StepKind:             strings.TrimSpace(kind),
			Phase:                strings.TrimSpace(builder.Phase),
			DefaultStepID:        strings.TrimSpace(builder.DefaultStepID),
			Summary:              strings.TrimSpace(builder.Summary),
			RequiresCapabilities: append([]string(nil), builder.RequiresCapabilities...),
			Bindings:             make([]Binding, 0, len(builder.Bindings)),
		}
		for _, binding := range builder.Bindings {
			projected.Bindings = append(projected.Bindings, Binding{Path: strings.TrimSpace(binding.Path), From: strings.TrimSpace(binding.From), Semantic: strings.TrimSpace(binding.Semantic), Required: binding.Required})
		}
		out = append(out, projected)
	}
	return out
}

func schemaFactsForSchemaFile(schemaFile string) schemafacts.DocumentFacts {
	raw, err := deckschemas.ToolSchema(strings.TrimSpace(schemaFile))
	if err != nil {
		return schemafacts.DocumentFacts{}
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return schemafacts.DocumentFacts{}
	}
	facts := schemafacts.Analyze(schema)
	if props, _ := schema["properties"].(map[string]any); len(props) > 0 {
		if spec, _ := props["spec"].(map[string]any); len(spec) > 0 {
			facts.RuleSummaries = schemafacts.ExtractRules(spec, "spec")
		}
	}
	return facts
}

func workflowRequiredFields() []string {
	raw, err := deckschemas.WorkflowSchema()
	if err != nil {
		return []string{"version"}
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return []string{"version"}
	}
	facts := schemafacts.Analyze(schema)
	fields := schemafacts.FilterDirectChildFields(facts.Fields, "")
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.Requirement == schemafacts.RequirementRequired {
			out = append(out, field.Path)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return []string{"version"}
	}
	return out
}
