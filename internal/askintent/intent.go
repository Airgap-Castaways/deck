package askintent

import "strings"

type Route string

const (
	RouteClarify  Route = "clarify"
	RouteQuestion Route = "question"
	RouteExplain  Route = "explain"
	RouteReview   Route = "review"
	RouteRefine   Route = "refine"
	RouteDraft    Route = "draft"
)

type LLMPolicy string

const (
	LLMDisallowed LLMPolicy = "disallowed"
	LLMOptional   LLMPolicy = "optional"
	LLMRequired   LLMPolicy = "required"
)

type Input struct {
	Prompt          string
	WriteFlag       bool
	CreateFlag      bool
	EditFlag        bool
	ReviewFlag      bool
	HasWorkflowTree bool
	HasPrepare      bool
	HasApply        bool
}

type Decision struct {
	Route           Route
	Confidence      float64
	Reason          string
	Target          Target
	AllowGeneration bool
	AllowRetry      bool
	RequiresLint    bool
	LLMPolicy       LLMPolicy
}

type Target struct {
	Kind string
	Path string
	Name string
}

func Classify(input Input) Decision {
	if input.ReviewFlag {
		return Decision{
			Route:           RouteReview,
			Confidence:      1.0,
			Reason:          "explicit --review flag",
			Target:          Target{Kind: "workspace"},
			AllowGeneration: false,
			AllowRetry:      false,
			RequiresLint:    false,
			LLMPolicy:       LLMOptional,
		}
	}
	if input.CreateFlag {
		return Decision{Route: RouteDraft, Confidence: 1.0, Reason: "explicit --create flag", Target: inferTarget(strings.TrimSpace(strings.ToLower(input.Prompt))), AllowGeneration: true, AllowRetry: true, RequiresLint: true, LLMPolicy: LLMRequired}
	}
	if input.EditFlag {
		return Decision{Route: RouteRefine, Confidence: 1.0, Reason: "explicit --edit flag", Target: inferTarget(strings.TrimSpace(strings.ToLower(input.Prompt))), AllowGeneration: true, AllowRetry: true, RequiresLint: true, LLMPolicy: LLMRequired}
	}
	prompt := strings.TrimSpace(strings.ToLower(input.Prompt))
	if prompt == "" {
		return clarify("empty prompt")
	}
	words := strings.Fields(prompt)
	if len(words) <= 2 && len(prompt) <= 12 {
		return clarify("low-information prompt")
	}
	return Decision{Route: RouteClarify, Confidence: 0.0, Reason: "classifier required", Target: inferTarget(prompt), AllowGeneration: false, AllowRetry: false, RequiresLint: false, LLMPolicy: LLMOptional}
}

func IsHardOverride(decision Decision) bool {
	switch strings.TrimSpace(decision.Reason) {
	case "explicit --review flag", "explicit --create flag", "explicit --edit flag", "empty prompt", "low-information prompt":
		return true
	default:
		return false
	}
}

func clarify(reason string) Decision {
	return Decision{Route: RouteClarify, Confidence: 0.95, Reason: reason, Target: Target{Kind: "unknown"}, AllowGeneration: false, AllowRetry: false, RequiresLint: false, LLMPolicy: LLMDisallowed}
}

func ParseRoute(value string) Route {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RouteClarify):
		return RouteClarify
	case string(RouteQuestion):
		return RouteQuestion
	case string(RouteExplain):
		return RouteExplain
	case string(RouteReview):
		return RouteReview
	case string(RouteRefine):
		return RouteRefine
	case string(RouteDraft):
		return RouteDraft
	default:
		return RouteClarify
	}
}

func inferTarget(prompt string) Target {
	if strings.Contains(prompt, "prepare and apply") || (strings.Contains(prompt, "prepare") && strings.Contains(prompt, "apply")) {
		return Target{Kind: "workspace"}
	}
	if strings.Contains(prompt, "apply") {
		return Target{Kind: "scenario", Name: "apply", Path: "workflows/scenarios/apply.yaml"}
	}
	if strings.Contains(prompt, "prepare") {
		return Target{Kind: "scenario", Name: "prepare", Path: "workflows/prepare.yaml"}
	}
	if strings.Contains(prompt, "component") {
		return Target{Kind: "component"}
	}
	if strings.Contains(prompt, "vars") {
		return Target{Kind: "vars", Path: "workflows/vars.yaml"}
	}
	return Target{Kind: "workspace"}
}
