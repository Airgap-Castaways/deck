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
)

func infoPrompts(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary, prompt string) (string, string) {
	switch route {
	case askintent.RouteExplain:
		return explainSystemPrompt(target, retrieval), explainUserPrompt(prompt, target)
	case askintent.RouteReview:
		return reviewSystemPrompt(target, retrieval, workspace), reviewUserPrompt(prompt, target)
	case askintent.RouteQuestion:
		return questionSystemPrompt(target, retrieval), questionUserPrompt(prompt, target)
	default:
		return infoSystemPrompt(route, target, retrieval), infoUserPrompt(prompt, route, target)
	}
}

func infoSystemPrompt(route askintent.Route, target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask.\n")
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	b.WriteString("Target kind: ")
	b.WriteString(target.Kind)
	b.WriteString("\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	b.WriteString("Do not return file content for this route.\n")
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func questionSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask answering a workflow question.\n")
	b.WriteString("Answer the user's question directly and use retrieved evidence.\n")
	b.WriteString("Keep the answer concise but specific.\n")
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	b.WriteString("If evidence is incomplete, say what is known from the workspace and avoid speculation.\n")
	if block := weakExternalEvidencePromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func explainSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult) string {
	b := &strings.Builder{}
	if repoBehaviorExplainPrompt(target, retrieval) {
		b.WriteString("You are deck ask explaining how this repository assembles workflow behavior.\n")
		b.WriteString("Anchor first on code-owned paths such as internal/stepmeta, internal/stepspec, internal/askdraft, and related compiler helpers.\n")
		b.WriteString("Explain the assembly path in code terms: registry/metadata -> builder selection -> binding resolution -> workflow document compilation.\n")
		b.WriteString("Use current workspace YAML only as a secondary example, not as the primary explanation.\n")
	} else {
		b.WriteString("You are deck ask explaining an existing deck workspace file or workflow.\n")
		b.WriteString("Explain what the target does, how it fits into the workflow, and call out imports, phases, major step kinds, and Command usage when present.\n")
		b.WriteString("Do not give a shallow file count summary.\n")
	}
	if block := weakExternalEvidencePromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if !repoBehaviorExplainPrompt(target, retrieval) {
		if block := retrievalDocumentSummaryBlock(retrieval); strings.TrimSpace(block) != "" {
			b.WriteString(block)
			b.WriteString("\n")
		}
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"suggestions\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
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
		if chunk.ID == "local-facts-stepmeta" || chunk.ID == "local-facts-stepspec" || chunk.ID == "local-facts-askdraft" {
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

func reviewSystemPrompt(target askintent.Target, retrieval askretrieve.RetrievalResult, workspace askretrieve.WorkspaceSummary) string {
	b := &strings.Builder{}
	b.WriteString("You are deck ask reviewing an existing deck workspace.\n")
	b.WriteString("Use the retrieved evidence and any local findings to produce a scoped review with practical concerns and suggested changes.\n")
	b.WriteString("Narrate the findings instead of only repeating raw warnings.\n")
	if block := evidenceBoundaryPromptBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := retrievalDocumentSummaryBlock(retrieval); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	if block := workspaceStructuredIssuePromptBlock(workspace, target); strings.TrimSpace(block) != "" {
		b.WriteString(block)
		b.WriteString("\n")
	}
	b.WriteString("Return strict JSON with shape {\"summary\":string,\"answer\":string,\"findings\":[]string,\"suggestedChanges\":[]string}.\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString(askretrieve.BuildChunkText(retrieval))
	return b.String()
}

func workspaceStructuredIssuePromptBlock(workspace askretrieve.WorkspaceSummary, target askintent.Target) string {
	issues := []validate.Issue{}
	for _, file := range workspace.Files {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == "" || !strings.HasPrefix(path, workflowRootPrefix) {
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

func infoUserPrompt(prompt string, route askintent.Route, target askintent.Target) string {
	b := &strings.Builder{}
	b.WriteString("Route: ")
	b.WriteString(string(route))
	b.WriteString("\n")
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func questionUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Target path: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User question:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func explainUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Explain target: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	return b.String()
}

func reviewUserPrompt(prompt string, target askintent.Target) string {
	b := &strings.Builder{}
	if target.Path != "" {
		b.WriteString("Review target: ")
		b.WriteString(target.Path)
		b.WriteString("\n")
	}
	b.WriteString("User request:\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\nProvide a scoped review with concrete suggested changes.")
	return b.String()
}
