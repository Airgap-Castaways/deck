package askpolicy

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func BuildPlanDefaults(req ScenarioRequirements, prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) askcontract.PlanResponse {
	files := []askcontract.PlanFile{{Path: workspacepaths.CanonicalApplyWorkflow, Kind: "scenario", Action: "create", Purpose: "Primary workflow entrypoint"}}
	if req.NeedsPrepare {
		files = append(files, askcontract.PlanFile{Path: workspacepaths.CanonicalPrepareWorkflow, Kind: "scenario", Action: "create", Purpose: "Prepare bundle inputs and dependencies"})
	}
	if strings.Contains(strings.ToLower(prompt), "vars") || len(req.VarsAdvisories) > 0 {
		files = append(files, askcontract.PlanFile{Path: workspacepaths.CanonicalVarsWorkflow, Kind: "vars", Action: "create", Purpose: "Workspace variables"})
	}
	if workspace.HasWorkflowTree {
		for i := range files {
			if workspacepaths.IsScenarioAuthoringPath(files[i].Path) {
				files[i].Action = "update"
			}
		}
	}
	brief := BriefFromRequirements(req, decision)
	execution := ExecutionModelFromRequirements(req)
	entryScenario := strings.TrimSpace(req.EntryScenario)
	if entryScenario == "" {
		for _, path := range req.RequiredFiles {
			clean := filepath.ToSlash(strings.TrimSpace(path))
			if workspacepaths.IsScenarioAuthoringPath(clean) {
				entryScenario = clean
				break
			}
		}
	}
	if entryScenario == "" {
		if clean := filepath.ToSlash(strings.TrimSpace(decision.Target.Path)); workspacepaths.IsScenarioAuthoringPath(clean) || clean == workspacepaths.CanonicalPrepareWorkflow {
			entryScenario = clean
		}
	}
	return askcontract.PlanResponse{
		Version:                 1,
		Request:                 strings.TrimSpace(prompt),
		Intent:                  string(decision.Route),
		Complexity:              inferRequestComplexity(prompt, req),
		AuthoringBrief:          brief,
		AuthoringProgram:        normalizeAuthoringProgram(askcontract.AuthoringProgram{}, brief, execution, prompt),
		ExecutionModel:          execution,
		OfflineAssumption:       req.Connectivity,
		NeedsPrepare:            req.NeedsPrepare,
		ArtifactKinds:           append([]string(nil), req.ArtifactKinds...),
		VarsRecommendation:      append([]string(nil), req.VarsAdvisories...),
		ComponentRecommendation: append([]string(nil), req.ComponentAdvisories...),
		TargetOutcome:           "Generate valid workflow files for the request.",
		Assumptions:             []string{"Use v1alpha1 workflow schema", "Prefer typed steps where possible"},
		Clarifications:          planClarificationsFromRequirements(prompt, req, decision, workspace),
		EntryScenario:           entryScenario,
		Files:                   files,
		ValidationChecklist:     defaultValidationChecklist(req),
	}
}

func planClarificationsFromRequirements(prompt string, req ScenarioRequirements, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) []askcontract.PlanClarification {
	facts := InferFacts(prompt, req.ArtifactKinds, req.Connectivity)
	items := clarificationCandidatesFromRequirements(prompt, req, decision, workspace, facts)
	for i := range items {
		applyClarificationHints(&items[i], facts)
	}
	return sortClarifications(dedupePlanClarifications(items))
}

func targetClarificationsFromRequirements(prompt string, req ScenarioRequirements, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) []askcontract.PlanClarification {
	if decision.Route != askintent.RouteRefine {
		return nil
	}
	if len(askintent.ExtractWorkflowPaths(prompt)) > 0 {
		return nil
	}
	options := []string{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == workspacepaths.CanonicalVarsWorkflow || path == workspacepaths.CanonicalPrepareWorkflow || workspacepaths.IsScenarioAuthoringPath(path) || workspacepaths.IsComponentAuthoringPath(path) {
			options = append(options, path)
		}
	}
	options = dedupeStrings(options)
	if len(options) <= 1 {
		return nil
	}
	defaultPath := ""
	for _, path := range options {
		if workspacepaths.IsScenarioAuthoringPath(path) {
			defaultPath = path
			break
		}
	}
	if defaultPath == "" {
		defaultPath = options[0]
	}
	return []askcontract.PlanClarification{{
		ID:                 "refine.anchorPath",
		Question:           "This refine request does not name a single workflow file to anchor the change. Which existing file should the refactor treat as the primary target?",
		Kind:               "path",
		Reason:             "Refine generation keeps one user-anchored file stable and may expand only into explicitly allowed companion files.",
		Decision:           "scope",
		Options:            options,
		RecommendedDefault: defaultPath,
		BlocksGeneration:   true,
		Affects:            []string{"authoringBrief.targetPaths", "authoringBrief.targetScope"},
	}}
}

func applyClarificationHints(item *askcontract.PlanClarification, facts Facts) {
	if item == nil || strings.TrimSpace(item.Answer) != "" {
		return
	}
	switch strings.TrimSpace(item.ID) {
	case "topology.nodeCount":
		if facts.NodeCount > 0 {
			item.RecommendedDefault = strconv.Itoa(facts.NodeCount)
		}
	case "topology.roleModel":
		if facts.ControlPlaneCount > 0 && facts.WorkerCount > 0 {
			item.RecommendedDefault = fmt.Sprintf("%dcp-%dworkers", facts.ControlPlaneCount, facts.WorkerCount)
		} else if facts.Topology == "ha" && facts.NodeCount >= 3 {
			item.RecommendedDefault = fmt.Sprintf("%dcp-ha", facts.NodeCount)
		}
	}
}
