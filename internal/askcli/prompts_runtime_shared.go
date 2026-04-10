package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
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

func normalizedAuthoringBrief(plan askcontract.PlanResponse, fallback askcontract.AuthoringBrief) askcontract.AuthoringBrief {
	if strings.TrimSpace(plan.AuthoringBrief.RouteIntent) != "" {
		return plan.AuthoringBrief
	}
	return fallback
}

func isYAMLParseFailure(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "parse yaml") || strings.Contains(lower, "parse vars yaml") || strings.Contains(lower, "yaml: line ") || strings.Contains(lower, "yaml: did not") || strings.Contains(lower, "yaml: could not")
}
