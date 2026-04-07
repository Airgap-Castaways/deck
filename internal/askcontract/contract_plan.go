package askcontract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func ParsePlan(raw string) (PlanResponse, error) {
	return parsePlan(raw, true)
}

func ParsePlanPartial(raw string) (PlanResponse, error) {
	return parsePlan(raw, false)
}

func parsePlan(raw string, validate bool) (PlanResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PlanResponse{}, fmt.Errorf("plan response is empty")
	}
	if !validate {
		resp, err := parseLoosePlan(cleaned)
		if err == nil {
			normalizePlanResponse(&resp)
			return resp, nil
		}
	}
	var resp PlanResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		repaired := repairLooseJSON(cleaned)
		if repaired == cleaned || json.Unmarshal([]byte(repaired), &resp) != nil {
			loose, looseErr := parseLoosePlan(cleaned)
			if looseErr != nil {
				return PlanResponse{}, fmt.Errorf("parse plan response: %w", err)
			}
			resp = loose
		}
	}
	normalizePlanResponse(&resp)
	if !validate {
		return resp, nil
	}
	return validatePlanResponse(resp)
}

func normalizePlanResponse(resp *PlanResponse) {
	if resp == nil {
		return
	}
	if resp.Version == 0 {
		resp.Version = 1
	}
	resp.Request = strings.TrimSpace(resp.Request)
	resp.Intent = strings.TrimSpace(resp.Intent)
	resp.Complexity = strings.TrimSpace(resp.Complexity)
	resp.AuthoringBrief.RouteIntent = strings.TrimSpace(resp.AuthoringBrief.RouteIntent)
	resp.AuthoringBrief.TargetScope = strings.TrimSpace(resp.AuthoringBrief.TargetScope)
	resp.AuthoringBrief.PlatformFamily = strings.TrimSpace(resp.AuthoringBrief.PlatformFamily)
	resp.AuthoringBrief.EscapeHatchMode = strings.TrimSpace(resp.AuthoringBrief.EscapeHatchMode)
	resp.AuthoringBrief.ModeIntent = strings.TrimSpace(resp.AuthoringBrief.ModeIntent)
	resp.AuthoringBrief.Connectivity = strings.TrimSpace(resp.AuthoringBrief.Connectivity)
	resp.AuthoringBrief.CompletenessTarget = strings.TrimSpace(resp.AuthoringBrief.CompletenessTarget)
	resp.AuthoringBrief.Topology = strings.TrimSpace(resp.AuthoringBrief.Topology)
	resp.ExecutionModel.RoleExecution.RoleSelector = strings.TrimSpace(resp.ExecutionModel.RoleExecution.RoleSelector)
	resp.ExecutionModel.RoleExecution.ControlPlaneFlow = strings.TrimSpace(resp.ExecutionModel.RoleExecution.ControlPlaneFlow)
	resp.ExecutionModel.RoleExecution.WorkerFlow = strings.TrimSpace(resp.ExecutionModel.RoleExecution.WorkerFlow)
	resp.ExecutionModel.Verification.BootstrapPhase = strings.TrimSpace(resp.ExecutionModel.Verification.BootstrapPhase)
	resp.ExecutionModel.Verification.FinalPhase = strings.TrimSpace(resp.ExecutionModel.Verification.FinalPhase)
	resp.ExecutionModel.Verification.FinalVerificationRole = strings.TrimSpace(resp.ExecutionModel.Verification.FinalVerificationRole)
	resp.AuthoringProgram.Platform.Family = strings.TrimSpace(resp.AuthoringProgram.Platform.Family)
	resp.AuthoringProgram.Platform.Release = strings.TrimSpace(resp.AuthoringProgram.Platform.Release)
	resp.AuthoringProgram.Platform.RepoType = strings.TrimSpace(resp.AuthoringProgram.Platform.RepoType)
	resp.AuthoringProgram.Platform.BackendImage = strings.TrimSpace(resp.AuthoringProgram.Platform.BackendImage)
	resp.AuthoringProgram.Artifacts.PackageOutputDir = strings.TrimSpace(resp.AuthoringProgram.Artifacts.PackageOutputDir)
	resp.AuthoringProgram.Artifacts.ImageOutputDir = strings.TrimSpace(resp.AuthoringProgram.Artifacts.ImageOutputDir)
	resp.AuthoringProgram.Cluster.JoinFile = strings.TrimSpace(resp.AuthoringProgram.Cluster.JoinFile)
	resp.AuthoringProgram.Cluster.PodCIDR = strings.TrimSpace(resp.AuthoringProgram.Cluster.PodCIDR)
	resp.AuthoringProgram.Cluster.KubernetesVersion = strings.TrimSpace(resp.AuthoringProgram.Cluster.KubernetesVersion)
	resp.AuthoringProgram.Cluster.CriSocket = strings.TrimSpace(resp.AuthoringProgram.Cluster.CriSocket)
	resp.AuthoringProgram.Cluster.RoleSelector = strings.TrimSpace(resp.AuthoringProgram.Cluster.RoleSelector)
	resp.AuthoringProgram.Verification.FinalVerificationRole = strings.TrimSpace(resp.AuthoringProgram.Verification.FinalVerificationRole)
	resp.AuthoringProgram.Verification.Interval = strings.TrimSpace(resp.AuthoringProgram.Verification.Interval)
	resp.AuthoringProgram.Verification.Timeout = strings.TrimSpace(resp.AuthoringProgram.Verification.Timeout)
	resp.OfflineAssumption = strings.TrimSpace(resp.OfflineAssumption)
	resp.TargetOutcome = strings.TrimSpace(resp.TargetOutcome)
	resp.EntryScenario = strings.TrimSpace(resp.EntryScenario)
	for i := range resp.Clarifications {
		resp.Clarifications[i].ID = strings.TrimSpace(resp.Clarifications[i].ID)
		resp.Clarifications[i].Question = strings.TrimSpace(resp.Clarifications[i].Question)
		resp.Clarifications[i].Kind = strings.TrimSpace(resp.Clarifications[i].Kind)
		resp.Clarifications[i].Reason = strings.TrimSpace(resp.Clarifications[i].Reason)
		resp.Clarifications[i].Decision = strings.TrimSpace(resp.Clarifications[i].Decision)
		resp.Clarifications[i].RecommendedDefault = strings.TrimSpace(resp.Clarifications[i].RecommendedDefault)
		resp.Clarifications[i].Answer = strings.TrimSpace(resp.Clarifications[i].Answer)
		for j := range resp.Clarifications[i].Options {
			resp.Clarifications[i].Options[j] = strings.TrimSpace(resp.Clarifications[i].Options[j])
		}
		for j := range resp.Clarifications[i].Affects {
			resp.Clarifications[i].Affects[j] = strings.TrimSpace(resp.Clarifications[i].Affects[j])
		}
	}
	for i := range resp.AuthoringBrief.TargetPaths {
		resp.AuthoringBrief.TargetPaths[i] = strings.TrimSpace(resp.AuthoringBrief.TargetPaths[i])
	}
	for i := range resp.AuthoringBrief.AnchorPaths {
		resp.AuthoringBrief.AnchorPaths[i] = strings.TrimSpace(resp.AuthoringBrief.AnchorPaths[i])
	}
	for i := range resp.AuthoringBrief.AllowedCompanionPaths {
		resp.AuthoringBrief.AllowedCompanionPaths[i] = strings.TrimSpace(resp.AuthoringBrief.AllowedCompanionPaths[i])
	}
	for i := range resp.AuthoringBrief.DisallowedExpansionPaths {
		resp.AuthoringBrief.DisallowedExpansionPaths[i] = strings.TrimSpace(resp.AuthoringBrief.DisallowedExpansionPaths[i])
	}
	for i := range resp.AuthoringBrief.RequiredCapabilities {
		resp.AuthoringBrief.RequiredCapabilities[i] = strings.TrimSpace(resp.AuthoringBrief.RequiredCapabilities[i])
	}
	for i := range resp.AuthoringProgram.Artifacts.Packages {
		resp.AuthoringProgram.Artifacts.Packages[i] = strings.TrimSpace(resp.AuthoringProgram.Artifacts.Packages[i])
	}
	for i := range resp.AuthoringProgram.Artifacts.Images {
		resp.AuthoringProgram.Artifacts.Images[i] = strings.TrimSpace(resp.AuthoringProgram.Artifacts.Images[i])
	}
	for i := range resp.ExecutionModel.ArtifactContracts {
		resp.ExecutionModel.ArtifactContracts[i].Kind = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].Kind)
		resp.ExecutionModel.ArtifactContracts[i].ProducerPath = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].ProducerPath)
		resp.ExecutionModel.ArtifactContracts[i].ConsumerPath = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].ConsumerPath)
		resp.ExecutionModel.ArtifactContracts[i].Description = strings.TrimSpace(resp.ExecutionModel.ArtifactContracts[i].Description)
	}
	for i := range resp.ExecutionModel.SharedStateContracts {
		resp.ExecutionModel.SharedStateContracts[i].Name = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].Name)
		resp.ExecutionModel.SharedStateContracts[i].ProducerPath = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].ProducerPath)
		resp.ExecutionModel.SharedStateContracts[i].AvailabilityModel = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].AvailabilityModel)
		resp.ExecutionModel.SharedStateContracts[i].Description = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].Description)
		for j := range resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths {
			resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths[j] = strings.TrimSpace(resp.ExecutionModel.SharedStateContracts[i].ConsumerPaths[j])
		}
	}
	for i := range resp.ExecutionModel.ApplyAssumptions {
		resp.ExecutionModel.ApplyAssumptions[i] = strings.TrimSpace(resp.ExecutionModel.ApplyAssumptions[i])
	}
	for i := range resp.Blockers {
		resp.Blockers[i] = strings.TrimSpace(resp.Blockers[i])
	}
	for i := range resp.Assumptions {
		resp.Assumptions[i] = strings.TrimSpace(resp.Assumptions[i])
	}
	for i := range resp.OpenQuestions {
		resp.OpenQuestions[i] = strings.TrimSpace(resp.OpenQuestions[i])
	}
	for i := range resp.ValidationChecklist {
		resp.ValidationChecklist[i] = strings.TrimSpace(resp.ValidationChecklist[i])
	}
	for i := range resp.ArtifactKinds {
		resp.ArtifactKinds[i] = strings.TrimSpace(resp.ArtifactKinds[i])
	}
	for i := range resp.VarsRecommendation {
		resp.VarsRecommendation[i] = strings.TrimSpace(resp.VarsRecommendation[i])
	}
	for i := range resp.ComponentRecommendation {
		resp.ComponentRecommendation[i] = strings.TrimSpace(resp.ComponentRecommendation[i])
	}
}

func validatePlanResponse(resp PlanResponse) (PlanResponse, error) {
	if resp.Request == "" {
		return PlanResponse{}, fmt.Errorf("plan response is missing request")
	}
	if resp.Intent == "" {
		return PlanResponse{}, fmt.Errorf("plan response is missing intent")
	}
	if len(resp.Files) == 0 {
		return PlanResponse{}, fmt.Errorf("plan response is missing files")
	}
	seenClarifications := map[string]bool{}
	for _, item := range resp.Clarifications {
		if item.ID == "" {
			return PlanResponse{}, fmt.Errorf("plan response has clarification with empty id")
		}
		if item.Question == "" {
			return PlanResponse{}, fmt.Errorf("plan response clarification %q is missing question", item.ID)
		}
		if seenClarifications[item.ID] {
			return PlanResponse{}, fmt.Errorf("plan response has duplicate clarification id %q", item.ID)
		}
		seenClarifications[item.ID] = true
	}
	for i := range resp.Files {
		resp.Files[i].Path = strings.TrimSpace(resp.Files[i].Path)
		resp.Files[i].Kind = strings.TrimSpace(resp.Files[i].Kind)
		resp.Files[i].Action = strings.TrimSpace(resp.Files[i].Action)
		resp.Files[i].Purpose = strings.TrimSpace(resp.Files[i].Purpose)
		if resp.Files[i].Path == "" {
			return PlanResponse{}, fmt.Errorf("plan response has file with empty path")
		}
		if resp.Files[i].Action == "" {
			resp.Files[i].Action = "create"
		}
		switch resp.Files[i].Action {
		case "modify", "update", "create-or-modify", "create-or-update":
			if strings.HasPrefix(resp.Files[i].Path, "workflows/") {
				resp.Files[i].Action = "update"
			}
		case "create":
			// keep as-is
		}
		if !workspacepaths.IsAllowedAuthoringPath(resp.Files[i].Path) {
			return PlanResponse{}, fmt.Errorf("plan response has file outside allowed ask paths: %s", resp.Files[i].Path)
		}
	}
	if resp.EntryScenario != "" {
		if resolved := resolvePlannedEntryScenario(resp.EntryScenario, resp.Files); resolved != "" {
			resp.EntryScenario = resolved
		}
		if !workspacepaths.IsAllowedAuthoringPath(resp.EntryScenario) || !workspacepaths.IsScenarioAuthoringPath(resp.EntryScenario) {
			return PlanResponse{}, fmt.Errorf("plan response entryScenario must be a scenario path under %s/: %s", workspacepaths.CanonicalScenariosDir, resp.EntryScenario)
		}
		matched := false
		for _, file := range resp.Files {
			if file.Path == resp.EntryScenario {
				matched = true
				break
			}
		}
		if !matched {
			return PlanResponse{}, fmt.Errorf("plan response entryScenario must match a planned file: %s", resp.EntryScenario)
		}
	}
	return resp, nil
}

func parseLoosePlan(cleaned string) (PlanResponse, error) {
	var loose planResponseLoose
	if err := json.Unmarshal([]byte(cleaned), &loose); err != nil {
		repaired := repairLooseJSON(cleaned)
		if repaired == cleaned || json.Unmarshal([]byte(repaired), &loose) != nil {
			return PlanResponse{}, err
		}
	}
	resp := PlanResponse{
		Version:                 loose.Version,
		Request:                 loose.Request,
		Intent:                  loose.Intent,
		Complexity:              loose.Complexity,
		AuthoringBrief:          loose.AuthoringBrief.toStrict(),
		AuthoringProgram:        loose.AuthoringProgram.toStrict(),
		ExecutionModel:          loose.ExecutionModel.toStrict(),
		OfflineAssumption:       loose.OfflineAssumption,
		NeedsPrepare:            loose.NeedsPrepare,
		ArtifactKinds:           []string(loose.ArtifactKinds),
		VarsRecommendation:      []string(loose.VarsRecommendation),
		ComponentRecommendation: []string(loose.ComponentRecommendation),
		Blockers:                []string(loose.Blockers),
		TargetOutcome:           loose.TargetOutcome,
		Assumptions:             []string(loose.Assumptions),
		OpenQuestions:           []string(loose.OpenQuestions),
		EntryScenario:           loose.EntryScenario,
		Files:                   loose.Files,
		ValidationChecklist:     []string(loose.ValidationChecklist),
	}
	for _, item := range loose.Clarifications {
		resp.Clarifications = append(resp.Clarifications, item.toStrict())
	}
	return resp, nil
}

func (b authoringBriefLoose) toStrict() AuthoringBrief {
	return AuthoringBrief{
		RouteIntent:              b.RouteIntent,
		TargetScope:              b.TargetScope,
		TargetPaths:              []string(b.TargetPaths),
		AnchorPaths:              []string(b.AnchorPaths),
		AllowedCompanionPaths:    []string(b.AllowedCompanionPaths),
		DisallowedExpansionPaths: []string(b.DisallowedExpansionPaths),
		ModeIntent:               b.ModeIntent,
		Connectivity:             b.Connectivity,
		CompletenessTarget:       b.CompletenessTarget,
		Topology:                 b.Topology,
		NodeCount:                b.NodeCount,
		PlatformFamily:           b.PlatformFamily,
		EscapeHatchMode:          b.EscapeHatchMode,
		RequiredCapabilities:     []string(b.RequiredCapabilities),
	}
}

func (p authoringProgramLoose) toStrict() AuthoringProgram {
	return AuthoringProgram{
		Platform: p.Platform,
		Artifacts: ProgramArtifacts{
			Packages:         []string(p.Artifacts.Packages),
			Images:           []string(p.Artifacts.Images),
			PackageOutputDir: p.Artifacts.PackageOutputDir,
			ImageOutputDir:   p.Artifacts.ImageOutputDir,
		},
		Cluster:      p.Cluster,
		Verification: p.Verification,
	}
}

func (m executionModelLoose) toStrict() ExecutionModel {
	out := ExecutionModel{
		ArtifactContracts: append([]ArtifactContract(nil), m.ArtifactContracts...),
		RoleExecution:     m.RoleExecution,
		Verification:      m.Verification,
		ApplyAssumptions:  []string(m.ApplyAssumptions),
	}
	for _, item := range m.SharedStateContracts {
		out.SharedStateContracts = append(out.SharedStateContracts, SharedStateContract{
			Name:              item.Name,
			ProducerPath:      item.ProducerPath,
			ConsumerPaths:     []string(item.ConsumerPaths),
			AvailabilityModel: item.AvailabilityModel,
			Description:       item.Description,
		})
	}
	return out
}

func (c planClarification) toStrict() PlanClarification {
	return PlanClarification{
		ID:                 c.ID,
		Question:           c.Question,
		Kind:               c.Kind,
		Reason:             c.Reason,
		Decision:           c.Decision,
		Options:            []string(c.Options),
		RecommendedDefault: c.RecommendedDefault,
		Answer:             c.Answer,
		BlocksGeneration:   c.BlocksGeneration,
		Affects:            []string(c.Affects),
	}
}

func resolvePlannedEntryScenario(entry string, files []PlanFile) string {
	entry = filepath.ToSlash(strings.TrimSpace(entry))
	if workspacepaths.IsScenarioAuthoringPath(entry) {
		return entry
	}
	matches := []string{}
	for _, file := range files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if !workspacepaths.IsScenarioAuthoringPath(path) {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if entry == path || entry == filepath.Base(path) || entry == base {
			matches = append(matches, path)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}
