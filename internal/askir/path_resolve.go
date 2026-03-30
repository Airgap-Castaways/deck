package askir

import (
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func resolveStructuredEditPath(rawPath string, doc askcontract.GeneratedDocument) string {
	if rewritten := rewriteWorkflowStepPath(rawPath, doc); rewritten != "" {
		rawPath = rewritten
	}
	segments := strings.Split(strings.TrimSpace(rawPath), ".")
	out := make([]string, 0, len(segments))
	current := any(doc)
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		resolved, next := resolveStructuredEditSegment(current, segment)
		out = append(out, resolved)
		current = next
	}
	return strings.Join(out, ".")
}

func rewriteWorkflowStepPath(rawPath string, doc askcontract.GeneratedDocument) string {
	if doc.Workflow == nil {
		return ""
	}
	path := strings.TrimSpace(rawPath)
	path = strings.ReplaceAll(path, "[", ".")
	path = strings.ReplaceAll(path, "]", "")
	path = strings.TrimPrefix(path, ".")
	if !strings.HasPrefix(path, "steps.") {
		return ""
	}
	segments := strings.Split(path, ".")
	if len(segments) < 2 {
		return ""
	}
	stepKey := strings.TrimSpace(segments[1])
	if _, err := strconv.Atoi(stepKey); err == nil {
		return path
	}
	for phaseIdx, phase := range doc.Workflow.Phases {
		for stepIdx, step := range phase.Steps {
			if strings.TrimSpace(step.ID) == stepKey {
				rest := strings.Join(segments[2:], ".")
				if rest == "" {
					return "phases." + strconv.Itoa(phaseIdx) + ".steps." + strconv.Itoa(stepIdx)
				}
				return "phases." + strconv.Itoa(phaseIdx) + ".steps." + strconv.Itoa(stepIdx) + "." + rest
			}
		}
	}
	return path
}

func resolveStructuredEditSegment(current any, segment string) (string, any) {
	switch node := current.(type) {
	case askcontract.GeneratedDocument:
		if node.Workflow != nil {
			return segment, *node.Workflow
		}
		if node.Component != nil {
			return segment, *node.Component
		}
		return segment, node.Vars
	case askcontract.WorkflowDocument:
		switch segment {
		case "steps":
			return segment, node.Steps
		case "phases":
			return segment, node.Phases
		case "vars":
			return segment, node.Vars
		default:
			return segment, nil
		}
	case askcontract.ComponentDocument:
		if segment == "steps" {
			return segment, node.Steps
		}
	case askcontract.WorkflowPhase:
		switch segment {
		case "steps":
			return segment, node.Steps
		case "imports":
			return segment, node.Imports
		default:
			return segment, nil
		}
	case []askcontract.WorkflowStep:
		if idx, ok := resolveStepIndex(node, segment); ok {
			return strconv.Itoa(idx), node[idx]
		}
	case []askcontract.WorkflowPhase:
		if idx, ok := resolvePhaseIndex(node, segment); ok {
			return strconv.Itoa(idx), node[idx]
		}
	case []askcontract.PhaseImport:
		if idx, err := strconv.Atoi(segment); err == nil && idx >= 0 && idx < len(node) {
			return strconv.Itoa(idx), node[idx]
		}
	}
	return segment, nil
}

func resolveStepIndex(steps []askcontract.WorkflowStep, segment string) (int, bool) {
	if idx, err := strconv.Atoi(segment); err == nil && idx >= 0 && idx < len(steps) {
		return idx, true
	}
	for i, step := range steps {
		if strings.TrimSpace(step.ID) == segment {
			return i, true
		}
	}
	return 0, false
}

func resolvePhaseIndex(phases []askcontract.WorkflowPhase, segment string) (int, bool) {
	if idx, err := strconv.Atoi(segment); err == nil && idx >= 0 && idx < len(phases) {
		return idx, true
	}
	for i, phase := range phases {
		if strings.TrimSpace(phase.Name) == segment {
			return i, true
		}
	}
	return 0, false
}
