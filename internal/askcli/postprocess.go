package askcli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

type postProcessSummary struct {
	Applied     bool
	Generation  askcontract.GenerationResponse
	Files       []askcontract.GeneratedFile
	LintSummary string
	Critic      askcontract.CriticResponse
	Judge       askcontract.JudgeResponse
	Notes       []string
}

func maybePostProcessGeneration(ctx context.Context, client askprovider.Client, req askprovider.Request, root string, logger askLogger, decision askintent.Decision, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, retrieval askretrieve.RetrievalResult, gen askcontract.GenerationResponse, files []askcontract.GeneratedFile, lintSummary string, critic askcontract.CriticResponse, judge askcontract.JudgeResponse, planCritic askcontract.PlanCriticResponse) (postProcessSummary, error) {
	if !shouldAutoPostProcess(plan, brief, critic, judge, files) {
		return postProcessSummary{}, fmt.Errorf("post-process not needed")
	}
	findings, err := critiquePostProcess(ctx, client, req, plan, brief, files, judge, critic, planCritic, logger)
	if err != nil {
		return postProcessSummary{}, err
	}
	notes := renderPostProcessNotes(findings)
	if len(findings.Blocking) == 0 {
		return postProcessSummary{Applied: false, Notes: notes}, nil
	}
	edited, err := applyPostProcessEdit(ctx, client, req, plan, brief, findings, files, planCritic, logger)
	if err != nil {
		return postProcessSummary{}, err
	}
	editedFiles, err := askir.MaterializeWithBase(root, files, edited)
	if err != nil {
		return postProcessSummary{}, err
	}
	edited.Files = append([]askcontract.GeneratedFile(nil), editedFiles...)
	newLint, newCritic, err := validateGeneration(ctx, root, edited, editedFiles, decision, plan, brief, retrieval)
	if err != nil {
		return postProcessSummary{}, err
	}
	newJudge, err := maybeJudgeGeneration(ctx, client, req, edited, editedFiles, newLint, newCritic, plan, brief, logger)
	if err != nil {
		logger.logf("debug", "[ask][phase:postprocess:judge-skip] error=%v\n", err)
		newJudge = judge
	}
	return postProcessSummary{Applied: true, Generation: edited, Files: editedFiles, LintSummary: newLint, Critic: newCritic, Judge: newJudge, Notes: append([]string{"post-process: applied targeted operational refinement"}, notes...)}, nil
}

func shouldAutoPostProcess(plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, critic askcontract.CriticResponse, judge askcontract.JudgeResponse, files []askcontract.GeneratedFile) bool {
	if len(files) == 0 {
		return false
	}
	if strings.TrimSpace(brief.CompletenessTarget) != "complete" {
		return false
	}
	if strings.TrimSpace(brief.ModeIntent) == "apply-only" && len(files) < 2 {
		return false
	}
	if len(critic.RequiredFixes) > 0 || len(critic.Blocking) > 0 || len(judge.Blocking) > 0 {
		return true
	}
	if strings.TrimSpace(brief.ModeIntent) == "prepare+apply" && len(files) >= 2 {
		return true
	}
	return false
}

func critiquePostProcess(ctx context.Context, client askprovider.Client, req askprovider.Request, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, files []askcontract.GeneratedFile, judge askcontract.JudgeResponse, critic askcontract.CriticResponse, planCritic askcontract.PlanCriticResponse, logger askLogger) (askcontract.PostProcessResponse, error) {
	systemPrompt := postProcessCriticSystemPrompt(brief, plan)
	userPrompt := postProcessCriticUserPrompt(plan, files, judge, critic, planCritic)
	logger.prompt("postprocess-critic", systemPrompt, userPrompt)
	resp, err := client.Generate(ctx, askprovider.Request{Kind: "postprocess-critic", Provider: req.Provider, Model: req.Model, APIKey: req.APIKey, OAuthToken: req.OAuthToken, AccountID: req.AccountID, Endpoint: req.Endpoint, SystemPrompt: systemPrompt, Prompt: userPrompt, MaxRetries: providerRetryCount("postprocess-critic"), Timeout: askRequestTimeout("postprocess-critic", 1, systemPrompt, userPrompt)})
	if err != nil {
		return askcontract.PostProcessResponse{}, err
	}
	logger.response("postprocess-critic", resp.Content)
	parsed, err := askcontract.ParsePostProcess(resp.Content)
	if err != nil {
		return askcontract.PostProcessResponse{}, err
	}
	return enrichPostProcessFindings(parsed, files), nil
}

func applyPostProcessEdit(ctx context.Context, client askprovider.Client, req askprovider.Request, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, findings askcontract.PostProcessResponse, files []askcontract.GeneratedFile, planCritic askcontract.PlanCriticResponse, logger askLogger) (askcontract.GenerationResponse, error) {
	systemPrompt := postProcessEditSystemPrompt(brief, plan)
	userPrompt := postProcessEditUserPrompt(files, findings, planCritic)
	logger.prompt("postprocess-edit", systemPrompt, userPrompt)
	resp, err := client.Generate(ctx, askprovider.Request{Kind: "postprocess-edit", Provider: req.Provider, Model: req.Model, APIKey: req.APIKey, OAuthToken: req.OAuthToken, AccountID: req.AccountID, Endpoint: req.Endpoint, SystemPrompt: systemPrompt, Prompt: userPrompt, MaxRetries: providerRetryCount("postprocess-edit"), Timeout: askRequestTimeout("postprocess-edit", 1, systemPrompt, userPrompt)})
	if err != nil {
		return askcontract.GenerationResponse{}, err
	}
	logger.response("postprocess-edit", resp.Content)
	return askcontract.ParseGeneration(resp.Content)
}

func renderPostProcessNotes(findings askcontract.PostProcessResponse) []string {
	lines := []string{}
	if strings.TrimSpace(findings.Summary) != "" {
		lines = append(lines, "post-process review: "+strings.TrimSpace(findings.Summary))
	}
	for _, item := range findings.Advisory {
		if strings.TrimSpace(item) != "" {
			lines = append(lines, "post-process advisory: "+strings.TrimSpace(item))
		}
	}
	for _, item := range findings.UpgradeCandidates {
		if strings.TrimSpace(item) != "" {
			lines = append(lines, "post-process candidate: "+strings.TrimSpace(item))
		}
	}
	for _, item := range findings.RequiredEdits {
		if strings.TrimSpace(item) != "" {
			lines = append(lines, "post-process required edit: "+strings.TrimSpace(item))
		}
	}
	return lines
}

func enrichPostProcessFindings(findings askcontract.PostProcessResponse, rendered []askcontract.GeneratedFile) askcontract.PostProcessResponse {
	files := filePathSet(rendered)
	if len(findings.ReviseFiles) == 0 && len(findings.Blocking) > 0 {
		if files["workflows/scenarios/apply.yaml"] {
			findings.ReviseFiles = append(findings.ReviseFiles, "workflows/scenarios/apply.yaml")
		}
	}
	for path := range files {
		if !containsTrimmed(findings.ReviseFiles, path) && !containsTrimmed(findings.PreserveFiles, path) {
			findings.PreserveFiles = append(findings.PreserveFiles, path)
		}
	}
	if len(findings.Blocking) == 0 {
		findings.ReviseFiles = nil
	}
	findings.PreserveFiles = dedupe(findings.PreserveFiles)
	findings.ReviseFiles = dedupe(findings.ReviseFiles)
	return findings
}

func filePathSet(files []askcontract.GeneratedFile) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		path := strings.TrimSpace(file.Path)
		if path != "" {
			out[path] = true
		}
	}
	return out
}

func containsTrimmed(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
