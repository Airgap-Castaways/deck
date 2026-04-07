package askcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func validateGeneration(ctx context.Context, root string, gen askcontract.GenerationResponse, files []askcontract.GeneratedFile, decision askintent.Decision, plan askcontract.PlanResponse, brief askcontract.AuthoringBrief, retrieval askretrieve.RetrievalResult) (string, askcontract.CriticResponse, error) {
	if len(files) == 0 {
		if decision.Route == askintent.RouteRefine && preserveOnlyDocuments(gen.Documents) {
			return "lint ok (0 yaml files, 0 scenario entrypoints)", askcontract.CriticResponse{}, nil
		}
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
	contractCritic := validatePlanContract(files, plan, decision)
	if len(contractCritic.Blocking) > 0 {
		return "", contractCritic, fmt.Errorf("plan contract validation failed: %s", strings.Join(contractCritic.Blocking, "; "))
	}
	critic := semanticCritic(gen, decision, plan, normalizedAuthoringBrief(plan, brief), retrieval)
	critic = mergeContractCritic(critic, contractCritic)
	if len(critic.Blocking) > 0 {
		return "", critic, fmt.Errorf("semantic validation failed: %s", strings.Join(critic.Blocking, "; "))
	}
	return fmt.Sprintf("lint ok (%d yaml files, %d scenario entrypoints)", directValidated, len(validated)), critic, nil
}

func preserveOnlyDocuments(documents []askcontract.GeneratedDocument) bool {
	if len(documents) == 0 {
		return false
	}
	for _, doc := range documents {
		if strings.ToLower(strings.TrimSpace(doc.Action)) != "preserve" {
			return false
		}
	}
	return true
}

func validatePlanContract(files []askcontract.GeneratedFile, plan askcontract.PlanResponse, decision askintent.Decision) askcontract.CriticResponse {
	critic := askcontract.CriticResponse{}
	generated := map[string]bool{}
	for _, file := range files {
		if file.Delete {
			continue
		}
		generated[filepath.ToSlash(strings.TrimSpace(file.Path))] = true
	}
	allowed := allowedPlanPaths(plan)
	for path := range generated {
		if len(allowed) > 0 && !allowed[path] {
			critic.Blocking = append(critic.Blocking, fmt.Sprintf("generated file %s is outside the clarified plan target paths", path))
			critic.CoverageGaps = append(critic.CoverageGaps, path)
			critic.RequiredFixes = append(critic.RequiredFixes, "Restrict generation to the clarified plan target paths and planned files")
		}
	}
	if decision.Route == askintent.RouteDraft {
		for _, path := range requiredDraftPaths(plan) {
			if !generated[path] {
				critic.Blocking = append(critic.Blocking, fmt.Sprintf("planned file %s was not generated", path))
				critic.MissingFiles = append(critic.MissingFiles, path)
			}
		}
	}
	entry := filepath.ToSlash(strings.TrimSpace(plan.EntryScenario))
	if entry != "" && requiresEntryScenario(plan) && !generated[entry] {
		critic.Blocking = append(critic.Blocking, fmt.Sprintf("planned entry scenario %s was not generated", entry))
		critic.MissingFiles = append(critic.MissingFiles, entry)
	}
	for _, contract := range plan.ExecutionModel.ArtifactContracts {
		producer := filepath.ToSlash(strings.TrimSpace(contract.ProducerPath))
		consumer := filepath.ToSlash(strings.TrimSpace(contract.ConsumerPath))
		if producer != "" && !generated[producer] && allowed[producer] {
			critic.Blocking = append(critic.Blocking, fmt.Sprintf("artifact producer %s for %s contract was not generated", producer, contract.Kind))
			critic.MissingFiles = append(critic.MissingFiles, producer)
		}
		if consumer != "" && !generated[consumer] && allowed[consumer] {
			critic.Blocking = append(critic.Blocking, fmt.Sprintf("artifact consumer %s for %s contract was not generated", consumer, contract.Kind))
			critic.MissingFiles = append(critic.MissingFiles, consumer)
		}
	}
	critic.Blocking = dedupe(critic.Blocking)
	critic.MissingFiles = dedupe(critic.MissingFiles)
	critic.CoverageGaps = dedupe(critic.CoverageGaps)
	critic.RequiredFixes = dedupe(critic.RequiredFixes)
	return critic
}

func allowedPlanPaths(plan askcontract.PlanResponse) map[string]bool {
	allowed := map[string]bool{}
	for _, path := range plan.AuthoringBrief.AnchorPaths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path != "" {
			allowed[path] = true
		}
	}
	for _, path := range plan.AuthoringBrief.AllowedCompanionPaths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path != "" {
			allowed[path] = true
		}
	}
	for _, path := range plan.AuthoringBrief.TargetPaths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path != "" {
			allowed[path] = true
		}
	}
	for _, file := range plan.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path != "" {
			allowed[path] = true
		}
	}
	if path := filepath.ToSlash(strings.TrimSpace(plan.EntryScenario)); path != "" {
		allowed[path] = true
	}
	return allowed
}

func requiredDraftPaths(plan askcontract.PlanResponse) []string {
	required := []string{}
	for _, file := range plan.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		action := strings.ToLower(strings.TrimSpace(file.Action))
		if path == "" {
			continue
		}
		if action == "create" || action == "replace" || action == "update" {
			required = append(required, path)
		}
	}
	if plan.NeedsPrepare {
		required = append(required, workspacepaths.CanonicalPrepareWorkflow)
	}
	if requiresEntryScenario(plan) && strings.TrimSpace(plan.EntryScenario) != "" {
		required = append(required, filepath.ToSlash(strings.TrimSpace(plan.EntryScenario)))
	}
	return dedupe(required)
}

func requiresEntryScenario(plan askcontract.PlanResponse) bool {
	scope := strings.TrimSpace(plan.AuthoringBrief.TargetScope)
	mode := strings.TrimSpace(plan.AuthoringBrief.ModeIntent)
	return scope == "scenario" || scope == "workspace" || mode == "prepare+apply" || mode == "apply-only"
}

func mergeContractCritic(base askcontract.CriticResponse, extra askcontract.CriticResponse) askcontract.CriticResponse {
	base.Blocking = dedupe(append(base.Blocking, extra.Blocking...))
	base.Advisory = dedupe(append(base.Advisory, extra.Advisory...))
	base.MissingFiles = dedupe(append(base.MissingFiles, extra.MissingFiles...))
	base.InvalidImports = dedupe(append(base.InvalidImports, extra.InvalidImports...))
	base.CoverageGaps = dedupe(append(base.CoverageGaps, extra.CoverageGaps...))
	base.RequiredFixes = dedupe(append(base.RequiredFixes, extra.RequiredFixes...))
	return base
}

func requiredFixesForValidation(message string) []string {
	fixes := []string{"Return only schema-valid files under allowed workflow paths"}
	lower := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(lower, "invalid map key") && (strings.Contains(lower, "{{") || strings.Contains(lower, ".vars.")) {
		fixes = append(fixes, "Do not use whole-value template expressions like `{{ .vars.* }}` for typed YAML arrays or objects such as spec.packages or spec.repositories; inline concrete YAML lists or objects instead")
	}
	if strings.Contains(lower, "parse yaml") && strings.Contains(lower, ".vars.") {
		fixes = append(fixes, fmt.Sprintf("Keep %s as plain YAML data only. Do not place template expressions inside vars values, and quote any literal strings that contain special YAML characters", workspacepaths.CanonicalVarsWorkflow))
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
		fixes = append(fixes, fmt.Sprintf("If component fragments keep failing validation, collapse them back into %s or %s first, then extract reusable components after validation passes", workspacepaths.CanonicalPrepareWorkflow, workspacepaths.CanonicalApplyWorkflow))
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
