package askpolicy

import (
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func normalizeRefineScope(plan askcontract.PlanResponse, prompt string, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) askcontract.PlanResponse {
	if decision.Route != askintent.RouteRefine {
		return plan
	}
	anchors, companions := refineScopePaths(prompt, plan)
	plan.AuthoringBrief.AnchorPaths = normalizeAllowedPaths(anchors, plan.AuthoringBrief.AnchorPaths)
	plan.AuthoringBrief.AllowedCompanionPaths = normalizeAllowedPaths(companions, plan.AuthoringBrief.AllowedCompanionPaths)
	targetPaths := append([]string{}, plan.AuthoringBrief.AnchorPaths...)
	targetPaths = append(targetPaths, plan.AuthoringBrief.AllowedCompanionPaths...)
	plan.AuthoringBrief.TargetPaths = normalizeAllowedPaths(targetPaths, plan.AuthoringBrief.TargetPaths)
	plan.AuthoringBrief.DisallowedExpansionPaths = disallowedRefinePaths(workspace, plan.AuthoringBrief.TargetPaths)
	plan.AuthoringBrief.TargetScope = refineTargetScope(plan.AuthoringBrief.TargetPaths)
	plan.Files = normalizeRefinePlanFiles(plan.Files, plan.AuthoringBrief.TargetPaths, workspace)
	if scenario := firstScenarioPath(plan.AuthoringBrief.AnchorPaths); scenario != "" {
		plan.EntryScenario = scenario
	}
	return plan
}

func refineScopePaths(prompt string, plan askcontract.PlanResponse) ([]string, []string) {
	explicitPaths := askintent.ExtractWorkflowPaths(prompt)
	anchors := normalizeAllowedPaths(plan.AuthoringBrief.AnchorPaths, nil)
	if len(anchors) == 0 {
		if anchor := preferredAnchorPath(explicitPaths); anchor != "" {
			anchors = []string{anchor}
		} else if anchor := preferredAnchorPath(plan.AuthoringBrief.TargetPaths); anchor != "" {
			anchors = []string{anchor}
		} else if anchor := preferredAnchorPath(planFilePaths(plan.Files)); anchor != "" {
			anchors = []string{anchor}
		}
	}
	companions := append([]string{}, plan.AuthoringBrief.AllowedCompanionPaths...)
	for _, path := range explicitPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean == "" || containsString(anchors, clean) {
			continue
		}
		companions = append(companions, clean)
	}
	for _, path := range plan.AuthoringBrief.TargetPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean == "" || containsString(anchors, clean) {
			continue
		}
		companions = append(companions, clean)
	}
	return dedupeStrings(anchors), dedupeStrings(companions)
}

func preferredAnchorPath(paths []string) string {
	for _, path := range paths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean == "" {
			continue
		}
		if clean == workspacepaths.CanonicalVarsWorkflow || strings.HasPrefix(clean, "workflows/components/") {
			continue
		}
		return clean
	}
	for _, path := range paths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean != "" {
			return clean
		}
	}
	return ""
}

func normalizeRefinePlanFiles(files []askcontract.PlanFile, targetPaths []string, workspace askretrieve.WorkspaceSummary) []askcontract.PlanFile {
	allowed := map[string]bool{}
	for _, path := range targetPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean != "" {
			allowed[clean] = true
		}
	}
	if len(allowed) == 0 {
		return files
	}
	byPath := map[string]askcontract.PlanFile{}
	for _, file := range files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == "" || !allowed[path] {
			continue
		}
		file.Path = path
		file.Action = normalizePlannedAction(file.Action, path)
		byPath[path] = file
	}
	existing := map[string]bool{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path != "" {
			existing[path] = true
		}
	}
	for _, path := range targetPaths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" {
			continue
		}
		if _, ok := byPath[path]; ok {
			continue
		}
		kind := "scenario"
		switch {
		case path == workspacepaths.CanonicalVarsWorkflow:
			kind = "vars"
		case strings.HasPrefix(path, "workflows/components/"):
			kind = "component"
		}
		action := "create"
		if existing[path] {
			action = "update"
		}
		purpose := "allowed refine target"
		switch kind {
		case "vars":
			purpose = "allowed refine vars companion"
		case "component":
			purpose = "allowed refine component companion"
		}
		byPath[path] = askcontract.PlanFile{Path: path, Kind: kind, Action: action, Purpose: purpose}
	}
	out := make([]askcontract.PlanFile, 0, len(byPath))
	for _, path := range dedupeStrings(targetPaths) {
		if file, ok := byPath[path]; ok {
			out = append(out, file)
		}
	}
	return out
}

func refineTargetScope(targetPaths []string) string {
	targetPaths = dedupeStrings(targetPaths)
	if len(targetPaths) > 1 {
		return "workspace"
	}
	if len(targetPaths) == 0 {
		return "workspace"
	}
	path := filepath.ToSlash(strings.TrimSpace(targetPaths[0]))
	switch {
	case path == workspacepaths.CanonicalVarsWorkflow:
		return "vars"
	case strings.HasPrefix(path, "workflows/components/"):
		return "component"
	default:
		return "scenario"
	}
}

func disallowedRefinePaths(workspace askretrieve.WorkspaceSummary, allowedPaths []string) []string {
	allowed := map[string]bool{}
	for _, path := range allowedPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean != "" {
			allowed[clean] = true
		}
	}
	blocked := []string{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == "" || !strings.HasPrefix(path, "workflows/") || allowed[path] {
			continue
		}
		blocked = append(blocked, path)
	}
	return dedupeStrings(blocked)
}

func firstScenarioPath(paths []string) string {
	for _, path := range paths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if clean == workspacepaths.CanonicalPrepareWorkflow || strings.HasPrefix(clean, "workflows/scenarios/") {
			return clean
		}
	}
	return ""
}

func planFilePaths(files []askcontract.PlanFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		if clean := filepath.ToSlash(strings.TrimSpace(file.Path)); clean != "" {
			paths = append(paths, clean)
		}
	}
	return paths
}
