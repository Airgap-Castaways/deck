package askcli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askevidenceplan"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func buildEvidencePlan(ctx context.Context, client askprovider.Client, cfg askconfig.EffectiveSettings, prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary, logger askLogger) (askcontract.EvidencePlan, []string, error) {
	plan := askevidenceplan.BuildEvidencePlan(prompt, workspace, decision)
	events := []string{renderEvidencePlanEvent(plan, "heuristic")}
	if askevidenceplan.ShouldUseLLMEvidencePlanner(plan, prompt, workspace, decision) && canUseLLM(cfg) {
		llmPlan, err := planEvidenceWithLLM(ctx, client, cfg, prompt, decision, workspace, logger)
		if err == nil {
			plan = mergeEvidencePlans(plan, llmPlan)
			events = append(events, renderEvidencePlanEvent(llmPlan, "llm"))
		} else {
			events = append(events, "evidence-plan: llm-fallback failed: "+err.Error())
		}
	}
	return plan, events, nil
}

func planEvidenceWithLLM(ctx context.Context, client askprovider.Client, cfg askconfig.EffectiveSettings, prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary, logger askLogger) (askcontract.EvidencePlan, error) {
	systemPrompt := evidencePlanningSystemPrompt()
	userPrompt := evidencePlanningUserPrompt(prompt, decision, workspace)
	logger.prompt("evidence-plan", systemPrompt, userPrompt)
	resp, err := client.Generate(ctx, askprovider.Request{
		Kind:         "evidence-plan",
		Provider:     cfg.Provider,
		Model:        cfg.Model,
		APIKey:       cfg.APIKey,
		OAuthToken:   cfg.OAuthToken,
		AccountID:    cfg.AccountID,
		Endpoint:     cfg.Endpoint,
		SystemPrompt: systemPrompt,
		Prompt:       userPrompt,
		MaxRetries:   providerRetryCount("evidence-plan"),
		Timeout:      askRequestTimeout("evidence-plan", 1, systemPrompt, userPrompt),
	})
	if err != nil {
		return askcontract.EvidencePlan{}, err
	}
	logger.response("evidence-plan", resp.Content)
	return askcontract.ParseEvidencePlan(resp.Content)
}

func mergeEvidencePlans(base askcontract.EvidencePlan, extra askcontract.EvidencePlan) askcontract.EvidencePlan {
	out := base
	if evidenceDecisionRank(extra.Decision) > evidenceDecisionRank(out.Decision) {
		out.Decision = extra.Decision
	}
	if strings.TrimSpace(extra.Reason) != "" {
		if strings.TrimSpace(out.Reason) == "" {
			out.Reason = strings.TrimSpace(extra.Reason)
		} else if !strings.Contains(strings.ToLower(out.Reason), strings.ToLower(strings.TrimSpace(extra.Reason))) {
			out.Reason = out.Reason + "; " + strings.TrimSpace(extra.Reason)
		}
	}
	out.FreshnessSensitive = out.FreshnessSensitive || extra.FreshnessSensitive
	out.InstallEvidence = out.InstallEvidence || extra.InstallEvidence
	out.CompatibilityEvidence = out.CompatibilityEvidence || extra.CompatibilityEvidence
	out.TroubleshootingEvidence = out.TroubleshootingEvidence || extra.TroubleshootingEvidence
	seen := map[string]bool{}
	merged := make([]askcontract.EvidenceEntity, 0, len(out.Entities)+len(extra.Entities))
	for _, entity := range append(append([]askcontract.EvidenceEntity{}, out.Entities...), extra.Entities...) {
		name := strings.TrimSpace(entity.Name)
		kind := strings.TrimSpace(entity.Kind)
		if name == "" {
			continue
		}
		key := strings.ToLower(name) + "::" + strings.ToLower(kind)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, askcontract.EvidenceEntity{Name: name, Kind: kind})
	}
	out.Entities = merged
	return out
}

func renderEvidencePlanEvent(plan askcontract.EvidencePlan, source string) string {
	entities := make([]string, 0, len(plan.Entities))
	for _, entity := range plan.Entities {
		if strings.TrimSpace(entity.Name) != "" {
			entities = append(entities, strings.TrimSpace(entity.Name))
		}
	}
	return fmt.Sprintf("evidence-plan: source=%s decision=%s freshness=%t install=%t compatibility=%t troubleshooting=%t entities=%s", strings.TrimSpace(source), strings.TrimSpace(plan.Decision), plan.FreshnessSensitive, plan.InstallEvidence, plan.CompatibilityEvidence, plan.TroubleshootingEvidence, strings.Join(entities, ","))
}

func evidenceDecisionRank(value string) int {
	switch strings.TrimSpace(value) {
	case "required":
		return 3
	case "optional":
		return 2
	case "unnecessary":
		return 1
	default:
		return 0
	}
}

func evidencePlanningSystemPrompt() string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask evidence planner. Return strict JSON only.\n")
	b.WriteString("Decide whether external upstream documentation evidence is required before answering or authoring.\n")
	b.WriteString("Use `required` when the request depends on fresh upstream facts such as versions, install steps, compatibility, prerequisites, or troubleshooting.\n")
	b.WriteString("Use `optional` when external docs may help but local deck context is still primary.\n")
	b.WriteString("Use `unnecessary` when the request is grounded in local deck files or workspace structure.\n")
	b.WriteString("JSON shape: {\"decision\":string,\"reason\":string,\"freshnessSensitive\":boolean,\"installEvidence\":boolean,\"compatibilityEvidence\":boolean,\"troubleshootingEvidence\":boolean,\"entities\":[{\"name\":string,\"kind\":string}]}.\n")
	b.WriteString("Keep entities short and name the upstream product, library, project, or technology to look up.\n")
	return b.String()
}

func evidencePlanningUserPrompt(prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) string {
	b := &strings.Builder{}
	b.WriteString("Route: ")
	b.WriteString(strings.TrimSpace(string(decision.Route)))
	b.WriteString("\nHas workflow tree: ")
	if workspace.HasWorkflowTree {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString("\nTarget kind: ")
	b.WriteString(strings.TrimSpace(decision.Target.Kind))
	b.WriteString("\nUser request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func requiredExternalEvidenceFailure(plan askcontract.EvidencePlan, chunks []askretrieve.Chunk, events []string) string {
	if strings.TrimSpace(plan.Decision) != "required" || len(chunks) > 0 {
		return ""
	}
	reasons := make([]string, 0, 2)
	for _, event := range events {
		trimmed := strings.TrimSpace(event)
		if trimmed == "" || !strings.HasPrefix(trimmed, "mcp:") {
			continue
		}
		if strings.Contains(trimmed, "call ") || strings.Contains(trimmed, "initialize failed") || strings.Contains(trimmed, "start failed") || strings.Contains(trimmed, "list tools failed") || strings.Contains(trimmed, "unknown built-in provider") || strings.Contains(trimmed, "disabled for default local pipeline") {
			reasons = append(reasons, trimmed)
		}
		if len(reasons) == 2 {
			break
		}
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "mcp evidence retrieval returned no usable external evidence")
	}
	return strings.Join(reasons, "; ")
}

func externalEvidenceFailureChunk(message string) askretrieve.Chunk {
	message = strings.TrimSpace(message)
	content := "External evidence status:\n- required upstream evidence could not be fetched\n- explain this limitation explicitly and avoid guessing fresh install/version/troubleshooting facts\n- detail: " + message
	return askretrieve.Chunk{ID: "external-evidence-status", Source: "external-evidence", Label: "required-evidence-status", Topic: askcontext.TopicExternalEvidence, Content: content, Score: 85}
}

func externalEvidenceWarningEvents(chunks []askretrieve.Chunk) []string {
	events := make([]string, 0)
	for _, chunk := range chunks {
		if chunk.Evidence == nil {
			continue
		}
		parts := make([]string, 0, 3)
		if len(chunk.Evidence.ArtifactKinds) > 0 {
			parts = append(parts, "artifactKinds="+strings.Join(chunk.Evidence.ArtifactKinds, ","))
		}
		if len(chunk.Evidence.InstallHints) > 0 {
			parts = append(parts, "installHints="+strconv.Itoa(len(chunk.Evidence.InstallHints)))
		}
		if len(chunk.Evidence.OfflineHints) > 0 {
			parts = append(parts, "offlineHints="+strconv.Itoa(len(chunk.Evidence.OfflineHints)))
		}
		if len(parts) == 0 {
			continue
		}
		label := strings.TrimSpace(chunk.Label)
		if label == "" {
			label = chunk.ID
		}
		events = append(events, "evidence-warning: external summaries are advisory only; "+label+" "+strings.Join(parts, " "))
	}
	return events
}
