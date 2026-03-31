package askir

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askrefine"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/structurededit"
)

func applyDocumentEdits(root string, baseContent map[string]string, path string, doc askcontract.GeneratedDocument) (string, []askcontract.GeneratedFile, error) {
	if len(doc.Edits) == 0 && len(doc.Transforms) == 0 {
		return "", nil, fmt.Errorf("document %s requested edit action without edits or transforms", path)
	}
	resolved, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(path), "/")...)
	if err != nil {
		return "", nil, err
	}
	raw, err := os.ReadFile(resolved) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			if existing, ok := baseContent[path]; ok {
				raw = []byte(existing)
			} else {
				return "", nil, fmt.Errorf("read refine target %s: %w", path, err)
			}
		} else {
			return "", nil, fmt.Errorf("read refine target %s: %w", path, err)
		}
	}
	parsedDoc, err := ParseDocument(path, raw)
	if err != nil {
		return "", nil, err
	}
	if len(doc.Transforms) > 0 {
		content, extraFiles, err := applyDocumentTransforms(root, baseContent, path, raw, parsedDoc, doc.Transforms)
		if err != nil {
			return "", nil, err
		}
		if len(doc.Edits) == 0 {
			return content, extraFiles, nil
		}
		raw = []byte(content)
	}
	edits := make([]stepspec.StructuredEdit, 0, len(doc.Edits))
	for _, edit := range doc.Edits {
		edits = append(edits, stepspec.StructuredEdit{Op: edit.Op, RawPath: resolveStructuredEditPath(edit.RawPath, parsedDoc), Value: edit.Value})
	}
	applied, err := applyStructuredEdits(raw, edits)
	if err != nil {
		return "", nil, fmt.Errorf("apply structured edits to %s: %w", path, err)
	}
	return normalizeRenderedContent(applied), nil, nil
}

func applyDocumentTransforms(root string, baseContent map[string]string, path string, raw []byte, parsedDoc askcontract.GeneratedDocument, transforms []askcontract.RefineTransformAction) (string, []askcontract.GeneratedFile, error) {
	extraFiles := []askcontract.GeneratedFile{}
	content := string(raw)
	for _, transform := range transforms {
		resolved, err := askrefine.ResolveCandidate(parsedDoc, transform)
		if err != nil {
			return "", nil, err
		}
		transform = resolved
		switch strings.TrimSpace(transform.Type) {
		case "extract-var":
			varsPath := strings.TrimSpace(transform.VarsPath)
			if varsPath == "" {
				varsPath = "workflows/vars.yaml"
			}
			varName := strings.TrimSpace(transform.VarName)
			if varName == "" {
				return "", nil, fmt.Errorf("transform extract-var on %s requires varName", path)
			}
			rawPath := resolveStructuredEditPath(strings.TrimSpace(transform.RawPath), parsedDoc)
			if rawPath == "" {
				return "", nil, fmt.Errorf("transform extract-var on %s requires rawPath", path)
			}
			updatedTarget, err := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "set", RawPath: rawPath, Value: fmt.Sprintf("{{ .vars.%s }}", varName)}})
			if err != nil {
				return "", nil, fmt.Errorf("apply extract-var transform to %s: %w", path, err)
			}
			content = normalizeRenderedContent(updatedTarget)
			varsContent, err := loadVarsContent(root, baseContent, varsPath)
			if err != nil {
				return "", nil, err
			}
			varsDoc, err := ParseDocument(varsPath, []byte(varsContent))
			if err != nil {
				return "", nil, err
			}
			varsMap := map[string]any{}
			if varsDoc.Vars != nil {
				for key, value := range varsDoc.Vars {
					varsMap[key] = value
				}
			}
			varsMap[varName] = transform.Value
			renderedVars, err := renderDocument(varsPath, askcontract.GeneratedDocument{Path: varsPath, Kind: "vars", Vars: varsMap})
			if err != nil {
				return "", nil, err
			}
			extraFiles = append(extraFiles, askcontract.GeneratedFile{Path: varsPath, Content: renderedVars})
		case "set-field":
			rawPath := resolveStructuredEditPath(strings.TrimSpace(transform.RawPath), parsedDoc)
			if rawPath == "" {
				return "", nil, fmt.Errorf("transform set-field on %s requires rawPath", path)
			}
			updatedTarget, err := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "set", RawPath: rawPath, Value: transform.Value}})
			if err != nil {
				return "", nil, fmt.Errorf("apply set-field transform to %s: %w", path, err)
			}
			content = normalizeRenderedContent(updatedTarget)
		case "delete-field":
			rawPath := resolveStructuredEditPath(strings.TrimSpace(transform.RawPath), parsedDoc)
			if rawPath == "" {
				return "", nil, fmt.Errorf("transform delete-field on %s requires rawPath", path)
			}
			updatedTarget, err := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "delete", RawPath: rawPath}})
			if err != nil {
				return "", nil, fmt.Errorf("apply delete-field transform to %s: %w", path, err)
			}
			content = normalizeRenderedContent(updatedTarget)
		case "extract-component":
			componentPath := filepath.ToSlash(strings.TrimSpace(transform.Path))
			if componentPath == "" {
				return "", nil, fmt.Errorf("transform extract-component on %s requires path", path)
			}
			workflow := parsedDoc.Workflow
			if workflow == nil {
				return "", nil, fmt.Errorf("transform extract-component on %s requires workflow document", path)
			}
			phaseIndex, err := componentPhaseIndex(strings.TrimSpace(transform.RawPath), *workflow)
			if err != nil {
				return "", nil, err
			}
			if phaseIndex < 0 || phaseIndex >= len(workflow.Phases) {
				return "", nil, fmt.Errorf("transform extract-component on %s references invalid phase %d", path, phaseIndex)
			}
			phase := workflow.Phases[phaseIndex]
			if len(phase.Steps) == 0 {
				return "", nil, fmt.Errorf("transform extract-component on %s requires inline phase steps", path)
			}
			componentDoc := askcontract.GeneratedDocument{Path: componentPath, Kind: "component", Component: &askcontract.ComponentDocument{Steps: phase.Steps}}
			renderedComponent, err := renderDocument(componentPath, componentDoc)
			if err != nil {
				return "", nil, err
			}
			workflowCopy := *workflow
			workflowCopy.Phases = append([]askcontract.WorkflowPhase(nil), workflow.Phases...)
			phaseCopy := workflowCopy.Phases[phaseIndex]
			phaseCopy.Imports = append(append([]askcontract.PhaseImport(nil), phaseCopy.Imports...), askcontract.PhaseImport{Path: filepath.Base(componentPath)})
			phaseCopy.Steps = nil
			workflowCopy.Phases[phaseIndex] = phaseCopy
			renderedWorkflow, err := renderDocument(path, askcontract.GeneratedDocument{Path: path, Kind: "workflow", Workflow: &workflowCopy})
			if err != nil {
				return "", nil, err
			}
			content = renderedWorkflow
			extraFiles = append(extraFiles, askcontract.GeneratedFile{Path: componentPath, Content: renderedComponent})
		default:
			return "", nil, fmt.Errorf("unsupported refine transform %q for %s", transform.Type, path)
		}
	}
	return content, extraFiles, nil
}

func componentPhaseIndex(rawPath string, workflow askcontract.WorkflowDocument) (int, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return -1, fmt.Errorf("extract-component requires a phase rawPath")
	}
	if strings.HasPrefix(rawPath, "phases[") && strings.HasSuffix(rawPath, "]") {
		idxText := strings.TrimSuffix(strings.TrimPrefix(rawPath, "phases["), "]")
		idx, err := strconv.Atoi(idxText)
		if err != nil {
			return -1, fmt.Errorf("extract-component uses invalid phase index %q", rawPath)
		}
		return idx, nil
	}
	if strings.HasPrefix(rawPath, "phases.") {
		name := strings.TrimPrefix(rawPath, "phases.")
		for i, phase := range workflow.Phases {
			if strings.TrimSpace(phase.Name) == strings.TrimSpace(name) {
				return i, nil
			}
		}
	}
	return -1, fmt.Errorf("extract-component could not resolve phase %q", rawPath)
}

func loadVarsContent(root string, baseContent map[string]string, path string) (string, error) {
	if existing, ok := baseContent[path]; ok {
		return existing, nil
	}
	resolved, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(path), "/")...)
	if err == nil {
		raw, readErr := os.ReadFile(resolved) //nolint:gosec
		if readErr == nil {
			return string(raw), nil
		}
		if !os.IsNotExist(readErr) {
			return "", fmt.Errorf("read vars target %s: %w", path, readErr)
		}
	}
	return "{}\n", nil
}

func renderedFileContentMap(files []askcontract.GeneratedFile) map[string]string {
	out := make(map[string]string, len(files))
	for _, file := range files {
		if file.Delete {
			continue
		}
		out[filepath.ToSlash(strings.TrimSpace(file.Path))] = file.Content
	}
	return out
}

func applyStructuredEdits(raw []byte, edits []stepspec.StructuredEdit) ([]byte, error) {
	return structurededit.Apply(structurededit.FormatYAML, raw, edits)
}
