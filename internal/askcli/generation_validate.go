package askcli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/validate"
)

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
