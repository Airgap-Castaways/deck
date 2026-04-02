package askir

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/structuredpath"
)

func resolveStructuredEditPath(rawPath string, doc askcontract.GeneratedDocument) string {
	rawPath = normalizeVarsDocumentEditPath(rawPath, doc)
	segments, err := structuredpath.Parse(rawPath)
	if err != nil {
		return strings.TrimSpace(rawPath)
	}
	segments = rewriteWorkflowStepPath(segments, doc)
	return structuredpath.ToPointer(segments)
}

func rewriteWorkflowStepPath(segments []structuredpath.Segment, doc askcontract.GeneratedDocument) []structuredpath.Segment {
	if doc.Workflow == nil || len(segments) < 2 || segments[0].IsIndex || segments[0].Key != "steps" || segments[1].IsIndex {
		return segments
	}
	stepKey := strings.TrimSpace(segments[1].Key)
	for phaseIdx, phase := range doc.Workflow.Phases {
		for stepIdx, step := range phase.Steps {
			if strings.TrimSpace(step.ID) == stepKey {
				prefix := []structuredpath.Segment{{Key: "phases"}, {Index: phaseIdx, IsIndex: true}, {Key: "steps"}, {Index: stepIdx, IsIndex: true}}
				return append(prefix, segments[2:]...)
			}
		}
	}
	if len(doc.Workflow.Steps) > 0 {
		for stepIdx, step := range doc.Workflow.Steps {
			if strings.TrimSpace(step.ID) == stepKey {
				prefix := []structuredpath.Segment{{Key: "steps"}, {Index: stepIdx, IsIndex: true}}
				return append(prefix, segments[2:]...)
			}
		}
	}
	return segments
}

func normalizeVarsDocumentEditPath(rawPath string, doc askcontract.GeneratedDocument) string {
	rawPath = strings.TrimSpace(rawPath)
	if doc.Vars == nil {
		return rawPath
	}
	if rawPath == "vars" {
		return ""
	}
	if strings.HasPrefix(rawPath, "vars.") {
		return strings.TrimPrefix(rawPath, "vars.")
	}
	return rawPath
}
