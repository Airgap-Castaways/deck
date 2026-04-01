package askpolicy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askauthoring"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func NormalizePlan(plan askcontract.PlanResponse, prompt string, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, decision askintent.Decision) askcontract.PlanResponse {
	req := BuildScenarioRequirements(prompt, retrieval, workspace, decision)
	fallbackBrief := BriefFromRequirements(req, decision)
	plan.AuthoringBrief = normalizeAuthoringBrief(plan.AuthoringBrief, fallbackBrief)
	fallbackExecutionModel := ExecutionModelFromRequirements(req)
	plan.ExecutionModel = normalizeExecutionModel(plan.ExecutionModel, fallbackExecutionModel, plan.AuthoringBrief)
	if strings.TrimSpace(plan.OfflineAssumption) == "" {
		plan.OfflineAssumption = req.Connectivity
	}
	plan.ArtifactKinds = NormalizeArtifactKinds(plan.ArtifactKinds)
	if len(plan.ArtifactKinds) == 0 {
		plan.ArtifactKinds = append([]string(nil), req.ArtifactKinds...)
	}
	if len(plan.ArtifactKinds) > 0 {
		plan.NeedsPrepare = true
	}
	if len(plan.VarsRecommendation) == 0 {
		plan.VarsRecommendation = append([]string(nil), req.VarsAdvisories...)
	}
	if len(plan.ComponentRecommendation) == 0 {
		plan.ComponentRecommendation = append([]string(nil), req.ComponentAdvisories...)
	}
	plan.Clarifications = normalizeClarifications(plan.Clarifications, req, prompt, decision, workspace)
	plan.Blockers, plan.OpenQuestions = clarificationLines(plan.Clarifications, plan.Blockers, plan.OpenQuestions)
	plan = applyClarificationAnswers(plan)
	plan = normalizeRefineScope(plan, prompt, workspace, decision)
	plan.AuthoringProgram = normalizeAuthoringProgram(plan.AuthoringProgram, plan.AuthoringBrief, plan.ExecutionModel, prompt)
	plan.Blockers = dedupeStrings(append(plan.Blockers, coverageBoundaryBlockers(prompt, req, decision)...))
	plan.EntryScenario = normalizeEntryScenario(plan.EntryScenario, req, plan.Files, plan.AuthoringBrief, decision)
	for i := range plan.Files {
		plan.Files[i].Action = normalizePlannedAction(plan.Files[i].Action, plan.Files[i].Path)
	}
	if req.AcceptanceLevel == "starter" {
		filtered := make([]askcontract.PlanFile, 0, len(plan.Files))
		for _, file := range plan.Files {
			clean := filepath.ToSlash(strings.TrimSpace(file.Path))
			if strings.HasPrefix(clean, "workflows/components/") {
				continue
			}
			filtered = append(filtered, file)
		}
		plan.Files = filtered
		plan.ComponentRecommendation = nil
	}
	return plan
}

func normalizeClarifications(items []askcontract.PlanClarification, req ScenarioRequirements, prompt string, decision askintent.Decision, workspace askretrieve.WorkspaceSummary) []askcontract.PlanClarification {
	facts := askauthoring.InferFacts(prompt, req.ArtifactKinds, req.Connectivity)
	defaults := planClarificationsFromRequirements(prompt, req, decision, workspace)
	byID := map[string]askcontract.PlanClarification{}
	for _, item := range defaults {
		byID[strings.TrimSpace(item.ID)] = item
	}
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		base := byID[id]
		if strings.TrimSpace(item.Question) != "" {
			base.Question = strings.TrimSpace(item.Question)
		}
		if strings.TrimSpace(item.Kind) != "" {
			base.Kind = strings.TrimSpace(item.Kind)
		}
		if strings.TrimSpace(item.Reason) != "" {
			base.Reason = strings.TrimSpace(item.Reason)
		}
		if strings.TrimSpace(item.Decision) != "" {
			base.Decision = strings.TrimSpace(item.Decision)
		}
		if len(item.Options) > 0 {
			base.Options = normalizeStringList(item.Options)
		}
		if strings.TrimSpace(item.RecommendedDefault) != "" {
			base.RecommendedDefault = strings.TrimSpace(item.RecommendedDefault)
		}
		if strings.TrimSpace(item.Answer) != "" {
			base.Answer = strings.TrimSpace(item.Answer)
		}
		if len(item.Affects) > 0 {
			base.Affects = normalizeStringList(item.Affects)
		}
		if item.BlocksGeneration {
			base.BlocksGeneration = true
		}
		applyClarificationHints(&base, facts)
		byID[id] = base
	}
	out := make([]askcontract.PlanClarification, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	return sortClarifications(out)
}

func sortClarifications(items []askcontract.PlanClarification) []askcontract.PlanClarification {
	out := append([]askcontract.PlanClarification(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func clarificationLines(items []askcontract.PlanClarification, existingBlockers []string, existingQuestions []string) ([]string, []string) {
	clarificationText := map[string]bool{}
	for _, item := range items {
		if text := strings.TrimSpace(item.Question); text != "" {
			clarificationText[text] = true
		}
	}
	blockers := []string{}
	for _, item := range existingBlockers {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] {
			continue
		}
		blockers = append(blockers, text)
	}
	questions := []string{}
	for _, item := range existingQuestions {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] {
			continue
		}
		questions = append(questions, text)
	}
	for _, item := range items {
		if strings.TrimSpace(item.Question) == "" || strings.TrimSpace(item.Answer) != "" {
			continue
		}
		if item.BlocksGeneration {
			blockers = append(blockers, item.Question)
		} else {
			questions = append(questions, item.Question)
		}
	}
	return dedupeStrings(blockers), dedupeStrings(questions)
}

func applyClarificationAnswers(plan askcontract.PlanResponse) askcontract.PlanResponse {
	byID := map[string]string{}
	for _, item := range plan.Clarifications {
		if id := strings.TrimSpace(item.ID); id != "" && strings.TrimSpace(item.Answer) != "" {
			byID[id] = strings.TrimSpace(item.Answer)
		}
	}
	if answer := byID["topology.kind"]; answer != "" {
		plan.AuthoringBrief.Topology = strings.TrimSpace(answer)
	}
	if answer := byID["topology.nodeCount"]; answer != "" {
		if n, err := strconv.Atoi(answer); err == nil && n > 0 {
			plan.AuthoringBrief.NodeCount = n
			if plan.ExecutionModel.Verification.ExpectedNodeCount <= 0 {
				plan.ExecutionModel.Verification.ExpectedNodeCount = n
			}
		}
	}
	if answer := byID["topology.roleModel"]; answer != "" {
		plan.ExecutionModel.RoleExecution.PerNodeInvocation = true
		if strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) == "" {
			plan.ExecutionModel.RoleExecution.RoleSelector = "vars.role"
		}
		switch strings.TrimSpace(answer) {
		case "3cp-ha":
			if plan.AuthoringBrief.NodeCount < 3 {
				plan.AuthoringBrief.NodeCount = 3
			}
			plan.AuthoringBrief.Topology = "ha"
			if plan.ExecutionModel.Verification.ExpectedNodeCount <= 0 {
				plan.ExecutionModel.Verification.ExpectedNodeCount = plan.AuthoringBrief.NodeCount
			}
			plan.ExecutionModel.Verification.ExpectedControlPlaneReady = maxInt(plan.ExecutionModel.Verification.ExpectedControlPlaneReady, plan.AuthoringBrief.NodeCount)
		case "1cp-2workers":
			if plan.AuthoringBrief.NodeCount < 3 {
				plan.AuthoringBrief.NodeCount = 3
			}
			if plan.AuthoringBrief.Topology == "unspecified" {
				plan.AuthoringBrief.Topology = "multi-node"
			}
			if plan.ExecutionModel.Verification.ExpectedNodeCount <= 0 {
				plan.ExecutionModel.Verification.ExpectedNodeCount = plan.AuthoringBrief.NodeCount
			}
			plan.ExecutionModel.Verification.ExpectedControlPlaneReady = maxInt(plan.ExecutionModel.Verification.ExpectedControlPlaneReady, 1)
		}
	}
	if answer := byID["refine.anchorPath"]; answer != "" {
		path := filepath.ToSlash(strings.TrimSpace(answer))
		paths := []string{path}
		if !containsString(paths, "workflows/vars.yaml") {
			for _, existing := range plan.AuthoringBrief.TargetPaths {
				if filepath.ToSlash(strings.TrimSpace(existing)) == "workflows/vars.yaml" {
					paths = append(paths, "workflows/vars.yaml")
				}
			}
		}
		plan.AuthoringBrief.TargetPaths = dedupeStrings(paths)
		switch {
		case path == "workflows/vars.yaml":
			plan.AuthoringBrief.TargetScope = "vars"
		case strings.HasPrefix(path, "workflows/components/"):
			plan.AuthoringBrief.TargetScope = "component"
		case strings.HasPrefix(path, "workflows/scenarios/") || path == "workflows/prepare.yaml":
			if len(plan.AuthoringBrief.TargetPaths) > 1 {
				plan.AuthoringBrief.TargetScope = "workspace"
			} else {
				plan.AuthoringBrief.TargetScope = "scenario"
			}
		}
	}
	if answer := byID["refine.companionVars"]; strings.EqualFold(answer, "yes") {
		plan.AuthoringBrief.AllowedCompanionPaths = dedupeStrings(append(plan.AuthoringBrief.AllowedCompanionPaths, "workflows/vars.yaml"))
		plan.AuthoringBrief.TargetPaths = dedupeStrings(append(plan.AuthoringBrief.TargetPaths, "workflows/vars.yaml"))
	}
	if answer := byID["refine.componentPath"]; answer != "" && !strings.EqualFold(answer, "none") {
		path := filepath.ToSlash(strings.TrimSpace(answer))
		if askcontractPathAllowed(path) {
			plan.AuthoringBrief.AllowedCompanionPaths = dedupeStrings(append(plan.AuthoringBrief.AllowedCompanionPaths, path))
			plan.AuthoringBrief.TargetPaths = dedupeStrings(append(plan.AuthoringBrief.TargetPaths, path))
		}
	}
	if answer := byID["cluster.implementation"]; answer != "" {
		if strings.TrimSpace(answer) == "kubeadm" {
			plan.AuthoringBrief.RequiredCapabilities = dedupeStrings(append(plan.AuthoringBrief.RequiredCapabilities, "kubeadm-bootstrap", "cluster-verification"))
			if plan.AuthoringBrief.NodeCount > 1 || plan.AuthoringBrief.Topology == "multi-node" || plan.AuthoringBrief.Topology == "ha" {
				plan.AuthoringBrief.RequiredCapabilities = dedupeStrings(append(plan.AuthoringBrief.RequiredCapabilities, "kubeadm-join"))
			}
		}
	}
	if answer := byID["runtime.platformFamily"]; answer != "" {
		plan.AuthoringBrief.PlatformFamily = strings.TrimSpace(answer)
	}
	if answer := byID["coverage.escapeHatch"]; answer != "" {
		plan.AuthoringBrief.EscapeHatchMode = strings.TrimSpace(answer)
	}
	return plan
}

func maxInt(current int, candidate int) int {
	if candidate > current {
		return candidate
	}
	return current
}

func normalizeExecutionModel(model askcontract.ExecutionModel, fallback askcontract.ExecutionModel, brief askcontract.AuthoringBrief) askcontract.ExecutionModel {
	model.ArtifactContracts = normalizeArtifactContracts(model.ArtifactContracts, fallback.ArtifactContracts)
	model.SharedStateContracts = normalizeSharedStateContracts(model.SharedStateContracts, fallback.SharedStateContracts)
	if strings.TrimSpace(model.RoleExecution.RoleSelector) == "" {
		model.RoleExecution.RoleSelector = fallback.RoleExecution.RoleSelector
	}
	if strings.TrimSpace(model.RoleExecution.ControlPlaneFlow) == "" {
		model.RoleExecution.ControlPlaneFlow = fallback.RoleExecution.ControlPlaneFlow
	}
	if strings.TrimSpace(model.RoleExecution.WorkerFlow) == "" {
		model.RoleExecution.WorkerFlow = fallback.RoleExecution.WorkerFlow
	}
	if !model.RoleExecution.PerNodeInvocation {
		model.RoleExecution.PerNodeInvocation = fallback.RoleExecution.PerNodeInvocation
	}
	if strings.TrimSpace(model.Verification.BootstrapPhase) == "" {
		model.Verification.BootstrapPhase = fallback.Verification.BootstrapPhase
	}
	if strings.TrimSpace(model.Verification.FinalPhase) == "" {
		model.Verification.FinalPhase = fallback.Verification.FinalPhase
	}
	if !isCanonicalVerificationRole(model.Verification.FinalVerificationRole) {
		model.Verification.FinalVerificationRole = fallback.Verification.FinalVerificationRole
	}
	if model.Verification.ExpectedNodeCount <= 0 {
		model.Verification.ExpectedNodeCount = fallback.Verification.ExpectedNodeCount
	}
	if model.Verification.ExpectedControlPlaneReady <= 0 {
		model.Verification.ExpectedControlPlaneReady = fallback.Verification.ExpectedControlPlaneReady
	}
	if len(model.ApplyAssumptions) == 0 {
		model.ApplyAssumptions = append([]string(nil), fallback.ApplyAssumptions...)
	} else {
		model.ApplyAssumptions = dedupeStrings(append(normalizeStringList(model.ApplyAssumptions), fallback.ApplyAssumptions...))
	}
	model = normalizeExecutionModelGuardrails(model, brief)
	return model
}

func normalizeExecutionModelGuardrails(model askcontract.ExecutionModel, brief askcontract.AuthoringBrief) askcontract.ExecutionModel {
	if brief.NodeCount > 0 && model.Verification.ExpectedNodeCount <= 0 {
		model.Verification.ExpectedNodeCount = brief.NodeCount
	}
	if briefIsVerificationOnly(brief) {
		model.RoleExecution.RoleSelector = ""
		model.RoleExecution.ControlPlaneFlow = ""
		model.RoleExecution.WorkerFlow = ""
		model.RoleExecution.PerNodeInvocation = false
		model.Verification.FinalVerificationRole = "local"
		if model.Verification.ExpectedControlPlaneReady <= 0 && brief.NodeCount > 0 {
			model.Verification.ExpectedControlPlaneReady = minInt(brief.NodeCount, 1)
		}
		return model
	}
	if !briefNeedsWorkerRole(brief) {
		model.RoleExecution.WorkerFlow = ""
		model.RoleExecution.PerNodeInvocation = false
	}
	if !briefNeedsRoleSelector(brief) {
		model.RoleExecution.RoleSelector = ""
	}
	if brief.Topology == "single-node" && model.Verification.ExpectedControlPlaneReady <= 0 && hasBriefCapability(brief, "cluster-verification") {
		model.Verification.ExpectedControlPlaneReady = 1
	}
	return model
}

func normalizeEntryScenario(current string, req ScenarioRequirements, files []askcontract.PlanFile, brief askcontract.AuthoringBrief, decision askintent.Decision) string {
	candidates := []string{}
	if clean := filepath.ToSlash(strings.TrimSpace(current)); strings.HasPrefix(clean, "workflows/scenarios/") {
		candidates = append(candidates, clean)
	}
	if clean := filepath.ToSlash(strings.TrimSpace(decision.Target.Path)); strings.HasPrefix(clean, "workflows/scenarios/") {
		candidates = append(candidates, clean)
	}
	if clean := filepath.ToSlash(strings.TrimSpace(req.EntryScenario)); strings.HasPrefix(clean, "workflows/scenarios/") {
		candidates = append(candidates, clean)
	}
	for _, path := range brief.AnchorPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if strings.HasPrefix(clean, "workflows/scenarios/") {
			candidates = append(candidates, clean)
		}
	}
	for _, path := range brief.TargetPaths {
		clean := filepath.ToSlash(strings.TrimSpace(path))
		if strings.HasPrefix(clean, "workflows/scenarios/") {
			candidates = append(candidates, clean)
		}
	}
	for _, file := range files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		if strings.HasPrefix(clean, "workflows/scenarios/") {
			candidates = append(candidates, clean)
		}
	}
	for _, candidate := range dedupeStrings(candidates) {
		return candidate
	}
	return ""
}

func normalizeAuthoringProgram(program askcontract.AuthoringProgram, brief askcontract.AuthoringBrief, execution askcontract.ExecutionModel, prompt string) askcontract.AuthoringProgram {
	family, release := inferPlatformFromPrompt(prompt)
	minimalSingleNodeBootstrap := isMinimalSingleNodeBootstrapPrompt(prompt, brief)
	if minimalSingleNodeBootstrap {
		program.Platform.Family = "custom"
		program.Platform.Release = "unspecified"
		program.Platform.RepoType = "none"
		program.Platform.BackendImage = "none"
		program.Artifacts.PackageOutputDir = ""
		program.Artifacts.ImageOutputDir = ""
		program.Cluster.RoleSelector = ""
		program.Verification.ExpectedReadyCount = 0
	}
	if strings.TrimSpace(program.Platform.Family) == "" {
		if strings.TrimSpace(brief.PlatformFamily) != "" {
			program.Platform.Family = strings.TrimSpace(brief.PlatformFamily)
		} else {
			program.Platform.Family = family
		}
	}
	if strings.TrimSpace(program.Platform.Release) == "" {
		program.Platform.Release = release
	}
	if strings.TrimSpace(program.Platform.RepoType) == "" && !minimalSingleNodeBootstrap {
		program.Platform.RepoType = defaultRepoType(program.Platform.Family)
	}
	if strings.TrimSpace(program.Platform.BackendImage) == "" && !minimalSingleNodeBootstrap {
		program.Platform.BackendImage = defaultBackendImage(program.Platform.Family, program.Platform.Release)
	}
	if strings.TrimSpace(program.Artifacts.PackageOutputDir) == "" && !minimalSingleNodeBootstrap {
		program.Artifacts.PackageOutputDir = defaultPackageOutputDir(program.Platform.Family, program.Platform.Release, program.Platform.RepoType)
	}
	if strings.TrimSpace(program.Artifacts.ImageOutputDir) == "" && !minimalSingleNodeBootstrap {
		program.Artifacts.ImageOutputDir = "images/control-plane"
	}
	if strings.TrimSpace(program.Cluster.JoinFile) == "" {
		program.Cluster.JoinFile = "/tmp/deck/join.txt"
	}
	if strings.TrimSpace(program.Cluster.PodCIDR) == "" {
		program.Cluster.PodCIDR = "10.244.0.0/16"
	}
	program.Cluster.RoleSelector = normalizeRoleSelector(firstNonEmpty(strings.TrimSpace(program.Cluster.RoleSelector), strings.TrimSpace(execution.RoleExecution.RoleSelector)))
	if !briefNeedsRoleSelector(brief) {
		program.Cluster.RoleSelector = ""
	}
	if program.Cluster.ControlPlaneCount <= 0 {
		program.Cluster.ControlPlaneCount = inferControlPlaneCount(brief, execution)
	}
	if program.Cluster.WorkerCount < 0 {
		program.Cluster.WorkerCount = 0
	}
	if program.Cluster.WorkerCount == 0 {
		program.Cluster.WorkerCount = inferWorkerCount(brief, execution, program.Cluster.ControlPlaneCount)
	}
	if program.Verification.ExpectedNodeCount <= 0 {
		program.Verification.ExpectedNodeCount = execution.Verification.ExpectedNodeCount
	}
	if program.Verification.ExpectedNodeCount <= 0 {
		program.Verification.ExpectedNodeCount = maxInt(brief.NodeCount, program.Cluster.ControlPlaneCount+program.Cluster.WorkerCount)
	}
	if program.Verification.ExpectedControlPlaneReady <= 0 {
		program.Verification.ExpectedControlPlaneReady = execution.Verification.ExpectedControlPlaneReady
	}
	if program.Verification.ExpectedControlPlaneReady <= 0 {
		program.Verification.ExpectedControlPlaneReady = maxInt(1, program.Cluster.ControlPlaneCount)
	}
	if program.Verification.ExpectedReadyCount <= 0 {
		if minimalSingleNodeBootstrap {
			program.Verification.ExpectedReadyCount = 0
		} else {
			program.Verification.ExpectedReadyCount = program.Verification.ExpectedNodeCount
		}
	}
	if strings.TrimSpace(program.Verification.FinalVerificationRole) == "" {
		program.Verification.FinalVerificationRole = strings.TrimSpace(execution.Verification.FinalVerificationRole)
	}
	if strings.TrimSpace(program.Verification.FinalVerificationRole) == "" {
		if program.Verification.ExpectedNodeCount > 1 {
			program.Verification.FinalVerificationRole = "control-plane"
		} else {
			program.Verification.FinalVerificationRole = "local"
		}
	}
	if strings.TrimSpace(program.Verification.Interval) == "" {
		program.Verification.Interval = "5s"
	}
	if strings.TrimSpace(program.Verification.Timeout) == "" {
		if program.Verification.ExpectedNodeCount > 1 {
			program.Verification.Timeout = "10m"
		} else {
			program.Verification.Timeout = "5m"
		}
	}
	return program
}

func isMinimalSingleNodeBootstrapPrompt(prompt string, brief askcontract.AuthoringBrief) bool {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if strings.TrimSpace(brief.ModeIntent) != "apply-only" || strings.TrimSpace(brief.Topology) != "single-node" {
		return false
	}
	if strings.Contains(lower, "prepare") || strings.Contains(lower, "join") || strings.Contains(lower, "worker") {
		return false
	}
	if !strings.Contains(lower, "init-kubeadm") || !strings.Contains(lower, "check-cluster") {
		return false
	}
	if strings.Contains(lower, "ready=1") || strings.Contains(lower, "ready 1") || strings.Contains(lower, "cni") {
		return false
	}
	return true
}

func inferPlatformFromPrompt(prompt string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	patterns := []struct {
		family string
		regex  *regexp.Regexp
	}{
		{family: "rhel", regex: regexp.MustCompile(`rhel\s*([0-9]+(?:\.[0-9]+)?)`)},
		{family: "rocky", regex: regexp.MustCompile(`rocky\s*([0-9]+(?:\.[0-9]+)?)`)},
		{family: "debian", regex: regexp.MustCompile(`debian\s*([0-9]+(?:\.[0-9]+)?)`)},
		{family: "ubuntu", regex: regexp.MustCompile(`ubuntu\s*([0-9]+(?:\.[0-9]+)?)`)},
	}
	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(lower); len(matches) == 2 {
			family := pattern.family
			if family == "rocky" {
				family = "rhel"
			}
			return family, strings.TrimSpace(matches[1])
		}
	}
	for _, item := range []struct{ token, family string }{{"rhel", "rhel"}, {"rocky", "rhel"}, {"debian", "debian"}, {"ubuntu", "debian"}} {
		if strings.Contains(lower, item.token) {
			return item.family, ""
		}
	}
	return "", ""
}

func defaultRepoType(family string) string {
	if strings.EqualFold(strings.TrimSpace(family), "debian") {
		return "deb-flat"
	}
	return "rpm"
}

func defaultBackendImage(family string, release string) string {
	if strings.EqualFold(strings.TrimSpace(family), "debian") {
		return "ubuntu:22.04"
	}
	_ = release
	return "rockylinux:9"
}

func defaultPackageOutputDir(family string, release string, repoType string) string {
	family = strings.ToLower(strings.TrimSpace(family))
	release = strings.TrimSpace(release)
	repoType = strings.ToLower(strings.TrimSpace(repoType))
	if release == "" {
		return "packages/"
	}
	if repoType == "deb-flat" || family == "debian" {
		return filepath.ToSlash(filepath.Join("packages", "deb", release))
	}
	return filepath.ToSlash(filepath.Join("packages", "rpm", release))
}

func normalizeRoleSelector(value string) string {
	value = strings.TrimSpace(value)
	if value == "nil" || value == "<nil>" {
		return ""
	}
	value = strings.TrimPrefix(value, "vars.")
	if value == "nil" || value == "<nil>" {
		return ""
	}
	return value
}

func inferControlPlaneCount(brief askcontract.AuthoringBrief, execution askcontract.ExecutionModel) int {
	if brief.Topology == "ha" {
		if execution.Verification.ExpectedControlPlaneReady > 0 {
			return execution.Verification.ExpectedControlPlaneReady
		}
		if brief.NodeCount > 0 {
			return brief.NodeCount
		}
		return 3
	}
	if execution.Verification.ExpectedControlPlaneReady > 0 {
		return execution.Verification.ExpectedControlPlaneReady
	}
	return 1
}

func inferWorkerCount(brief askcontract.AuthoringBrief, execution askcontract.ExecutionModel, controlPlaneCount int) int {
	total := maxInt(brief.NodeCount, execution.Verification.ExpectedNodeCount)
	if total <= controlPlaneCount {
		return 0
	}
	return total - controlPlaneCount
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func briefNeedsWorkerRole(brief askcontract.AuthoringBrief) bool {
	if brief.Topology == "multi-node" || brief.Topology == "ha" || brief.NodeCount > 1 {
		return true
	}
	return hasBriefCapability(brief, "kubeadm-join")
}

func briefNeedsRoleSelector(brief askcontract.AuthoringBrief) bool {
	return briefNeedsWorkerRole(brief)
}

func briefIsVerificationOnly(brief askcontract.AuthoringBrief) bool {
	if len(brief.RequiredCapabilities) != 1 {
		return false
	}
	return strings.TrimSpace(brief.RequiredCapabilities[0]) == "cluster-verification"
}

func hasBriefCapability(brief askcontract.AuthoringBrief, want string) bool {
	for _, capability := range brief.RequiredCapabilities {
		if strings.TrimSpace(capability) == want {
			return true
		}
	}
	return false
}

func minInt(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func isCanonicalVerificationRole(value string) bool {
	switch strings.TrimSpace(value) {
	case "control-plane", "worker", "local", "any":
		return true
	default:
		return false
	}
}

func normalizeArtifactContracts(contracts []askcontract.ArtifactContract, fallback []askcontract.ArtifactContract) []askcontract.ArtifactContract {
	allowedKinds := map[string]bool{"package": true, "image": true, "repository-setup": true}
	out := make([]askcontract.ArtifactContract, 0, len(contracts)+len(fallback))
	presentKinds := map[string]bool{}
	for _, item := range contracts {
		kind := strings.ToLower(strings.TrimSpace(item.Kind))
		if !allowedKinds[kind] {
			continue
		}
		item.Kind = kind
		item.ProducerPath = filepath.ToSlash(strings.TrimSpace(item.ProducerPath))
		item.ConsumerPath = filepath.ToSlash(strings.TrimSpace(item.ConsumerPath))
		item.Description = strings.TrimSpace(item.Description)
		if !askcontractPathAllowed(item.ProducerPath) || !askcontractPathAllowed(item.ConsumerPath) {
			continue
		}
		out = append(out, item)
		presentKinds[kind] = true
	}
	if len(out) == 0 {
		out = append(out, fallback...)
	} else {
		for _, item := range fallback {
			if !presentKinds[item.Kind] {
				out = append(out, item)
			}
		}
	}
	return dedupeArtifactContracts(out)
}

func normalizeSharedStateContracts(contracts []askcontract.SharedStateContract, fallback []askcontract.SharedStateContract) []askcontract.SharedStateContract {
	allowedAvailability := map[string]bool{"published-for-worker-consumption": true, "local-only": true}
	out := make([]askcontract.SharedStateContract, 0, len(contracts)+len(fallback))
	presentNames := map[string]bool{}
	for _, item := range contracts {
		item.Name = canonicalSharedStateName(strings.TrimSpace(item.Name))
		item.ProducerPath = strings.TrimSpace(item.ProducerPath)
		item.AvailabilityModel = strings.TrimSpace(item.AvailabilityModel)
		item.Description = strings.TrimSpace(item.Description)
		if item.Name == "" || item.ProducerPath == "" || !allowedAvailability[item.AvailabilityModel] {
			continue
		}
		item.ConsumerPaths = normalizeStringList(item.ConsumerPaths)
		out = append(out, item)
		presentNames[item.Name] = true
	}
	if len(out) == 0 {
		out = append(out, fallback...)
	} else {
		for _, item := range fallback {
			if !presentNames[canonicalSharedStateName(item.Name)] {
				out = append(out, item)
			}
		}
	}
	return dedupeSharedStateContracts(out)
}

func canonicalSharedStateName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.Contains(lower, "join") {
		return "join-file"
	}
	return strings.TrimSpace(name)
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func dedupeSharedStateContracts(contracts []askcontract.SharedStateContract) []askcontract.SharedStateContract {
	seen := map[string]bool{}
	out := make([]askcontract.SharedStateContract, 0, len(contracts))
	for _, item := range contracts {
		key := item.Name + "|" + item.ProducerPath + "|" + item.AvailabilityModel
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func normalizeAuthoringBrief(brief askcontract.AuthoringBrief, fallback askcontract.AuthoringBrief) askcontract.AuthoringBrief {
	if !isCanonicalTargetScope(brief.TargetScope) {
		brief.TargetScope = fallback.TargetScope
	}
	if !isCanonicalModeIntent(brief.ModeIntent) {
		brief.ModeIntent = fallback.ModeIntent
	}
	if !isCanonicalCompleteness(brief.CompletenessTarget) {
		brief.CompletenessTarget = fallback.CompletenessTarget
	}
	if !isCanonicalTopology(brief.Topology) {
		brief.Topology = fallback.Topology
	}
	if strings.TrimSpace(brief.RouteIntent) == "" || len(strings.Fields(brief.RouteIntent)) > 6 {
		brief.RouteIntent = fallback.RouteIntent
	}
	if strings.TrimSpace(brief.Connectivity) == "" || len(strings.Fields(brief.Connectivity)) > 4 {
		brief.Connectivity = fallback.Connectivity
	}
	if brief.NodeCount <= 0 && fallback.NodeCount > 0 {
		brief.NodeCount = fallback.NodeCount
	}
	if len(brief.TargetPaths) == 0 {
		brief.TargetPaths = append([]string(nil), fallback.TargetPaths...)
	}
	brief.TargetPaths = normalizeAllowedPaths(brief.TargetPaths, fallback.TargetPaths)
	brief.AnchorPaths = normalizeAllowedPaths(brief.AnchorPaths, nil)
	brief.AllowedCompanionPaths = normalizeAllowedPaths(brief.AllowedCompanionPaths, nil)
	brief.DisallowedExpansionPaths = normalizeAllowedPaths(brief.DisallowedExpansionPaths, nil)
	if len(brief.AnchorPaths) == 0 && len(brief.TargetPaths) == 1 {
		brief.AnchorPaths = append([]string(nil), brief.TargetPaths...)
	}
	if strings.TrimSpace(brief.PlatformFamily) != "" {
		brief.PlatformFamily = strings.ToLower(strings.TrimSpace(brief.PlatformFamily))
	}
	if strings.TrimSpace(brief.EscapeHatchMode) != "" {
		brief.EscapeHatchMode = strings.ToLower(strings.TrimSpace(brief.EscapeHatchMode))
	}
	brief.RequiredCapabilities = normalizeCapabilities(brief.RequiredCapabilities, fallback.RequiredCapabilities)
	if len(brief.RequiredCapabilities) == 0 {
		brief.RequiredCapabilities = append([]string(nil), fallback.RequiredCapabilities...)
	}
	return brief
}

func normalizeAllowedPaths(paths []string, fallback []string) []string {
	allowed := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if askcontractPathAllowed(path) {
			allowed = append(allowed, path)
		}
	}
	if len(allowed) == 0 {
		return append([]string(nil), fallback...)
	}
	return dedupeStrings(allowed)
}

func normalizeCapabilities(values []string, fallback []string) []string {
	allowed := map[string]bool{
		"prepare-artifacts":    true,
		"package-staging":      true,
		"image-staging":        true,
		"repository-setup":     true,
		"kubeadm-bootstrap":    true,
		"kubeadm-join":         true,
		"cluster-verification": true,
	}
	canonical := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.ReplaceAll(value, "_", "-")
		value = strings.ReplaceAll(value, " ", "-")
		if value == "check-cluster" {
			value = "cluster-verification"
		}
		if value == "" || strings.ContainsAny(value, ":/()") || !allowed[value] {
			continue
		}
		canonical = append(canonical, value)
	}
	canonical = append(canonical, fallback...)
	return dedupeStrings(canonical)
}

func isCanonicalTargetScope(value string) bool {
	switch strings.TrimSpace(value) {
	case "workspace", "scenario", "vars", "component":
		return true
	default:
		return false
	}
}

func isCanonicalModeIntent(value string) bool {
	switch strings.TrimSpace(value) {
	case "prepare+apply", "prepare-only", "apply-only", "workspace":
		return true
	default:
		return false
	}
}

func isCanonicalCompleteness(value string) bool {
	switch strings.TrimSpace(value) {
	case "starter", "complete", "refine":
		return true
	default:
		return false
	}
}

func isCanonicalTopology(value string) bool {
	switch strings.TrimSpace(value) {
	case "single-node", "multi-node", "ha", "unspecified":
		return true
	default:
		return false
	}
}

func askcontractPathAllowed(path string) bool {
	return path == "workflows/prepare.yaml" || path == "workflows/vars.yaml" || strings.HasPrefix(path, "workflows/scenarios/") || strings.HasPrefix(path, "workflows/components/")
}

func EvaluatePlanConformance(plan askcontract.PlanResponse, gen askcontract.GenerationResponse, decision askintent.Decision) EvaluationResult {
	findings := []EvaluationFinding{}
	generated := generatedMap(gen.Files)
	planned := map[string]string{}
	for _, file := range plan.Files {
		clean := filepath.ToSlash(strings.TrimSpace(file.Path))
		planned[clean] = normalizePlannedAction(file.Action, clean)
	}
	for path := range planned {
		if _, ok := generated[path]; !ok {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "planned_file_missing", Message: fmt.Sprintf("planned file missing from generation: %s", path), Path: path})
		}
	}
	if planRequiresVarsFile(plan) {
		if _, ok := generated["workflows/vars.yaml"]; !ok {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "vars_required_by_checklist", Message: "validation checklist requires vars but workflows/vars.yaml was not generated", Path: "workflows/vars.yaml"})
		}
	}
	if decision.Route == askintent.RouteRefine && len(planned) > 0 {
		for _, file := range gen.Files {
			clean := filepath.ToSlash(strings.TrimSpace(file.Path))
			action, ok := planned[clean]
			if !ok {
				findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "refine_unplanned_file", Message: fmt.Sprintf("refine generated unplanned file: %s", clean), Fix: "Only update or create files declared in the plan during refine", Path: clean})
			}
			if action != "" && action != "update" && action != "create" {
				findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "invalid_planned_action", Message: fmt.Sprintf("invalid planned action for %s", clean), Path: clean})
			}
			if action == "update" && strings.HasPrefix(clean, "workflows/scenarios/") && strings.Contains(strings.ToLower(clean), "apply") {
				findings = append(findings, EvaluationFinding{Severity: "advisory", Code: "refine_updates_entry", Message: fmt.Sprintf("refine updates existing entry scenario: %s", clean), Path: clean})
			}
		}
	}
	if entry := filepath.ToSlash(strings.TrimSpace(plan.EntryScenario)); entry != "" {
		if _, ok := generated[entry]; !ok {
			findings = append(findings, EvaluationFinding{Severity: "blocking", Code: "entry_scenario_missing", Message: fmt.Sprintf("planned entry scenario missing from generation: %s", entry), Path: entry})
		}
	}
	return EvaluationResult{Findings: findings}
}

func planRequiresVarsFile(plan askcontract.PlanResponse) bool {
	for _, path := range plan.AuthoringBrief.TargetPaths {
		if path == "workflows/vars.yaml" {
			return true
		}
	}
	for _, file := range plan.Files {
		if filepath.ToSlash(strings.TrimSpace(file.Path)) == "workflows/vars.yaml" {
			return true
		}
	}
	return false
}

// ValidatePlanStructure enforces only pre-generation viability.
// Recoverable execution-detail weaknesses are carried forward into generation,
// judge, repair, and post-processing instead of stopping planning.
func ValidatePlanStructure(plan askcontract.PlanResponse) error {
	if plan.NeedsPrepare && !containsPlannedPath(plan.Files, "workflows/prepare.yaml") {
		return fmt.Errorf("plan response requires prepare but does not include workflows/prepare.yaml")
	}
	if strings.TrimSpace(plan.AuthoringBrief.ModeIntent) == "prepare+apply" {
		if !containsPlannedPath(plan.Files, "workflows/prepare.yaml") {
			return fmt.Errorf("plan response authoring brief requires prepare+apply but does not include workflows/prepare.yaml")
		}
		if entry := strings.TrimSpace(plan.EntryScenario); entry == "" || !containsPlannedPath(plan.Files, entry) {
			return fmt.Errorf("plan response authoring brief requires prepare+apply with a scenario entrypoint")
		}
	}
	if strings.TrimSpace(plan.AuthoringBrief.Topology) == "multi-node" || strings.TrimSpace(plan.AuthoringBrief.Topology) == "ha" {
		if strings.TrimSpace(plan.ExecutionModel.RoleExecution.RoleSelector) == "" && strings.TrimSpace(plan.AuthoringBrief.ModeIntent) != "prepare+apply" {
			return fmt.Errorf("plan response multi-node topology requires executionModel.roleExecution.roleSelector")
		}
		if plan.ExecutionModel.Verification.ExpectedNodeCount <= 0 && plan.AuthoringBrief.NodeCount <= 0 {
			return fmt.Errorf("plan response multi-node topology requires executionModel.verification.expectedNodeCount")
		}
	}
	if hasBriefCapability(plan.AuthoringBrief, "package-staging") && strings.TrimSpace(plan.AuthoringProgram.Platform.Family) == "" {
		return fmt.Errorf("plan response package authoring requires authoringProgram.platform.family")
	}
	if hasBriefCapability(plan.AuthoringBrief, "cluster-verification") && plan.AuthoringProgram.Verification.ExpectedNodeCount <= 0 {
		return fmt.Errorf("plan response cluster verification requires authoringProgram.verification.expectedNodeCount")
	}
	if hasBriefCapability(plan.AuthoringBrief, "kubeadm-bootstrap") && strings.TrimSpace(plan.AuthoringProgram.Cluster.JoinFile) == "" {
		return fmt.Errorf("plan response kubeadm authoring requires authoringProgram.cluster.joinFile")
	}
	return nil
}

func containsPlannedPath(files []askcontract.PlanFile, want string) bool {
	want = strings.TrimSpace(want)
	for _, file := range files {
		if strings.TrimSpace(file.Path) == want {
			return true
		}
	}
	return false
}

func normalizePlannedAction(action string, path string) string {
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
