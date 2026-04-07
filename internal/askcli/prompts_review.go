package askcli

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func judgeSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask semantic judge. Return strict JSON only.\n")
	b.WriteString("Judge whether generated workflow files satisfy the requested outcome and execution model after local lint/schema validation already passed.\n")
	b.WriteString("Focus on operational workflow design quality: artifact producer/consumer contracts, shared-state availability such as join files, role-aware execution, and topology-aware verification.\n")
	b.WriteString("Do not re-litigate syntax or schema unless it causes an obvious intent mismatch.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"blocking\":[]string,\"advisory\":[]string,\"missingCapabilities\":[]string,\"suggestedFixes\":[]string}.\n")
	b.WriteString("Use blocking only when the generated workflow clearly misses a required capability, execution contract, or collapses the request scope.\n")
	b.WriteString("When possible, mention the affected workflow file and phase directly in each finding.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	if strings.TrimSpace(plan.Request) != "" {
		b.WriteString("Planned request: ")
		b.WriteString(strings.TrimSpace(plan.Request))
		b.WriteString("\n")
	}
	if strings.TrimSpace(plan.TargetOutcome) != "" {
		b.WriteString("Planned target outcome: ")
		b.WriteString(strings.TrimSpace(plan.TargetOutcome))
		b.WriteString("\n")
	}
	return b.String()
}

func planCriticSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask plan critic. Return strict JSON only.\n")
	b.WriteString("Review whether the workflow plan is viable enough to proceed into generation-first workflow authoring.\n")
	b.WriteString("Focus on artifact producer/consumer contracts, shared-state contracts such as join files, role-aware execution, role cardinality, topology fidelity, join publication/consumption, artifact contract naming, and verification staging realism.\n")
	b.WriteString("Do not restate schema rules unless the plan violates them in a way that affects execution design.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"blocking\":[]string,\"advisory\":[]string,\"missingContracts\":[]string,\"suggestedFixes\":[]string,\"findings\":[{\"code\":string,\"severity\":string,\"message\":string,\"path\":string,\"recoverable\":boolean}]}.\n")
	b.WriteString("Finding severity must be one of blocking, advisory, or missing_contract.\n")
	b.WriteString("Supported finding codes: ")
	b.WriteString(strings.Join(workflowissues.SupportedCriticCodeStrings(), ", "))
	b.WriteString(".\n")
	b.WriteString("Every blocking/advisory/missingContracts item should have a matching structured finding with the same meaning.\n")
	b.WriteString("Use blocking only for true pre-generation non-viability: no viable entry scenario, no viable role selector/branching model, no viable artifact consumer path, or structurally unusable planning.\n")
	b.WriteString("Treat ambiguous join contracts, artifact detail gaps, role cardinality detail, worker synchronization detail, and verification staging weakness as advisory or missingContracts unless generation would be impossible.\n")
	b.WriteString("When possible, mention the affected file or execution-model section directly in each finding.\n")
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
	b.WriteString("Execution model:\n")
	for _, item := range plan.ExecutionModel.ArtifactContracts {
		b.WriteString("- artifact ")
		b.WriteString(item.Kind)
		b.WriteString(": ")
		b.WriteString(item.ProducerPath)
		b.WriteString(" -> ")
		b.WriteString(item.ConsumerPath)
		if strings.TrimSpace(item.Description) != "" {
			b.WriteString(" (")
			b.WriteString(strings.TrimSpace(item.Description))
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	for _, item := range plan.ExecutionModel.SharedStateContracts {
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
	if strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) != "" {
		b.WriteString("- role selector: ")
		b.WriteString(strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector))
		b.WriteString("\n")
	}
	if strings.TrimSpace(plan.ExecutionModel.RoleExecution.ControlPlaneFlow) != "" {
		b.WriteString("- control-plane flow: ")
		b.WriteString(strings.TrimSpace(plan.ExecutionModel.RoleExecution.ControlPlaneFlow))
		b.WriteString("\n")
	}
	if strings.TrimSpace(plan.ExecutionModel.RoleExecution.WorkerFlow) != "" {
		b.WriteString("- worker flow: ")
		b.WriteString(strings.TrimSpace(plan.ExecutionModel.RoleExecution.WorkerFlow))
		b.WriteString("\n")
	}
	for _, item := range plan.ExecutionModel.ApplyAssumptions {
		b.WriteString("- apply assumption: ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
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

func postProcessCriticSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask post-processing critic. Return strict JSON only.\n")
	b.WriteString("Review a valid generated workflow set for operational upgrade opportunities after generation, lint, and design review.\n")
	b.WriteString("Focus first on operational defects: shared-state publication, artifact handoff exactness, verification placement, and runtime prerequisite realism.\n")
	b.WriteString("Treat vars/components cleanup as advisory only. Default to preserve-inline when extraction benefit is weak.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"blocking\":[]string,\"advisory\":[]string,\"upgradeCandidates\":[]string,\"reviseFiles\":[]string,\"preserveFiles\":[]string,\"requiredEdits\":[]string,\"verificationExpectations\":[]string,\"suggestedFixes\":[]string}.\n")
	b.WriteString("Use blocking only for operational defects. Keep vars/components extraction advisory unless clearly necessary. Mention affected files and phases directly when possible.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	return b.String()
}

func postProcessCriticUserPrompt(plan askcontract.PlanResponse, files []askcontract.GeneratedFile, judge askcontract.JudgeResponse, critic askcontract.CriticResponse, planCritic askcontract.PlanCriticResponse) string {
	b := &strings.Builder{}
	b.WriteString("Planned request: ")
	b.WriteString(strings.TrimSpace(plan.Request))
	b.WriteString("\n")
	if summary := generatedDocumentSummaryBlock(files); strings.TrimSpace(summary) != "" {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	if advisory := planAdvisoryPromptBlock(plan, planCritic); strings.TrimSpace(advisory) != "" {
		b.WriteString(advisory)
		b.WriteString("\n")
	}
	if strings.TrimSpace(judge.Summary) != "" {
		b.WriteString("Design review summary: ")
		b.WriteString(strings.TrimSpace(judge.Summary))
		b.WriteString("\n")
	}
	if len(judge.Advisory) > 0 {
		b.WriteString("Design review advisory:\n")
		for _, item := range judge.Advisory {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	if len(critic.Advisory) > 0 {
		b.WriteString("Local semantic advisory:\n")
		for _, item := range critic.Advisory {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	b.WriteString("Rendered files from current documents:\n")
	for _, file := range files {
		b.WriteString("- path: ")
		b.WriteString(strings.TrimSpace(file.Path))
		b.WriteString("\n")
		b.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func postProcessEditSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask post-processing editor. Return strict JSON only using the document generation response shape.\n")
	b.WriteString("Edit only the files required to address blocking operational defects. Preserve valid files when possible.\n")
	b.WriteString("Do not extract vars or components unless explicitly required by the findings and clearly beneficial. Preserve inline structure by default.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	return b.String()
}

func postProcessEditUserPrompt(files []askcontract.GeneratedFile, findings askcontract.PostProcessResponse, planCritic askcontract.PlanCriticResponse) string {
	b := &strings.Builder{}
	if summary := generatedDocumentSummaryBlock(files); strings.TrimSpace(summary) != "" {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	if advisory := planAdvisoryPromptBlock(askcontract.PlanResponse{}, planCritic); strings.TrimSpace(advisory) != "" {
		b.WriteString(advisory)
		b.WriteString("\n")
	}
	b.WriteString("Blocking operational findings:\n")
	for _, item := range findings.Blocking {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	for _, item := range findings.SuggestedFixes {
		b.WriteString("- fix: ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	for _, item := range findings.RequiredEdits {
		b.WriteString("- required edit: ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	for _, item := range findings.VerificationExpectations {
		b.WriteString("- verify after edit: ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("Revise these files first:\n")
	for _, item := range findings.ReviseFiles {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("Preserve these files if they are already valid:\n")
	for _, item := range findings.PreserveFiles {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("Rendered files from current documents:\n")
	for _, file := range files {
		b.WriteString("- path: ")
		b.WriteString(strings.TrimSpace(file.Path))
		b.WriteString("\n")
		b.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func judgeUserPrompt(files []askcontract.GeneratedFile, lintSummary string, critic askcontract.CriticResponse) string {
	b := &strings.Builder{}
	b.WriteString("Local validation summary: ")
	b.WriteString(strings.TrimSpace(lintSummary))
	b.WriteString("\n")
	if summary := generatedDocumentSummaryBlock(files); strings.TrimSpace(summary) != "" {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	if len(critic.Advisory) > 0 {
		b.WriteString("Local semantic advisory:\n")
		for _, item := range critic.Advisory {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(item))
			b.WriteString("\n")
		}
	}
	b.WriteString("Rendered files from current documents:\n")
	for _, file := range files {
		b.WriteString("- path: ")
		b.WriteString(file.Path)
		b.WriteString("\n")
		b.WriteString(file.Content)
		if !strings.HasSuffix(file.Content, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
