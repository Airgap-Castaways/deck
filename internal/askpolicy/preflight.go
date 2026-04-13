package askpolicy

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func BuildAuthoringPreflight(prompt string, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, decision askintent.Decision, base *askcontract.PlanResponse) (askcontract.PlanResponse, ScenarioRequirements) {
	req := BuildScenarioRequirements(prompt, retrieval, workspace, decision)
	var plan askcontract.PlanResponse
	if base != nil {
		plan = *base
		plan.Request = firstNonEmptyString(strings.TrimSpace(plan.Request), strings.TrimSpace(prompt))
		plan.Intent = firstNonEmptyString(strings.TrimSpace(plan.Intent), string(decision.Route))
		req = MergeRequirementsWithPlan(req, plan)
	} else {
		plan = BuildPlanDefaults(req, prompt, decision, workspace)
	}
	plan = NormalizePlan(plan, prompt, retrieval, workspace, decision)
	return plan, req
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
