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
	"github.com/Airgap-Castaways/deck/internal/askrepair"
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
			if repairedFiles, autoNotes, autoApplied, autoErr := askrepair.TryAutoRepairWithProgram(root, lastFiles, repairDiags, repairPaths, plan.AuthoringProgram); autoErr == nil && autoApplied {
				autoGen := askcontract.GenerationResponse{Summary: "generated workflows after automatic repair", Review: autoNotes, Files: append([]askcontract.GeneratedFile(nil), repairedFiles...)}
				lintSummary, critic, validateErr := validateGeneration(ctx, root, autoGen, repairedFiles, decision, plan, brief, retrieval)
				lastFiles = repairedFiles
				lastCritic = critic
				if validateErr == nil {
					judge, judgeErr := maybeJudgeGeneration(ctx, client, req, autoGen, repairedFiles, lintSummary, critic, plan, brief, logger)
					if judgeErr == nil {
						lastJudge = judge
						critic = mergeJudgeIntoCritic(critic, judge, attempt == attempts)
						if len(judge.Blocking) > 0 && attempt < attempts {
							lastValidation = "semantic judge requested revision: " + strings.Join(judge.Blocking, "; ")
							lastValidationErr = nil
							lastCritic = critic
							continue
						}
					}
					logger.debug("repair_auto_applied", "phase", "repair", "applied", len(autoNotes))
					return autoGen, repairedFiles, lintSummary, critic, lastJudge, attempt - 1, nil
				}
				lastValidation = validateErr.Error()
			}
			if isGenerationParseFailure(lastValidation) {
				currentPrompt = jsonResponseRetryPrompt(req.Prompt, lastValidation, decision.Route)
			} else {
				repairDiags = append(repairDiags, askdiagnostic.FromPlanCritic(planCritic)...)
				repairDiags = append(repairDiags, askdiagnostic.FromCritic(lastCritic)...)
				logger.trace("repair_diagnostics", "phase", "repair", "content", askdiagnostic.JSON(repairDiags))
				if decision.Route == askintent.RouteDraft && !legacyAuthoringFallbackEnabled() {
					currentPrompt = draftSelectionRetryPrompt(req.Prompt, lastValidation, repairDiags)
				} else {
					currentSystemPrompt = strings.TrimSpace(req.SystemPrompt) + "\n\n" + documentRepairSystemPrompt(normalizedAuthoringBrief(plan, brief), plan)
					currentPrompt = documentRepairUserPrompt(lastFiles, lastValidation, repairDiags, repairPaths)
				}
			}
		}
		logger.info("attempt_started", "phase", "generation", "attempt", attempt, "max_attempts", attempts)
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
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, err
		}
		logger.response("generation", resp.Content)
		gen, err := askcontract.ParseGeneration(resp.Content)
		if err != nil {
			lastValidation = err.Error()
			lastValidationErr = err
			logger.debug("attempt_failed", "phase", "generation", "attempt", attempt, "reason", "parse-error", "error", lastValidation)
			if !repairableValidationError(lastValidation) {
				return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
			}
			if attempt < attempts {
				continue
			}
			if len(lastFiles) > 0 && len(repairDiags) > 0 {
				if repairedFiles, autoNotes, autoApplied, autoErr := askrepair.TryAutoRepairWithProgram(root, lastFiles, repairDiags, repairPaths, plan.AuthoringProgram); autoErr == nil && autoApplied {
					autoGen := askcontract.GenerationResponse{Summary: "generated workflows after automatic repair", Review: autoNotes, Files: append([]askcontract.GeneratedFile(nil), repairedFiles...)}
					lintSummary, critic, validateErr := validateGeneration(ctx, root, autoGen, repairedFiles, decision, plan, brief, retrieval)
					if validateErr == nil {
						judge, judgeErr := maybeJudgeGeneration(ctx, client, req, autoGen, repairedFiles, lintSummary, critic, plan, brief, logger)
						if judgeErr == nil {
							lastJudge = judge
							critic = mergeJudgeIntoCritic(critic, judge, true)
						}
						logger.debug("repair_auto_applied", "phase", "repair", "source", "parse", "applied", len(autoNotes))
						return autoGen, repairedFiles, lintSummary, critic, lastJudge, attempt - 1, nil
					}
				}
			}
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation returned invalid JSON: %s", lastValidation)
		}
		if err := validatePrimaryAuthoringContract(decision.Route, gen, attempt); err != nil {
			lastValidation = err.Error()
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
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
		gen.Program = &plan.AuthoringProgram
		files, err := askir.MaterializeWithBase(root, lastFiles, gen)
		if err != nil {
			lastValidation = err.Error()
			lastValidationErr = err
			logger.debug("attempt_failed", "phase", "generation", "attempt", attempt, "reason", "materialize-error", "error", lastValidation)
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
		logger.debug("phase_started", "phase", "semantic-validate", "attempt", attempt, "max_attempts", attempts)
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
					logger.debug("phase_retry", "phase", "judge", "blocking", len(judge.Blocking))
					continue
				}
			} else {
				logger.debug("phase_skipped", "phase", "judge", "error", judgeErr)
			}
			return gen, files, lintSummary, critic, lastJudge, attempt - 1, nil
		}
		lastValidation = err.Error()
		lastValidationErr = err
		logger.debug("attempt_failed", "phase", "generation", "attempt", attempt, "reason", "validation-error", "error", lastValidation)
		if !repairableValidationError(lastValidation) {
			return askcontract.GenerationResponse{}, nil, lastValidation, critic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
		}
	}
	if lastValidation == "" {
		lastValidation = "generation failed without a parseable response"
	}
	if len(lastFiles) > 0 && lastValidation != "" {
		validationDiags := askdiagnostic.FromValidationError(lastValidationErr, lastValidation, bundle)
		repairPaths := repairTargetFiles(lastFiles, validationDiags, taintedFiles)
		repairPaths = restrictRepairTargetsToPlan(repairPaths, plan)
		if repairedFiles, autoNotes, autoApplied, autoErr := askrepair.TryAutoRepairWithProgram(root, lastFiles, validationDiags, repairPaths, plan.AuthoringProgram); autoErr == nil && autoApplied {
			autoGen := askcontract.GenerationResponse{Summary: "generated workflows after automatic repair", Review: autoNotes, Files: append([]askcontract.GeneratedFile(nil), repairedFiles...)}
			lintSummary, critic, validateErr := validateGeneration(ctx, root, autoGen, repairedFiles, decision, plan, brief, retrieval)
			if validateErr == nil {
				judge, judgeErr := maybeJudgeGeneration(ctx, client, req, autoGen, repairedFiles, lintSummary, critic, plan, brief, logger)
				if judgeErr == nil {
					lastJudge = judge
					critic = mergeJudgeIntoCritic(critic, judge, true)
				}
				logger.debug("repair_auto_applied", "phase", "repair", "source", "final", "applied", len(autoNotes))
				return autoGen, repairedFiles, lintSummary, critic, lastJudge, attempts - 1, nil
			}
			lastValidation = validateErr.Error()
			lastCritic = critic
		}
	}
	return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempts - 1, fmt.Errorf("ask generation did not validate after %d attempts: %s", attempts, lastValidation)
}

func draftSelectionRetryPrompt(base string, lastValidation string, diags []askdiagnostic.Diagnostic) string {
	b := &strings.Builder{}
	b.WriteString(strings.TrimSpace(base))
	b.WriteString("\n\nRetry requirements:\n")
	b.WriteString("- Return draft selection only under selection.targets[].builders.\n")
	b.WriteString("- Do not return documents or raw step specs.\n")
	b.WriteString("- Do not set low-level path, runtime, repo, distro, source, or role-expression overrides when the authoring program already provides them.\n")
	b.WriteString("- Prefer selecting the minimal recipe set and let code derive canonical fields from source-of-truth metadata.\n")
	if strings.TrimSpace(lastValidation) != "" {
		b.WriteString("Previous attempt failed validation/materialization: ")
		b.WriteString(strings.TrimSpace(lastValidation))
		b.WriteString("\n")
	}
	if len(diags) > 0 {
		b.WriteString("Structured diagnostics:\n")
		for _, diag := range diags {
			message := strings.TrimSpace(diag.Message)
			if message == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(message)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
