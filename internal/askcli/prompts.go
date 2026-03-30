package askcli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askscaffold"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func classifierSystemPrompt() string {
	return strings.Join([]string{
		"You are a classifier for deck ask.",
		"Return strict JSON only.",
		"Valid route values: clarify, question, explain, review, refine, draft.",
		"Only choose draft/refine when user clearly asks to create or modify workflow files.",
		"When user asks analyze/explain/summarize existing scenario, choose explain or review.",
		"Include target.kind (workspace|scenario|component|vars|unknown) and optional target.path/name when inferable.",
		"JSON shape: {\"route\":string,\"confidence\":number,\"reason\":string,\"target\":{\"kind\":string,\"path\":string,\"name\":string},\"generationAllowed\":boolean}",
	}, "\n")
}

func classifierUserPrompt(prompt string, reviewFlag bool, workspace askretrieve.WorkspaceSummary) string {
	b := &strings.Builder{}
	b.WriteString("User prompt:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
	_, _ = fmt.Fprintf(b, "review flag: %t\n", reviewFlag)
	_, _ = fmt.Fprintf(b, "has workflow tree: %t\n", workspace.HasWorkflowTree)
	_, _ = fmt.Fprintf(b, "has prepare scenario: %t\n", workspace.HasPrepare)
	_, _ = fmt.Fprintf(b, "has apply scenario: %t\n", workspace.HasApply)
	b.WriteString("workspace files:\n")
	for _, file := range workspace.Files {
		b.WriteString("- ")
		b.WriteString(file.Path)
		b.WriteString("\n")
	}
	return b.String()
}

func generationSystemPrompt(route askintent.Route, target askintent.Target, requestText string, retrieval askretrieve.RetrievalResult, requirements askpolicy.ScenarioRequirements, brief askcontract.AuthoringBrief, executionModel askcontract.ExecutionModel, scaffold askscaffold.Scaffold) string {
	bundle := askknowledge.Current()
	b := &strings.Builder{}
	b.WriteString("You are deck ask, a workflow authoring assistant.\n")
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	b.WriteString("Target kind: ")
	b.WriteString(target.Kind)
	b.WriteString("\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("Rules:\n")
	b.WriteString("- Produce only strict JSON.\n")
	b.WriteString("- ")
	b.WriteString(generationResponseShapeRule(route))
	b.WriteString("\n")
	b.WriteString("- Start from the closest repository examples and workspace files first, then adapt them to the request.\n")
	b.WriteString("- Return structured workflow documents, not final YAML text. The caller validates and renders YAML.\n")
	b.WriteString("- Keep existing repo-native workflow structure and file boundaries whenever possible.\n")
	b.WriteString("- ")
	b.WriteString(workflowissues.MustSpec(workflowissues.CodeDuplicateStepID).Details)
	b.WriteString(" ")
	b.WriteString(workflowissues.MustSpec(workflowissues.CodeDuplicateStepID).PromptHint)
	b.WriteString("\n")
	b.WriteString("Primary repository context follows. Prefer workspace snippets first, then the closest repository examples.\n")
	b.WriteString(generationRetrievalPromptBlock(retrieval))
	b.WriteString("\n")
	b.WriteString(bundle.WorkflowPromptBlock())
	b.WriteString("\n")
	b.WriteString(askpolicy.RequirementsPromptBlock(requirements))
	b.WriteString("\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	if len(executionModel.ArtifactContracts) > 0 || len(executionModel.SharedStateContracts) > 0 || strings.TrimSpace(executionModel.RoleExecution.RoleSelector) != "" {
		b.WriteString(executionModelPromptBlock(executionModel))
		b.WriteString("\n")
	}
	if stepSchemaFacts := compactRelevantSchemaPromptBlock(requestText, target, requirements, brief); strings.TrimSpace(stepSchemaFacts) != "" {
		b.WriteString(stepSchemaFacts)
		b.WriteString("\n")
	}
	if strings.TrimSpace(brief.CompletenessTarget) == "starter" || route == askintent.RouteRefine {
		b.WriteString(askscaffold.PromptBlock(scaffold))
		b.WriteString("\n")
	}
	b.WriteString(bundle.PolicyPromptBlock())
	b.WriteString("\n")
	b.WriteString("- Never place summary, description, or review fields inside workflow YAML content.\n")
	b.WriteString("- Keep the file set minimal unless the request explicitly requires more files or the workspace already depends on them.\n")
	b.WriteString("- For workspace-scoped complex drafts, prefer a first schema-valid inline result in `workflows/prepare.yaml` and `workflows/scenarios/apply.yaml`; extract `workflows/components/` only when reuse is explicit, the workspace already imports them, or component files are clearly required final outputs.\n")
	b.WriteString("- Keep document structure schema-focused: allowed paths, workflow invariants, execution contracts, and repository examples take priority over free-form step prose.\n")
	b.WriteString("- Do not use Kubernetes-style fields such as apiVersion, kind, metadata, or spec wrappers at the workflow top level.\n")
	b.WriteString("- Do not invent unsupported fields.\n")
	return b.String()
}

func generationResponseShapeRule(route askintent.Route) string {
	if route == askintent.RouteRefine {
		return "JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"workflow\":object?,\"component\":object?,\"vars\":object?,\"edits\":[]object?}]}. Use actions preserve|replace|create|edit|delete."
	}
	return "JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"workflow\":object?,\"component\":object?,\"vars\":object?}]}."
}

func compactRelevantSchemaPromptBlock(requestText string, target askintent.Target, requirements askpolicy.ScenarioRequirements, brief askcontract.AuthoringBrief) string {
	seedParts := []string{strings.TrimSpace(requestText), brief.ModeIntent, brief.Topology, brief.Connectivity, strings.Join(brief.RequiredCapabilities, " ")}
	seedParts = append(seedParts, requirements.ScenarioIntent...)
	seedParts = append(seedParts, requirements.ArtifactKinds...)
	seedParts = append(seedParts, target.Kind, target.Name, target.Path)
	selected := askcontext.DiscoverCandidateStepsWithOptions(strings.Join(seedParts, " "), askcontext.StepGuidanceOptions{ModeIntent: brief.ModeIntent, Topology: brief.Topology, RequiredCapabilities: brief.RequiredCapabilities})
	if len(selected) == 0 {
		return ""
	}
	if len(selected) > 3 {
		selected = selected[:3]
	}
	b := &strings.Builder{}
	b.WriteString("Relevant typed-step schemas:\n")
	b.WriteString("- Use these only when they match the requested workflow. Treat them as schema facts, not mandatory choices.\n")
	for _, item := range selected {
		b.WriteString("- ")
		b.WriteString(item.Step.Kind)
		if len(item.Step.AllowedRoles) > 0 {
			b.WriteString(" [roles: ")
			b.WriteString(strings.Join(item.Step.AllowedRoles, ", "))
			b.WriteString("]")
		}
		b.WriteString("\n")
		for _, field := range item.Step.KeyFields {
			if strings.TrimSpace(field.Path) == "" {
				continue
			}
			requirement := strings.TrimSpace(field.Requirement)
			if requirement == "" {
				requirement = "optional"
			}
			if requirement == "optional" {
				continue
			}
			b.WriteString("  - ")
			b.WriteString(field.Path)
			b.WriteString(" [")
			b.WriteString(requirement)
			b.WriteString("]")
			if strings.TrimSpace(field.Description) != "" {
				b.WriteString(": ")
				b.WriteString(strings.TrimSpace(field.Description))
			}
			b.WriteString("\n")
		}
		for _, rule := range item.Step.SchemaRuleSummaries {
			if strings.TrimSpace(rule) == "" {
				continue
			}
			b.WriteString("  - rule: ")
			b.WriteString(strings.TrimSpace(rule))
			b.WriteString("\n")
			break
		}
		for _, hint := range item.Step.ValidationHints {
			if strings.TrimSpace(hint.Fix) == "" {
				continue
			}
			b.WriteString("  - validation: ")
			b.WriteString(strings.TrimSpace(hint.Fix))
			b.WriteString("\n")
			break
		}
		for _, field := range item.Step.ConstrainedLiteralFields {
			if strings.TrimSpace(field.Path) == "" {
				continue
			}
			b.WriteString("  - constrained: ")
			b.WriteString(strings.TrimSpace(field.Path))
			if len(field.AllowedValues) > 0 {
				b.WriteString(" [allowed: ")
				b.WriteString(strings.Join(field.AllowedValues, ", "))
				b.WriteString("]")
			}
			if strings.TrimSpace(field.Guidance) != "" {
				b.WriteString(": ")
				b.WriteString(strings.TrimSpace(field.Guidance))
			}
			b.WriteString("\n")
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func generationRetrievalPromptBlock(retrieval askretrieve.RetrievalResult) string {
	priority := map[string]int{
		"workspace":      0,
		"plan-workspace": 1,
		"example":        2,
		"repo-map":       3,
		"plan":           4,
		"state":          5,
		"project":        6,
		"mcp":            7,
		"lsp":            8,
	}
	excludedTopics := map[askcontext.Topic]bool{
		askcontext.TopicWorkflowInvariants:   true,
		askcontext.TopicPolicy:               true,
		askcontext.TopicWorkspaceTopology:    true,
		askcontext.TopicPrepareApplyGuidance: true,
		askcontext.TopicComponentsImports:    true,
		askcontext.TopicVarsGuidance:         true,
		askcontext.TopicTypedSteps:           true,
		askcontext.TopicCLIHints:             true,
	}
	chunks := make([]askretrieve.Chunk, 0, len(retrieval.Chunks))
	exampleCount := 0
	for _, chunk := range retrieval.Chunks {
		if excludedTopics[chunk.Topic] {
			continue
		}
		if chunk.Source == "project" {
			continue
		}
		if strings.Contains(chunk.Content, "\n...\n") || strings.HasSuffix(strings.TrimSpace(chunk.Content), "...") {
			continue
		}
		if chunk.Source == "example" {
			if exampleCount >= 2 {
				continue
			}
			exampleCount++
		}
		chunks = append(chunks, chunk)
	}
	sort.SliceStable(chunks, func(i, j int) bool {
		pi, okI := priority[chunks[i].Source]
		pj, okJ := priority[chunks[j].Source]
		if !okI {
			pi = 50
		}
		if !okJ {
			pj = 50
		}
		if pi == pj {
			if chunks[i].Score == chunks[j].Score {
				return chunks[i].ID < chunks[j].ID
			}
			return chunks[i].Score > chunks[j].Score
		}
		return pi < pj
	})
	return askretrieve.BuildChunkText(askretrieve.RetrievalResult{Chunks: chunks})
}

func documentRepairSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask document repair assistant. Return strict JSON only using the document generation response shape.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"workflow\":object?,\"component\":object?,\"vars\":object?,\"edits\":[]object?}]}. documents must contain at least one revised document.\n")
	b.WriteString("Repair document structure and schema issues with the smallest possible edits. Do not redesign the workflow unless a validator message explicitly requires it.\n")
	b.WriteString("Keep preserve-if-valid documents byte-for-byte identical after rendering. Revise only documents implicated by the parse or schema error when possible.\n")
	b.WriteString("Return only revised documents when possible; unchanged rendered files will be preserved by the caller.\n")
	b.WriteString("Every rendered workflow file must stay standalone-valid and preserve existing structure unless the validator requires a targeted change.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	return b.String()
}

func documentRepairUserPrompt(prevFiles []askcontract.GeneratedFile, validation string, diags []askdiagnostic.Diagnostic, repairPaths []string) string {
	b := &strings.Builder{}
	b.WriteString("Repair these generated documents without redesigning them. Return only the revised documents if possible.\n")
	b.WriteString("Do not introduce new step kinds, new workflow files, or new execution contracts unless the validator error explicitly requires them.\n")
	b.WriteString("Focus only on the affected file paths named by the validator.\n")
	b.WriteString("Validator summary:\n")
	b.WriteString(summarizeValidationError(validation))
	b.WriteString("\nRaw validator error:\n")
	b.WriteString(strings.TrimSpace(validation))
	b.WriteString("\n")
	b.WriteString(askdiagnostic.RepairPromptBlock(diags))
	b.WriteString("\n")
	b.WriteString(documentStructureRepairPromptBlock(prevFiles, validation, repairPaths))
	b.WriteString("\n")
	b.WriteString(targetedRepairPromptBlock(prevFiles, diags, repairPaths))
	return strings.TrimSpace(b.String())
}

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
	for _, item := range plan.Blockers {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, "planner carry-forward: "+item)
		}
	}
	for _, item := range plan.OpenQuestions {
		item = strings.TrimSpace(item)
		if item != "" && !strings.HasPrefix(strings.ToLower(item), "blocking:") {
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
	if brief.NodeCount > 0 {
		appendLine("node count", fmt.Sprintf("%d", brief.NodeCount))
	}
	appendList("target paths", brief.TargetPaths)
	appendList("required capabilities", brief.RequiredCapabilities)
	return strings.TrimSpace(b.String())
}

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

func structuralCleanupEditSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask structural cleanup editor. Return strict JSON only using the document generation response shape.\n")
	b.WriteString("Apply only optional readability or reuse improvements after operational defects are already resolved.\n")
	b.WriteString("Extract vars or components only when repeated values or repeated step groups clearly justify it. Preserve inline structure by default.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(executionModelPromptBlock(plan.ExecutionModel))
	b.WriteString("\n")
	return b.String()
}

func structuralCleanupEditUserPrompt(files []askcontract.GeneratedFile, findings askcontract.PostProcessResponse) string {
	b := &strings.Builder{}
	b.WriteString("Structural cleanup candidates:\n")
	for _, item := range findings.UpgradeCandidates {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("Advisory guidance:\n")
	for _, item := range findings.Advisory {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(item))
		b.WriteString("\n")
	}
	b.WriteString("Preserve these files unless cleanup clearly improves the result:\n")
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

func postProcessEditUserPrompt(files []askcontract.GeneratedFile, findings askcontract.PostProcessResponse, planCritic askcontract.PlanCriticResponse) string {
	b := &strings.Builder{}
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

func infoPrompts(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult, prompt string) (string, string) {
	switch route {
	case askintent.RouteExplain:
		return explainSystemPrompt(target, retrieval), explainUserPrompt(prompt, target)
	case askintent.RouteReview:
		return reviewSystemPrompt(target, retrieval), reviewUserPrompt(prompt, target)
	case askintent.RouteQuestion:
		return questionSystemPrompt(target, retrieval), questionUserPrompt(prompt, target)
	default:
		return infoSystemPrompt(route, target, retrieval), infoUserPrompt(prompt, route, target)
	}
}

func generationUserPrompt(workspace askretrieve.WorkspaceSummary, state askstate.Context, prompt string, fromLabel string, route askintent.Route) string {
	b := &strings.Builder{}
	b.WriteString("Workspace root: ")
	b.WriteString(workspace.Root)
	b.WriteString("\n")
	_, _ = fmt.Fprintf(b, "Has workflow tree: %t\n", workspace.HasWorkflowTree)
	_, _ = fmt.Fprintf(b, "Has prepare scenario: %t\n", workspace.HasPrepare)
	_, _ = fmt.Fprintf(b, "Has apply scenario: %t\n", workspace.HasApply)
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	if state.LastLint != "" {
		b.WriteString("Last lint summary: ")
		b.WriteString(state.LastLint)
		b.WriteString("\n")
	}
	if fromLabel != "" {
		b.WriteString("Attached request source: ")
		b.WriteString(fromLabel)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
	if !workspace.HasWorkflowTree && route == askintent.RouteDraft {
		b.WriteString("This is an empty workspace. Return the minimum valid workflow files needed to satisfy the request.\n")
		b.WriteString("At minimum, the result should usually include a valid workflows/scenarios/apply.yaml file.\n")
	}
	if route == askintent.RouteRefine {
		b.WriteString("For refine requests, prefer structured `edit` actions for narrow in-place changes to existing YAML documents.\n")
		b.WriteString("Use `replace` only when a local edit is not practical, `delete` only when removal is explicit, and `preserve` when a planned file should remain untouched.\n")
		if summaries := currentWorkspaceDocumentSummaries(workspace); len(summaries) > 0 {
			b.WriteString("Current parsed workspace documents:\n")
			for _, summary := range summaries {
				b.WriteString("- ")
				b.WriteString(summary)
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("Return the minimum complete file set needed for this request.\n")
	return b.String()
}

func currentWorkspaceDocumentSummaries(workspace askretrieve.WorkspaceSummary) []string {
	documents := make([]askcontract.GeneratedDocument, 0, len(workspace.Files))
	for _, file := range workspace.Files {
		if !strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(file.Path)), "workflows/") {
			continue
		}
		doc, err := askir.ParseDocument(file.Path, []byte(file.Content))
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}
	return askir.Summaries(documents)
}

func infoSystemPrompt(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask.\n")
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	b.WriteString("Target kind: ")
	b.WriteString(target.Kind)
	b.WriteString("\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	b.WriteString("Do not return file content for this route.\n")
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func questionSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask answering a workflow question.\n")
	b.WriteString("Answer the user's question directly and use retrieved evidence.\n")
	b.WriteString("Keep the answer concise but specific.\n")
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	b.WriteString("If evidence is incomplete, say what is known from the workspace and avoid speculation.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func explainSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask explaining an existing deck workspace file or workflow.\n")
	b.WriteString("Explain what the target does, how it fits into the workflow, and call out imports, phases, major step kinds, and Command usage when present.\n")
	b.WriteString("Do not give a shallow file count summary.\n")
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func reviewSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask reviewing an existing deck workspace.\n")
	b.WriteString("Use the retrieved evidence and any local findings to produce a scoped review with practical concerns and suggested changes.\n")
	b.WriteString("Narrate the findings instead of only repeating raw warnings.\n")
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func infoUserPrompt(prompt string, route askintent.Route, target askintent.Target) string {
	b := &strings.Builder{}
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func questionUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User question:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func explainUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Explain target: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func reviewUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Review target: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\nProvide a scoped review with concrete suggested changes.")
	return b.String()
}
