package askcatalog

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Airgap-Castaways/deck/internal/schemadoc"
	"github.com/Airgap-Castaways/deck/internal/schemafacts"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
	deckschemas "github.com/Airgap-Castaways/deck/schemas"
)

var (
	once     sync.Once
	logOnce  sync.Once
	data     Catalog
	errBuild error
)

func Current() Catalog {
	once.Do(func() {
		data, errBuild = build()
	})
	if errBuild != nil {
		logOnce.Do(func() {
			log.Printf("askcatalog: build failed: %v", errBuild)
		})
	}
	return data
}

func AllowedGeneratedPath(path string) bool {
	return workspacepaths.IsAllowedAuthoringPath(path)
}

func AllowedGeneratedPathPatterns() []string {
	return workspacepaths.AllowedAuthoringPathPatterns()
}

func build() (Catalog, error) {
	defs, err := workflowexec.BuiltInTypeDefinitions()
	if err != nil {
		return Catalog{}, err
	}
	steps := map[string]Step{}
	ordered := make([]string, 0, len(defs))
	for _, def := range defs {
		entry, ok, err := stepmeta.LookupCatalogEntry(def.Step.Kind)
		if err != nil || !ok {
			continue
		}
		workflowMeta := stepmeta.ProjectWorkflow(entry, def.Step.ToolSchemaGenerator)
		toolMeta := schemadoc.ToolMetaForDefinition(def.Step)
		schemaMeta := stepmeta.ProjectSchema(entry)
		askMeta := stepmeta.ProjectAsk(entry)
		groupMeta, _ := schemadoc.GroupMeta(workflowMeta.Group)
		facts := schemaFactsForSchemaFile(workflowMeta.SchemaFile)
		stepSourceRefs := uniqueSourceRefs(
			sourceRefString(entry.Docs.Source),
			sourceRefString(schemaMeta.Source),
			"internal/stepmeta/registry.go",
		)
		steps[workflowMeta.Kind] = Step{
			Kind:                     workflowMeta.Kind,
			Category:                 workflowMeta.Category,
			Group:                    strings.TrimSpace(workflowMeta.Group),
			GroupTitle:               strings.TrimSpace(groupMeta.Title),
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
			Fields:                   projectFields(facts, toolMeta, askMeta, workflowMeta.SchemaFile),
			RuleSummaries:            append([]string(nil), facts.RuleSummaries...),
			Builders:                 projectBuilders(workflowMeta.Kind, askMeta.Builders, stepSourceRefs),
			SourceRefs:               stepSourceRefs,
		}
		ordered = append(ordered, workflowMeta.Kind)
	}
	sort.Strings(ordered)
	return Catalog{
		Workflow: WorkflowRules{
			Summary:          schemadoc.WorkflowMeta().Summary,
			SupportedVersion: validate.SupportedWorkflowVersion(),
			TopLevelModes:    validate.WorkflowTopLevelModes(),
			SupportedModes:   validate.SupportedWorkflowRoles(),
			RequiredFields:   workflowRequiredFields(),
			ImportRule:       validate.WorkflowImportRule(),
			PhaseRules: []string{
				"Each phase needs a non-empty name.",
				"Each phase must define steps or imports.",
				"Phase objects do not support an id field.",
			},
			StepRules: []string{
				"Each step needs id, kind, and spec.",
				"Step ids belong on steps, not phases.",
				workflowissues.MustSpec(workflowissues.CodeDuplicateStepID).Details,
			},
			PhaseExample: strings.TrimSpace(`version: v1alpha1
phases:
  - name: bootstrap
    steps:
      - id: check-host
        kind: CheckHost
        spec:
          checks: [os, arch, swap]
          failFast: true`),
			StepsExample: strings.TrimSpace(`version: v1alpha1
steps:
  - id: run-command
    kind: Command
    spec:
      command: [echo, hello]`),
			InvariantNotes: append([]string(nil), validate.WorkflowInvariantNotes()...),
			SourceRefs:     []string{"schemas/workflow.schema.json", "internal/validate", "internal/workflowissues"},
		},
		Workspace: WorkspaceRules{
			WorkflowRoot:      workspacepaths.WorkflowRootDir,
			ScenarioDir:       workspacepaths.CanonicalScenariosDir,
			ComponentDir:      workspacepaths.CanonicalComponentsDir,
			VarsPath:          workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowVarsRel,
			AllowedPaths:      AllowedGeneratedPathPatterns(),
			CanonicalPrepare:  workspacepaths.WorkflowRootDir + "/" + workspacepaths.CanonicalPrepareWorkflowRel,
			CanonicalApply:    workspacepaths.WorkflowRootDir + "/" + workspacepaths.CanonicalApplyWorkflowRel,
			GeneratedPathNote: fmt.Sprintf("New ask-generated files must stay under %s, %s/, %s/, or %s.", workspacepaths.CanonicalPrepareWorkflow, workspacepaths.CanonicalScenariosDir, workspacepaths.CanonicalComponentsDir, workspacepaths.CanonicalVarsWorkflow),
			SourceRefs:        []string{"internal/workspacepaths"},
		},
		Policy: PolicyRules{
			AssumeOfflineByDefault: true,
			PreferTypedSteps:       true,
			PrepareArtifactKinds:   []string{"package", "image", "binary", "archive", "bundle", "repository-mirror"},
			ForbiddenApplyActions: []string{
				"remote package download",
				"remote image pull",
				"remote binary download",
				"remote archive fetch",
				"online repository sync",
			},
			VarsAdvisory: []string{
				fmt.Sprintf("Repeated package lists, image lists, paths, versions, or environment-specific values should move to %s.", workspacepaths.CanonicalVarsWorkflow),
				"Missing vars should not block generation on its own.",
				fmt.Sprintf("Detected local host facts belong under runtime.host in when expressions, not in %s.", workspacepaths.CanonicalVarsWorkflow),
				fmt.Sprintf("%s must remain plain YAML data. Do not place template expressions in vars values, keys, or unquoted scalar positions.", workspacepaths.CanonicalVarsWorkflow),
				"Do not replace schema-typed arrays or objects with string templates. Keep arrays as YAML arrays and objects as YAML objects so schema validation still passes.",
			},
			ComponentAdvisory: []string{
				fmt.Sprintf("Reusable repeated logic across phases or scenarios should usually move into %s/.", workspacepaths.CanonicalComponentsDir),
				"Missing components should not block generation on its own.",
			},
			SourceRefs: []string{"internal/askpolicy", "internal/validate"},
		},
		Modes: []ModeRules{
			{
				Mode:        "prepare",
				Summary:     "Prepare collects online inputs and produces offline-ready artifacts.",
				WhenToUse:   "Use prepare when the request needs downloads, mirrored images, package caches, or bundle content created before apply.",
				Prefer:      []string{"download-oriented File, Image, and Package steps", "variables shared by later apply steps", "named phases when collection has multiple stages"},
				Avoid:       []string{"live node reconfiguration that belongs in apply", "service management on the target node"},
				OutputFiles: []string{workspacepaths.CanonicalPrepareWorkflowRel, workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowVarsRel},
				SourceRefs:  []string{"internal/askpolicy", "internal/workspacepaths"},
			},
			{
				Mode:        "apply",
				Summary:     "Apply changes the local node using prepared inputs and typed host actions.",
				WhenToUse:   "Use apply for package installation, file writes, service changes, runtime config, host convergence steps, and host suitability validation.",
				Prefer:      []string{"typed steps such as File, ConfigureRepository, RefreshRepository, ManageService, WriteContainerdConfig, Package, and CheckHost", "runtime.host.* for detected local host branching", "named phases for multi-step installs", "components for reusable imported logic"},
				Avoid:       []string{"online collection logic that should happen during prepare", fmt.Sprintf("large repeated literals that belong in %s", workspacepaths.CanonicalVarsWorkflow)},
				OutputFiles: []string{workspacepaths.WorkflowRootDir + "/" + workspacepaths.CanonicalApplyWorkflowRel, workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowVarsRel},
				SourceRefs:  []string{"internal/askpolicy", "internal/workspacepaths"},
			},
		},
		Components: ComponentRules{
			Summary:         fmt.Sprintf("Reusable workflow fragments belong in %s/ and are imported into scenario phases.", workspacepaths.CanonicalComponentsDir),
			ImportRule:      fmt.Sprintf("Imports are only valid under phases[].imports and resolve from %s/ using component-relative paths.", workspacepaths.CanonicalComponentsDir),
			ReuseRule:       "Split repeated or reusable logic into components instead of duplicating steps across scenarios.",
			LocationRule:    fmt.Sprintf("Scenario entrypoints live under %s/ while imported fragments live under %s/.", workspacepaths.CanonicalScenariosDir, workspacepaths.CanonicalComponentsDir),
			FragmentRule:    "Component files are fragment documents, not full workflow documents. They should usually contain a top-level `steps:` mapping only and must not add workflow-level fields like version or phases.",
			ImportExample:   strings.TrimSpace("phases:\n  - name: preflight\n    imports:\n      - path: check-host.yaml"),
			FragmentExample: strings.TrimSpace("steps:\n  - id: check-host\n    kind: CheckHost\n    spec:\n      checks: [os, arch, swap]\n      failFast: true"),
			AllowedRootKeys: []string{"steps"},
			SourceRefs:      []string{"internal/workspacepaths", "internal/validate"},
		},
		Vars: VarsRules{
			Path:        workspacepaths.WorkflowRootDir + "/" + workspacepaths.WorkflowVarsRel,
			Summary:     fmt.Sprintf("Prefer %s for configurable values that would otherwise be repeated inline across steps or files.", workspacepaths.CanonicalVarsWorkflow),
			PreferFor:   []string{"package lists", "repository URLs", "service names", "paths and ports that may vary by environment"},
			AvoidFor:    []string{"runtime-only outputs registered from previous steps", "detected local host facts such as osFamily or arch that already exist under runtime.host", "tiny one-off literals with no reuse value", "typed step fields whose schema expects a native YAML array or object but the template engine would turn into a string", "typed enum fields or constrained scalar fields that must stay literal to satisfy schema validation"},
			ExampleKeys: []string{"dockerRepoURL", "dockerPackages", "containerRuntimeConfigPath"},
			SourceRefs:  []string{"internal/askpolicy", "internal/workspacepaths"},
		},
		Steps:   steps,
		ordered: ordered,
	}, nil
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

func projectFields(facts schemafacts.DocumentFacts, toolMeta schemadoc.ToolMetadata, ask stepmeta.AskMetadata, schemaFile string) map[string]Field {
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
			SourceRef:   strings.TrimSpace(schemaFile),
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

func projectBuilders(kind string, builders []stepmeta.AuthoringBuilder, sourceRefs []string) []Builder {
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
			SourceRefs:           append([]string(nil), sourceRefs...),
		}
		for _, binding := range builder.Bindings {
			projected.Bindings = append(projected.Bindings, Binding{Path: strings.TrimSpace(binding.Path), From: strings.TrimSpace(binding.From), Semantic: strings.TrimSpace(binding.Semantic), Required: binding.Required})
		}
		out = append(out, projected)
	}
	return out
}

func sourceRefString(ref stepmeta.SourceRef) string {
	if strings.TrimSpace(ref.File) == "" {
		return ""
	}
	if ref.Line > 0 {
		return strings.TrimSpace(ref.File) + ":" + strings.TrimSpace(strconv.Itoa(ref.Line))
	}
	return strings.TrimSpace(ref.File)
}

func uniqueSourceRefs(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
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
