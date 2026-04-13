package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

const workflowRootPrefix = "workflows/"

func authoringBriefPromptBlock(brief askcontract.AuthoringBrief) string {
	b := &strings.Builder{}
	b.WriteString("Authoring brief:\n")
	if strings.TrimSpace(brief.RouteIntent) != "" {
		b.WriteString("- route intent: ")
		b.WriteString(strings.TrimSpace(brief.RouteIntent))
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.TargetScope) != "" {
		b.WriteString("- target scope: ")
		b.WriteString(strings.TrimSpace(brief.TargetScope))
		b.WriteString("\n")
	}
	if len(brief.TargetPaths) > 0 {
		b.WriteString("- target paths: ")
		b.WriteString(strings.Join(brief.TargetPaths, ", "))
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.ModeIntent) != "" {
		b.WriteString("- mode intent: ")
		b.WriteString(strings.TrimSpace(brief.ModeIntent))
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.Connectivity) != "" {
		b.WriteString("- connectivity: ")
		b.WriteString(strings.TrimSpace(brief.Connectivity))
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.CompletenessTarget) != "" {
		b.WriteString("- completeness: ")
		b.WriteString(strings.TrimSpace(brief.CompletenessTarget))
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.Topology) != "" {
		b.WriteString("- topology: ")
		b.WriteString(strings.TrimSpace(brief.Topology))
		b.WriteString("\n")
	}
	if brief.NodeCount > 0 {
		_, _ = fmt.Fprintf(b, "- node count: %d\n", brief.NodeCount)
	}
	if strings.TrimSpace(brief.PlatformFamily) != "" {
		b.WriteString("- platform family: ")
		b.WriteString(strings.TrimSpace(brief.PlatformFamily))
		b.WriteString("\n")
	}
	if len(brief.RequiredCapabilities) > 0 {
		b.WriteString("- required capabilities: ")
		b.WriteString(strings.Join(brief.RequiredCapabilities, ", "))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func authoringProgramPromptBlock(program askcontract.AuthoringProgram) string {
	b := &strings.Builder{}
	b.WriteString("Normalized authoring program:\n")
	if strings.TrimSpace(program.Platform.Family) != "" {
		b.WriteString("- platform.family: ")
		b.WriteString(strings.TrimSpace(program.Platform.Family))
		b.WriteString("\n")
	}
	if strings.TrimSpace(program.Platform.Release) != "" {
		b.WriteString("- platform.release: ")
		b.WriteString(strings.TrimSpace(program.Platform.Release))
		b.WriteString("\n")
	}
	if strings.TrimSpace(program.Platform.RepoType) != "" {
		b.WriteString("- platform.repoType: ")
		b.WriteString(strings.TrimSpace(program.Platform.RepoType))
		b.WriteString("\n")
	}
	if len(program.Artifacts.Packages) > 0 {
		b.WriteString("- artifacts.packages: ")
		b.WriteString(strings.Join(program.Artifacts.Packages, ", "))
		b.WriteString("\n")
	}
	if len(program.Artifacts.Images) > 0 {
		b.WriteString("- artifacts.images: ")
		b.WriteString(strings.Join(program.Artifacts.Images, ", "))
		b.WriteString("\n")
	}
	if strings.TrimSpace(program.Cluster.KubernetesVersion) != "" {
		b.WriteString("- cluster.kubernetesVersion: ")
		b.WriteString(strings.TrimSpace(program.Cluster.KubernetesVersion))
		b.WriteString("\n")
	}
	if strings.TrimSpace(program.Cluster.RoleSelector) != "" {
		b.WriteString("- cluster.roleSelector: ")
		b.WriteString(strings.TrimSpace(program.Cluster.RoleSelector))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func executionModelPromptBlock(model askcontract.ExecutionModel) string {
	b := &strings.Builder{}
	b.WriteString("Execution model:\n")
	for _, item := range model.ArtifactContracts {
		b.WriteString("- artifact ")
		b.WriteString(item.Kind)
		b.WriteString(": ")
		b.WriteString(item.ProducerPath)
		b.WriteString(" -> ")
		b.WriteString(item.ConsumerPath)
		b.WriteString("\n")
	}
	for _, item := range model.SharedStateContracts {
		b.WriteString("- shared state ")
		b.WriteString(item.Name)
		b.WriteString(": ")
		b.WriteString(item.ProducerPath)
		if len(item.ConsumerPaths) > 0 {
			b.WriteString(" -> ")
			b.WriteString(strings.Join(item.ConsumerPaths, ", "))
		}
		if strings.TrimSpace(item.AvailabilityModel) != "" {
			b.WriteString(" [")
			b.WriteString(strings.TrimSpace(item.AvailabilityModel))
			b.WriteString("]")
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(model.RoleExecution.RoleSelector) != "" {
		b.WriteString("- role selector: ")
		b.WriteString(strings.TrimSpace(model.RoleExecution.RoleSelector))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func planCriticSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask plan critic. Return strict JSON only.\n")
	b.WriteString("Review whether the workflow plan is viable enough to proceed into generation-first workflow authoring.\n")
	b.WriteString("Focus on artifact producer/consumer contracts, shared-state contracts such as join files, role-aware execution, role cardinality, topology fidelity, and verification staging realism.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"blocking\":[]string,\"advisory\":[]string,\"missingContracts\":[]string,\"suggestedFixes\":[]string,\"findings\":[{\"code\":string,\"severity\":string,\"message\":string,\"path\":string,\"recoverable\":boolean}]}.\n")
	b.WriteString("Supported finding codes: ")
	b.WriteString(strings.Join(workflowissues.SupportedCriticCodeStrings(), ", "))
	b.WriteString(".\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(authoringProgramPromptBlock(plan.AuthoringProgram))
	b.WriteString("\n")
	return b.String()
}

func planCriticUserPrompt(plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("Planned request: ")
	b.WriteString(strings.TrimSpace(plan.Request))
	b.WriteString("\n")
	b.WriteString("Target outcome: ")
	b.WriteString(strings.TrimSpace(plan.TargetOutcome))
	b.WriteString("\n")
	b.WriteString("Entry scenario: ")
	b.WriteString(strings.TrimSpace(plan.EntryScenario))
	b.WriteString("\n")
	b.WriteString("Planned files:\n")
	for _, file := range plan.Files {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(file.Path))
		if strings.TrimSpace(file.Purpose) != "" {
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(file.Purpose))
		}
		b.WriteString("\n")
	}
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	b.WriteString(authoringProgramPromptBlock(plan.AuthoringProgram))
	b.WriteString("\n")
	b.WriteString("Validation checklist:\n")
	for _, item := range plan.ValidationChecklist {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
