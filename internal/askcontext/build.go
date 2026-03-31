package askcontext

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/schemadoc"
	"github.com/Airgap-Castaways/deck/internal/schemafacts"
	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
	deckschemas "github.com/Airgap-Castaways/deck/schemas"
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
	workflow := schemadoc.WorkflowMeta()
	cli := AskCommandMeta()
	manifest := Manifest{
		CLI: CLIContext{
			Command:             "deck ask",
			PlanSubcommand:      "deck ask plan",
			ConfigSubcommand:    "deck ask config",
			TopLevelDescription: cli.Short,
			ImportantFlags:      append([]CLIFlag(nil), cli.Flags...),
			Examples: []string{
				`deck ask "explain what workflows/scenarios/apply.yaml does"`,
				`deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"`,
				`deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"`,
			},
		},
		Topology: WorkspaceTopology{
			WorkflowRoot:      workspacepaths.WorkflowRootDir,
			ScenarioDir:       pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowScenariosDir),
			ComponentDir:      pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir),
			VarsPath:          pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel),
			AllowedPaths:      AllowedGeneratedPathPatterns(),
			CanonicalPrepare:  pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.CanonicalPrepareWorkflowRel),
			CanonicalApply:    pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.CanonicalApplyWorkflowRel),
			GeneratedPathNote: "New ask-generated files must stay under workflows/prepare.yaml, workflows/scenarios/, workflows/components/, or workflows/vars.yaml.",
		},
		Workflow: WorkflowRules{
			Summary:          workflow.Summary,
			TopLevelModes:    validate.WorkflowTopLevelModes(),
			SupportedModes:   validate.SupportedWorkflowRoles(),
			SupportedVersion: validate.SupportedWorkflowVersion(),
			ImportRule:       validate.WorkflowImportRule(),
			RequiredFields:   workflowRequiredFields(),
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
			Notes: append([]string(nil), validate.WorkflowInvariantNotes()...),
		},
		Policy: AuthoringPolicy{
			AssumeOfflineByDefault: true,
			PrepareArtifactKinds:   []string{"package", "image", "binary", "archive", "bundle", "repository-mirror"},
			ForbiddenApplyActions: []string{
				"remote package download",
				"remote image pull",
				"remote binary download",
				"remote archive fetch",
				"online repository sync",
			},
			VarsAdvisory: []string{
				"Repeated package lists, image lists, paths, versions, or environment-specific values should move to workflows/vars.yaml.",
				"Missing vars should not block generation on its own.",
				"Detected local host facts belong under runtime.host in when expressions, not in workflows/vars.yaml.",
				"workflows/vars.yaml must remain plain YAML data. Do not place template expressions in vars values, keys, or unquoted scalar positions.",
			},
			ComponentAdvisory: []string{
				"Reusable repeated logic across phases or scenarios should usually move into workflows/components/.",
				"Missing components should not block generation on its own.",
			},
		},
		Modes: []ModeGuidance{
			{
				Mode:        "prepare",
				Summary:     "Prepare collects online inputs and produces offline-ready artifacts.",
				WhenToUse:   "Use prepare when the request needs downloads, mirrored images, package caches, or bundle content created before apply.",
				Prefer:      []string{"download-oriented File, Image, and Package steps", "variables shared by later apply steps", "named phases when collection has multiple stages"},
				Avoid:       []string{"live node reconfiguration that belongs in apply", "service management on the target node"},
				OutputFiles: []string{workspacepaths.CanonicalPrepareWorkflowRel, pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)},
			},
			{
				Mode:        "apply",
				Summary:     "Apply changes the local node using prepared inputs and typed host actions.",
				WhenToUse:   "Use apply for package installation, file writes, service changes, runtime config, host convergence steps, and host suitability validation.",
				Prefer:      []string{"typed steps such as File, ConfigureRepository, RefreshRepository, ManageService, WriteContainerdConfig, Package, and CheckHost", "runtime.host.* for detected local host branching", "named phases for multi-step installs", "components for reusable imported logic"},
				Avoid:       []string{"online collection logic that should happen during prepare", "large repeated literals that belong in vars.yaml"},
				OutputFiles: []string{pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.CanonicalApplyWorkflowRel), pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)},
			},
		},
		Components: ComponentGuidance{
			Summary:      "Reusable workflow fragments belong in workflows/components/ and are imported into scenario phases.",
			ImportRule:   "Imports are only valid under phases[].imports and resolve from workflows/components/ using component-relative paths.",
			ReuseRule:    "Split repeated or reusable logic into components instead of duplicating steps across scenarios.",
			LocationRule: "Scenario entrypoints live under workflows/scenarios/ while imported fragments live under workflows/components/.",
			FragmentRule: "Component files are fragment documents, not full workflow documents. They should usually contain a top-level `steps:` mapping only and must not add workflow-level fields like version or phases.",
			ImportExample: strings.TrimSpace(`phases:
  - name: preflight
    imports:
      - path: check-host.yaml`),
			FragmentExample: strings.TrimSpace(`steps:
  - id: check-host
    kind: CheckHost
    spec:
      checks: [os, arch, swap]
      failFast: true`),
		},
		Vars: VarsGuidance{
			Path:        pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel),
			Summary:     "Prefer workflows/vars.yaml for configurable values that would otherwise be repeated inline across steps or files.",
			PreferFor:   []string{"package lists", "repository URLs", "service names", "paths and ports that may vary by environment"},
			AvoidFor:    []string{"runtime-only outputs registered from previous steps", "detected local host facts such as osFamily or arch that already exist under runtime.host", "tiny one-off literals with no reuse value", "typed step fields whose schema expects a native YAML array or object but the template engine would turn into a string", "typed enum fields or constrained scalar fields that must stay literal to satisfy schema validation"},
			ExampleKeys: []string{"dockerRepoURL", "dockerPackages", "containerRuntimeConfigPath"},
		},
		StepKinds: buildStepKinds(),
	}
	return manifest
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

func AllowedGeneratedPathPatterns() []string {
	return workspacepaths.AllowedAuthoringPathPatterns()
}

func AllowedGeneratedPath(path string) bool {
	return workspacepaths.IsAllowedAuthoringPath(path)
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
			if field.Requirement == schemafacts.RequirementRequired {
				keys = append(keys, path)
			}
		}
		sort.Strings(keys)
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

func pathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
