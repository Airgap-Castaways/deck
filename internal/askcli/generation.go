package askcli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func generateWithValidation(ctx context.Context, client askprovider.Client, req askprovider.Request, root string, attempts int, logger askLogger, decision askintent.Decision, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, retrieval askretrieve.RetrievalResult, planCritic askcontract.PlanCriticResponse) (askcontract.GenerationResponse, []askcontract.GeneratedFile, string, askcontract.CriticResponse, askcontract.JudgeResponse, int, error) {
	_ = planCritic
	var lastValidation string
	var lastValidationErr error
	var lastCritic askcontract.CriticResponse
	var lastJudge askcontract.JudgeResponse
	var lastFiles []askcontract.GeneratedFile
	taintedFiles := map[string]bool{}
	bundle := askknowledge.Current()
	for attempt := 1; attempt <= attempts; attempt++ {
		currentPrompt := req.Prompt
		currentSystemPrompt := req.SystemPrompt
		var repairDiags []askdiagnostic.Diagnostic
		var repairPaths []string
		if attempt > 1 && lastValidation != "" {
			validationDiags := askdiagnostic.FromValidationError(lastValidationErr, lastValidation, bundle)
			markTaintedFiles(taintedFiles, validationDiags)
			repairPaths = repairTargetFiles(lastFiles, validationDiags, taintedFiles)
			repairPaths = restrictRepairTargetsToPlan(repairPaths, plan)
			repairDiags = append([]askdiagnostic.Diagnostic{}, validationDiags...)
			if isGenerationParseFailure(lastValidation) {
				currentPrompt = jsonResponseRetryPrompt(req.Prompt, lastValidation, decision.Route)
			} else {
				repairDiags = append(repairDiags, askdiagnostic.FromPlanCritic(planCritic)...)
				repairDiags = append(repairDiags, askdiagnostic.FromCritic(lastCritic)...)
				logger.logf("debug", "\n[ask][phase:repair:diagnostics]\n%s\n", askdiagnostic.JSON(repairDiags))
				currentSystemPrompt = strings.TrimSpace(req.SystemPrompt) + "\n\n" + documentRepairSystemPrompt(normalizedAuthoringBrief(plan, brief), plan)
				currentPrompt = documentRepairUserPrompt(lastFiles, lastValidation, repairDiags, repairPaths)
			}
		}
		logger.logf("basic", "[ask][phase:generation:attempt] attempt=%d/%d\n", attempt, attempts)
		logger.prompt("generation", currentSystemPrompt, currentPrompt)
		resp, err := client.Generate(ctx, askprovider.Request{
			Kind:         req.Kind,
			Provider:     req.Provider,
			Model:        req.Model,
			APIKey:       req.APIKey,
			OAuthToken:   req.OAuthToken,
			Endpoint:     req.Endpoint,
			SystemPrompt: currentSystemPrompt,
			Prompt:       currentPrompt,
			MaxRetries:   providerRetryCount(req.Kind),
			Timeout:      askRequestTimeout(req.Kind, attempts, currentSystemPrompt, currentPrompt),
		})
		if err != nil {
			lastValidationErr = err
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, err
		}
		logger.response("generation", resp.Content)
		gen, err := askcontract.ParseGeneration(resp.Content)
		if err != nil {
			lastValidation = err.Error()
			lastValidationErr = err
			logger.logf("debug", "[ask][phase:generation:parse-error] error=%s\n", lastValidation)
			if !repairableValidationError(lastValidation) {
				return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
			}
			if attempt < attempts {
				continue
			}
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation returned invalid JSON: %s", lastValidation)
		}
		if attempt > 1 {
			if err := validateRepairDocumentStrategy(gen.Documents, repairDiags, repairPaths, decision.Route); err != nil {
				lastValidation = err.Error()
				lastValidationErr = err
				if attempt < attempts {
					continue
				}
				return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, err
			}
		}
		files, err := askir.MaterializeWithBase(root, lastFiles, gen)
		if err != nil {
			lastValidation = err.Error()
			lastValidationErr = err
			logger.logf("debug", "[ask][phase:generation:materialize-error] error=%s\n", lastValidation)
			if attempt < attempts {
				continue
			}
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation returned invalid document payload: %s", lastValidation)
		}
		if attempt > 1 && len(lastFiles) > 0 {
			files = mergeGeneratedFiles(dropGeneratedFiles(lastFiles, mapKeys(taintedFiles)), files)
		}
		files = normalizeGeneratedFiles(files)
		gen.Files = append([]askcontract.GeneratedFile(nil), files...)
		logger.logf("debug", "[ask][phase:semantic-validate] attempt=%d/%d\n", attempt, attempts)
		lastFiles = files
		lintSummary, critic, err := validateGeneration(ctx, root, gen, files, decision, plan, brief, retrieval)
		lastCritic = critic
		if err == nil {
			judge, judgeErr := maybeJudgeGeneration(ctx, client, req, gen, files, lintSummary, critic, plan, brief, logger)
			if judgeErr == nil {
				lastJudge = judge
				critic = mergeJudgeIntoCritic(critic, judge, attempt == attempts)
				if len(judge.Blocking) > 0 && attempt < attempts {
					lastValidation = "semantic judge requested revision: " + strings.Join(judge.Blocking, "; ")
					lastValidationErr = nil
					lastCritic = critic
					logger.logf("debug", "[ask][phase:judge:retry] blocking=%d\n", len(judge.Blocking))
					continue
				}
			} else {
				logger.logf("debug", "[ask][phase:judge:skip] error=%v\n", judgeErr)
			}
			return gen, files, lintSummary, critic, lastJudge, attempt - 1, nil
		}
		lastValidation = err.Error()
		lastValidationErr = err
		logger.logf("debug", "[ask][phase:generation:validation-error] error=%s\n", lastValidation)
		if !repairableValidationError(lastValidation) {
			return askcontract.GenerationResponse{}, nil, lastValidation, critic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
		}
	}
	if lastValidation == "" {
		lastValidation = "generation failed without a parseable response"
	}
	return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempts - 1, fmt.Errorf("ask generation did not validate after %d attempts: %s", attempts, lastValidation)
}
