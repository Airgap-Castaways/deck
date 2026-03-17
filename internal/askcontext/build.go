package askcontext

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/taedi90/deck/internal/schemadoc"
	"github.com/taedi90/deck/internal/validate"
	"github.com/taedi90/deck/internal/workflowexec"
	"github.com/taedi90/deck/internal/workspacepaths"
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
			AuthSubcommand:      "deck ask auth",
			TopLevelDescription: cli.Short,
			ImportantFlags:      append([]CLIFlag(nil), cli.Flags...),
			Examples: []string{
				`deck ask "explain what workflows/scenarios/apply.yaml does"`,
				`deck ask --write "create an air-gapped rhel9 kubeadm cluster workflow"`,
				`deck ask plan "create an air-gapped rhel9 kubeadm cluster workflow"`,
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
			GeneratedPathNote: "New ask-generated files must stay under workflows/scenarios/, workflows/components/, or workflows/vars.yaml.",
		},
		Workflow: WorkflowRules{
			Summary:          workflow.Summary,
			TopLevelModes:    validate.WorkflowTopLevelModes(),
			SupportedRoles:   validate.SupportedWorkflowRoles(),
			SupportedVersion: validate.SupportedWorkflowVersion(),
			ImportRule:       validate.WorkflowImportRule(),
			Notes:            append([]string(nil), validate.WorkflowInvariantNotes()...),
		},
		Roles: []RoleGuidance{
			{
				Role:        "prepare",
				Summary:     "Prepare collects online inputs and produces offline-ready artifacts.",
				WhenToUse:   "Use prepare when the request needs downloads, mirrored images, package caches, or bundle content created before apply.",
				Prefer:      []string{"artifacts for bundle inventory", "download-oriented File or Image steps", "variables shared by later apply steps"},
				Avoid:       []string{"live node reconfiguration that belongs in apply", "service management on the target node"},
				OutputFiles: []string{pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.CanonicalPrepareWorkflowRel), pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)},
			},
			{
				Role:        "apply",
				Summary:     "Apply changes the local node using prepared inputs and typed host actions.",
				WhenToUse:   "Use apply for package installation, file writes, service changes, runtime config, and host convergence steps.",
				Prefer:      []string{"typed steps such as File, Repository, Service, Containerd, Packages", "named phases for multi-step installs", "components for reusable imported logic"},
				Avoid:       []string{"online collection logic that should happen during prepare", "large repeated literals that belong in vars.yaml"},
				OutputFiles: []string{pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.CanonicalApplyWorkflowRel), pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)},
			},
		},
		Components: ComponentGuidance{
			Summary:      "Reusable workflow fragments belong in workflows/components/ and are imported into scenario phases.",
			ImportRule:   "Imports are only valid under phases[].imports and resolve from workflows/components/ using component-relative paths.",
			ReuseRule:    "Split repeated or reusable logic into components instead of duplicating steps across scenarios.",
			LocationRule: "Scenario entrypoints live under workflows/scenarios/ while imported fragments live under workflows/components/.",
		},
		Vars: VarsGuidance{
			Path:        pathJoin(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel),
			Summary:     "Prefer workflows/vars.yaml for configurable values that would otherwise be repeated inline across steps or files.",
			PreferFor:   []string{"package lists", "repository URLs", "service names", "paths and ports that may vary by environment"},
			AvoidFor:    []string{"runtime-only outputs registered from previous steps", "tiny one-off literals with no reuse value"},
			ExampleKeys: []string{"dockerRepoURL", "dockerPackages", "containerRuntimeConfigPath"},
		},
		StepKinds: buildStepKinds(),
	}
	return manifest
}

func AllowedGeneratedPathPatterns() []string {
	return []string{"workflows/scenarios/*.yaml", "workflows/components/*.yaml", "workflows/vars.yaml"}
}

func AllowedGeneratedPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" || strings.Contains(clean, "..") {
		return false
	}
	return strings.HasPrefix(clean, "workflows/scenarios/") || strings.HasPrefix(clean, "workflows/components/") || clean == "workflows/vars.yaml"
}

func buildStepKinds() []StepKindContext {
	kinds := workflowexec.StepKinds()
	out := make([]StepKindContext, 0, len(kinds))
	for _, kind := range kinds {
		meta := schemadoc.ToolMeta(kind)
		contract, _ := workflowexec.StepContractForKind(kind)
		ctx := StepKindContext{
			Kind:         kind,
			Category:     meta.Category,
			Summary:      meta.Summary,
			WhenToUse:    meta.WhenToUse,
			SchemaFile:   contract.SchemaFile,
			AllowedRoles: sortedKeys(contract.Roles),
			Actions:      sortedActionKeys(contract.Actions),
			Outputs:      sortedKeys(contract.Outputs),
			Notes:        append([]string(nil), meta.Notes...),
		}
		for _, action := range ctx.Actions {
			ctx.Outputs = append(ctx.Outputs, sortedKeys(contract.Actions[action].Outputs)...)
		}
		ctx.Outputs = dedupe(ctx.Outputs)
		out = append(out, ctx)
	}
	return out
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value, ok := range values {
		if ok {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func sortedActionKeys(values map[string]workflowexec.ActionContract) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dedupe(values []string) []string {
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

func pathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
