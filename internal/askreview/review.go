package askreview

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Finding struct {
	Severity string
	Message  string
}

func Workspace(root string) []Finding {
	findings := make([]Finding, 0)
	files := workspaceReviewPaths(root)
	commandCount := 0
	validFiles := make([]string, 0, len(files))
	for _, path := range files {
		//nolint:gosec // Review paths are derived from the current workspace layout.
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hasValidationError := false
		for _, err := range reviewValidationErrors(path, raw) {
			findings = append(findings, validationFindings(path, err)...)
			hasValidationError = true
		}
		if !hasValidationError {
			validFiles = append(validFiles, path)
		}
		count, hasTopLevelSteps := inspectWorkflow(raw)
		commandCount += count
		if strings.HasSuffix(path, "apply.yaml") && hasTopLevelSteps {
			findings = append(findings, Finding{
				Severity: "warn",
				Message:  fmt.Sprintf("%s uses top-level steps; named phases are usually easier to review for apply workflows", filepath.ToSlash(path)),
			})
		}
	}
	analysisFindings, err := validate.AnalyzeFiles(validFiles)
	if err == nil {
		findings = append(findings, analysisReviewFindings(analysisFindings)...)
	}
	if commandCount >= 3 {
		findings = append(findings, Finding{
			Severity: "warn",
			Message:  fmt.Sprintf("workspace uses %d Command steps; prefer typed steps where possible", commandCount),
		})
	}
	return dedupeFindings(findings)
}

func Candidate(files map[string]string) []Finding {
	findings := make([]Finding, 0)
	commandCount := 0
	paths := make([]string, 0, len(files))
	for path := range files {
		if isReviewablePath(path) {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		content := files[path]
		if !workspacepaths.IsComponentAuthoringPath(path) {
			if err := validate.Bytes(path, []byte(content)); err != nil {
				findings = append(findings, validationFindings(path, err)...)
			}
		}
		count, hasTopLevelSteps := inspectWorkflow([]byte(content))
		commandCount += count
		if strings.HasSuffix(path, "/apply.yaml") && hasTopLevelSteps {
			findings = append(findings, Finding{
				Severity: "warn",
				Message:  fmt.Sprintf("%s uses top-level steps; consider named phases for apply workflows", filepath.ToSlash(path)),
			})
		}
	}
	if commandCount >= 3 {
		findings = append(findings, Finding{
			Severity: "warn",
			Message:  fmt.Sprintf("candidate output uses %d Command steps; prefer typed steps where possible", commandCount),
		})
	}
	return dedupeFindings(findings)
}

func reviewValidationErrors(path string, raw []byte) []error {
	if workspacepaths.IsComponentWorkflowPath(path) {
		return nil
	}
	errs := make([]error, 0, 2)
	if err := validate.Bytes(path, raw); err != nil {
		errs = append(errs, err)
	}
	if workspacepaths.IsScenarioWorkflowPath(path) || workspacepaths.IsCanonicalPrepareWorkflowPath(path) {
		if _, err := validate.Entrypoint(path); err != nil && !hasErrorMessage(errs, err) {
			errs = append(errs, err)
		}
	}
	return errs
}

func workspaceReviewPaths(root string) []string {
	paths := make([]string, 0)
	preparePath := workspacepaths.CanonicalPrepareWorkflowPath(root)
	if fileExists(preparePath) {
		paths = append(paths, preparePath)
	}
	paths = append(paths, yamlFilesUnder(workspacepaths.WorkflowScenariosPath(root))...)
	paths = append(paths, yamlFilesUnder(workspacepaths.WorkflowComponentsPath(root))...)
	return dedupeAndSort(paths)
}

func yamlFilesUnder(root string) []string {
	paths := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil && !errors.Is(err, os.ErrNotExist) {
		return paths
	}
	return paths
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isReviewablePath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	return workspacepaths.IsCanonicalPrepareWorkflowPath(clean) || workspacepaths.IsScenarioAuthoringPath(clean) || workspacepaths.IsComponentAuthoringPath(clean)
}

func validationFindings(path string, err error) []Finding {
	var issueErr interface{ ValidationIssues() []validate.Issue }
	if errors.As(err, &issueErr) {
		issues := issueErr.ValidationIssues()
		findings := make([]Finding, 0, len(issues))
		for _, issue := range issues {
			findings = append(findings, validationIssueFinding(path, issue))
		}
		if len(findings) > 0 {
			return findings
		}
	}
	return []Finding{{Severity: "blocking", Message: fmt.Sprintf("%s validation failed: %s", filepath.ToSlash(path), err.Error())}}
}

func validationIssueFinding(path string, issue validate.Issue) Finding {
	message := strings.TrimSpace(issue.Message)
	if message == "" {
		message = "validation failed"
	}
	if code := strings.TrimSpace(issue.Code); code != "" {
		message = fmt.Sprintf("%s: %s", code, message)
	}
	if stepID := strings.TrimSpace(issue.StepID); stepID != "" {
		message = fmt.Sprintf("%s (step %s)", message, stepID)
	}
	displayPath := strings.TrimSpace(issue.File)
	if displayPath == "" {
		displayPath = path
	}
	return Finding{Severity: normalizeSeverity(issue.Severity), Message: fmt.Sprintf("%s validation: %s", filepath.ToSlash(displayPath), message)}
}

func analysisReviewFindings(findings []validate.Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		message := strings.TrimSpace(finding.Message)
		if message == "" {
			continue
		}
		if finding.Path != "" {
			message = fmt.Sprintf("%s: %s", filepath.ToSlash(finding.Path), message)
		}
		if finding.Hint != "" {
			message = message + " " + strings.TrimSpace(finding.Hint)
		}
		out = append(out, Finding{Severity: normalizeSeverity(finding.Severity), Message: message})
	}
	return out
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "blocking", "error", "fatal":
		return "blocking"
	case "warning", "warn", "advisory":
		return "warn"
	default:
		return "warn"
	}
}

func dedupeAndSort(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasErrorMessage(errs []error, err error) bool {
	message := strings.TrimSpace(err.Error())
	for _, existing := range errs {
		if strings.TrimSpace(existing.Error()) == message {
			return true
		}
	}
	return false
}

func dedupeFindings(findings []Finding) []Finding {
	seen := map[string]bool{}
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		key := finding.Severity + "\x00" + finding.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

type workflowDoc struct {
	Steps  []stepDoc  `yaml:"steps"`
	Phases []phaseDoc `yaml:"phases"`
}

type phaseDoc struct {
	Steps []stepDoc `yaml:"steps"`
}

type stepDoc struct {
	Kind string `yaml:"kind"`
}

func inspectWorkflow(raw []byte) (commandCount int, hasTopLevelSteps bool) {
	var doc workflowDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return 0, false
	}
	hasTopLevelSteps = len(doc.Steps) > 0
	for _, step := range doc.Steps {
		if step.Kind == "Command" {
			commandCount++
		}
	}
	for _, phase := range doc.Phases {
		for _, step := range phase.Steps {
			if step.Kind == "Command" {
				commandCount++
			}
		}
	}
	return commandCount, hasTopLevelSteps
}
