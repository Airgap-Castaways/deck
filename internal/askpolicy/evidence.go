package askpolicy

import (
	"regexp"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

var versionPattern = regexp.MustCompile(`(?i)\bv?\d+\.\d+(?:\.\d+)?\b`)
var repoEntityPattern = regexp.MustCompile(`(?i)\b(?:github\.com|gitlab\.com|golang\.org)/[^\s)]+`)
var tokenPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+._/-]*`)

func BuildEvidencePlan(prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) askcontract.EvidencePlan {
	trimmed := strings.TrimSpace(prompt)
	lower := strings.ToLower(trimmed)
	plan := askcontract.EvidencePlan{Decision: "unnecessary", Reason: "request does not require external documentation evidence"}
	if trimmed == "" {
		return plan
	}
	if looksLikeLocalDeckPrompt(lower, decision, workspace) && !mentionsExternalEvidenceNeed(lower) {
		plan.Reason = "request is grounded in the local deck workspace"
		return plan
	}
	plan.FreshnessSensitive = promptNeedsFreshnessEvidence(lower)
	plan.InstallEvidence = promptNeedsInstallEvidence(lower)
	plan.CompatibilityEvidence = promptNeedsCompatibilityEvidence(lower)
	plan.TroubleshootingEvidence = promptNeedsTroubleshootingEvidence(lower)
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

func ShouldUseLLMEvidencePlanner(plan askcontract.EvidencePlan, prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) bool {
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

func promptNeedsFreshnessEvidence(lower string) bool {
	if versionPattern.MatchString(lower) {
		return true
	}
	for _, token := range []string{" latest ", " current ", " newest ", " recent ", " release ", " upgrade ", " version "} {
		if strings.Contains(" "+lower+" ", token) {
			return true
		}
	}
	return false
}

func promptNeedsInstallEvidence(lower string) bool {
	for _, token := range []string{"install", "setup", "set up", "bootstrap", "upgrade", "deploy"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func promptNeedsCompatibilityEvidence(lower string) bool {
	for _, token := range []string{"compatible", "compatibility", "requirement", "requirements", "prerequisite", "prerequisites", "supported on", "support matrix"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func promptNeedsTroubleshootingEvidence(lower string) bool {
	for _, token := range []string{"troubleshoot", "debug", "not working", "cannot", "can't", "unable", "failing", "fails", "error", "issue", "problem", "crash"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func mentionsExternalEvidenceNeed(lower string) bool {
	return promptNeedsFreshnessEvidence(lower) || promptNeedsInstallEvidence(lower) || promptNeedsCompatibilityEvidence(lower) || promptNeedsTroubleshootingEvidence(lower)
}

func looksLikeLocalDeckPrompt(lower string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) bool {
	if strings.Contains(lower, "workflows/") || strings.Contains(lower, ".yaml") || strings.Contains(lower, "this workspace") || strings.Contains(lower, "current workspace") || strings.Contains(lower, "deck ask") {
		return true
	}
	if decision.Route == askintent.RouteReview && !mentionsExternalEvidenceNeed(lower) {
		return true
	}
	if workspace.HasWorkflowTree && strings.Contains(lower, "scenario") && !mentionsExternalEvidenceNeed(lower) {
		return true
	}
	return false
}

func extractEvidenceEntities(prompt string) []askcontract.EvidenceEntity {
	entities := make([]askcontract.EvidenceEntity, 0)
	seen := map[string]bool{}
	add := func(name string, kind string) {
		name = strings.TrimSpace(strings.Trim(name, `"'.,:;()[]{} `))
		if name == "" || looksGenericEntityName(name) {
			return
		}
		key := strings.ToLower(name) + "::" + strings.ToLower(strings.TrimSpace(kind))
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, askcontract.EvidenceEntity{Name: name, Kind: kind})
	}
	for _, match := range repoEntityPattern.FindAllString(prompt, -1) {
		add(match, "library")
	}
	for _, match := range versionAttachedEntities(prompt) {
		add(match, "technology")
	}
	for _, match := range cueDrivenEntities(prompt) {
		add(match, "technology")
	}
	return entities
}

func versionAttachedEntities(prompt string) []string {
	pattern := regexp.MustCompile(`(?i)\b([A-Za-z][A-Za-z0-9+._/-]*)\s+v?\d+\.\d+(?:\.\d+)?\b`)
	matches := pattern.FindAllStringSubmatch(prompt, -1)
	results := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		}
	}
	return results
}

func cueDrivenEntities(prompt string) []string {
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
			if clean == "" || versionPattern.MatchString(lowerClean) || cueStopword(lowerClean) {
				break
			}
			if strings.EqualFold(lowerClean, "and") || strings.EqualFold(lowerClean, "or") {
				break
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
	for _, token := range tokenPattern.FindAllString(prompt, -1) {
		if token != strings.ToLower(token) && !looksGenericEntityName(token) {
			results = append(results, token)
		}
	}
	return results
}

func cueStopword(value string) bool {
	switch value {
	case "for", "on", "with", "using", "into", "from", "in", "to", "the", "a", "an", "and", "or", "version", "release", "linux", "ubuntu", "debian", "rhel", "rocky", "almalinux", "fedora", "macos", "windows":
		return true
	default:
		return false
	}
}

func looksGenericEntityName(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return true
	}
	switch lower {
	case "latest", "current", "version", "release", "requirements", "prerequisites", "install", "setup", "upgrade", "debug", "troubleshoot", "issue", "problem", "project", "application", "tool", "cluster", "workflow", "workspace", "ubuntu", "debian", "rhel", "rocky", "almalinux", "fedora", "linux", "macos", "windows":
		return true
	default:
		return false
	}
}
