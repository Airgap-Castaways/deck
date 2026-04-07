package askcontext

import (
	"fmt"
	"sort"
	"strings"
)

type StepGuidanceOptions struct {
	ModeIntent           string
	Topology             string
	RequiredCapabilities []string
}

const (
	confidenceHigh       = "high"
	confidenceMedium     = "medium"
	confidenceLow        = "low"
	candidatePromptLimit = 5
)

type candidateScore struct {
	step       StepKindContext
	score      int
	confidence string
	why        []string
}

func StepKind(kind string) (StepKindContext, bool) {
	for _, step := range Current().StepKinds {
		if step.Kind == strings.TrimSpace(kind) {
			return step, true
		}
	}
	return StepKindContext{}, false
}

func ValidationFixesForError(message string) []string {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return nil
	}
	fixes := make([]string, 0)
	for _, step := range Current().StepKinds {
		for _, hint := range step.ValidationHints {
			needle := strings.ToLower(strings.TrimSpace(hint.ErrorContains))
			if needle == "" || !strings.Contains(message, needle) {
				continue
			}
			if fix := strings.TrimSpace(hint.Fix); fix != "" {
				fixes = append(fixes, fix)
			}
		}
	}
	return dedupeStrings(fixes)
}

func StrongTypedAlternatives(prompt string) []StepKindContext {
	selected := DiscoverCandidateSteps(prompt)
	out := make([]StepKindContext, 0, len(selected))
	for _, item := range selected {
		if item.Step.Kind == "Command" {
			continue
		}
		out = append(out, item.Step)
	}
	return out
}

func StrongTypedAlternativesWithOptions(prompt string, options StepGuidanceOptions) []StepKindContext {
	selected := DiscoverCandidateStepsWithOptions(prompt, options)
	out := make([]StepKindContext, 0, len(selected))
	for _, item := range selected {
		if item.Step.Kind == "Command" {
			continue
		}
		out = append(out, item.Step)
	}
	return out
}

func SelectStepGuidance(prompt string) []SelectedStepGuidance {
	return DiscoverCandidateSteps(prompt)
}

func SelectStepGuidanceWithOptions(prompt string, options StepGuidanceOptions) []SelectedStepGuidance {
	return DiscoverCandidateStepsWithOptions(prompt, options)
}

func DiscoverCandidateSteps(prompt string) []SelectedStepGuidance {
	return DiscoverCandidateStepsWithOptions(prompt, StepGuidanceOptions{})
}

func DiscoverCandidateStepsWithOptions(prompt string, options StepGuidanceOptions) []SelectedStepGuidance {
	manifest := Current()
	lower := strings.ToLower(strings.TrimSpace(prompt))
	capabilities := map[string]bool{}
	for _, capability := range options.RequiredCapabilities {
		capability = strings.ToLower(strings.TrimSpace(capability))
		if capability != "" {
			capabilities[capability] = true
		}
	}
	modeIntent := strings.ToLower(strings.TrimSpace(options.ModeIntent))
	topology := strings.ToLower(strings.TrimSpace(options.Topology))
	scoredKinds := make([]candidateScore, 0, len(manifest.StepKinds))
	for _, step := range manifest.StepKinds {
		score := 0
		why := make([]string, 0, 4)
		if strings.Contains(lower, strings.ToLower(step.Kind)) {
			score += 100
			why = append(why, "request names the step kind")
		}
		if group := strings.ToLower(strings.TrimSpace(step.GroupTitle)); group != "" && strings.Contains(lower, group) {
			score += 20
			why = append(why, fmt.Sprintf("matches %s group", step.GroupTitle))
		} else if group := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(step.Group, "-", " "))); group != "" && strings.Contains(lower, group) {
			score += 18
			if strings.TrimSpace(step.GroupTitle) != "" {
				why = append(why, fmt.Sprintf("matches %s group", step.GroupTitle))
			} else {
				why = append(why, fmt.Sprintf("matches %s group", strings.TrimSpace(step.Group)))
			}
		}
		for _, signal := range step.MatchSignals {
			signal = strings.ToLower(strings.TrimSpace(signal))
			if signal == "" || !strings.Contains(lower, signal) {
				continue
			}
			score += 28
			why = append(why, fmt.Sprintf("matches %q", signal))
		}
		for _, token := range strings.Fields(strings.ToLower(step.WhenToUse)) {
			if len(token) > 4 && strings.Contains(lower, token) {
				score += 4
			}
		}
		for _, anti := range step.AntiSignals {
			anti = strings.ToLower(strings.TrimSpace(anti))
			if anti != "" && strings.Contains(lower, anti) {
				score -= 10
			}
		}
		if strings.Contains(lower, "typed") || strings.Contains(lower, "where possible") {
			if step.Kind == "Command" {
				score -= 40
			} else {
				score += 10
			}
		}
		if step.Kind == "Command" {
			score -= 15
		}
		if (capabilities["prepare-artifacts"] || capabilities["package-staging"] || capabilities["image-staging"]) && step.Kind == "Command" {
			score -= 60
			why = append(why, "deprioritized for typed artifact staging flow")
		}
		if modeIntent == "prepare+apply" && !containsGuidanceString(step.AllowedRoles, "prepare") && (capabilities["prepare-artifacts"] || capabilities["package-staging"] || capabilities["image-staging"]) {
			score -= 45
		}
		applyCapabilityScore(&score, &why, step, capabilities, modeIntent, topology)
		if score > 0 {
			scoredKinds = append(scoredKinds, candidateScore{step: step, score: score, confidence: confidenceForScore(score), why: dedupeStrings(why)})
		}
	}
	sort.Slice(scoredKinds, func(i, j int) bool {
		if scoredKinds[i].score == scoredKinds[j].score {
			return scoredKinds[i].step.Kind < scoredKinds[j].step.Kind
		}
		return scoredKinds[i].score > scoredKinds[j].score
	})
	selected := selectCandidateSteps(scoredKinds, capabilities)
	return selected
}

func RelevantStepKinds(prompt string) []StepKindContext {
	selected := DiscoverCandidateSteps(prompt)
	out := make([]StepKindContext, 0, len(selected))
	for _, item := range selected {
		out = append(out, item.Step)
	}
	return out
}

func RelevantStepKindsWithOptions(prompt string, options StepGuidanceOptions) []StepKindContext {
	selected := DiscoverCandidateStepsWithOptions(prompt, options)
	out := make([]StepKindContext, 0, len(selected))
	for _, item := range selected {
		out = append(out, item.Step)
	}
	return out
}

func confidenceForScore(score int) string {
	switch {
	case score >= 70:
		return confidenceHigh
	case score >= 35:
		return confidenceMedium
	default:
		return confidenceLow
	}
}

func selectCandidateSteps(scoredKinds []candidateScore, capabilities map[string]bool) []SelectedStepGuidance {
	selectedKinds := map[string]bool{}
	out := make([]SelectedStepGuidance, 0, candidatePromptLimit)
	appendCandidate := func(item candidateScore) {
		if selectedKinds[item.step.Kind] {
			return
		}
		selectedKinds[item.step.Kind] = true
		out = append(out, SelectedStepGuidance{Step: item.step, Confidence: item.confidence, Reasons: append([]string(nil), item.why...), WhyRelevant: strings.Join(item.why, "; ")})
	}
	for _, item := range scoredKinds {
		if item.confidence == confidenceHigh {
			appendCandidate(item)
		}
	}
	for capability := range capabilities {
		ensureCapabilityCandidate(&out, selectedKinds, scoredKinds, capabilities, capability)
	}
	for _, item := range scoredKinds {
		if len(out) >= candidatePromptLimit {
			break
		}
		if item.confidence == confidenceMedium {
			appendCandidate(item)
		}
	}
	for _, item := range scoredKinds {
		if len(out) >= candidatePromptLimit {
			break
		}
		if item.confidence == confidenceLow {
			appendCandidate(item)
		}
	}
	return out
}

func ensureCapabilityCandidate(out *[]SelectedStepGuidance, selectedKinds map[string]bool, scoredKinds []candidateScore, capabilities map[string]bool, capability string) {
	if !capabilities[capability] {
		return
	}
	for _, selected := range *out {
		if stepSupportsCapability(selected.Step, capability) {
			return
		}
	}
	for _, item := range scoredKinds {
		if stepSupportsCapability(item.step, capability) {
			if !selectedKinds[item.step.Kind] {
				selectedKinds[item.step.Kind] = true
				*out = append(*out, SelectedStepGuidance{Step: item.step, Confidence: item.confidence, Reasons: append([]string(nil), item.why...), WhyRelevant: strings.Join(item.why, "; ")})
			}
			return
		}
	}
}

func containsGuidanceString(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func applyCapabilityScore(score *int, why *[]string, step StepKindContext, capabilities map[string]bool, modeIntent string, topology string) {
	boost := func(points int, reason string) {
		*score += points
		*why = append(*why, reason)
	}
	for capability := range capabilities {
		if !stepSupportsCapability(step, capability) {
			continue
		}
		points := 24
		reason := "supports " + capability + " capability"
		switch capability {
		case "package-staging", "image-staging":
			points = 35
		case "repository-setup", "cluster-verification":
			points = 30
		case "kubeadm-join":
			points = 40
		case "kubeadm-bootstrap":
			points = 28
		}
		boost(points, reason)
	}
	if capabilities["prepare-artifacts"] && modeIntent == "prepare+apply" {
		if stepSupportsCapability(step, "prepare-artifacts") || stepSupportsCapability(step, "package-staging") || stepSupportsCapability(step, "image-staging") {
			boost(20, "fits prepare stage in prepare+apply workflow")
		}
		if step.Kind == "Command" {
			*score -= 25
		}
	}
	if topology == "multi-node" || topology == "ha" {
		switch {
		case stepSupportsCapability(step, "kubeadm-join"):
			boost(35, "topology requires node join flow")
		case stepSupportsCapability(step, "cluster-verification"):
			boost(18, "topology benefits from cluster verification")
		}
	}
}

func stepSupportsCapability(step StepKindContext, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	return containsGuidanceString(step.Capabilities, capability)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
