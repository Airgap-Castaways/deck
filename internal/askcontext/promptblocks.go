package askcontext

import (
	"sort"
	"strings"
)

func GlobalAuthoringBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Workflow authoring rules:\n")
	b.WriteString("- ")
	b.WriteString(manifest.Workflow.Summary)
	b.WriteString("\n")
	for _, note := range manifest.Workflow.Notes {
		b.WriteString("- ")
		b.WriteString(note)
		b.WriteString("\n")
	}
	b.WriteString("- Prefer typed steps over Command whenever a typed step expresses the change clearly.\n")
	return strings.TrimSpace(b.String())
}

func WorkspaceTopologyBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Workspace topology:\n")
	b.WriteString("- Scenario entrypoints: ")
	b.WriteString(manifest.Topology.ScenarioDir)
	b.WriteString("\n")
	b.WriteString("- Reusable components: ")
	b.WriteString(manifest.Topology.ComponentDir)
	b.WriteString("\n")
	b.WriteString("- Shared variables file: ")
	b.WriteString(manifest.Topology.VarsPath)
	b.WriteString("\n")
	b.WriteString("- Canonical prepare scenario: ")
	b.WriteString(manifest.Topology.CanonicalPrepare)
	b.WriteString("\n")
	b.WriteString("- Canonical apply scenario: ")
	b.WriteString(manifest.Topology.CanonicalApply)
	b.WriteString("\n")
	b.WriteString("- Allowed generated paths: ")
	b.WriteString(strings.Join(manifest.Topology.AllowedPaths, ", "))
	b.WriteString("\n")
	b.WriteString("- ")
	b.WriteString(manifest.Topology.GeneratedPathNote)
	return strings.TrimSpace(b.String())
}

func RoleGuidanceBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Prepare/apply guidance:\n")
	for _, role := range manifest.Roles {
		b.WriteString("- ")
		b.WriteString(role.Role)
		b.WriteString(": ")
		b.WriteString(role.Summary)
		b.WriteString(" Use when: ")
		b.WriteString(role.WhenToUse)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func ComponentGuidanceBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Components and imports:\n")
	b.WriteString("- ")
	b.WriteString(manifest.Components.Summary)
	b.WriteString("\n- ")
	b.WriteString(manifest.Components.ImportRule)
	b.WriteString("\n- ")
	b.WriteString(manifest.Components.ReuseRule)
	b.WriteString("\n- ")
	b.WriteString(manifest.Components.LocationRule)
	return strings.TrimSpace(b.String())
}

func VarsGuidanceBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Variables guidance:\n")
	b.WriteString("- ")
	b.WriteString(manifest.Vars.Summary)
	b.WriteString("\n- Prefer vars.yaml for: ")
	b.WriteString(strings.Join(manifest.Vars.PreferFor, ", "))
	b.WriteString("\n- Avoid vars.yaml for: ")
	b.WriteString(strings.Join(manifest.Vars.AvoidFor, ", "))
	b.WriteString("\n- Example vars keys: ")
	b.WriteString(strings.Join(manifest.Vars.ExampleKeys, ", "))
	return strings.TrimSpace(b.String())
}

func CLIHintsBlock() string {
	manifest := Current()
	b := &strings.Builder{}
	b.WriteString("Relevant CLI usage:\n")
	b.WriteString("- ")
	b.WriteString(manifest.CLI.Command)
	b.WriteString(" previews by default; add --write to write files.\n")
	b.WriteString("- ")
	b.WriteString(manifest.CLI.PlanSubcommand)
	b.WriteString(" saves a reusable plan artifact without writing workflow files.\n")
	for _, flag := range manifest.CLI.ImportantFlags {
		b.WriteString("- ")
		b.WriteString(flag.Name)
		b.WriteString(": ")
		b.WriteString(flag.Description)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func RelevantStepKindsBlock(prompt string) string {
	relevant := RelevantStepKinds(prompt)
	if len(relevant) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Relevant typed steps:\n")
	for _, step := range relevant {
		b.WriteString("- ")
		b.WriteString(step.Kind)
		b.WriteString(": ")
		b.WriteString(step.Summary)
		if step.WhenToUse != "" {
			b.WriteString(" When to use: ")
			b.WriteString(step.WhenToUse)
		}
		if len(step.AllowedRoles) > 0 {
			b.WriteString(" Roles: ")
			b.WriteString(strings.Join(step.AllowedRoles, ", "))
		}
		if len(step.Actions) > 0 {
			b.WriteString(" Actions: ")
			b.WriteString(strings.Join(step.Actions, ", "))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func RelevantStepKinds(prompt string) []StepKindContext {
	manifest := Current()
	lower := strings.ToLower(strings.TrimSpace(prompt))
	type scored struct {
		step  StepKindContext
		score int
	}
	scoredKinds := make([]scored, 0, len(manifest.StepKinds))
	for _, step := range manifest.StepKinds {
		score := 0
		if strings.Contains(lower, strings.ToLower(step.Kind)) {
			score += 100
		}
		if strings.Contains(lower, strings.ToLower(step.Category)) {
			score += 15
		}
		if strings.Contains(lower, strings.ToLower(step.Summary)) {
			score += 10
		}
		for _, action := range step.Actions {
			if strings.Contains(lower, strings.ToLower(action)) {
				score += 20
			}
		}
		for _, token := range strings.Fields(strings.ToLower(step.WhenToUse)) {
			if len(token) > 4 && strings.Contains(lower, token) {
				score += 4
			}
		}
		if strings.Contains(lower, "repo") || strings.Contains(lower, "repository") {
			if step.Kind == "Repository" {
				score += 60
			}
		}
		if strings.Contains(lower, "docker") || strings.Contains(lower, "package") || strings.Contains(lower, "dnf") {
			if step.Kind == "Packages" || step.Kind == "Repository" || step.Kind == "Service" {
				score += 30
			}
		}
		if strings.Contains(lower, "file") || strings.Contains(lower, "config") {
			if step.Kind == "File" || step.Kind == "Directory" {
				score += 20
			}
		}
		if score > 0 {
			scoredKinds = append(scoredKinds, scored{step: step, score: score})
		}
	}
	sort.Slice(scoredKinds, func(i, j int) bool {
		if scoredKinds[i].score == scoredKinds[j].score {
			return scoredKinds[i].step.Kind < scoredKinds[j].step.Kind
		}
		return scoredKinds[i].score > scoredKinds[j].score
	})
	limit := 5
	if len(scoredKinds) < limit {
		limit = len(scoredKinds)
	}
	out := make([]StepKindContext, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scoredKinds[i].step)
	}
	if len(out) == 0 {
		for _, kind := range manifest.StepKinds {
			if kind.Kind == "File" || kind.Kind == "Repository" || kind.Kind == "Service" || kind.Kind == "Command" {
				out = append(out, kind)
			}
		}
	}
	return out
}
