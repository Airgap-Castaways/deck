package askcontract

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func ParseInfo(raw string) InfoResponse {
	cleaned := clean(raw)
	if cleaned == "" {
		return InfoResponse{Summary: "No response returned.", Answer: ""}
	}
	var resp InfoResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		trimmed := strings.TrimSpace(raw)
		return InfoResponse{Summary: "Answer", Answer: trimmed}
	}
	if strings.TrimSpace(resp.Summary) == "" {
		resp.Summary = "Answer"
	}
	if strings.TrimSpace(resp.Answer) == "" {
		resp.Answer = strings.TrimSpace(raw)
	}
	return resp
}

func ParseClassification(raw string) (ClassificationResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return ClassificationResponse{}, fmt.Errorf("classification response is empty")
	}
	var resp ClassificationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return ClassificationResponse{}, fmt.Errorf("parse classification response: %w", err)
	}
	resp.Route = strings.TrimSpace(resp.Route)
	resp.Reason = strings.TrimSpace(resp.Reason)
	resp.Target.Kind = strings.TrimSpace(resp.Target.Kind)
	resp.Target.Path = strings.TrimSpace(resp.Target.Path)
	resp.Target.Name = strings.TrimSpace(resp.Target.Name)
	if resp.Confidence < 0 {
		resp.Confidence = 0
	}
	if resp.Confidence > 1 {
		resp.Confidence = 1
	}
	return resp, nil
}

func normalizeEvidenceDecision(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "required", "require", "must":
		return "required"
	case "optional", "recommended", "prefer":
		return "optional"
	case "unnecessary", "none", "skip", "not-needed":
		return "unnecessary"
	default:
		return ""
	}
}

func ParseJudge(raw string) (JudgeResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return JudgeResponse{}, fmt.Errorf("judge response is empty")
	}
	var resp JudgeResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return JudgeResponse{}, fmt.Errorf("parse judge response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.MissingCapabilities {
		resp.MissingCapabilities[i] = strings.TrimSpace(resp.MissingCapabilities[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	return resp, nil
}

func ParsePlanCritic(raw string) (PlanCriticResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PlanCriticResponse{}, fmt.Errorf("plan critic response is empty")
	}
	var resp PlanCriticResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return PlanCriticResponse{}, fmt.Errorf("parse plan critic response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.MissingContracts {
		resp.MissingContracts[i] = strings.TrimSpace(resp.MissingContracts[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	for i := range resp.Findings {
		resp.Findings[i].Code = workflowissues.Code(strings.TrimSpace(string(resp.Findings[i].Code)))
		resp.Findings[i].Severity = workflowissues.Severity(strings.TrimSpace(string(resp.Findings[i].Severity)))
		resp.Findings[i].Message = strings.TrimSpace(resp.Findings[i].Message)
		resp.Findings[i].Path = strings.TrimSpace(resp.Findings[i].Path)
		if resp.Findings[i].Code == "" {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding is missing code")
		}
		if !workflowissues.IsSupportedCriticCode(resp.Findings[i].Code) {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q uses unsupported code", resp.Findings[i].Code)
		}
		if resp.Findings[i].Message == "" {
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q is missing message", resp.Findings[i].Code)
		}
		switch resp.Findings[i].Severity {
		case workflowissues.SeverityBlocking, workflowissues.SeverityAdvisory, workflowissues.SeverityMissingContract:
			// ok
		case "":
			resp.Findings[i].Severity = workflowissues.SeverityAdvisory
		default:
			return PlanCriticResponse{}, fmt.Errorf("plan critic finding %q has invalid severity %q", resp.Findings[i].Code, resp.Findings[i].Severity)
		}
	}
	return resp, nil
}

func ParsePostProcess(raw string) (PostProcessResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return PostProcessResponse{}, fmt.Errorf("post-process response is empty")
	}
	var resp PostProcessResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return PostProcessResponse{}, fmt.Errorf("parse post-process response: %w", err)
	}
	resp.Summary = strings.TrimSpace(resp.Summary)
	for i := range resp.Blocking {
		resp.Blocking[i] = strings.TrimSpace(resp.Blocking[i])
	}
	for i := range resp.Advisory {
		resp.Advisory[i] = strings.TrimSpace(resp.Advisory[i])
	}
	for i := range resp.UpgradeCandidates {
		resp.UpgradeCandidates[i] = strings.TrimSpace(resp.UpgradeCandidates[i])
	}
	for i := range resp.ReviseFiles {
		resp.ReviseFiles[i] = strings.TrimSpace(resp.ReviseFiles[i])
	}
	for i := range resp.PreserveFiles {
		resp.PreserveFiles[i] = strings.TrimSpace(resp.PreserveFiles[i])
	}
	for i := range resp.RequiredEdits {
		resp.RequiredEdits[i] = strings.TrimSpace(resp.RequiredEdits[i])
	}
	for i := range resp.VerificationExpectations {
		resp.VerificationExpectations[i] = strings.TrimSpace(resp.VerificationExpectations[i])
	}
	for i := range resp.SuggestedFixes {
		resp.SuggestedFixes[i] = strings.TrimSpace(resp.SuggestedFixes[i])
	}
	return resp, nil
}

func clean(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}
	return strings.TrimSpace(response)
}

func repairLooseJSON(response string) string {
	response = strings.ReplaceAll(response, ",]", "]")
	response = strings.ReplaceAll(response, ", }", " }")
	response = strings.ReplaceAll(response, ",}", "}")
	response = strings.ReplaceAll(response, ",\n]", "\n]")
	response = strings.ReplaceAll(response, ",\n}", "\n}")
	return response
}
