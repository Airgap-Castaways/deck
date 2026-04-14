package askcli

import (
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func applyPlanAnswers(plan askcontract.PlanResponse, answers []string) (askcontract.PlanResponse, error) {
	if len(answers) == 0 {
		return plan, nil
	}
	plan = adaptPlanBoundary(plan)
	byID := map[string]int{}
	for i, item := range plan.Clarifications {
		byID[strings.TrimSpace(item.ID)] = i
	}
	for _, raw := range answers {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 {
			return plan, fmt.Errorf("invalid --answer %q: expected key=value", raw)
		}
		id := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		id = strings.TrimPrefix(id, "clarification.")
		id = adaptClarificationID(askcontract.PlanClarification{ID: id})
		value = adaptClarificationAnswer(id, value)
		idx, ok := byID[id]
		if !ok {
			return plan, fmt.Errorf("unknown clarification id %q", id)
		}
		if value == "" {
			return plan, fmt.Errorf("clarification %q requires a non-empty answer", id)
		}
		if len(plan.Clarifications[idx].Options) > 0 && strings.TrimSpace(plan.Clarifications[idx].Kind) != "path" && !containsAnswerOption(plan.Clarifications[idx].Options, value) && value != "custom" {
			return plan, fmt.Errorf("clarification %q answer %q must match one of: %s", id, value, strings.Join(plan.Clarifications[idx].Options, ", "))
		}
		plan.Clarifications[idx].Answer = value
	}
	decision := askintent.Decision{Route: planRoute(plan), Target: planTarget(plan, askintent.Target{Kind: plan.AuthoringBrief.TargetScope})}
	plan = adaptPlanBoundary(plan)
	return askpolicy.NormalizePlan(plan, plan.Request, askretrieve.RetrievalResult{}, askretrieve.WorkspaceSummary{}, decision), nil
}

func planRoute(plan askcontract.PlanResponse) askintent.Route {
	route := askintent.ParseRoute(plan.Intent)
	if route == askintent.RouteDraft || route == askintent.RouteRefine {
		return route
	}
	if strings.EqualFold(strings.TrimSpace(plan.AuthoringBrief.CompletenessTarget), "refine") {
		return askintent.RouteRefine
	}
	for _, file := range plan.Files {
		action := strings.ToLower(strings.TrimSpace(file.Action))
		switch action {
		case "update", "edit", "delete", "preserve":
			return askintent.RouteRefine
		}
	}
	return askintent.RouteDraft
}

func containsAnswerOption(options []string, value string) bool {
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}
