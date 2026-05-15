package askcli

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdiagnostic"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/validate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func infoPrompts(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, prompt string) (string, string) {
	return buildInfoSystemPrompt(route, target, retrieval, workspace), buildInfoUserPrompt(prompt, route, target)
}

func buildInfoSystemPrompt(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary) string {
	b := &strings.Builder{}
	switch route {
	case askintent.RouteQuestion:
		b.WriteString("You are deck ask answering a workflow question.\n")
		b.WriteString("Answer the user's question directly and use retrieved evidence.\n")
		b.WriteString("Keep the answer concise but specific.\n")
		b.WriteString("If evidence is incomplete, say what is known from the workspace and avoid speculation.\n")
	case askintent.RouteExplain:
		if repoBehaviorExplainPrompt(target, retrieval) {
			b.WriteString("You are deck ask explaining how this repository assembles workflow behavior.\n")
			b.WriteString("Anchor first on code-owned paths such as internal/stepmeta, internal/stepspec, and related compiler helpers.\n")
			b.WriteString("Explain the assembly path in code terms: registry/metadata -> builder selection -> binding resolution -> workflow document compilation.\n")
			b.WriteString("Use current workspace YAML only as a secondary example, not as the primary explanation.\n")
		} else {
			b.WriteString("You are deck ask explaining an existing deck workspace file or workflow.\n")
			b.WriteString("Explain what the target does, how it fits into the workflow, and call out imports, phases, major step kinds, and Command usage when present.\n")
			b.WriteString("Do not give a shallow file count summary.\n")
		}
	case askintent.RouteReview:
		b.WriteString("You are deck ask reviewing an existing deck workspace.\n")
		b.WriteString("Use the retrieved evidence and any local findings to produce a scoped review with practical concerns and suggested changes.\n")
		b.WriteString("Narrate the findings instead of only repeating raw warnings.\n")
		b.WriteString("Preserve local severity labels: do not convert advisory warn findings into blocking findings unless schema or validation evidence marks them blocking.\n")
	default:
		b.WriteString("You are deck ask.\n")
		b.WriteString("Route: ")
		b.WriteString(string(route))
		b.WriteString("\nTarget kind: ")
		b.WriteString(target.Kind)
		b.WriteString("\n")
		b.WriteString("Do not return file content for this route.\n")
	}
	if route == askintent.RouteQuestion || route == askintent.RouteExplain {
		if block := weakExternalEvidencePromptBlock(retrieval); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if route == askintent.RouteExplain && !repoBehaviorExplainPrompt(target, retrieval) || route == askintent.RouteReview {
		if block := retrievalDocumentSummaryBlock(retrieval); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	if route == askintent.RouteReview {
		if block := workspaceStructuredIssuePromptBlock(workspace, target); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	switch route {
	case askintent.RouteReview:
		b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	case askintent.RouteQuestion, askintent.RouteExplain:
		b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	default:
		b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	}
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func buildInfoUserPrompt(prompt string, route askintent.Route, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		switch route {
		case askintent.RouteExplain:
			b.WriteString("Explain target: ")
		case askintent.RouteReview:
			b.WriteString("Review target: ")
		default:
			b.WriteString("Target path: ")
		}
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	if route == askintent.RouteReview {
		b.WriteString("\nProvide a scoped review with concrete suggested changes.")
	}
	return b.String()
}

func repoBehaviorExplainPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) bool {
	if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(target.Path)), "internal/") {
		return true
	}
	for _, chunk := range retrieval.Chunks {
		if chunk.Source != "local-facts" {
			continue
		}
		if chunk.ID == "local-facts-stepmeta" || chunk.ID == "local-facts-stepspec" {
			return true
		}
	}
	return false
}

func weakExternalEvidencePromptBlock(retrieval askretrieve.RetrievalResult) string {
	for _, chunk := range retrieval.Chunks {
		if chunk.Source != "external-evidence" || strings.TrimSpace(chunk.Label) != "weak-evidence-status" {
			continue
		}
		return "When install/setup evidence is weak, say official source retrieval was incomplete and keep the answer narrowly bounded as general guidance only. Do not present distro-specific or version-specific install steps as verified unless the retrieved evidence explicitly supports them."
	}
	return ""
}

func workspaceStructuredIssuePromptBlock(workspace askretrieve.WorkspaceSummary, target askintent.Target) string {
	issues := []validate.Issue{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if !structuredIssueReviewPath(path) {
			continue
		}
		if target.Path != "" && path != filepath.ToSlash(strings.TrimSpace(target.Path)) {
			continue
		}
		err := validate.Bytes(path, []byte(file.Content))
		var validationErr *validate.ValidationError
		if !errors.As(err, &validationErr) {
			continue
		}
		issues = append(issues, validationErr.ValidationIssues()...)
	}
	if len(issues) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Structured validation issues:\n")
	for _, diag := range askdiagnostic.FromValidationIssues(issues) {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(diag.Message))
		if strings.TrimSpace(diag.Path) != "" {
			b.WriteString(" path=")
			b.WriteString(strings.TrimSpace(diag.Path))
		}
		if strings.TrimSpace(diag.SuggestedFix) != "" {
			b.WriteString(" fix=")
			b.WriteString(strings.TrimSpace(diag.SuggestedFix))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func structuredIssueReviewPath(path string) bool {
	if path == "" || !strings.HasPrefix(path, workflowRootPrefix) {
		return false
	}
	return workspacepaths.IsCanonicalPrepareWorkflowPath(path) || workspacepaths.IsScenarioAuthoringPath(path)
}

func retrievalDocumentSummaryBlock(retrieval askretrieve.RetrievalResult) string {
	documents := make([]askcontract.GeneratedDocument, 0, len(retrieval.Chunks))
	for _, chunk := range retrieval.Chunks {
		path := strings.TrimSpace(chunk.Label)
		if path == "" || !strings.HasPrefix(filepath.ToSlash(path), workflowRootPrefix) {
			continue
		}
		doc, err := askir.ParseDocument(path, []byte(chunk.Content))
		if err != nil {
			continue
		}
		documents = append(documents, doc)
	}
	if len(documents) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Parsed workflow summaries:\n")
	for _, summary := range askir.Summaries(documents) {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(summary))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
