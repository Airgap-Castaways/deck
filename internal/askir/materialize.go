package askir

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/structurededit"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type workflowRender struct {
	Version string         `yaml:"version"`
	Vars    map[string]any `yaml:"vars,omitempty"`
	Phases  []phaseRender  `yaml:"phases,omitempty"`
	Steps   []stepRender   `yaml:"steps,omitempty"`
}

type phaseRender struct {
	Name           string         `yaml:"name"`
	MaxParallelism int            `yaml:"maxParallelism,omitempty"`
	Imports        []importRender `yaml:"imports,omitempty"`
	Steps          []stepRender   `yaml:"steps,omitempty"`
}

type importRender struct {
	Path string `yaml:"path"`
	When string `yaml:"when,omitempty"`
}

type stepRender struct {
	ID            string            `yaml:"id"`
	APIVersion    string            `yaml:"apiVersion,omitempty"`
	Kind          string            `yaml:"kind"`
	Metadata      map[string]any    `yaml:"metadata,omitempty"`
	When          string            `yaml:"when,omitempty"`
	ParallelGroup string            `yaml:"parallelGroup,omitempty"`
	Register      map[string]string `yaml:"register,omitempty"`
	Retry         int               `yaml:"retry,omitempty"`
	Timeout       string            `yaml:"timeout,omitempty"`
	Spec          map[string]any    `yaml:"spec"`
}

type componentRender struct {
	Steps []stepRender `yaml:"steps"`
}

func Materialize(root string, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	return MaterializeWithBase(root, nil, gen)
}

func MaterializeWithBase(root string, base []askcontract.GeneratedFile, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	if len(gen.Documents) == 0 {
		return nil, nil
	}
	baseContent := renderedFileContentMap(base)
	materialized := append([]askcontract.GeneratedFile(nil), base...)
	index := map[string]int{}
	for i, file := range materialized {
		index[strings.TrimSpace(file.Path)] = i
	}
	for _, doc := range gen.Documents {
		files, err := materializeDocument(root, baseContent, doc)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			path := strings.TrimSpace(file.Path)
			if idx, ok := index[path]; ok {
				materialized[idx] = file
				continue
			}
			index[path] = len(materialized)
			materialized = append(materialized, file)
		}
	}
	return materialized, nil
}

func materializeDocument(root string, baseContent map[string]string, doc askcontract.GeneratedDocument) ([]askcontract.GeneratedFile, error) {
	action := normalizeAction(doc)
	path := filepath.ToSlash(strings.TrimSpace(doc.Path))
	if path == "" {
		return nil, fmt.Errorf("generated document path is empty")
	}
	switch action {
	case "preserve":
		return nil, nil
	case "delete":
		return []askcontract.GeneratedFile{{Path: path, Delete: true}}, nil
	case "edit":
		content, err := applyDocumentEdits(root, baseContent, path, doc)
		if err != nil {
			return nil, err
		}
		return []askcontract.GeneratedFile{{Path: path, Content: content}}, nil
	case "replace", "create":
		content, err := renderDocument(path, doc)
		if err != nil {
			return nil, err
		}
		return []askcontract.GeneratedFile{{Path: path, Content: content}}, nil
	default:
		return nil, fmt.Errorf("unsupported generated document action %q for %s", action, path)
	}
}

func normalizeAction(doc askcontract.GeneratedDocument) string {
	action := strings.ToLower(strings.TrimSpace(doc.Action))
	if action != "" {
		return action
	}
	if len(doc.Edits) > 0 {
		return "edit"
	}
	if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil {
		return "replace"
	}
	return "preserve"
}

func documentKind(path string, doc askcontract.GeneratedDocument) string {
	kind := strings.ToLower(strings.TrimSpace(doc.Kind))
	if kind != "" {
		return kind
	}
	switch {
	case doc.Workflow != nil:
		return "workflow"
	case doc.Component != nil:
		return "component"
	case doc.Vars != nil:
		return "vars"
	case filepath.ToSlash(path) == filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)):
		return "vars"
	case workspacepaths.IsComponentWorkflowPath(path):
		return "component"
	default:
		return "workflow"
	}
}

func renderDocument(path string, doc askcontract.GeneratedDocument) (string, error) {
	switch documentKind(path, doc) {
	case "workflow":
		if doc.Workflow == nil {
			return "", fmt.Errorf("document %s is missing workflow content", path)
		}
		return renderYAML(workflowFromIR(*doc.Workflow))
	case "component":
		if doc.Component == nil {
			return "", fmt.Errorf("document %s is missing component content", path)
		}
		return renderYAML(componentFromIR(*doc.Component))
	case "vars":
		if doc.Vars == nil {
			return "", fmt.Errorf("document %s is missing vars content", path)
		}
		return renderYAML(doc.Vars)
	default:
		return "", fmt.Errorf("document %s uses unsupported kind %q", path, doc.Kind)
	}
}

func applyDocumentEdits(root string, baseContent map[string]string, path string, doc askcontract.GeneratedDocument) (string, error) {
	if len(doc.Edits) == 0 {
		return "", fmt.Errorf("document %s requested edit action without edits", path)
	}
	resolved, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(path), "/")...)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(resolved) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			if existing, ok := baseContent[path]; ok {
				raw = []byte(existing)
			} else {
				return "", fmt.Errorf("read refine target %s: %w", path, err)
			}
		} else {
			return "", fmt.Errorf("read refine target %s: %w", path, err)
		}
	}
	if _, err := ParseDocument(path, raw); err != nil {
		return "", err
	}
	parsedDoc, err := ParseDocument(path, raw)
	if err != nil {
		return "", err
	}
	edits := make([]stepspec.StructuredEdit, 0, len(doc.Edits))
	for _, edit := range doc.Edits {
		edits = append(edits, stepspec.StructuredEdit{Op: edit.Op, RawPath: resolveStructuredEditPath(edit.RawPath, parsedDoc), Value: edit.Value})
	}
	applied, err := applyStructuredEdits(raw, edits)
	if err != nil {
		return "", fmt.Errorf("apply structured edits to %s: %w", path, err)
	}
	return normalizeRenderedContent(applied), nil
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

func resolveStructuredEditPath(rawPath string, doc askcontract.GeneratedDocument) string {
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

func applyStructuredEdits(raw []byte, edits []stepspec.StructuredEdit) ([]byte, error) {
	return structurededit.Apply(structurededit.FormatYAML, raw, edits)
}

func workflowFromIR(doc askcontract.WorkflowDocument) workflowRender {
	out := workflowRender{Version: strings.TrimSpace(doc.Version)}
	if len(doc.Vars) > 0 {
		out.Vars = doc.Vars
	}
	if len(doc.Phases) > 0 {
		out.Phases = make([]phaseRender, 0, len(doc.Phases))
		for _, phase := range doc.Phases {
			out.Phases = append(out.Phases, phaseRender{
				Name:           phase.Name,
				MaxParallelism: phase.MaxParallelism,
				Imports:        importsFromIR(phase.Imports),
				Steps:          stepsFromIR(phase.Steps),
			})
		}
	}
	if len(doc.Steps) > 0 {
		out.Steps = stepsFromIR(doc.Steps)
	}
	return out
}

func componentFromIR(doc askcontract.ComponentDocument) componentRender {
	return componentRender{Steps: stepsFromIR(doc.Steps)}
}

func importsFromIR(items []askcontract.PhaseImport) []importRender {
	out := make([]importRender, 0, len(items))
	for _, item := range items {
		out = append(out, importRender{Path: item.Path, When: item.When})
	}
	return out
}

func stepsFromIR(items []askcontract.WorkflowStep) []stepRender {
	out := make([]stepRender, 0, len(items))
	for _, item := range items {
		out = append(out, stepRender{
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

func renderYAML(doc any) (string, error) {
	raw, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshal document yaml: %w", err)
	}
	return normalizeRenderedContent(raw), nil
}

func normalizeRenderedContent(raw []byte) string {
	trimmed := strings.TrimRight(string(raw), "\n")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}
