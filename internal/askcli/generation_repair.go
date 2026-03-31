package askcli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/workflowissues"
)

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

func validateRepairDocumentStrategy(documents []askcontract.GeneratedDocument, diags []askdiagnostic.Diagnostic, repairPaths []string, route askintent.Route) error {
	if route != askintent.RouteRefine || len(diags) == 0 {
		return nil
	}
	narrowByFile := map[string]bool{}
	for _, diag := range diags {
		op := strings.TrimSpace(diag.RepairOp)
		if op == "fill-field" || op == "remove-field" || op == "fix-literal" || op == "rename-step" {
			path := strings.TrimSpace(diag.File)
			if path == "" {
				path = strings.TrimSpace(diag.Path)
			}
			if path != "" {
				narrowByFile[path] = true
			}
		}
	}
	for _, doc := range documents {
		path := filepath.ToSlash(strings.TrimSpace(doc.Path))
		action := strings.TrimSpace(doc.Action)
		if action == "" {
			action = "replace"
		}
		if !stringSliceContains(repairPaths, path) || !narrowByFile[path] {
			continue
		}
		if action == "replace" && len(doc.Edits) == 0 && len(doc.Transforms) == 0 {
			return fmt.Errorf("repair response for %s must use structured edits or transforms for narrow validator issues", path)
		}
	}
	return nil
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

func restrictRepairTargetsToPlan(paths []string, plan askcontract.PlanResponse) []string {
	allowed := allowedPlanPaths(plan)
	if len(allowed) == 0 {
		return paths
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path != "" && allowed[path] {
			out = append(out, path)
		}
	}
	if len(out) == 0 {
		return paths
	}
	return dedupe(out)
}

func markTaintedFiles(tainted map[string]bool, diags []askdiagnostic.Diagnostic) {
	for _, path := range diagnosticFiles(diags) {
		tainted[path] = true
	}
}

func diagnosticFiles(diags []askdiagnostic.Diagnostic) []string {
	paths := []string{}
	for _, diag := range diags {
		path := strings.TrimSpace(diag.File)
		if path == "" {
			path = strings.TrimSpace(diag.Path)
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
		path := strings.TrimSpace(diag.File)
		if path == "" {
			path = strings.TrimSpace(diag.Path)
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
