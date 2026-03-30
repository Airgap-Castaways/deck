package askir

import (
	"fmt"
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
