package askcli

import (
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func adaptPlanBoundary(plan askcontract.PlanResponse) askcontract.PlanResponse {
	plan.Clarifications = adaptPlanClarifications(plan.Clarifications)
	for i := range plan.Files {
		plan.Files[i].Action = adaptPlannedAction(plan.Files[i].Action, plan.Files[i].Path)
		plan.Files[i].Path = filepath.ToSlash(strings.TrimSpace(plan.Files[i].Path))
	}
	return plan
}

func adaptPlanClarifications(items []askcontract.PlanClarification) []askcontract.PlanClarification {
	out := append([]askcontract.PlanClarification(nil), items...)
	for i := range out {
		out[i].ID = adaptClarificationID(out[i])
		if strings.TrimSpace(out[i].ID) != "" {
			for j := range out[i].Options {
				out[i].Options[j] = adaptClarificationAnswer(out[i].ID, out[i].Options[j])
			}
			if strings.TrimSpace(out[i].Answer) != "" {
				out[i].Answer = adaptClarificationAnswer(out[i].ID, out[i].Answer)
			}
		}
	}
	return out
}

func adaptClarificationID(item askcontract.PlanClarification) string {
	id := strings.TrimSpace(item.ID)
	switch id {
	case "runtime.platformFamily", "platform-family", "os-family":
		return "runtime.platformFamily"
	case "repo-delivery", "repo-source", "repo-access-mode", "repo-location":
		return "repo-delivery"
	case "kubernetes-version", "topology.kind", "topology.nodeCount", "topology.roleModel", "cluster.implementation", "refine.anchorPath", "refine.companionVars", "refine.componentPath":
		return id
	}
	if id != "" {
		return id
	}
	text := strings.ToLower(strings.TrimSpace(item.Question + " " + strings.Join(item.Options, " ")))
	switch {
	case strings.Contains(text, "control-plane") || strings.Contains(text, "control plane"):
		if strings.Contains(text, "worker") {
			return "topology.roleModel"
		}
	case strings.Contains(text, "node count") || strings.Contains(text, "how many nodes") || strings.Contains(text, "total node count"):
		return "topology.nodeCount"
	case strings.Contains(text, "single-node") || strings.Contains(text, "multi-node") || strings.Contains(text, "topology") || strings.Contains(text, "ha"):
		return "topology.kind"
	case strings.Contains(text, "implementation") && strings.Contains(text, "kubeadm"):
		return "cluster.implementation"
	case strings.Contains(text, "platform family") || strings.Contains(text, "os family") || strings.Contains(text, "distro"):
		return "runtime.platformFamily"
	case strings.Contains(text, "repository") || strings.Contains(text, "repo"):
		return "repo-delivery"
	}
	return id
}

func adaptClarificationAnswer(id string, answer string) string {
	answer = strings.TrimSpace(answer)
	lower := strings.ToLower(answer)
	switch strings.TrimSpace(id) {
	case "topology.kind":
		switch {
		case strings.Contains(lower, "single"):
			return "single-node"
		case strings.Contains(lower, "multi"):
			return "multi-node"
		case lower == "ha" || strings.Contains(lower, "high availability"):
			return "ha"
		}
	case "topology.roleModel":
		switch {
		case strings.Contains(lower, "1 control-plane") && strings.Contains(lower, "1 worker"):
			return "1cp-1worker"
		case strings.Contains(lower, "1 control-plane") && strings.Contains(lower, "2 worker"):
			return "1cp-2workers"
		case strings.Contains(lower, "2 control-plane"):
			return "2cp-ha"
		case strings.Contains(lower, "3 control-plane") || strings.Contains(lower, "3 control plane"):
			return "3cp-ha"
		}
	case "runtime.platformFamily":
		switch lower {
		case "rhel", "debian", "custom":
			return lower
		}
	case "repo-delivery":
		switch {
		case strings.Contains(lower, "file-based") || strings.Contains(lower, "filesystem"):
			return "filesystem-path"
		case strings.Contains(lower, "http") || strings.Contains(lower, "lan-hosted"):
			return "local-http-server"
		case strings.Contains(lower, "prebaked") || strings.Contains(lower, "pre-seeded") || strings.Contains(lower, "portable") || strings.Contains(lower, "copied repo"):
			return "prebaked-node-repo"
		}
	}
	return answer
}

func adaptPlannedAction(action string, path string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	action = strings.ReplaceAll(action, "_", "-")
	switch action {
	case "create", "update":
		return action
	case "add":
		return "create"
	case "modify", "create-or-modify", "create-or-update", "createorupdate", "createormodify":
		if strings.HasPrefix(strings.TrimSpace(path), "workflows/") {
			return "update"
		}
	}
	if action == "" {
		return "create"
	}
	return action
}
