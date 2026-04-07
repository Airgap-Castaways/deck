package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askir"
)

func appendPlanAdvisoryPrompt(base string, plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) string {
	block := planAdvisoryPromptBlock(plan, critic)
	if strings.TrimSpace(block) == "" {
		return base
	}
	if strings.TrimSpace(base) == "" {
		return block
	}
	return strings.TrimSpace(base) + "\n\n" + block
}

func planAdvisoryPromptBlock(plan askcontract.PlanResponse, critic askcontract.PlanCriticResponse) string {
	items := []string{}
	for _, item := range plan.OpenQuestions {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, "planner carry-forward: "+item)
		}
	}
	for _, item := range critic.Advisory {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, "plan advisory: "+item)
		}
	}
	for _, item := range critic.MissingContracts {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, "recoverable missing contract: "+item)
		}
	}
	for _, item := range critic.SuggestedFixes {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, "plan suggested fix: "+item)
		}
	}
	items = dedupe(items)
	if len(items) > 10 {
		items = items[:10]
	}
	if len(items) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Plan review carry-forward:\n")
	b.WriteString("- These are recoverable quality targets. Do not stop at planning; generate the best viable draft and address as many items as possible now.\n")
	b.WriteString("- Keep the requested file set intact even if some details still need repair or post-processing.\n")
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func executionModelPromptBlock(model askcontract.ExecutionModel) string {
	b := &strings.Builder{}
	b.WriteString("Normalized execution model:\n")
	if len(model.ArtifactContracts) == 0 && len(model.SharedStateContracts) == 0 && strings.TrimSpace(model.RoleExecution.RoleSelector) == "" && len(model.ApplyAssumptions) == 0 && model.Verification.ExpectedNodeCount == 0 {
		b.WriteString("- none\n")
		return strings.TrimSpace(b.String())
	}
	for _, item := range model.ArtifactContracts {
		b.WriteString("- artifact ")
		b.WriteString(strings.TrimSpace(item.Kind))
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(item.ProducerPath))
		b.WriteString(" -> ")
		b.WriteString(strings.TrimSpace(item.ConsumerPath))
		if strings.TrimSpace(item.Description) != "" {
			b.WriteString(" (")
			b.WriteString(strings.TrimSpace(item.Description))
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	for _, item := range model.SharedStateContracts {
		b.WriteString("- shared state ")
		b.WriteString(strings.TrimSpace(item.Name))
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(item.ProducerPath))
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
	if strings.TrimSpace(model.RoleExecution.ControlPlaneFlow) != "" {
		b.WriteString("- control-plane flow: ")
		b.WriteString(strings.TrimSpace(model.RoleExecution.ControlPlaneFlow))
		b.WriteString("\n")
	}
	if strings.TrimSpace(model.RoleExecution.WorkerFlow) != "" {
		b.WriteString("- worker flow: ")
		b.WriteString(strings.TrimSpace(model.RoleExecution.WorkerFlow))
		b.WriteString("\n")
	}
	if model.Verification.ExpectedNodeCount > 0 {
		_, _ = fmt.Fprintf(b, "- verification expected nodes: %d\n", model.Verification.ExpectedNodeCount)
	}
	if strings.TrimSpace(model.Verification.FinalVerificationRole) != "" {
		b.WriteString("- verification final role: ")
		b.WriteString(strings.TrimSpace(model.Verification.FinalVerificationRole))
		b.WriteString("\n")
	}
	if model.Verification.ExpectedControlPlaneReady > 0 {
		_, _ = fmt.Fprintf(b, "- verification control-plane ready: %d\n", model.Verification.ExpectedControlPlaneReady)
	}
	for _, item := range model.ApplyAssumptions {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- apply assumption: ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func authoringBriefPromptBlock(brief askcontract.AuthoringBrief) string {
	b := &strings.Builder{}
	b.WriteString("Normalized authoring brief:\n")
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		b.WriteString("- ")
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\n")
	}
	appendList := func(label string, values []string) {
		values = dedupe(values)
		if len(values) == 0 {
			return
		}
		b.WriteString("- ")
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(strings.Join(values, ", "))
		b.WriteString("\n")
	}
	appendLine("route intent", brief.RouteIntent)
	appendLine("target scope", brief.TargetScope)
	appendLine("mode intent", brief.ModeIntent)
	appendLine("connectivity", brief.Connectivity)
	appendLine("completeness target", brief.CompletenessTarget)
	appendLine("topology", brief.Topology)
	appendLine("platform family", brief.PlatformFamily)
	if brief.NodeCount > 0 {
		appendLine("node count", fmt.Sprintf("%d", brief.NodeCount))
	}
	appendList("target paths", brief.TargetPaths)
	appendList("anchor paths", brief.AnchorPaths)
	appendList("allowed companion paths", brief.AllowedCompanionPaths)
	appendList("disallowed expansion paths", brief.DisallowedExpansionPaths)
	appendList("required capabilities", brief.RequiredCapabilities)
	return strings.TrimSpace(b.String())
}

func authoringProgramPromptBlock(program askcontract.AuthoringProgram) string {
	b := &strings.Builder{}
	b.WriteString("Normalized authoring program:\n")
	lines := 0
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		lines++
		b.WriteString("- ")
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\n")
	}
	appendList := func(label string, values []string) {
		values = dedupe(values)
		if len(values) == 0 {
			return
		}
		lines++
		b.WriteString("- ")
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(strings.Join(values, ", "))
		b.WriteString("\n")
	}
	appendInt := func(label string, value int) {
		if value <= 0 {
			return
		}
		appendLine(label, fmt.Sprintf("%d", value))
	}
	appendLine("platform family", program.Platform.Family)
	appendLine("platform release", program.Platform.Release)
	appendLine("platform repo type", program.Platform.RepoType)
	appendLine("platform backend image", program.Platform.BackendImage)
	appendList("artifact packages", program.Artifacts.Packages)
	appendList("artifact images", program.Artifacts.Images)
	appendLine("artifact package output", program.Artifacts.PackageOutputDir)
	appendLine("artifact image output", program.Artifacts.ImageOutputDir)
	appendLine("cluster join file", program.Cluster.JoinFile)
	appendLine("cluster pod cidr", program.Cluster.PodCIDR)
	appendLine("cluster kubernetes version", program.Cluster.KubernetesVersion)
	appendLine("cluster cri socket", program.Cluster.CriSocket)
	appendLine("cluster role selector", program.Cluster.RoleSelector)
	appendInt("cluster control-plane count", program.Cluster.ControlPlaneCount)
	appendInt("cluster worker count", program.Cluster.WorkerCount)
	appendInt("verification expected nodes", program.Verification.ExpectedNodeCount)
	appendInt("verification expected ready", program.Verification.ExpectedReadyCount)
	appendInt("verification expected control-plane ready", program.Verification.ExpectedControlPlaneReady)
	appendLine("verification final role", program.Verification.FinalVerificationRole)
	appendLine("verification interval", program.Verification.Interval)
	appendLine("verification timeout", program.Verification.Timeout)
	if lines == 0 {
		b.WriteString("- none\n")
	}
	return strings.TrimSpace(b.String())
}

func generatedDocumentSummaryBlock(files []askcontract.GeneratedFile) string {
	documents := make([]askcontract.GeneratedDocument, 0, len(files))
	for _, file := range files {
		if file.Delete {
			continue
		}
		doc, err := askir.ParseDocument(file.Path, []byte(file.Content))
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}
	if len(documents) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Parsed document summaries:\n")
	for _, summary := range askir.Summaries(documents) {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(summary))
		b.WriteString("\n")
	}
	for _, doc := range documents {
		if doc.Workflow == nil {
			continue
		}
		b.WriteString("- structure ")
		b.WriteString(strings.TrimSpace(doc.Path))
		b.WriteString(": ")
		b.WriteString(structuralWorkflowSummary(*doc.Workflow))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func structuralWorkflowSummary(doc askcontract.WorkflowDocument) string {
	tokens := []string{}
	for _, step := range doc.Steps {
		if strings.TrimSpace(step.Kind) != "" {
			tokens = append(tokens, strings.TrimSpace(step.Kind))
		}
		if strings.TrimSpace(step.When) != "" {
			tokens = append(tokens, strings.TrimSpace(step.When))
		}
	}
	for _, phase := range doc.Phases {
		for _, imp := range phase.Imports {
			if strings.TrimSpace(imp.When) != "" {
				tokens = append(tokens, strings.TrimSpace(imp.When))
			}
		}
		for _, step := range phase.Steps {
			if strings.TrimSpace(step.Kind) != "" {
				tokens = append(tokens, strings.TrimSpace(step.Kind))
			}
			if strings.TrimSpace(step.When) != "" {
				tokens = append(tokens, strings.TrimSpace(step.When))
			}
		}
	}
	tokens = dedupe(tokens)
	return fmt.Sprintf("phases=%d topLevelSteps=%d tokens=%s", len(doc.Phases), len(doc.Steps), strings.Join(tokens, ", "))
}
