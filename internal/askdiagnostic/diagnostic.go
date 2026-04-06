package askdiagnostic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

type Diagnostic struct {
	Code         string   `json:"code"`
	Severity     string   `json:"severity"`
	File         string   `json:"file,omitempty"`
	Path         string   `json:"path,omitempty"`
	StepID       string   `json:"stepId,omitempty"`
	StepKind     string   `json:"stepKind,omitempty"`
	RepairOp     string   `json:"repairOp,omitempty"`
	Message      string   `json:"message"`
	Expected     string   `json:"expected,omitempty"`
	Actual       string   `json:"actual,omitempty"`
	Allowed      []string `json:"allowed,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	SourceRef    string   `json:"sourceRef,omitempty"`
	SuggestedFix string   `json:"suggestedFix,omitempty"`
}

func FromValidationError(err error, message string, bundle askknowledge.Bundle) []Diagnostic {
	var validationErr *validate.ValidationError
	if errors.As(err, &validationErr) {
		structured := FromValidationIssues(validationErr.ValidationIssues())
		if len(structured) > 0 {
			fallbackFile := diagnosticMessageFile(message)
			for i := range structured {
				if strings.TrimSpace(structured[i].File) == "" {
					structured[i].File = fallbackFile
				}
			}
			return structured
		}
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	lower := strings.ToLower(message)
	diags := []Diagnostic{}
	appendDiag := func(diag Diagnostic) {
		if strings.TrimSpace(diag.Severity) == "" {
			diag.Severity = "blocking"
		}
		if strings.TrimSpace(diag.Message) == "" {
			return
		}
		diags = append(diags, diag)
	}
	appendDiag(Diagnostic{Code: "validation_error", Severity: "blocking", Message: message, SourceRef: "validate"})
	if strings.Contains(lower, "invalid map key") && strings.Contains(lower, ".vars.") {
		appendDiag(Diagnostic{Code: "typed_collection_template", Severity: "blocking", Message: "typed YAML array or object was templated as a single vars scalar", Expected: "native YAML array or object", Actual: "whole-value vars template", SourceRef: "workflow schema", SuggestedFix: "Inline schema-valid YAML arrays and objects instead of using a whole-value vars template."})
	}
	if strings.Contains(lower, "imports.0") && strings.Contains(lower, "expected: object") {
		appendDiag(Diagnostic{Code: "import_shape", Severity: "blocking", Path: "phases[].imports[]", Message: "phase import must be an object with path", Expected: "{path: component.yaml}", Actual: "string import entry", SourceRef: "workflow import rule", SuggestedFix: "Use imports entries like `- path: check-host.yaml`."})
	}
	if strings.Contains(lower, "additional property version is not allowed") && strings.Contains(lower, "workflows/components/") {
		appendDiag(Diagnostic{Code: "component_fragment_shape", Severity: "blocking", File: "workflows/components/*.yaml", Message: "component fragment includes workflow-level fields", Expected: "fragment object with top-level steps only", Actual: "full workflow document with version/phases", Allowed: bundle.Components.AllowedRootKeys, SourceRef: "deck-component-fragment.schema.json", SuggestedFix: "Keep component files as fragment documents with a top-level `steps:` key only."})
	}
	if strings.Contains(lower, "invalid type. expected: object, given: array") && strings.Contains(lower, "workflows/components/") {
		appendDiag(Diagnostic{Code: "component_fragment_shape", Severity: "blocking", File: "workflows/components/*.yaml", Message: "component fragment was emitted as a bare YAML array", Expected: "YAML object", Actual: "array", Allowed: bundle.Components.AllowedRootKeys, SourceRef: "deck-component-fragment.schema.json", SuggestedFix: "Wrap component steps under a top-level `steps:` mapping."})
	}
	if strings.Contains(lower, "is not supported for role prepare") {
		appendDiag(Diagnostic{Code: "role_support", Severity: "blocking", Message: "step kind is not supported for prepare", Expected: "prepare-supported typed step", Actual: "unsupported step kind for prepare", SourceRef: "workflow step role declarations", SuggestedFix: "Use a typed prepare step such as DownloadImage or DownloadPackage for artifact collection."})
	}
	if builderID, path, ok := extractUnsupportedDraftBuilder(message); ok {
		supported := askcatalog.Current().BuilderIDsForPath(path)
		suggestedFix := "Use only supported draft builder ids for the planned file."
		expected := "supported draft builder ids"
		if len(supported) > 0 {
			expected = strings.Join(supported, ", ")
			suggestedFix = fmt.Sprintf("Replace %s with a supported builder id for %s: %s.", builderID, path, expected)
		}
		appendDiag(Diagnostic{Code: "unsupported_draft_builder", Severity: "blocking", File: path, Message: fmt.Sprintf("draft builder %s is not supported for %s", builderID, path), Expected: expected, Actual: builderID, SourceRef: "askcatalog", SuggestedFix: suggestedFix})
	}
	if path, ok := extractMissingPlannedFile(message); ok {
		supported := askcatalog.Current().BuilderIDsForPath(path)
		suggestedFix := fmt.Sprintf("Ensure the draft selection generates the planned file %s.", path)
		expected := "planned file present"
		if len(supported) > 0 {
			suggestedFix = fmt.Sprintf("Generate %s with supported builder ids such as %s.", path, strings.Join(supported, ", "))
			expected = strings.Join(supported, ", ")
		}
		appendDiag(Diagnostic{Code: "missing_planned_file", Severity: "blocking", File: path, Message: fmt.Sprintf("planned file %s was not generated", path), Expected: expected, SourceRef: "askcatalog", SuggestedFix: suggestedFix})
	}
	if stepID, ok := extractDuplicateStepID(message); ok {
		spec := workflowissues.MustSpec(workflowissues.CodeDuplicateStepID)
		appendDiag(Diagnostic{
			Code:         string(workflowissues.CodeDuplicateStepID),
			Severity:     "blocking",
			File:         diagnosticMessageFile(message),
			Path:         "steps[].id",
			Message:      fmt.Sprintf("workflow reuses step id %q", stepID),
			Expected:     strings.TrimSpace(spec.Details),
			Actual:       stepID,
			SourceRef:    "workflowissues." + string(workflowissues.CodeDuplicateStepID),
			SuggestedFix: fmt.Sprintf("Rename duplicated step id %q so every step id is unique across the workflow. Prefer role- or phase-specific ids such as control-plane-%s or worker-%s.", stepID, stepID, stepID),
		})
	}
	if stepKind, specMessage, ok := extractStepSpecFailure(message); ok {
		if step, found := findStep(bundle, stepKind); found {
			if prop, ok := extractAdditionalProperty(specMessage); ok {
				expected := renderKeyFieldList(step)
				if rules := renderRuleSummaryList(step); rules != "" {
					expected += "; rules: " + rules
				}
				appendDiag(Diagnostic{
					Code:         "unknown_step_field",
					Severity:     "blocking",
					Path:         fmt.Sprintf("%s.spec.%s", stepKind, prop),
					Message:      fmt.Sprintf("%s does not support spec.%s", stepKind, prop),
					Expected:     expected,
					Actual:       "spec." + prop,
					SourceRef:    step.SchemaFile,
					SuggestedFix: fmt.Sprintf("Use documented %s fields such as %s.", stepKind, expected),
				})
			}
			if field, ok := extractRequiredField(specMessage); ok {
				suggested := fmt.Sprintf("Add required field spec.%s to %s.", field, stepKind)
				for _, key := range step.KeyFields {
					if strings.TrimSpace(key.Path) == "spec."+field && strings.TrimSpace(key.Description) != "" {
						suggested = fmt.Sprintf("Add required field spec.%s to %s. %s", field, stepKind, strings.TrimSpace(key.Description))
						break
					}
				}
				if rules := renderRuleSummaryList(step); rules != "" {
					suggested += " Rules: " + rules
				}
				appendDiag(Diagnostic{
					Code:         "missing_step_field",
					Severity:     "blocking",
					Path:         fmt.Sprintf("%s.spec.%s", stepKind, field),
					Message:      fmt.Sprintf("%s requires spec.%s", stepKind, field),
					Expected:     "spec." + field,
					SourceRef:    step.SchemaFile,
					SuggestedFix: suggested,
				})
			}
		}
	}
	for _, constraint := range bundle.Constraints {
		if strings.Contains(lower, strings.ToLower(constraint.Path)) && strings.Contains(lower, "must be one of") {
			appendDiag(Diagnostic{Code: "constrained_literal", Severity: "blocking", Path: constraint.Path, Message: "constrained field rejected a non-literal value", Allowed: append([]string(nil), constraint.AllowedValues...), SourceRef: constraint.SourceRef, SuggestedFix: constraint.Guidance})
		}
		if strings.Contains(lower, strings.ToLower(constraint.Path)) && strings.Contains(lower, "does not match pattern") {
			appendDiag(Diagnostic{Code: "constrained_pattern", Severity: "blocking", Path: constraint.Path, Message: "pattern-constrained field failed validation", Pattern: "schema pattern", SourceRef: constraint.SourceRef, SuggestedFix: constraint.Guidance})
		}
	}
	return dedupe(diags)
}

func FromValidationIssues(issues []validate.Issue) []Diagnostic {
	if len(issues) == 0 {
		return nil
	}
	diags := make([]Diagnostic, 0, len(issues))
	for _, issue := range issues {
		file := strings.TrimSpace(issue.File)
		if file == "" {
			file = diagnosticMessageFile(strings.TrimSpace(issue.Message))
		}
		message := strings.TrimSpace(issue.Message)
		if message == "" {
			message = strings.TrimSpace(issue.Expected)
		}
		if message == "" {
			continue
		}
		diags = append(diags, Diagnostic{
			Code:         strings.TrimSpace(issue.Code),
			Severity:     strings.TrimSpace(issue.Severity),
			File:         file,
			Path:         strings.TrimSpace(issue.Path),
			StepID:       strings.TrimSpace(issue.StepID),
			StepKind:     strings.TrimSpace(issue.StepKind),
			RepairOp:     classifyRepairOp(issue),
			Message:      message,
			Expected:     strings.TrimSpace(issue.Expected),
			Actual:       strings.TrimSpace(issue.Actual),
			SourceRef:    strings.TrimSpace(issue.SourceRef),
			SuggestedFix: strings.TrimSpace(issue.SuggestedFix),
		})
	}
	return dedupe(diags)
}

func classifyRepairOp(issue validate.Issue) string {
	lowerCode := strings.ToLower(strings.TrimSpace(issue.Code))
	lowerMessage := strings.ToLower(strings.TrimSpace(issue.Message))
	lowerActual := strings.ToLower(strings.TrimSpace(issue.Actual))
	switch {
	case lowerCode == "missing_step_field" || strings.Contains(lowerMessage, " is required"):
		return "fill-field"
	case lowerCode == "unknown_step_field" || strings.Contains(lowerMessage, "additional property"):
		return "remove-field"
	case lowerCode == "duplicate_step_id":
		return "rename-step"
	case strings.Contains(lowerMessage, "invalid type") && strings.Contains(lowerActual, "{{ .vars."):
		return "fix-literal"
	case strings.Contains(lowerMessage, "must be one of") || strings.Contains(lowerMessage, "does not match pattern"):
		return "fix-literal"
	case strings.Contains(lowerMessage, "parse yaml"):
		return "repair-structure"
	default:
		return "review-diagnostic"
	}
}

func extractDuplicateStepID(message string) (string, bool) {
	const prefix = "E_DUPLICATE_STEP_ID:"
	idx := strings.Index(strings.TrimSpace(message), prefix)
	if idx < 0 {
		return "", false
	}
	stepID := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message[idx:]), prefix))
	if stepID == "" {
		return "", false
	}
	return stepID, true
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

func extractStepSpecFailure(message string) (string, string, bool) {
	marker := "): spec: "
	start := strings.Index(message, "(")
	end := strings.Index(message, marker)
	if start < 0 || end < 0 || end <= start+1 {
		return "", "", false
	}
	return strings.TrimSpace(message[start+1 : strings.Index(message[start:], ")")+start]), strings.TrimSpace(message[end+len(marker):]), true
}

func extractAdditionalProperty(message string) (string, bool) {
	const prefix = "Additional property "
	if !strings.HasPrefix(strings.TrimSpace(message), prefix) {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message), prefix))
	idx := strings.Index(rest, " ")
	if idx < 0 {
		return strings.TrimSpace(rest), true
	}
	return strings.TrimSpace(rest[:idx]), true
}

func extractRequiredField(message string) (string, bool) {
	const suffix = " is required"
	trimmed := strings.TrimSpace(message)
	if !strings.HasSuffix(trimmed, suffix) {
		return "", false
	}
	field := strings.TrimSpace(strings.TrimSuffix(trimmed, suffix))
	field = strings.ReplaceAll(field, ": ", ".")
	field = strings.ReplaceAll(field, ":", ".")
	field = strings.ReplaceAll(field, " ", "")
	return field, true
}

func extractUnsupportedDraftBuilder(message string) (string, string, bool) {
	const prefix = "unsupported draft builder "
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, prefix) {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	parts := strings.SplitN(rest, " for ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	builderID := strings.Trim(strings.TrimSpace(parts[0]), `"`)
	path := strings.TrimSpace(parts[1])
	if builderID == "" || path == "" {
		return "", "", false
	}
	return builderID, path, true
}

func extractMissingPlannedFile(message string) (string, bool) {
	const prefix = "plan contract validation failed: planned file "
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, prefix) || !strings.HasSuffix(trimmed, " was not generated") {
		return "", false
	}
	path := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, prefix), " was not generated"))
	if path == "" {
		return "", false
	}
	return path, true
}

func findStep(bundle askknowledge.Bundle, kind string) (askknowledge.StepKnowledge, bool) {
	for _, step := range bundle.Steps {
		if step.Kind == strings.TrimSpace(kind) {
			return step, true
		}
	}
	return askknowledge.StepKnowledge{}, false
}

func renderKeyFieldList(step askknowledge.StepKnowledge) string {
	paths := make([]string, 0, len(step.KeyFields))
	for _, field := range step.KeyFields {
		if strings.TrimSpace(field.Path) != "" {
			label := strings.TrimSpace(field.Path)
			requirement := strings.TrimSpace(field.Requirement)
			if requirement == "" {
				requirement = "optional"
			}
			label += " (" + requirement + ")"
			paths = append(paths, label)
		}
	}
	if len(paths) == 0 {
		return "documented step fields"
	}
	return strings.Join(paths, ", ")
}

func renderRuleSummaryList(step askknowledge.StepKnowledge) string {
	rules := make([]string, 0, len(step.SchemaRuleSummaries))
	for _, rule := range step.SchemaRuleSummaries {
		rule = strings.TrimSpace(rule)
		if rule != "" {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return ""
	}
	if len(rules) > 2 {
		rules = rules[:2]
	}
	return strings.Join(rules, " ")
}

func FromEvaluation(findings []askpolicy.EvaluationFinding) []Diagnostic {
	diags := make([]Diagnostic, 0, len(findings))
	for _, finding := range findings {
		diags = append(diags, Diagnostic{
			Code:         finding.Code,
			Severity:     finding.Severity,
			File:         finding.Path,
			Path:         finding.Path,
			Message:      finding.Message,
			SuggestedFix: finding.Fix,
		})
	}
	return dedupe(diags)
}

func FromCritic(critic askcontract.CriticResponse) []Diagnostic {
	diags := []Diagnostic{}
	for _, item := range critic.Blocking {
		diags = append(diags, Diagnostic{Code: "critic_blocking", Severity: "blocking", Message: item})
	}
	for _, item := range critic.Advisory {
		diags = append(diags, Diagnostic{Code: "critic_advisory", Severity: "advisory", Message: item})
	}
	for _, item := range critic.RequiredFixes {
		diags = append(diags, Diagnostic{Code: "required_fix", Severity: "blocking", Message: item, SuggestedFix: item})
	}
	return dedupe(diags)
}

func FromJudge(judge askcontract.JudgeResponse) []Diagnostic {
	diags := []Diagnostic{}
	for _, item := range judge.Blocking {
		diags = append(diags, Diagnostic{Code: "judge_blocking", Severity: "blocking", Message: item})
	}
	for _, item := range judge.Advisory {
		diags = append(diags, Diagnostic{Code: "judge_advisory", Severity: "advisory", Message: item})
	}
	for _, item := range judge.MissingCapabilities {
		diags = append(diags, Diagnostic{Code: "judge_missing_capability", Severity: "blocking", Message: item, SuggestedFix: item})
	}
	for _, item := range judge.SuggestedFixes {
		diags = append(diags, Diagnostic{Code: "judge_suggested_fix", Severity: "blocking", Message: item, SuggestedFix: item})
	}
	return dedupe(diags)
}

func FromPlanCritic(critic askcontract.PlanCriticResponse) []Diagnostic {
	diags := []Diagnostic{}
	for _, item := range critic.Blocking {
		diags = append(diags, Diagnostic{Code: "plan_critic_blocking", Severity: "blocking", Message: item})
	}
	for _, item := range critic.Advisory {
		diags = append(diags, Diagnostic{Code: "plan_critic_advisory", Severity: "advisory", Message: item})
	}
	for _, item := range critic.MissingContracts {
		diags = append(diags, Diagnostic{Code: "plan_critic_missing_contract", Severity: "advisory", Message: item, SuggestedFix: item})
	}
	for _, item := range critic.SuggestedFixes {
		diags = append(diags, Diagnostic{Code: "plan_critic_suggested_fix", Severity: "advisory", Message: item, SuggestedFix: item})
	}
	return dedupe(diags)
}

func FromPostProcess(resp askcontract.PostProcessResponse) []Diagnostic {
	diags := []Diagnostic{}
	for _, item := range resp.Blocking {
		diags = append(diags, Diagnostic{Code: "postprocess_blocking", Severity: "blocking", Message: item})
	}
	for _, item := range resp.Advisory {
		diags = append(diags, Diagnostic{Code: "postprocess_advisory", Severity: "advisory", Message: item})
	}
	for _, item := range resp.SuggestedFixes {
		diags = append(diags, Diagnostic{Code: "postprocess_suggested_fix", Severity: "blocking", Message: item, SuggestedFix: item})
	}
	for _, item := range resp.RequiredEdits {
		diags = append(diags, Diagnostic{Code: "postprocess_required_edit", Severity: "blocking", Message: item, SuggestedFix: item})
	}
	return dedupe(diags)
}

func JSON(diags []Diagnostic) string {
	raw, err := json.MarshalIndent(diags, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func RepairPromptBlock(diags []Diagnostic) string {
	if len(diags) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Structured diagnostics JSON:\n")
	b.WriteString(JSON(diags))
	b.WriteString("\nDiagnostic repair priorities:\n")
	for _, diag := range diags {
		b.WriteString("- ")
		b.WriteString(diag.Code)
		b.WriteString(": ")
		b.WriteString(diag.Message)
		if strings.TrimSpace(diag.SuggestedFix) != "" {
			b.WriteString(" Fix: ")
			b.WriteString(strings.TrimSpace(diag.SuggestedFix))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func dedupe(diags []Diagnostic) []Diagnostic {
	seen := map[string]bool{}
	out := make([]Diagnostic, 0, len(diags))
	for _, diag := range diags {
		key := strings.Join([]string{diag.Code, diag.Severity, diag.File, diag.Path, diag.Message, diag.SuggestedFix}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, diag)
	}
	return out
}
