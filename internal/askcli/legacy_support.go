package askcli

import (
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askir"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func containsTrimmed(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
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

func enrichPostProcessFindings(findings askcontract.PostProcessResponse, rendered []askcontract.GeneratedFile) askcontract.PostProcessResponse {
	files := filePathSet(rendered)
	if len(findings.ReviseFiles) == 0 && len(findings.Blocking) > 0 {
		if files[workspacepaths.CanonicalApplyWorkflow] {
			findings.ReviseFiles = append(findings.ReviseFiles, workspacepaths.CanonicalApplyWorkflow)
		}
	}
	for path := range files {
		if !containsTrimmed(findings.ReviseFiles, path) && !containsTrimmed(findings.PreserveFiles, path) {
			findings.PreserveFiles = append(findings.PreserveFiles, path)
		}
	}
	if len(findings.Blocking) == 0 {
		findings.ReviseFiles = nil
	}
	findings.PreserveFiles = dedupe(findings.PreserveFiles)
	findings.ReviseFiles = dedupe(findings.ReviseFiles)
	return findings
}

func filePathSet(files []askcontract.GeneratedFile) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		path := strings.TrimSpace(file.Path)
		if path != "" {
			out[path] = true
		}
	}
	return out
}

func currentWorkspaceDocumentSummaries(workspace askretrieve.WorkspaceSummary) []string {
	out := make([]string, 0, len(workspace.Files))
	for _, file := range workspace.Files {
		doc, err := askir.ParseDocument(file.Path, []byte(file.Content))
		if err != nil {
			continue
		}
		kind := "unknown"
		switch {
		case doc.Workflow != nil:
			kind = "workflow"
		case doc.Component != nil:
			kind = "component"
		case doc.Vars != nil:
			kind = "vars"
		}
		out = append(out, file.Path+" ["+kind+"]")
	}
	return out
}

func structuralWorkflowSummary(doc askcontract.WorkflowDocument) string {
	parts := askir.Summaries([]askcontract.GeneratedDocument{{Path: workspacepaths.CanonicalApplyWorkflow, Kind: "workflow", Workflow: &doc}})
	for _, step := range doc.Steps {
		if strings.TrimSpace(step.Kind) != "" {
			parts = append(parts, strings.TrimSpace(step.Kind))
		}
		if strings.TrimSpace(step.When) != "" {
			parts = append(parts, strings.TrimSpace(step.When))
		}
	}
	for _, phase := range doc.Phases {
		for _, step := range phase.Steps {
			if strings.TrimSpace(step.Kind) != "" {
				parts = append(parts, strings.TrimSpace(step.Kind))
			}
			if strings.TrimSpace(step.When) != "" {
				parts = append(parts, strings.TrimSpace(step.When))
			}
		}
	}
	return strings.Join(parts, "\n")
}
