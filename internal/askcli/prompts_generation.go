package askcli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdraft"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askrefine"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askscaffold"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
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
		fmt.Sprintf("Examples: 'Explain how %s/worker-join.yaml works' -> explain.", workspacepaths.CanonicalScenariosDir),
		fmt.Sprintf("Examples: 'Review %s for offline issues' -> review.", workspacepaths.CanonicalApplyWorkflow),
		fmt.Sprintf("Examples: 'Refactor %s to use %s' -> refine.", workspacepaths.CanonicalApplyWorkflow, workspacepaths.CanonicalVarsWorkflow),
		"Examples: 'Create an air-gapped kubeadm workflow' -> draft.",
		"Examples: '3 노드 쿠버네티스 클러스터링 워크플로우를 구성해줘' -> draft.",
		fmt.Sprintf("Examples: '%s 을 설명해줘' -> explain.", workspacepaths.CanonicalApplyWorkflow),
		fmt.Sprintf("Examples: '%s 을 검토해줘' -> review.", workspacepaths.CanonicalApplyWorkflow),
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
		b.WriteString("- Do not return `documents` for draft generation. Return builder selection only.\n")
	}
	b.WriteString("- Never place summary, description, or review fields inside workflow YAML content.\n")
	b.WriteString("- Keep the file set minimal unless the request explicitly requires more files or the workspace already depends on them.\n")
	_, _ = fmt.Fprintf(b, "- For workspace-scoped complex drafts, prefer a first schema-valid inline result in `%s` and `%s`; extract `%s/` only when reuse is explicit, the workspace already imports them, or component files are clearly required final outputs.\n", workspacepaths.CanonicalPrepareWorkflow, workspacepaths.CanonicalApplyWorkflow, workspacepaths.CanonicalComponentsDir)
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
	b.WriteString("- Primary refine generation should use `edit` actions with code-owned transforms. Candidate ids are preferred when available; otherwise include explicit raw paths. Full replacement documents are not allowed.\n")
	_, _ = fmt.Fprintf(b, "- Prefer `transforms` with type `extract-var` for repeated literal extraction into %s when the request is about hoisting repeated values.\n", workspacepaths.CanonicalVarsWorkflow)
	b.WriteString("- Prefer `transforms` with type `set-field` or `delete-field` for narrow step field changes instead of broad document rewrites.\n")
	_, _ = fmt.Fprintf(b, "- Prefer `transforms` with type `extract-component` when moving inline phase steps into %s/ while preserving the scenario phase layout.\n", workspacepaths.CanonicalComponentsDir)
	b.WriteString("- Do not use model-authored `replace` output on the primary refine path.\n")
	if promptContainsTrimmed(paths, workspacepaths.CanonicalVarsWorkflow) {
		_, _ = fmt.Fprintf(b, "- When extracting repeated values into %s, update the scenario file and vars file together as one transform.\n", workspacepaths.CanonicalVarsWorkflow)
	}
	_, _ = fmt.Fprintf(b, "- For `extract-var`, put the variable key in `varName`. Use `varsPath` only for the companion file path such as `%s`.\n", workspacepaths.CanonicalVarsWorkflow)
	if len(plan.VarsRecommendation) > 0 {
		_, _ = fmt.Fprintf(b, "- Only extract values into %s when they are explicitly recommended or genuinely repeated. Keep other literals inline.\n", workspacepaths.CanonicalVarsWorkflow)
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
		return fmt.Sprintf("JSON shape: {\"summary\":string,\"review\":[]string,\"documents\":[{\"path\":string,\"kind\":string,\"action\":string,\"transforms\":[{\"type\":string,\"candidate\":string?,\"rawPath\":string?,\"value\":any?,\"varName\":string?,\"varsPath\":string?,\"path\":string?}]}]}. For `extract-var`, `varName` is the variable key and `varsPath` is only the file path like `%s`. On the primary refine path, use actions preserve|delete|edit and prefer transform candidate ids over raw paths.", workspacepaths.CanonicalVarsWorkflow)
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
	if len(selected) > maxRelevantSchemaItems {
		selected = selected[:maxRelevantSchemaItems]
	}
	b := &strings.Builder{}
	b.WriteString("Relevant typed-step schemas:\n")
	b.WriteString("- Use these only when they match the requested workflow. Treat them as schema facts, not mandatory choices.\n")
	for _, item := range selected {
		b.WriteString("- ")
		b.WriteString(item.Step.Kind)
		group := item.Step.GroupTitle
		if group == "" {
			group = item.Step.Group
		}
		if group != "" {
			b.WriteString(" [group: ")
			b.WriteString(group)
			b.WriteString("]")
		}
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
			if exampleCount >= maxPromptExamples {
				continue
			}
			exampleCount++
		}
		chunks = append(chunks, chunk)
	}
	return askretrieve.BuildChunkText(askretrieve.RetrievalResult{Chunks: chunks})
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
		_, _ = fmt.Fprintf(b, "At minimum, the result should usually include a valid %s file.\n", workspacepaths.CanonicalApplyWorkflow)
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
		if !strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(file.Path)), workflowRootPrefix) {
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
