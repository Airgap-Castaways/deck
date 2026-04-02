package askir

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func ParseDocument(path string, raw []byte) (askcontract.GeneratedDocument, error) {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return askcontract.GeneratedDocument{}, fmt.Errorf("document path is empty")
	}
	if clean == filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) {
		var vars map[string]any
		if err := yaml.Unmarshal(raw, &vars); err != nil {
			return askcontract.GeneratedDocument{}, fmt.Errorf("parse vars document %s: %w", clean, err)
		}
		vars = unwrapVarsDocument(vars)
		return askcontract.GeneratedDocument{Path: clean, Kind: "vars", Vars: vars}, nil
	}
	if workspacepaths.IsComponentWorkflowPath(clean) {
		var component componentRender
		if err := yaml.Unmarshal(raw, &component); err != nil {
			return askcontract.GeneratedDocument{}, fmt.Errorf("parse component document %s: %w", clean, err)
		}
		return askcontract.GeneratedDocument{Path: clean, Kind: "component", Component: &askcontract.ComponentDocument{Steps: stepsToIR(component.Steps)}}, nil
	}
	var workflow workflowRender
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		return askcontract.GeneratedDocument{}, fmt.Errorf("parse workflow document %s: %w", clean, err)
	}
	return askcontract.GeneratedDocument{Path: clean, Kind: "workflow", Workflow: &askcontract.WorkflowDocument{Version: workflow.Version, Vars: workflow.Vars, Phases: phasesToIR(workflow.Phases), Steps: stepsToIR(workflow.Steps)}}, nil
}

func phasesToIR(items []phaseRender) []askcontract.WorkflowPhase {
	out := make([]askcontract.WorkflowPhase, 0, len(items))
	for _, item := range items {
		out = append(out, askcontract.WorkflowPhase{
			Name:           item.Name,
			MaxParallelism: item.MaxParallelism,
			Imports:        importsToIR(item.Imports),
			Steps:          stepsToIR(item.Steps),
		})
	}
	return out
}

func importsToIR(items []importRender) []askcontract.PhaseImport {
	out := make([]askcontract.PhaseImport, 0, len(items))
	for _, item := range items {
		out = append(out, askcontract.PhaseImport{Path: item.Path, When: item.When})
	}
	return out
}

func stepsToIR(items []stepRender) []askcontract.WorkflowStep {
	out := make([]askcontract.WorkflowStep, 0, len(items))
	for _, item := range items {
		out = append(out, askcontract.WorkflowStep{
			ID:            item.ID,
			APIVersion:    item.APIVersion,
			Kind:          item.Kind,
			Metadata:      item.Metadata,
			When:          item.When,
			ParallelGroup: item.ParallelGroup,
			Register:      item.Register,
			Retry:         item.Retry,
			Timeout:       item.Timeout,
			Spec:          item.Spec,
		})
	}
	return out
}

func Summaries(documents []askcontract.GeneratedDocument) []string {
	out := make([]string, 0, len(documents))
	for _, doc := range documents {
		switch {
		case doc.Workflow != nil:
			out = append(out, fmt.Sprintf("%s [workflow] phases=%d top-level-steps=%d", doc.Path, len(doc.Workflow.Phases), len(doc.Workflow.Steps)))
		case doc.Component != nil:
			out = append(out, fmt.Sprintf("%s [component] steps=%d", doc.Path, len(doc.Component.Steps)))
		case doc.Vars != nil:
			out = append(out, fmt.Sprintf("%s [vars] keys=%d", doc.Path, len(doc.Vars)))
		}
	}
	return out
}
