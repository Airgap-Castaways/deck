package legacygen

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func MaybeConvert(kind string, raw string) string {
	switch strings.TrimSpace(kind) {
	case "generate", "generate-fast", "postprocess-edit":
		return ToDocuments(raw)
	default:
		return raw
	}
}

func ToDocuments(raw string) string {
	type legacyFile struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	type legacyResponse struct {
		Summary string       `json:"summary"`
		Review  []string     `json:"review"`
		Files   []legacyFile `json:"files"`
	}
	var legacy legacyResponse
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil || len(legacy.Files) == 0 {
		return raw
	}
	resp := askcontract.GenerationResponse{Summary: legacy.Summary, Review: legacy.Review, Documents: make([]askcontract.GeneratedDocument, 0, len(legacy.Files))}
	for _, file := range legacy.Files {
		doc := askcontract.GeneratedDocument{Path: file.Path, Action: "replace"}
		switch {
		case strings.HasSuffix(file.Path, "vars.yaml"):
			var vars map[string]any
			if err := yaml.Unmarshal([]byte(file.Content), &vars); err != nil {
				return raw
			}
			if vars == nil {
				vars = map[string]any{}
			}
			doc.Kind = "vars"
			doc.Vars = vars
		case strings.Contains(file.Path, "/components/"):
			var component struct {
				Steps []askcontract.WorkflowStep `yaml:"steps"`
			}
			if err := yaml.Unmarshal([]byte(file.Content), &component); err != nil {
				return raw
			}
			doc.Kind = "component"
			doc.Component = &askcontract.ComponentDocument{Steps: component.Steps}
		default:
			var workflow askcontract.WorkflowDocument
			if err := yaml.Unmarshal([]byte(file.Content), &workflow); err != nil {
				return raw
			}
			doc.Kind = "workflow"
			doc.Workflow = &workflow
		}
		resp.Documents = append(resp.Documents, doc)
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return raw
	}
	return string(out)
}
