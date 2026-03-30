package askcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

func generateWithValidation(ctx context.Context, client askprovider.Client, req askprovider.Request, root string, attempts int, logger askLogger, decision askintent.Decision, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, retrieval askretrieve.RetrievalResult, planCritic askcontract.PlanCriticResponse) (askcontract.GenerationResponse, []askcontract.GeneratedFile, string, askcontract.CriticResponse, askcontract.JudgeResponse, int, error) {
	_ = planCritic
	var lastValidation string
	var lastCritic askcontract.CriticResponse
	var lastJudge askcontract.JudgeResponse
	var lastFiles []askcontract.GeneratedFile
	taintedFiles := map[string]bool{}
	bundle := askknowledge.Current()
	for attempt := 1; attempt <= attempts; attempt++ {
		currentPrompt := req.Prompt
		currentSystemPrompt := req.SystemPrompt
		if attempt > 1 && lastValidation != "" {
			validationDiags := askdiagnostic.FromValidationError(lastValidation, bundle)
			markTaintedFiles(taintedFiles, validationDiags)
			repairPaths := repairTargetFiles(lastFiles, validationDiags, taintedFiles)
			diags := append([]askdiagnostic.Diagnostic{}, validationDiags...)
			if isGenerationParseFailure(lastValidation) {
				currentPrompt = jsonResponseRetryPrompt(req.Prompt, lastValidation, decision.Route)
			} else {
				diags = append(diags, askdiagnostic.FromPlanCritic(planCritic)...)
				diags = append(diags, askdiagnostic.FromCritic(lastCritic)...)
				logger.logf("debug", "\n[ask][phase:repair:diagnostics]\n%s\n", askdiagnostic.JSON(diags))
				currentSystemPrompt = strings.TrimSpace(req.SystemPrompt) + "\n\n" + documentRepairSystemPrompt(normalizedAuthoringBrief(plan, brief), plan)
				currentPrompt = documentRepairUserPrompt(lastFiles, lastValidation, diags, repairPaths)
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
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, err
		}
		logger.response("generation", resp.Content)
		gen, err := askcontract.ParseGeneration(resp.Content)
		if err != nil {
			lastValidation = err.Error()
			logger.logf("debug", "[ask][phase:generation:parse-error] error=%s\n", lastValidation)
			if !repairableValidationError(lastValidation) {
				return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation stopped without repair: %s", lastValidation)
			}
			if attempt < attempts {
				continue
			}
			return askcontract.GenerationResponse{}, nil, lastValidation, lastCritic, lastJudge, attempt - 1, fmt.Errorf("ask generation returned invalid JSON: %s", lastValidation)
		}
		files, err := askir.MaterializeWithBase(root, lastFiles, gen)
		if err != nil {
			lastValidation = err.Error()
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

func isYAMLParseFailure(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "parse yaml") || strings.Contains(lower, "parse vars yaml") || strings.Contains(lower, "yaml: line ") || strings.Contains(lower, "yaml: did not") || strings.Contains(lower, "yaml: could not")
}

func isGenerationParseFailure(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.HasPrefix(lower, "parse generation response:") || strings.Contains(lower, "model returned empty response") || strings.Contains(lower, "invalid character")
}

func jsonResponseRetryPrompt(basePrompt string, validation string, route askintent.Route) string {
	b := &strings.Builder{}
	b.WriteString(strings.TrimSpace(basePrompt))
	b.WriteString("\n\nThe previous response was not valid JSON for the required document generation schema. Re-emit the full response as strict JSON only.\n")
	b.WriteString("Do not add commentary, markdown fences, or unsupported action names.\n")
	if route == askintent.RouteRefine {
		b.WriteString("For refine routes, use only actions preserve|replace|create|edit|delete.\n")
	}
	b.WriteString("Previous parse error:\n")
	b.WriteString(strings.TrimSpace(validation))
	return strings.TrimSpace(b.String())
}

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

func targetedRepairPromptBlock(prevFiles []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic, repairPaths []string) string {
	if len(prevFiles) == 0 {
		return ""
	}
	affected := map[string]bool{}
	for _, path := range repairPaths {
		if strings.TrimSpace(path) != "" {
			affected[strings.TrimSpace(path)] = true
		}
	}
	if len(affected) == 0 {
		for _, file := range prevFiles {
			affected[strings.TrimSpace(file.Path)] = true
		}
	}
	b := &strings.Builder{}
	b.WriteString("Targeted repair mode:\n")
	b.WriteString("- Preserve unchanged files when they are already valid.\n")
	b.WriteString("- For files marked preserve-if-valid, keep content byte-for-byte unless a diagnostic explicitly requires a change.\n")
	b.WriteString("- Prefer editing only the files implicated by diagnostics or execution/design review findings.\n")
	if hasDiagnosticCode(diags, string(workflowissues.CodeDuplicateStepID)) {
		b.WriteString("- Duplicate step id repair: rename only the conflicting ids; do not duplicate or rewrite unaffected steps.\n")
		spec := workflowissues.MustSpec(workflowissues.CodeDuplicateStepID)
		if strings.TrimSpace(spec.PromptHint) != "" {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(spec.PromptHint))
			b.WriteString(" For example `control-plane-preflight-host` and `worker-preflight-host`.\n")
		}
	}
	b.WriteString("- Keep rendered workflow structure stable: preserve top-level keys, list structure, and unaffected phases or steps unless a diagnostic requires a precise change.\n")
	b.WriteString("- Do not rewrite every document from scratch when only one targeted structural change is needed.\n")
	if hasComponentRepairTarget(prevFiles, repairPaths) {
		b.WriteString("- If repeated schema failures are isolated to `workflows/components/`, you may collapse that logic back into the nearest entry workflow first, then re-extract components after the draft validates.\n")
	}
	b.WriteString("- Return structured documents, not raw file payloads. Revised documents may omit unaffected files because the caller preserves them.\n")
	if len(affected) > 0 {
		b.WriteString("Affected files to revise first:\n")
		for _, file := range prevFiles {
			if affected[strings.TrimSpace(file.Path)] {
				b.WriteString("- ")
				b.WriteString(strings.TrimSpace(file.Path))
				b.WriteString("\n")
			}
		}
	}
	b.WriteString("File status from previous attempt:\n")
	for _, file := range prevFiles {
		path := strings.TrimSpace(file.Path)
		status := "preserve-if-valid"
		if affected[path] {
			status = "revise"
		}
		b.WriteString("- path: ")
		b.WriteString(path)
		b.WriteString(" [")
		b.WriteString(status)
		b.WriteString("]\n")
		for _, detail := range diagnosticDetailsForFile(path, diags) {
			b.WriteString("  - ")
			b.WriteString(detail)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func hasComponentRepairTarget(prevFiles []askcontract.GeneratedFile, repairPaths []string) bool {
	for _, path := range repairPaths {
		if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(path)), "workflows/components/") {
			return true
		}
	}
	for _, file := range prevFiles {
		if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(file.Path)), "workflows/components/") {
			return true
		}
	}
	return false
}

func hasDiagnosticCode(diags []askdiagnostic.Diagnostic, code string) bool {
	code = strings.TrimSpace(code)
	for _, diag := range diags {
		if strings.TrimSpace(diag.Code) == code {
			return true
		}
	}
	return false
}

func diagnosticDetailsForFile(path string, diags []askdiagnostic.Diagnostic) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	items := []string{}
	for _, diag := range diags {
		diagPath := strings.TrimSpace(diag.Path)
		if diagPath == "" {
			diagPath = strings.TrimSpace(diag.File)
		}
		if diagPath == "" {
			diagPath = diagnosticMessageFile(diag.Message)
		}
		if diagPath != path {
			continue
		}
		msg := strings.TrimSpace(diag.Message)
		if msg != "" {
			items = append(items, msg)
		}
		fix := strings.TrimSpace(diag.SuggestedFix)
		if fix != "" {
			items = append(items, "suggested fix: "+fix)
		}
	}
	return dedupe(items)
}

func documentStructureRepairPromptBlock(prevFiles []askcontract.GeneratedFile, validation string, repairPaths []string) string {
	lower := strings.ToLower(strings.TrimSpace(validation))
	if !strings.Contains(lower, "parse yaml") && !strings.Contains(lower, "yaml:") {
		return ""
	}
	affected := repairPaths
	if len(affected) == 0 {
		affected = affectedFilesFromDiagnostics(prevFiles, nil)
	}
	b := &strings.Builder{}
	b.WriteString("Document structure repair requirements:\n")
	b.WriteString("- Fix structured document shape before changing workflow design. Prioritize required keys, object/list structure, and exact field placement.\n")
	b.WriteString("- Preserve already-valid rendered files exactly; only revise documents implicated by the parse or render error when possible.\n")
	b.WriteString("- Keep workflow roots stable with top-level `version`, then either `phases` or `steps`, but never both.\n")
	if len(affected) > 0 {
		b.WriteString("- Parse-error files to fix first:\n")
		for _, path := range affected {
			b.WriteString("  - ")
			b.WriteString(path)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func repairTargetFiles(prevFiles []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic, tainted map[string]bool) []string {
	targets := diagnosticFiles(diags)
	if len(targets) == 0 {
		targets = affectedFilesFromDiagnostics(prevFiles, diags)
	}
	for path := range tainted {
		if !stringSliceContains(targets, path) {
			targets = append(targets, path)
		}
	}
	return targets
}

func markTaintedFiles(tainted map[string]bool, diags []askdiagnostic.Diagnostic) {
	for _, path := range diagnosticFiles(diags) {
		tainted[path] = true
	}
}

func diagnosticFiles(diags []askdiagnostic.Diagnostic) []string {
	paths := []string{}
	for _, diag := range diags {
		path := strings.TrimSpace(diag.Path)
		if path == "" {
			path = strings.TrimSpace(diag.File)
		}
		if path == "" {
			path = diagnosticMessageFile(diag.Message)
		}
		if path != "" && !stringSliceContains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths
}

func mapKeys(items map[string]bool) []string {
	out := make([]string, 0, len(items))
	for key := range items {
		out = append(out, key)
	}
	return out
}

func affectedFilesFromDiagnostics(prevFiles []askcontract.GeneratedFile, diags []askdiagnostic.Diagnostic) []string {
	affected := map[string]bool{}
	for _, diag := range diags {
		path := strings.TrimSpace(diag.Path)
		if path == "" {
			path = strings.TrimSpace(diag.File)
		}
		if path != "" {
			affected[path] = true
		}
	}
	if len(affected) == 0 {
		for _, file := range prevFiles {
			affected[strings.TrimSpace(file.Path)] = true
		}
	}
	out := make([]string, 0, len(affected))
	for _, file := range prevFiles {
		path := strings.TrimSpace(file.Path)
		if affected[path] {
			out = append(out, path)
		}
	}
	for path := range affected {
		if !stringSliceContains(out, path) {
			out = append(out, path)
		}
	}
	return out
}

func stringSliceContains(items []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, item := range items {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}

func diagnosticMessageFile(message string) string {
	message = strings.TrimSpace(message)
	if !strings.HasPrefix(message, "workflows/") {
		return ""
	}
	idx := strings.Index(message, ":")
	if idx <= 0 {
		return ""
	}
	path := strings.TrimSpace(message[:idx])
	if !strings.HasPrefix(path, "workflows/") {
		return ""
	}
	return path
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

func repairableValidationError(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	nonRepairable := []string{
		"generation response did not include documents",
		"response did not include any files",
		"generated file path is empty",
		"generated file path is not allowed",
		"generated file path escapes workspace",
	}
	for _, token := range nonRepairable {
		if strings.Contains(message, token) {
			return false
		}
	}
	return true
}

func normalizedAuthoringBrief(plan askcontract.PlanResponse, fallback askcontract.AuthoringBrief) askcontract.AuthoringBrief {
	if strings.TrimSpace(plan.AuthoringBrief.RouteIntent) != "" {
		return plan.AuthoringBrief
	}
	return fallback
}

func validateGeneration(ctx context.Context, root string, gen askcontract.GenerationResponse, files []askcontract.GeneratedFile, decision askintent.Decision, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, retrieval askretrieve.RetrievalResult) (string, askcontract.CriticResponse, error) {
	if len(files) == 0 {
		critic := askcontract.CriticResponse{Blocking: []string{"response did not include any files"}, MissingFiles: filePathsFromPlan(plan), RequiredFixes: []string{"Return the planned workflow files"}}
		return "", critic, fmt.Errorf("response did not include any files")
	}
	staged, err := stageWorkspace(root, files)
	if err != nil {
		return "", askcontract.CriticResponse{Blocking: []string{err.Error()}}, err
	}
	defer func() { _ = os.RemoveAll(staged) }()
	paths := make([]string, 0, len(files))
	directValidated := 0
	for _, file := range files {
		if err := validateGeneratedFile(staged, file); err != nil {
			return "", askcontract.CriticResponse{Blocking: []string{err.Error()}, RequiredFixes: requiredFixesForValidation(err.Error())}, err
		}
		paths = append(paths, file.Path)
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(file.Path)), ".yaml") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(file.Path)), ".yml") {
			directValidated++
		}
	}
	entrypoints := scenarioPaths(staged, paths)
	validated := make([]string, 0, len(entrypoints))
	for _, path := range entrypoints {
		files, err := validate.EntrypointWithContext(ctx, path)
		if err != nil {
			return "", askcontract.CriticResponse{Blocking: []string{err.Error()}, RequiredFixes: requiredFixesForValidation(err.Error())}, err
		}
		validated = append(validated, files...)
	}
	validated = dedupe(validated)
	critic := semanticCritic(gen, decision, plan, normalizedAuthoringBrief(plan, brief), retrieval)
	if len(critic.Blocking) > 0 {
		return "", critic, fmt.Errorf("semantic validation failed: %s", strings.Join(critic.Blocking, "; "))
	}
	return fmt.Sprintf("lint ok (%d yaml files, %d scenario entrypoints)", directValidated, len(validated)), critic, nil
}

func requiredFixesForValidation(message string) []string {
	fixes := []string{"Return only schema-valid files under allowed workflow paths"}
	lower := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(lower, "invalid map key") && (strings.Contains(lower, "{{") || strings.Contains(lower, ".vars.")) {
		fixes = append(fixes, "Do not use whole-value template expressions like `{{ .vars.* }}` for typed YAML arrays or objects such as spec.packages or spec.repositories; inline concrete YAML lists or objects instead")
	}
	if strings.Contains(lower, "parse yaml") && strings.Contains(lower, ".vars.") {
		fixes = append(fixes, "Keep workflows/vars.yaml as plain YAML data only. Do not place template expressions inside vars values, and quote any literal strings that contain special YAML characters")
	}
	if strings.Contains(lower, "imports.0") && strings.Contains(lower, "expected: object") && strings.Contains(lower, "given: string") {
		fixes = append(fixes, "Use phase imports as objects like `imports: [{path: check-host.yaml}]` rather than bare strings")
	}
	if strings.Contains(lower, "additional property version is not allowed") {
		fixes = append(fixes, "Do not add workflow-level fields like version to component fragments under workflows/components/. Component files should usually contain only a top-level steps mapping")
	}
	if strings.Contains(lower, "invalid type. expected: object, given: array") {
		fixes = append(fixes, "Do not make a component file a bare YAML array. Component files should be YAML objects, usually with a top-level steps: key")
	}
	if strings.Contains(lower, "workflows/components/") {
		fixes = append(fixes, "For starter drafts, avoid generating workflows/components/ unless reusable fragments are clearly required; inline the first working version into prepare/apply instead")
		fixes = append(fixes, "If component fragments keep failing validation, collapse them back into workflows/prepare.yaml or workflows/scenarios/apply.yaml first, then extract reusable components after validation passes")
	}
	if strings.Contains(lower, "command") && strings.Contains(lower, "is not supported for role prepare") {
		fixes = append(fixes, "Use typed prepare steps like DownloadImage or DownloadPackage instead of Command when collecting offline artifacts in prepare")
	}
	fixes = append(fixes, askcontext.ValidationFixesForError(message)...)
	return dedupe(fixes)
}

func summarizeValidationError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "- validation failed with no additional detail"
	}
	lower := strings.ToLower(message)
	workflowRules := askcontext.Current().Workflow
	points := []string{}
	appendPoint := func(point string) {
		point = strings.TrimSpace(point)
		if point == "" {
			return
		}
		points = append(points, point)
	}
	switch {
	case isYAMLParseFailure(message):
		appendPoint("- YAML parse failure: fix indentation, list markers, or template placement before changing step logic")
	case strings.Contains(lower, "e_schema_invalid") || strings.Contains(lower, " is required") || strings.Contains(lower, "additional property"):
		appendPoint("- Schema validation failure: keep only supported fields and include required workflow and step fields")
	case strings.Contains(lower, "semantic validation failed"):
		appendPoint("- Semantic validation failure: generated files are inconsistent with the request or plan")
	}
	for _, line := range strings.Split(message, ";") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(strings.ToLower(line), "(root): version is required") {
			appendPoint("- Add top-level `version: " + workflowRules.SupportedVersion + "` to every workflow file")
		}
		if strings.Contains(strings.ToLower(line), ": id is required") {
			appendPoint("- Add an `id` field to every step item")
		}
		if strings.Contains(strings.ToLower(line), "additional property id is not allowed") && strings.Contains(strings.ToLower(line), "phases.") {
			appendPoint("- Remove `id` from phases and keep a non-empty `name`; only steps carry ids")
		}
		if strings.Contains(strings.ToLower(line), "additional property") && strings.Contains(strings.ToLower(line), "phases.") {
			appendPoint("- Phase objects support `name`, `steps`, `imports`, and optional `maxParallelism` only")
		}
		if strings.Contains(strings.ToLower(line), "invalid map key") {
			appendPoint("- Do not use whole-value template expressions where YAML arrays or objects are required")
		}
		if strings.Contains(strings.ToLower(line), "must be one of") {
			appendPoint("- Keep constrained enum fields as literal allowed values instead of replacing them with vars templates")
		}
		if strings.Contains(strings.ToLower(line), "does not match pattern") {
			appendPoint("- Keep pattern-constrained scalar fields as literal values that satisfy the documented schema pattern instead of replacing them with vars templates")
		}
		if strings.Contains(strings.ToLower(line), "did not find expected node content") {
			appendPoint("- Keep YAML list items and template directives in valid YAML structure")
		}
	}
	if len(points) == 0 {
		appendPoint("- Fix the validator error exactly as reported and keep the response schema-valid")
	}
	return strings.Join(dedupe(points), "\n")
}
