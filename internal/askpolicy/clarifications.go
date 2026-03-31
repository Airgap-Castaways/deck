package askpolicy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askauthoring"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func clarificationCandidatesFromRequirements(prompt string, req ScenarioRequirements, decision askintent.Decision, workspace askretrieve.WorkspaceSummary, facts askauthoring.Facts) []askcontract.PlanClarification {
	items := append([]askcontract.PlanClarification(nil), facts.Clarifications...)
	items = append(items, targetClarificationsFromRequirements(prompt, req, decision, workspace)...)
	items = append(items, refineCompanionClarifications(prompt, decision, workspace)...)
	items = append(items, runtimeClarifications(prompt, req, decision)...)
	items = append(items, escapeHatchClarifications(prompt, req, decision)...)
	return items
}

func refineCompanionClarifications(prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) []askcontract.PlanClarification {
	if decision.Route != askintent.RouteRefine {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(prompt))
	explicitPaths := askintent.ExtractWorkflowPaths(prompt)
	items := []askcontract.PlanClarification{}
	if mentionsVarsRefine(lower) && !containsString(explicitPaths, "workflows/vars.yaml") {
		items = append(items, askcontract.PlanClarification{
			ID:                 "refine.companionVars",
			Question:           "This refine request mentions vars or repeated values, but it does not explicitly allow workflows/vars.yaml to change. Should the refactor be allowed to edit workflows/vars.yaml too?",
			Kind:               "enum",
			Reason:             "Refine keeps anchor files stable and only edits companion files when they are explicitly approved.",
			Decision:           "scope",
			Options:            []string{"yes", "no"},
			RecommendedDefault: "yes",
			BlocksGeneration:   true,
			Affects:            []string{"authoringBrief.allowedCompanionPaths", "authoringBrief.targetPaths"},
		})
	}
	if mentionsComponentRefine(lower) && !hasComponentPath(explicitPaths) {
		suggested := suggestedComponentPath(prompt, explicitPaths, workspace)
		items = append(items, askcontract.PlanClarification{
			ID:                 "refine.componentPath",
			Question:           "This refine request suggests extracting or updating a reusable component, but the companion component path is not explicit. Which component path should the plan allow?",
			Kind:               "path",
			Reason:             "Refine may create or update a component only when the allowed companion path is explicit.",
			Decision:           "scope",
			Options:            []string{suggested, "none"},
			RecommendedDefault: suggested,
			BlocksGeneration:   true,
			Affects:            []string{"authoringBrief.allowedCompanionPaths", "authoringBrief.targetPaths", "files"},
		})
	}
	return items
}

func runtimeClarifications(prompt string, req ScenarioRequirements, decision askintent.Decision) []askcontract.PlanClarification {
	if decision.Route != askintent.RouteDraft && decision.Route != askintent.RouteRefine {
		return nil
	}
	if !needsRuntimePlatformClarification(prompt, req) {
		return nil
	}
	return []askcontract.PlanClarification{{
		ID:                 "runtime.platformFamily",
		Question:           "This request depends on distro-specific package or repository behavior, but the target platform family is not explicit. Which platform family should the plan target?",
		Kind:               "enum",
		Reason:             "Typed package and repository steps depend on distro-family-specific schema and offline bundle layout.",
		Decision:           "runtime",
		Options:            []string{"rhel", "debian", "custom"},
		RecommendedDefault: "rhel",
		BlocksGeneration:   true,
		Affects:            []string{"authoringBrief.platformFamily", "validationChecklist"},
	}}
}

func escapeHatchClarifications(prompt string, req ScenarioRequirements, decision askintent.Decision) []askcontract.PlanClarification {
	if decision.Route != askintent.RouteDraft && decision.Route != askintent.RouteRefine {
		return nil
	}
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if !mentionsEscapeHatch(lower) {
		return nil
	}
	options := askcontext.StepGuidanceOptions{ModeIntent: inferModeIntent(req), Topology: inferTopology(req), RequiredCapabilities: inferRequiredCapabilities(req)}
	if len(askcontext.StrongTypedAlternativesWithOptions(prompt, options)) > 0 {
		return nil
	}
	return []askcontract.PlanClarification{{
		ID:                 "coverage.escapeHatch",
		Question:           "This request looks like a custom shell or free-form authoring task. Should ask stay inside typed workflow coverage or continue in experimental free-form mode?",
		Kind:               "enum",
		Reason:             "Free-form authoring is a migration fallback and should be selected explicitly when typed coverage is weak.",
		Decision:           "coverage",
		Options:            []string{"typed-only", "experimental-freeform"},
		RecommendedDefault: "typed-only",
		BlocksGeneration:   true,
		Affects:            []string{"authoringBrief.escapeHatchMode"},
	}}
}

func coverageBoundaryBlockers(prompt string, req ScenarioRequirements, decision askintent.Decision) []string {
	if decision.Route != askintent.RouteDraft && decision.Route != askintent.RouteRefine {
		return nil
	}
	unsupported, supportedKinds := unsupportedCapabilities(req)
	if len(unsupported) == 0 {
		return nil
	}
	files := strings.Join(askcontext.AllowedGeneratedPathPatterns(), ", ")
	kinds := strings.Join(supportedKinds, ", ")
	if strings.TrimSpace(kinds) == "" {
		kinds = "none"
	}
	return []string{fmt.Sprintf("unsupported authoring coverage: missing capability support for %s. Supported output files: %s. Current typed coverage includes step kinds: %s. Add more detail only if the request can be reframed inside that coverage; otherwise the request is outside supported authoring coverage.", strings.Join(unsupported, ", "), files, kinds)}
}

func unsupportedCapabilities(req ScenarioRequirements) ([]string, []string) {
	capabilityKinds := map[string][]string{}
	for _, step := range askcontext.Current().StepKinds {
		for _, capability := range step.Capabilities {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				continue
			}
			capabilityKinds[capability] = append(capabilityKinds[capability], step.Kind)
		}
	}
	required := inferRequiredCapabilities(req)
	unsupported := []string{}
	supportedKinds := []string{}
	for _, capability := range required {
		kinds := dedupeStrings(capabilityKinds[strings.TrimSpace(capability)])
		if len(kinds) == 0 {
			unsupported = append(unsupported, capability)
			continue
		}
		supportedKinds = append(supportedKinds, kinds...)
	}
	return dedupeStrings(unsupported), dedupeStrings(supportedKinds)
}

func dedupePlanClarifications(items []askcontract.PlanClarification) []askcontract.PlanClarification {
	byID := map[string]askcontract.PlanClarification{}
	order := []string{}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		base, ok := byID[id]
		if !ok {
			base = askcontract.PlanClarification{ID: id}
			order = append(order, id)
		}
		base = mergeClarification(base, item)
		byID[id] = base
	}
	out := make([]askcontract.PlanClarification, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func mergeClarification(base askcontract.PlanClarification, item askcontract.PlanClarification) askcontract.PlanClarification {
	if base.ID == "" {
		base.ID = strings.TrimSpace(item.ID)
	}
	if strings.TrimSpace(item.Question) != "" {
		base.Question = strings.TrimSpace(item.Question)
	}
	if strings.TrimSpace(item.Kind) != "" {
		base.Kind = strings.TrimSpace(item.Kind)
	}
	if strings.TrimSpace(item.Reason) != "" {
		base.Reason = strings.TrimSpace(item.Reason)
	}
	if strings.TrimSpace(item.Decision) != "" {
		base.Decision = strings.TrimSpace(item.Decision)
	}
	if len(item.Options) > 0 {
		base.Options = dedupeStrings(item.Options)
	}
	if strings.TrimSpace(item.RecommendedDefault) != "" {
		base.RecommendedDefault = strings.TrimSpace(item.RecommendedDefault)
	}
	if strings.TrimSpace(item.Answer) != "" {
		base.Answer = strings.TrimSpace(item.Answer)
	}
	if item.BlocksGeneration {
		base.BlocksGeneration = true
	}
	if len(item.Affects) > 0 {
		base.Affects = dedupeStrings(append(base.Affects, item.Affects...))
	}
	return base
}

func mentionsVarsRefine(lower string) bool {
	return strings.Contains(lower, "vars") || strings.Contains(lower, "variable") || strings.Contains(lower, "variables") || strings.Contains(lower, "repeated value") || strings.Contains(lower, "hoist")
}

func mentionsComponentRefine(lower string) bool {
	return strings.Contains(lower, "component") || strings.Contains(lower, "extract component") || strings.Contains(lower, "reusable fragment") || strings.Contains(lower, "shared fragment")
}

func mentionsEscapeHatch(lower string) bool {
	tokens := []string{"custom shell", "shell script", "bash", "script", "raw yaml", "free-form", "freeform", "manual command", "arbitrary command"}
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func needsRuntimePlatformClarification(prompt string, req ScenarioRequirements) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if containsPlatformToken(lower) {
		return false
	}
	if containsString(req.ArtifactKinds, "package") || strings.Contains(lower, "package") || strings.Contains(lower, "repo") || strings.Contains(lower, "repository") || strings.Contains(lower, "kubeadm") || strings.Contains(lower, "containerd") || strings.Contains(lower, "docker") {
		return true
	}
	return false
}

func containsPlatformToken(lower string) bool {
	tokens := []string{"rhel", "rocky", "centos", "alma", "fedora", "debian", "ubuntu", "sles", "opensuse"}
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func hasComponentPath(paths []string) bool {
	for _, path := range paths {
		if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(path)), "workflows/components/") {
			return true
		}
	}
	return false
}

func suggestedComponentPath(prompt string, explicitPaths []string, workspace askretrieve.WorkspaceSummary) string {
	for _, path := range explicitPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if strings.HasPrefix(clean, "workflows/scenarios/") || clean == "workflows/prepare.yaml" {
			return "workflows/components/" + strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean)) + "-shared.yaml"
		}
	}
	for _, file := range workspace.Files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		if strings.HasPrefix(clean, "workflows/scenarios/") || clean == "workflows/prepare.yaml" {
			return "workflows/components/" + strings.TrimSuffix(filepath.Base(clean), filepath.Ext(clean)) + "-shared.yaml"
		}
	}
	base := "refined-shared"
	if strings.TrimSpace(prompt) != "" {
		fields := strings.Fields(strings.ToLower(prompt))
		if len(fields) > 0 {
			base = fields[0]
		}
	}
	base = strings.Trim(base, "-_")
	if base == "" {
		base = "refined-shared"
	}
	return "workflows/components/" + base + ".yaml"
}
