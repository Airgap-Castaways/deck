package validate

import (
	"fmt"
	"strings"
)

type Issue struct {
	Code         string
	Severity     string
	File         string
	Path         string
	StepID       string
	StepKind     string
	Message      string
	Expected     string
	Actual       string
	SourceRef    string
	SuggestedFix string
}

type ValidationError struct {
	Message string
	Issues  []Issue
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func (e *ValidationError) ValidationIssues() []Issue {
	if e == nil {
		return nil
	}
	out := make([]Issue, 0, len(e.Issues))
	for _, issue := range e.Issues {
		if strings.TrimSpace(issue.Severity) == "" {
			issue.Severity = "blocking"
		}
		out = append(out, issue)
	}
	return out
}

func schemaValidationError(message string, issues []Issue) error {
	return &ValidationError{Message: strings.TrimSpace(message), Issues: issues}
}

func wrapIssue(file string, issue Issue) Issue {
	if strings.TrimSpace(issue.File) == "" {
		issue.File = strings.TrimSpace(file)
	}
	if strings.TrimSpace(issue.Severity) == "" {
		issue.Severity = "blocking"
	}
	return issue
}

func issueForDuplicateStep(file string, stepID string) Issue {
	return wrapIssue(file, Issue{
		Code:         "duplicate_step_id",
		Severity:     "blocking",
		Path:         "steps[].id",
		Message:      fmt.Sprintf("workflow reuses step id %q", stepID),
		Actual:       stepID,
		SourceRef:    "workflowissues.duplicate_step_id",
		SuggestedFix: fmt.Sprintf("Rename duplicated step id %q so every step id is unique across the workflow.", stepID),
	})
}

func issueForDuplicatePhase(file string, phaseName string) Issue {
	return wrapIssue(file, Issue{
		Code:         "duplicate_phase_name",
		Severity:     "blocking",
		Path:         "phases[].name",
		Message:      fmt.Sprintf("workflow reuses phase name %q", phaseName),
		Actual:       phaseName,
		SourceRef:    "workflowissues.duplicate_phase_name",
		SuggestedFix: "Rename duplicate phases so every phase name is unique within the workflow.",
	})
}
