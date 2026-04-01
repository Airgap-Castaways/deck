package askpattern

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

type Pattern struct {
	Name        string
	Description string
	Applies     func(req askpolicy.ScenarioRequirements, workspace askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) bool
}

func Registry() []Pattern {
	return []Pattern{
		{Name: "preserve-existing", Description: "preserve existing planned files during refine", Applies: func(req askpolicy.ScenarioRequirements, workspace askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) bool {
			return req.AcceptanceLevel == "refine" || workspace.HasWorkflowTree && strings.EqualFold(plan.AuthoringBrief.CompletenessTarget, "refine")
		}},
		{Name: "role-aware-apply", Description: "role-aware apply flow with bootstrap, join, and final verification", Applies: func(req askpolicy.ScenarioRequirements, _ askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) bool {
			return strings.EqualFold(plan.AuthoringBrief.Topology, "multi-node") || strings.EqualFold(plan.AuthoringBrief.Topology, "ha") || plan.AuthoringBrief.NodeCount > 1 || strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) != ""
		}},
		{Name: "cluster-bootstrap", Description: "cluster bootstrap starter", Applies: func(req askpolicy.ScenarioRequirements, _ askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) bool {
			return contains(plan.AuthoringBrief.RequiredCapabilities, "kubeadm-bootstrap") || contains(req.ScenarioIntent, "kubeadm")
		}},
		{Name: "artifact-staging", Description: "prepare and apply artifact staging", Applies: func(req askpolicy.ScenarioRequirements, workspace askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) bool {
			return req.NeedsPrepare || len(req.ArtifactKinds) > 0 || workspace.HasPrepare || strings.EqualFold(plan.AuthoringBrief.ModeIntent, "prepare+apply")
		}},
		{Name: "apply-only", Description: "apply only local change", Applies: func(_ askpolicy.ScenarioRequirements, _ askretrieve.WorkspaceSummary, _ askcontract.PlanResponse) bool {
			return true
		}},
	}
}

func Compose(req askpolicy.ScenarioRequirements, workspace askretrieve.WorkspaceSummary, plan askcontract.PlanResponse) []Pattern {
	out := []Pattern{}
	for _, pattern := range Registry() {
		if pattern.Applies(req, workspace, plan) {
			out = append(out, pattern)
		}
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
