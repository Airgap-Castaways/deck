package askcli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askdraft"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askrefine"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askscaffold"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func classifierSystemPrompt() string {
	return strings.Join([]string{
		"You are a classifier for deck ask.",
		"Return strict JSON only.",
		"Valid route values: clarify, question, explain, review, refine, draft.",
		"Only choose draft/refine when user clearly asks to create or modify workflow files.",
		"When user asks analyze/explain/summarize existing scenario, choose explain or review.",
		"Do not treat words like workflow, scenario, prepare, or apply as authoring intent by themselves.",
		"Prefer clarify over guessing when the user intent between explain, review, create, and edit is ambiguous.",
		"Examples: 'Explain how workflows/scenarios/worker-join.yaml works' -> explain.",
		"Examples: 'Review workflows/scenarios/apply.yaml for offline issues' -> review.",
		"Examples: 'Refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml' -> refine.",
		"Examples: 'Create an air-gapped kubeadm workflow' -> draft.",
		"Examples: '3 노드 쿠버네티스 클러스터링 워크플로우를 구성해줘' -> draft.",
		"Examples: 'workflows/scenarios/apply.yaml 을 설명해줘' -> explain.",
		"Examples: 'workflows/scenarios/apply.yaml 을 검토해줘' -> review.",
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

func generationSystemPrompt(route askintent.Route, target askintent.Target, requestText string, retrieval askretrieve.RetrievalResult, requirements askpolicy.ScenarioRequirements, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, executionModel askcontract.ExecutionModel, scaffold askscaffold.Scaffold) string {
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
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	b.WriteString(bundle.WorkflowPromptBlock())
	b.WriteString("\n")
	b.WriteString(askpolicy.RequirementsPromptBlock(requirements))
	b.WriteString("\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(authoringProgramPromptBlock(plan.AuthoringProgram))
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
	if route == askintent.RouteDraft {
		if block := askdraft.PromptBlock(plan, brief); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	if route == askintent.RouteRefine {
		if block := refineTransformPromptBlock(plan, brief); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	b.WriteString(bundle.PolicyPromptBlock())
	b.WriteString("\n")
	if route == askintent.RouteDraft {
		b.WriteString("- For draft generation, use `selection.targets[].builders` as the primary path and set only documented override keys.\n")
		b.WriteString("- Do not author arbitrary typed step specs on the primary draft path. Let code assemble documents from the selected builders.\n")
		b.WriteString("- Do not return `documents` for draft generation unless an explicit migration fallback is enabled by the caller.\n")
	}
	b.WriteString("- Never place summary, description, or review fields inside workflow YAML content.\n")
	b.WriteString("- Keep the file set minimal unless the request explicitly requires more files or the workspace already depends on them.\n")
	b.WriteString("- For workspace-scoped complex drafts, prefer a first schema-valid inline result in `workflows/prepare.yaml` and `workflows/scenarios/apply.yaml`; extract `workflows/components/` only when reuse is explicit, the workspace already imports them, or component files are clearly required final outputs.\n")
	b.WriteString("- Keep document structure schema-focused: allowed paths, workflow invariants, execution contracts, and repository examples take priority over free-form step prose.\n")
	b.WriteString("- Do not use Kubernetes-style fields such as apiVersion, kind, metadata, or spec wrappers at the workflow top level.\n")
	b.WriteString("- Do not invent unsupported fields.\n")
	return b.String()
}

func evidenceBoundaryPromptBlock(retrieval askretrieve.RetrievalResult) string {
	hasLocalFacts := false
	hasExternalEvidence := false
	externalLines := make([]string, 0)
	for _, chunk := range retrieval.Chunks {
		switch chunk.Source {
		case "local-facts":
			hasLocalFacts = true
		case "mcp":
			hasExternalEvidence = true
			if chunk.Evidence != nil {
				line := "- external source"
				if strings.TrimSpace(chunk.Evidence.Title) != "" {
					line += ": " + strings.TrimSpace(chunk.Evidence.Title)
				}
				parts := make([]string, 0, 6)
				if strings.TrimSpace(chunk.Evidence.Domain) != "" {
					parts = append(parts, "domain="+strings.TrimSpace(chunk.Evidence.Domain))
				}
				if strings.TrimSpace(chunk.Evidence.DomainCategory) != "" {
					parts = append(parts, "category="+strings.TrimSpace(chunk.Evidence.DomainCategory))
				}
				if strings.TrimSpace(chunk.Evidence.Freshness) != "" {
					parts = append(parts, "freshness="+strings.TrimSpace(chunk.Evidence.Freshness))
				}
				if chunk.Evidence.Official {
					parts = append(parts, "official=true")
				}
				if strings.TrimSpace(chunk.Evidence.TrustLevel) != "" {
					parts = append(parts, "trust="+strings.TrimSpace(chunk.Evidence.TrustLevel))
				}
				if strings.TrimSpace(chunk.Evidence.VersionSupport) != "" {
					parts = append(parts, "versionSupport="+strings.TrimSpace(chunk.Evidence.VersionSupport))
				}
				if len(parts) > 0 {
					line += " [" + strings.Join(parts, ", ") + "]"
				}
				externalLines = append(externalLines, line)
			}
		}
	}
	if !hasLocalFacts && !hasExternalEvidence {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Evidence boundaries:\n")
	if hasLocalFacts {
		b.WriteString("- Local facts are authoritative for deck workflow validity, typed step metadata, builder behavior, and repair semantics.\n")
	}
	if hasExternalEvidence {
		b.WriteString("- External evidence is only for upstream product behavior, install steps, compatibility, versions, or troubleshooting recency.\n")
		b.WriteString("- Do not let external docs override local deck workflow truth, schema rules, validator behavior, or path constraints.\n")
		for _, line := range externalLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func refineTransformPromptBlock(plan askcontract.PlanResponse, brief askcontract.AuthoringBrief) string {
	paths := dedupe(append([]string(nil), brief.TargetPaths...))
	if len(paths) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Refine transform contract:\n")
	b.WriteString("- Treat refine as a targeted transform over existing documents, not a full workspace rewrite.\n")
	b.WriteString("- Only touch plan-approved target paths unless the plan explicitly requires another file.\n")
	b.WriteString("- Prefer transform candidate ids over model-authored raw paths. Raw code-owned transforms are allowed only when they still point at an explicit target path.\n")
	if len(paths) > 0 {
		b.WriteString("- Approved target paths: ")
		b.WriteString(strings.Join(paths, ", "))
		b.WriteString("\n")
	}
	b.WriteString("- Primary refine generation should use `edit` actions with code-owned transforms. Candidate ids are preferred when available; otherwise include explicit raw paths. Full replacement documents are fallback-only.\n")
	b.WriteString("- Prefer `transforms` with type `extract-var` for repeated literal extraction into workflows/vars.yaml when the request is about hoisting repeated values.\n")
	b.WriteString("- Prefer `transforms` with type `set-field` or `delete-field` for narrow step field changes instead of broad document rewrites.\n")
	b.WriteString("- Prefer `transforms` with type `extract-component` when moving inline phase steps into workflows/components/ while preserving the scenario phase layout.\n")
	b.WriteString("- Do not use model-authored `replace` output on the primary refine path.\n")
	if promptContainsTrimmed(paths, "workflows/vars.yaml") {
		b.WriteString("- When extracting repeated values into workflows/vars.yaml, update the scenario file and vars file together as one transform.\n")
	}
	b.WriteString("- For `extract-var`, put the variable key in `varName`. Use `varsPath` only for the companion file path such as `workflows/vars.yaml`.\n")
	if len(plan.VarsRecommendation) > 0 {
		b.WriteString("- Only extract values into workflows/vars.yaml when they are explicitly recommended or genuinely repeated. Keep other literals inline.\n")
	}
	if len(paths) > 1 {
		b.WriteString("- Keep cross-file transforms coordinated: do not update one target path while silently dropping required companion edits in another approved target path.\n")
	}
	for _, advisory := range dedupe(append([]string{}, plan.VarsRecommendation...)) {
		b.WriteString("- vars advisory: ")
		b.WriteString(strings.TrimSpace(advisory))
		b.WriteString("\n")
	}
	for _, advisory := range dedupe(append([]string{}, plan.ComponentRecommendation...)) {
		b.WriteString("- component advisory: ")
		b.WriteString(strings.TrimSpace(advisory))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func promptContainsTrimmed(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func generationResponseShapeRule(route askintent.Route) string {
	if route == askintent.RouteRefine {
		return "JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"transforms\":[{\"type\":string,\"candidate\":string?,\"rawPath\":string?,\"value\":any?,\"varName\":string?,\"varsPath\":string?,\"path\":string?}]}]}. For `extract-var`, `varName` is the variable key and `varsPath` is only the file path like `workflows/vars.yaml`. On the primary refine path, use actions preserve|delete|edit and prefer transform candidate ids over raw paths."
	}
	return "JSON shape: {\"summary\":string,\"review\":[]string,\"selection\":{\"patterns\":[]string,\"targets\":[{\"path\":string,\"kind\":string,\"builders\":[{\"id\":string,\"overrides\":object}],\"vars\":object}],\"vars\":object}}. On the primary draft path, return builder selection only and let code compile the workflow documents."
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
	return askretrieve.BuildChunkText(askretrieve.RetrievalResult{Chunks: chunks})
}

func documentRepairSystemPrompt(brief askcontract.AuthoringBrief, plan askcontract.PlanResponse) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask document repair assistant. Return strict JSON only using the document generation response shape.\n")
	b.WriteString("JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"workflow\":object?,\"component\":object?,\"vars\":object?,\"edits\":[]object?}]}. documents must contain at least one revised document.\n")
	b.WriteString("Refine repair may use structured transforms when a code-owned operation is more reliable than open-ended edits. Supported transforms: {type: extract-var, candidate: string, varName?: string, varsPath?: string, value: any}, {type: set-field, candidate: string, value: any}, {type: delete-field, candidate: string}, {type: extract-component, candidate: string, path?: string}. Use rawPath only when no candidate id exists.\n")
	b.WriteString("Repair document structure and schema issues with the smallest possible edits. Do not redesign the workflow unless a validator message explicitly requires it.\n")
	b.WriteString("Keep preserve-if-valid documents byte-for-byte identical after rendering. Revise only documents implicated by the parse or schema error when possible.\n")
	b.WriteString("Return only revised documents when possible; unchanged rendered files will be preserved by the caller.\n")
	b.WriteString("Every rendered workflow file must stay standalone-valid and preserve existing structure unless the validator requires a targeted change.\n")
	b.WriteString(authoringBriefPromptBlock(brief))
	b.WriteString("\n")
	b.WriteString(authoringProgramPromptBlock(plan.AuthoringProgram))
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
	if len(diags) == 0 {
		b.WriteString("Validator summary:\n")
		b.WriteString(summarizeValidationError(validation))
		b.WriteString("\nRaw validator error:\n")
		b.WriteString(strings.TrimSpace(validation))
		b.WriteString("\n")
	} else {
		b.WriteString("Structured validator findings:\n")
		for _, diag := range diags {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(diag.Message))
			if strings.TrimSpace(diag.Path) != "" {
				b.WriteString(" path=")
				b.WriteString(strings.TrimSpace(diag.Path))
			}
			if strings.TrimSpace(diag.RepairOp) != "" {
				b.WriteString(" op=")
				b.WriteString(strings.TrimSpace(diag.RepairOp))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(askdiagnostic.RepairPromptBlock(diags))
	b.WriteString("\n")
	b.WriteString(repairOperationPromptBlock(diags))
	b.WriteString("\n")
	b.WriteString(documentStructureRepairPromptBlock(prevFiles, validation, repairPaths))
	b.WriteString("\n")
	b.WriteString(targetedRepairPromptBlock(prevFiles, diags, repairPaths))
	return strings.TrimSpace(b.String())
}

func repairOperationPromptBlock(diags []askdiagnostic.Diagnostic) string {
	ops := map[string][]string{}
	for _, diag := range diags {
		op := strings.TrimSpace(diag.RepairOp)
		if op == "" {
			continue
		}
		detail := strings.TrimSpace(diag.Path)
		if detail == "" {
			detail = strings.TrimSpace(diag.Message)
		}
		if strings.TrimSpace(diag.StepKind) != "" {
			detail = strings.TrimSpace(diag.StepKind) + " " + detail
		}
		ops[op] = append(ops[op], detail)
	}
	if len(ops) == 0 {
		return ""
	}
	order := []string{"fill-field", "remove-field", "fix-literal", "rename-step", "repair-structure", "review-diagnostic"}
	b := &strings.Builder{}
	b.WriteString("Suggested repair operations:\n")
	for _, op := range order {
		items := dedupe(ops[op])
		if len(items) == 0 {
			continue
		}
		b.WriteString("- ")
		b.WriteString(op)
		b.WriteString(": ")
		b.WriteString(strings.Join(items, ", "))
		b.WriteString("\n")
	}
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
	appendLine("escape hatch mode", brief.EscapeHatchMode)
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

func infoPrompts(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, prompt string) (string, string) {
	switch route {
	case askintent.RouteExplain:
		return explainSystemPrompt(target, retrieval), explainUserPrompt(prompt, target)
	case askintent.RouteReview:
		return reviewSystemPrompt(target, retrieval, workspace), reviewUserPrompt(prompt, target)
	case askintent.RouteQuestion:
		return questionSystemPrompt(target, retrieval), questionUserPrompt(prompt, target)
	default:
		return infoSystemPrompt(route, target, retrieval), infoUserPrompt(prompt, route, target)
	}
}

func generationUserPrompt(workspace askretrieve.WorkspaceSummary, state askstate.Context, prompt string, fromLabel string, route askintent.Route, plan askcontract.PlanResponse) string {
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
		if len(plan.AuthoringBrief.TargetPaths) > 0 {
			b.WriteString("Clarified refine target paths:\n")
			for _, path := range dedupe(plan.AuthoringBrief.TargetPaths) {
				b.WriteString("- ")
				b.WriteString(strings.TrimSpace(path))
				b.WriteString("\n")
			}
		}
		if summaries := currentWorkspaceDocumentSummaries(workspace); len(summaries) > 0 {
			b.WriteString("Current parsed workspace documents:\n")
			for _, summary := range summaries {
				b.WriteString("- ")
				b.WriteString(summary)
				b.WriteString("\n")
			}
		}
		if block := askrefine.PromptBlock(plan, currentWorkspaceDocuments(workspace)); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	b.WriteString("Return the minimum complete file set needed for this request.\n")
	return b.String()
}

func currentWorkspaceDocumentSummaries(workspace askretrieve.WorkspaceSummary) []string {
	return askir.Summaries(currentWorkspaceDocuments(workspace))
}

func currentWorkspaceDocuments(workspace askretrieve.WorkspaceSummary) []askcontract.GeneratedDocument {
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
	return documents
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
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
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
	if block := weakExternalEvidencePromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
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
	if repoBehaviorExplainPrompt(target, retrieval) {
		b.WriteString("You are deck ask explaining how this repository assembles workflow behavior.\n")
		b.WriteString("Anchor first on code-owned paths such as internal/stepmeta, internal/stepspec, internal/askdraft, and related compiler helpers.\n")
		b.WriteString("Explain the assembly path in code terms: registry/metadata -> builder selection -> binding resolution -> workflow document compilation.\n")
		b.WriteString("Use current workspace YAML only as a secondary example, not as the primary explanation.\n")
	} else {
		b.WriteString("You are deck ask explaining an existing deck workspace file or workflow.\n")
		b.WriteString("Explain what the target does, how it fits into the workflow, and call out imports, phases, major step kinds, and Command usage when present.\n")
		b.WriteString("Do not give a shallow file count summary.\n")
	}
	if block := weakExternalEvidencePromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if !repoBehaviorExplainPrompt(target, retrieval) {
		if block := retrievalDocumentSummaryBlock(retrieval); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func repoBehaviorExplainPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) bool {
	if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(target.Path)), "internal/") {
		return true
	}
	for _, chunk := range retrieval.Chunks {
		if chunk.Source != "local-facts" {
			continue
		}
		if chunk.ID == "local-facts-stepmeta" || chunk.ID == "local-facts-stepspec" || chunk.ID == "local-facts-askdraft" {
			return true
		}
	}
	return false
}

func weakExternalEvidencePromptBlock(retrieval askretrieve.RetrievalResult) string {
	for _, chunk := range retrieval.Chunks {
		if chunk.Source != "external-evidence" || strings.TrimSpace(chunk.Label) != "weak-evidence-status" {
			continue
		}
		return "When install/setup evidence is weak, say official source retrieval was incomplete and keep the answer narrowly bounded as general guidance only. Do not present distro-specific or version-specific install steps as verified unless the retrieved evidence explicitly supports them."
	}
	return ""
}

func reviewSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask reviewing an existing deck workspace.\n")
	b.WriteString("Use the retrieved evidence and any local findings to produce a scoped review with practical concerns and suggested changes.\n")
	b.WriteString("Narrate the findings instead of only repeating raw warnings.\n")
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := retrievalDocumentSummaryBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := workspaceStructuredIssuePromptBlock(workspace, target); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func workspaceStructuredIssuePromptBlock(workspace askretrieve.WorkspaceSummary, target askintent.Target) string {
	issues := []validate.Issue{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == "" || !strings.HasPrefix(path, "workflows/") {
			continue
		}
		if target.Path != "" && path != filepath.ToSlash(strings.TrimSpace(target.Path)) {
			continue
		}
		err := validate.Bytes(path, []byte(file.Content))
		var validationErr *validate.ValidationError
		if !errors.As(err, &validationErr) {
			continue
		}
		issues = append(issues, validationErr.ValidationIssues()...)
	}
	if len(issues) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Structured validation issues:\n")
	for _, diag := range askdiagnostic.FromValidationIssues(issues) {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(diag.Message))
		if strings.TrimSpace(diag.Path) != "" {
			b.WriteString(" path=")
			b.WriteString(strings.TrimSpace(diag.Path))
		}
		if strings.TrimSpace(diag.SuggestedFix) != "" {
			b.WriteString(" fix=")
			b.WriteString(strings.TrimSpace(diag.SuggestedFix))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func retrievalDocumentSummaryBlock(retrieval askretrieve.RetrievalResult) string {
	documents := make([]askcontract.GeneratedDocument, 0, len(retrieval.Chunks))
	for _, chunk := range retrieval.Chunks {
		path := strings.TrimSpace(chunk.Label)
		if path == "" || !strings.HasPrefix(filepath.ToSlash(path), "workflows/") {
			continue
		}
		doc, err := askir.ParseDocument(path, []byte(chunk.Content))
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}
	if len(documents) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Parsed workflow summaries:\n")
	for _, summary := range askir.Summaries(documents) {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(summary))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
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
