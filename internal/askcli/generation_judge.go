package askcli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
)

func maybeJudgeGeneration(ctx context.Context, client askprovider.Client, req askprovider.Request, gen askcontract.GenerationResponse, files []askcontract.GeneratedFile, lintSummary string, critic askcontract.CriticResponse, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, logger askLogger) (askcontract.JudgeResponse, error) {
	if strings.TrimSpace(req.Kind) != "generate" {
		return askcontract.JudgeResponse{}, fmt.Errorf("judge disabled for default generation path")
	}
	if strings.TrimSpace(brief.RouteIntent) == "" {
		return askcontract.JudgeResponse{}, fmt.Errorf("judge skipped without authoring brief")
	}
	systemPrompt := judgeSystemPrompt(brief, plan)
	userPrompt := judgeUserPrompt(files, lintSummary, critic)
	logger.prompt("judge", systemPrompt, userPrompt)
	resp, err := client.Generate(ctx, askprovider.Request{
		Kind:         "judge",
		Provider:     req.Provider,
		Model:        req.Model,
		APIKey:       req.APIKey,
		OAuthToken:   req.OAuthToken,
		AccountID:    req.AccountID,
		Endpoint:     req.Endpoint,
		SystemPrompt: systemPrompt,
		Prompt:       userPrompt,
		MaxRetries:   providerRetryCount("judge"),
		Timeout:      askRequestTimeout("judge", 1, systemPrompt, userPrompt),
	})
	if err != nil {
		return askcontract.JudgeResponse{}, err
	}
	logger.response("judge", resp.Content)
	return askcontract.ParseJudge(resp.Content)
}

func mergeJudgeIntoCritic(critic askcontract.CriticResponse, judge askcontract.JudgeResponse, finalAttempt bool) askcontract.CriticResponse {
	critic.Advisory = append(critic.Advisory, judge.Advisory...)
	critic.Advisory = append(critic.Advisory, judge.MissingCapabilities...)
	if finalAttempt {
		critic.Advisory = append(critic.Advisory, judge.Blocking...)
	}
	critic.RequiredFixes = append(critic.RequiredFixes, judge.SuggestedFixes...)
	critic.Advisory = dedupe(critic.Advisory)
	critic.RequiredFixes = dedupe(critic.RequiredFixes)
	return critic
}
