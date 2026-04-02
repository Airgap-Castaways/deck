package askir

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
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

func renderDocument(path string, doc askcontract.GeneratedDocument) (string, error) {
	switch documentKind(path, doc) {
	case "workflow":
		if doc.Workflow == nil {
			return "", fmt.Errorf("document %s is missing workflow content", path)
		}
		doc.Workflow = normalizeWorkflowDocument(doc.Workflow)
		return renderYAML(workflowFromIR(*doc.Workflow))
	case "component":
		if doc.Component == nil {
			return "", fmt.Errorf("document %s is missing component content", path)
		}
		doc.Component = normalizeComponentDocument(doc.Component)
		return renderYAML(componentFromIR(*doc.Component))
	case "vars":
		if doc.Vars == nil {
			return "", fmt.Errorf("document %s is missing vars content", path)
		}
		return renderYAML(unwrapVarsDocument(normalizeMapValues(doc.Vars)))
	default:
		return "", fmt.Errorf("document %s uses unsupported kind %q", path, doc.Kind)
	}
}

var templateAliasRE = regexp.MustCompile(`\$?\{\{\s*([a-zA-Z0-9_.\[\]-]+)\s*\}\}`)

func normalizeWorkflowDocument(doc *askcontract.WorkflowDocument) *askcontract.WorkflowDocument {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.Vars = unwrapVarsDocument(normalizeMapValues(doc.Vars))
	copyDoc.Phases = make([]askcontract.WorkflowPhase, 0, len(doc.Phases))
	for _, phase := range doc.Phases {
		phaseCopy := phase
		phaseCopy.Steps = normalizeSteps(phase.Steps)
		copyDoc.Phases = append(copyDoc.Phases, phaseCopy)
	}
	copyDoc.Steps = normalizeSteps(doc.Steps)
	return &copyDoc
}

func normalizeComponentDocument(doc *askcontract.ComponentDocument) *askcontract.ComponentDocument {
	if doc == nil {
		return nil
	}
	copyDoc := *doc
	copyDoc.Steps = normalizeSteps(doc.Steps)
	return &copyDoc
}

func normalizeSteps(items []askcontract.WorkflowStep) []askcontract.WorkflowStep {
	out := make([]askcontract.WorkflowStep, 0, len(items))
	for _, item := range items {
		step := item
		step.When = normalizeTemplateAliases(step.When)
		step.Timeout = normalizeTemplateAliases(step.Timeout)
		step.Metadata = normalizeMapValues(step.Metadata)
		step.Spec = normalizeMapValues(step.Spec)
		out = append(out, step)
	}
	return out
}

func normalizeMapValues(values map[string]any) map[string]any {
	if len(values) == 0 {
		return values
	}
	out := map[string]any{}
	for key, value := range values {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeSliceValues(values []any) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, normalizeValue(value))
	}
	return out
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return normalizeTemplateAliases(typed)
	case map[string]any:
		return normalizeMapValues(typed)
	case []any:
		return normalizeSliceValues(typed)
	default:
		return value
	}
}

func unwrapVarsDocument(values map[string]any) map[string]any {
	if len(values) != 1 {
		return values
	}
	nested, ok := values["vars"].(map[string]any)
	if !ok {
		return values
	}
	return normalizeMapValues(nested)
}

func normalizeTemplateAliases(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	return templateAliasRE.ReplaceAllStringFunc(input, func(match string) string {
		parts := templateAliasRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		expr := strings.TrimSpace(parts[1])
		expr = strings.TrimPrefix(expr, ".")
		if strings.HasPrefix(expr, "vars.") || strings.HasPrefix(expr, "runtime.") {
			return "{{ ." + expr + " }}"
		}
		return match
	})
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
	raw, err = quoteWholeValueTemplateScalars(raw)
	if err != nil {
		return "", err
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

func quoteWholeValueTemplateScalars(raw []byte) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse rendered yaml for template quoting: %w", err)
	}
	markWholeValueTemplateScalars(&node)
	var out strings.Builder
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(4)
	if err := encoder.Encode(&node); err != nil {
		_ = encoder.Close()
		return nil, fmt.Errorf("encode rendered yaml with quoted templates: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close rendered yaml encoder: %w", err)
	}
	return []byte(out.String()), nil
}

func markWholeValueTemplateScalars(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.ScalarNode {
		if expr, ok := wholeValueTemplate(node.Value); ok {
			node.Value = expr
			node.Tag = "!!str"
			node.Style = yaml.DoubleQuotedStyle
		}
		return
	}
	for i := range node.Content {
		markWholeValueTemplateScalars(node.Content[i])
	}
}

func wholeValueTemplate(value string) (string, bool) {
	normalized := normalizeTemplateAliases(strings.TrimSpace(value))
	if normalized == "" {
		return "", false
	}
	if normalized != value {
		value = normalized
	}
	if strings.HasPrefix(value, "{{ .") && strings.HasSuffix(value, " }}") {
		return value, true
	}
	return "", false
}
