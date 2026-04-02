package askir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askrefine"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/structurededit"
	"github.com/Airgap-Castaways/deck/internal/structuredpath"
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
			} else if filepath.ToSlash(strings.TrimSpace(path)) == "workflows/vars.yaml" {
				raw = []byte("{}\n")
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
	content := string(raw)
	pending := map[string]string{}
	orderedExtraPaths := []string{}
	for _, transform := range transforms {
		currentDoc, err := ParseDocument(path, []byte(content))
		if err != nil {
			return "", nil, err
		}
		parsedDoc = currentDoc
		resolved, err := askrefine.ResolveCandidate(parsedDoc, transform)
		if err != nil {
			var unknown askrefine.UnknownCandidateError
			if errors.As(err, &unknown) && unknown.Ignorable {
				continue
			}
			return "", nil, err
		}
		transform = resolved
		switch strings.TrimSpace(transform.Type) {
		case "extract-var":
			varsPath := strings.TrimSpace(transform.VarsPath)
			if varsPath == "" || !strings.HasPrefix(filepath.ToSlash(varsPath), "workflows/") {
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
			if transform.Value == nil {
				transform.Value = currentDocumentValue([]byte(content), rawPath, parsedDoc)
			}
			updatedTarget, err := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "set", RawPath: rawPath, Value: fmt.Sprintf("{{ .vars.%s }}", varName)}})
			if err != nil {
				return "", nil, fmt.Errorf("apply extract-var transform to %s: %w", path, err)
			}
			content = normalizeRenderedContent(updatedTarget)
			varsContent, err := loadVarsContent(root, baseContent, pending, varsPath)
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
			if _, ok := pending[varsPath]; !ok {
				orderedExtraPaths = append(orderedExtraPaths, varsPath)
			}
			pending[varsPath] = renderedVars
		case "set-field":
			rawPathValue := strings.TrimSpace(transform.RawPath)
			if rawPathValue == "" {
				rawPathValue = strings.TrimSpace(transform.Path)
			}
			rawPath := resolveStructuredEditPath(rawPathValue, parsedDoc)
			if rawPath == "" {
				return "", nil, fmt.Errorf("transform set-field on %s requires rawPath", path)
			}
			updatedTarget, err := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "set", RawPath: rawPath, Value: transform.Value}})
			if err != nil {
				return "", nil, fmt.Errorf("apply set-field transform to %s: %w", path, err)
			}
			content = normalizeRenderedContent(updatedTarget)
		case "delete-field":
			rawPathValue := strings.TrimSpace(transform.RawPath)
			if rawPathValue == "" {
				rawPathValue = strings.TrimSpace(transform.Path)
			}
			rawPath := resolveStructuredEditPath(rawPathValue, parsedDoc)
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
			if _, ok := pending[componentPath]; !ok {
				orderedExtraPaths = append(orderedExtraPaths, componentPath)
			}
			pending[componentPath] = renderedComponent
		default:
			return "", nil, fmt.Errorf("unsupported refine transform %q for %s", transform.Type, path)
		}
	}
	content, err := normalizeEmptyWorkflowVars(content, path)
	if err != nil {
		return "", nil, err
	}
	pending, orderedExtraPaths, err = prunePendingVarsWrites(root, baseContent, path, content, pending, orderedExtraPaths)
	if err != nil {
		return "", nil, err
	}
	extraFiles := make([]askcontract.GeneratedFile, 0, len(orderedExtraPaths))
	for _, extraPath := range orderedExtraPaths {
		extraFiles = append(extraFiles, askcontract.GeneratedFile{Path: extraPath, Content: pending[extraPath]})
	}
	return content, extraFiles, nil
}

func prunePendingVarsWrites(root string, baseContent map[string]string, currentPath string, currentContent string, pending map[string]string, orderedExtraPaths []string) (map[string]string, []string, error) {
	varsPath := filepath.ToSlash("workflows/vars.yaml")
	used := map[string]bool{}
	for path, content := range baseContent {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || path == varsPath || path == filepath.ToSlash(strings.TrimSpace(currentPath)) {
			continue
		}
		collectReferencedVarsFromString(used, content)
	}
	collectReferencedVarsFromString(used, currentContent)
	for path, content := range pending {
		path = filepath.ToSlash(strings.TrimSpace(path))
		if path == "" || path == varsPath {
			continue
		}
		collectReferencedVarsFromString(used, content)
	}
	if len(used) == 0 {
		used = map[string]bool{}
	}
	filteredPending := map[string]string{}
	filteredOrder := make([]string, 0, len(orderedExtraPaths))
	existingVarsKeys, err := existingVarsKeys(root, baseContent, varsPath)
	if err != nil {
		return nil, nil, err
	}
	for _, extraPath := range orderedExtraPaths {
		normalized := filepath.ToSlash(strings.TrimSpace(extraPath))
		content, ok := pending[extraPath]
		if !ok {
			content, ok = pending[normalized]
		}
		if !ok {
			continue
		}
		if normalized != varsPath {
			filteredPending[extraPath] = content
			filteredOrder = append(filteredOrder, extraPath)
			continue
		}
		varsDoc, err := ParseDocument(normalized, []byte(content))
		if err != nil {
			return nil, nil, err
		}
		filteredVars := map[string]any{}
		for key, value := range varsDoc.Vars {
			if used[strings.TrimSpace(key)] || existingVarsKeys[strings.TrimSpace(key)] {
				filteredVars[key] = value
			}
		}
		if len(filteredVars) == 0 {
			continue
		}
		rendered, err := renderDocument(normalized, askcontract.GeneratedDocument{Path: normalized, Kind: "vars", Vars: filteredVars})
		if err != nil {
			return nil, nil, err
		}
		filteredPending[extraPath] = rendered
		filteredOrder = append(filteredOrder, extraPath)
	}
	return filteredPending, filteredOrder, nil
}

func existingVarsKeys(root string, baseContent map[string]string, varsPath string) (map[string]bool, error) {
	keys := map[string]bool{}
	content, ok := baseContent[varsPath]
	if !ok {
		resolved := filepath.Join(root, filepath.FromSlash(varsPath))
		raw, err := os.ReadFile(resolved) //nolint:gosec
		if err != nil {
			if os.IsNotExist(err) {
				return keys, nil
			}
			return nil, err
		}
		content = string(raw)
	}
	varsDoc, err := ParseDocument(varsPath, []byte(content))
	if err != nil {
		return nil, err
	}
	for key := range varsDoc.Vars {
		keys[strings.TrimSpace(key)] = true
	}
	return keys, nil
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

func loadVarsContent(root string, baseContent map[string]string, pending map[string]string, path string) (string, error) {
	if existing, ok := pending[path]; ok {
		return existing, nil
	}
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

func currentDocumentValue(raw []byte, rawPath string, doc askcontract.GeneratedDocument) any {
	if len(raw) == 0 {
		return nil
	}
	var model any
	if err := yaml.Unmarshal(raw, &model); err != nil {
		return nil
	}
	normalized := normalizeEditableValue(model)
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return nil
	}
	segments, err := structuredpath.Parse(rawPath)
	if err != nil {
		segments, err = structuredpath.Parse(resolveStructuredEditPath(rawPath, doc))
		if err != nil {
			return nil
		}
	}
	value, ok := valueAtStructuredPath(normalized, segments)
	if ok {
		return value
	}
	resolvedPath := resolveStructuredEditPath(rawPath, doc)
	if strings.TrimSpace(resolvedPath) == "" || resolvedPath == rawPath {
		return nil
	}
	segments, err = structuredpath.Parse(resolvedPath)
	if err != nil {
		return nil
	}
	value, ok = valueAtStructuredPath(normalized, segments)
	if !ok {
		return nil
	}
	return value
}

func normalizeEditableValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			out[key] = normalizeEditableValue(item)
		}
		return out
	case map[any]any:
		out := map[string]any{}
		for key, item := range typed {
			out[fmt.Sprint(key)] = normalizeEditableValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeEditableValue(item))
		}
		return out
	default:
		return typed
	}
}

func valueAtStructuredPath(current any, segments []structuredpath.Segment) (any, bool) {
	for _, segment := range segments {
		if segment.IsIndex {
			items, ok := current.([]any)
			if !ok || segment.Index < 0 || segment.Index >= len(items) {
				return nil, false
			}
			current = items[segment.Index]
			continue
		}
		values, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := values[segment.Key]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func normalizeEmptyWorkflowVars(content string, path string) (string, error) {
	doc, parseErr := ParseDocument(path, []byte(content))
	if parseErr == nil && doc.Workflow != nil && len(doc.Workflow.Vars) == 0 {
		updated, applyErr := applyStructuredEdits([]byte(content), []stepspec.StructuredEdit{{Op: "delete", RawPath: "vars"}})
		if applyErr == nil {
			return normalizeRenderedContent(updated), nil
		}
	}
	return content, nil
}
