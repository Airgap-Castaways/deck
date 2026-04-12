package askcli

import (
	"regexp"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

var (
	evidenceVersionPattern               = regexp.MustCompile(`(?i)\bv?\d+\.\d+(?:\.\d+)?\b`)
	evidenceRepoEntityPattern            = regexp.MustCompile(`(?i)\b(?:github\.com|gitlab\.com|golang\.org)/[^\s)]+`)
	evidenceTokenPattern                 = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+._/-]*`)
	evidenceVersionAttachedEntityPattern = regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9+._/-]*)\s+v?\d+\.\d+(?:\.\d+)?\b`)
	evidencePathLikePattern              = regexp.MustCompile(`(?i)(?:\b[a-z0-9_.-]+/)+[a-z0-9_.-]+\b|\b[a-z0-9_.-]+\.ya?ml\b`)
	evidenceInstallCuePattern            = regexp.MustCompile(`(?i)\b(?:install|setup|set up|bootstrap|upgrade|deploy)\b`)
	evidenceCompatibilityCuePattern      = regexp.MustCompile(`(?i)\b(?:compatible|compatibility|requirement|requirements|prerequisite|prerequisites)\b|supported on|support matrix`)
	evidenceTroubleshootingCuePattern    = regexp.MustCompile(`(?i)\b(?:troubleshoot|debug|failing|fails|error|issue|problem|crash)\b|not working|cannot|can't|unable`)
)

func buildHeuristicEvidencePlan(prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) askcontract.EvidencePlan {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	cuePrompt := evidenceCueRelevantPrompt(lower)
	plan := askcontract.EvidencePlan{Decision: "unnecessary", Reason: "request does not require external documentation evidence"}
	if trimmed == "" {
		return plan
	}
	if decision.Route == askintent.RouteRefine {
		plan.Reason = "refine requests should stay grounded in the local deck workspace"
		return plan
	}
	if looksLikeLocalDeckPrompt(lower, decision, workspace) && !evidenceMentionsExternalNeed(cuePrompt) {
		plan.Reason = "request is grounded in the local deck workspace"
		return plan
	}
	plan.FreshnessSensitive = evidenceNeedsFreshness(cuePrompt)
	plan.InstallEvidence = evidenceNeedsInstall(cuePrompt)
	plan.CompatibilityEvidence = evidenceNeedsCompatibility(cuePrompt)
	plan.TroubleshootingEvidence = evidenceNeedsTroubleshooting(cuePrompt)
	plan.Entities = extractEvidenceEntities(trimmed)
	if !plan.FreshnessSensitive && !plan.InstallEvidence && !plan.CompatibilityEvidence && !plan.TroubleshootingEvidence {
		if len(plan.Entities) > 0 && !looksLikeLocalDeckPrompt(lower, decision, workspace) {
			plan.Decision = "optional"
			plan.Reason = "request mentions external technologies but is not strongly freshness-sensitive"
		}
		return plan
	}
	plan.Decision = "required"
	reasons := make([]string, 0, 4)
	if plan.FreshnessSensitive {
		reasons = append(reasons, "freshness-sensitive versions or release details are requested")
	}
	if plan.InstallEvidence {
		reasons = append(reasons, "install or upgrade guidance is requested")
	}
	if plan.CompatibilityEvidence {
		reasons = append(reasons, "compatibility or prerequisite guidance is requested")
	}
	if plan.TroubleshootingEvidence {
		reasons = append(reasons, "troubleshooting guidance is requested")
	}
	plan.Reason = strings.Join(reasons, "; ")
	return plan
}

func shouldUseLLMEvidencePlanner(plan askcontract.EvidencePlan, prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) bool {
	if strings.TrimSpace(plan.Decision) == "unnecessary" {
		return false
	}
	if len(plan.Entities) > 0 {
		return false
	}
	if looksLikeLocalDeckPrompt(strings.ToLower(strings.TrimSpace(prompt)), decision, workspace) {
		return false
	}
	return plan.FreshnessSensitive || plan.InstallEvidence || plan.CompatibilityEvidence || plan.TroubleshootingEvidence
}

func evidenceNeedsFreshness(lower string) bool {
	if evidenceVersionPattern.MatchString(lower) {
		return true
	}
	for _, token := range []string{" latest ", " current ", " newest ", " recent ", " release ", " upgrade ", " version "} {
		if strings.Contains(" "+lower+" ", token) {
			return true
		}
	}
	return false
}

func evidenceNeedsInstall(lower string) bool {
	return evidenceInstallCuePattern.MatchString(lower)
}

func evidenceNeedsCompatibility(lower string) bool {
	return evidenceCompatibilityCuePattern.MatchString(lower)
}

func evidenceNeedsTroubleshooting(lower string) bool {
	return evidenceTroubleshootingCuePattern.MatchString(lower)
}

func evidenceMentionsExternalNeed(lower string) bool {
	return evidenceNeedsFreshness(lower) || evidenceNeedsInstall(lower) || evidenceNeedsCompatibility(lower) || evidenceNeedsTroubleshooting(lower)
}

func looksLikeLocalDeckPrompt(lower string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) bool {
	if strings.Contains(lower, "workflows/") || strings.Contains(lower, "internal/") || strings.Contains(lower, "cmd/") || strings.Contains(lower, "docs/") || strings.Contains(lower, ".yaml") || strings.Contains(lower, "this workspace") || strings.Contains(lower, "current workspace") || strings.Contains(lower, "deck ask") {
		return true
	}
	targetPath := strings.ToLower(strings.TrimSpace(decision.Target.Path))
	if strings.HasPrefix(targetPath, "workflows/") || strings.HasPrefix(targetPath, "internal/") || strings.HasPrefix(targetPath, "cmd/") || strings.HasPrefix(targetPath, "docs/") {
		return true
	}
	if decision.Route == askintent.RouteRefine {
		return true
	}
	if decision.Route == askintent.RouteReview && !evidenceMentionsExternalNeed(lower) {
		return true
	}
	if workspace.HasWorkflowTree && strings.Contains(lower, "scenario") && !evidenceMentionsExternalNeed(lower) {
		return true
	}
	return false
}

func evidenceCueRelevantPrompt(lower string) string {
	cleaned := evidencePathLikePattern.ReplaceAllString(lower, " ")
	cleaned = strings.ReplaceAll(cleaned, "_", " ")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	return cleaned
}

func extractEvidenceEntities(prompt string) []askcontract.EvidenceEntity {
	entities := make([]askcontract.EvidenceEntity, 0)
	seen := map[string]bool{}
	add := func(name string, kind string) {
		name = strings.TrimSpace(strings.Trim(name, `"'.,:;()[]{} `))
		name = strings.TrimSpace(strings.TrimSuffix(name, "?"))
		if name == "" || evidenceLooksGenericName(name) {
			return
		}
		if evidenceDigitsOnly(name) {
			return
		}
		if evidenceVersionPattern.MatchString(name) {
			return
		}
		key := strings.ToLower(name) + "::" + strings.ToLower(strings.TrimSpace(kind))
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, askcontract.EvidenceEntity{Name: name, Kind: kind})
	}
	for _, match := range evidenceRepoEntityPattern.FindAllString(prompt, -1) {
		add(match, "library")
	}
	for _, match := range evidenceVersionAttachedEntities(prompt) {
		add(match, "technology")
	}
	for _, match := range evidenceCueDrivenEntities(prompt) {
		add(match, "technology")
	}
	return entities
}

func evidenceVersionAttachedEntities(prompt string) []string {
	matches := evidenceVersionAttachedEntityPattern.FindAllStringSubmatch(prompt, -1)
	results := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		}
	}
	return results
}

func evidenceCueDrivenEntities(prompt string) []string {
	lower := strings.ToLower(prompt)
	results := make([]string, 0)
	for _, cue := range []string{"install", "setup", "set up", "upgrade", "configure", "troubleshoot", "debug", "compatibility", "requirements", "prerequisites", "latest", "current"} {
		idx := strings.Index(lower, cue)
		if idx < 0 {
			continue
		}
		segment := strings.TrimSpace(prompt[idx+len(cue):])
		if segment == "" {
			continue
		}
		parts := strings.Fields(segment)
		candidate := make([]string, 0, 3)
		for _, part := range parts {
			clean := strings.TrimSpace(strings.Trim(part, `"'.,:;()[]{} `))
			lowerClean := strings.ToLower(clean)
			if clean == "" || evidenceVersionPattern.MatchString(lowerClean) {
				break
			}
			if evidenceCueStopword(lowerClean) {
				continue
			}
			if strings.EqualFold(lowerClean, "and") || strings.EqualFold(lowerClean, "or") {
				break
			}
			if evidenceLooksGenericName(clean) {
				continue
			}
			candidate = append(candidate, clean)
			if len(candidate) == 3 {
				break
			}
		}
		if len(candidate) > 0 {
			results = append(results, strings.Join(candidate, " "))
		}
	}
	for _, token := range evidenceTokenPattern.FindAllString(prompt, -1) {
		if token != strings.ToLower(token) && !evidenceLooksGenericName(token) {
			results = append(results, token)
		}
	}
	return results
}

func evidenceCueStopword(value string) bool {
	switch value {
	case "for", "on", "with", "using", "into", "from", "in", "to", "the", "a", "an", "and", "or", "version", "release", "linux", "ubuntu", "debian", "rhel", "rocky", "almalinux", "fedora", "macos", "windows":
		return true
	default:
		return false
	}
}

func evidenceLooksGenericName(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return true
	}
	switch lower {
	case "latest", "current", "version", "release", "requirements", "prerequisites", "install", "setup", "upgrade", "debug", "troubleshoot", "issue", "problem", "project", "application", "tool", "cluster", "workflow", "workspace", "ubuntu", "debian", "rhel", "rocky", "almalinux", "fedora", "linux", "macos", "windows", "explain", "review", "refactor", "create":
		return true
	default:
		return false
	}
}

func evidenceDigitsOnly(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
