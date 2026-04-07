package askcontract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/structuredpath"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func GenerationResponseSchema() json.RawMessage {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"summary", "review"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"review":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"selection": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"patterns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"vars":     openObjectSchema(),
					"targets": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"path"},
							"properties": map[string]any{
								"path": map[string]any{"type": "string"},
								"kind": map[string]any{"type": "string"},
								"builders": map[string]any{
									"type": "array",
									"items": map[string]any{
										"type":                 "object",
										"additionalProperties": false,
										"required":             []string{"id"},
										"properties": map[string]any{
											"id":        map[string]any{"type": "string"},
											"overrides": openObjectSchema(),
										},
									},
								},
								"steps":  openObjectSchema(),
								"phases": openObjectSchema(),
								"vars":   openObjectSchema(),
							},
						},
					},
				},
			},
			"documents": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"path"},
					"properties": map[string]any{
						"path":      map[string]any{"type": "string"},
						"kind":      map[string]any{"type": "string"},
						"action":    map[string]any{"type": "string"},
						"workflow":  openObjectSchema(),
						"component": openObjectSchema(),
						"vars":      openObjectSchema(),
						"transforms": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required":             []string{"type"},
								"properties": map[string]any{
									"type":      map[string]any{"type": "string"},
									"candidate": map[string]any{"type": "string"},
									"rawPath":   map[string]any{"type": "string"},
									"varName":   map[string]any{"type": "string"},
									"varsPath":  map[string]any{"type": "string"},
									"path":      map[string]any{"type": "string"},
									"value":     map[string]any{},
								},
							},
						},
						"edits": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required":             []string{"op", "rawPath"},
								"properties": map[string]any{
									"op":      map[string]any{"type": "string"},
									"rawPath": map[string]any{"type": "string"},
									"value":   map[string]any{},
								},
							},
						},
					},
				},
			},
		},
		"anyOf": []any{
			map[string]any{"required": []string{"documents"}},
			map[string]any{"required": []string{"selection"}},
		},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

func openObjectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

func ParseGeneration(raw string) (GenerationResponse, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return GenerationResponse{}, fmt.Errorf("model returned empty response")
	}
	var resp GenerationResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return GenerationResponse{}, fmt.Errorf("parse generation response: %w", err)
	}
	if strings.TrimSpace(resp.Summary) == "" {
		resp.Summary = "No summary provided."
	}
	for i := range resp.Documents {
		resp.Documents[i].Path = strings.TrimSpace(resp.Documents[i].Path)
		resp.Documents[i].Kind = normalizeDocumentKind(resp.Documents[i].Kind)
		resp.Documents[i].Action = normalizeDocumentAction(resp.Documents[i].Action)
		if resp.Documents[i].Action == "edit" && len(resp.Documents[i].Edits) == 0 && len(resp.Documents[i].Transforms) == 0 && (resp.Documents[i].Workflow != nil || resp.Documents[i].Component != nil || resp.Documents[i].Vars != nil) {
			resp.Documents[i].Action = "replace"
		}
		if strings.EqualFold(resp.Documents[i].Kind, "vars") && resp.Documents[i].Vars == nil {
			resp.Documents[i].Vars = map[string]any{}
		}
		for j := range resp.Documents[i].Edits {
			resp.Documents[i].Edits[j].Op = normalizeEditOp(resp.Documents[i].Edits[j].Op)
			resp.Documents[i].Edits[j].RawPath = normalizeEditPath(resp.Documents[i].Edits[j].RawPath, resp.Documents[i].Edits[j].Path, firstNonEmpty(resp.Documents[i].Edits[j].StepID, resp.Documents[i].Edits[j].TargetStepID), resp.Documents[i].Edits[j].Target)
			resp.Documents[i].Edits[j].RawPath = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Edits[j].RawPath)
			resp.Documents[i].Edits[j].Path = ""
			resp.Documents[i].Edits[j].StepID = ""
			resp.Documents[i].Edits[j].TargetStepID = ""
			resp.Documents[i].Edits[j].Target = nil
		}
		for j := range resp.Documents[i].Transforms {
			resp.Documents[i].Transforms[j].Type = normalizeTransformType(resp.Documents[i].Transforms[j].Type)
			resp.Documents[i].Transforms[j].Candidate = strings.TrimSpace(resp.Documents[i].Transforms[j].Candidate)
			resp.Documents[i].Transforms[j].RawPath = strings.TrimSpace(resp.Documents[i].Transforms[j].RawPath)
			resp.Documents[i].Transforms[j].VarName = strings.TrimSpace(resp.Documents[i].Transforms[j].VarName)
			resp.Documents[i].Transforms[j].VarsPath = strings.TrimSpace(resp.Documents[i].Transforms[j].VarsPath)
			normalizeExtractVarShape(&resp.Documents[i].Transforms[j])
			resp.Documents[i].Transforms[j].Path = strings.TrimSpace(resp.Documents[i].Transforms[j].Path)
			if resp.Documents[i].Transforms[j].RawPath == "" && resp.Documents[i].Transforms[j].Type != "extract-component" {
				resp.Documents[i].Transforms[j].RawPath = resp.Documents[i].Transforms[j].Path
			}
			resp.Documents[i].Transforms[j].RawPath = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Transforms[j].RawPath)
			resp.Documents[i].Transforms[j].Path = normalizeVarsDocumentRawPath(resp.Documents[i].Path, resp.Documents[i].Transforms[j].Path)
		}
	}
	if resp.Selection != nil {
		for i := range resp.Selection.Patterns {
			resp.Selection.Patterns[i] = strings.TrimSpace(resp.Selection.Patterns[i])
		}
		for i := range resp.Selection.Targets {
			resp.Selection.Targets[i].Path = strings.TrimSpace(resp.Selection.Targets[i].Path)
			resp.Selection.Targets[i].Kind = normalizeDocumentKind(resp.Selection.Targets[i].Kind)
			for j := range resp.Selection.Targets[i].Builders {
				resp.Selection.Targets[i].Builders[j].ID = strings.TrimSpace(resp.Selection.Targets[i].Builders[j].ID)
			}
		}
	}
	if len(resp.Documents) == 0 && resp.Selection == nil {
		return GenerationResponse{}, fmt.Errorf("generation response did not include documents or selection")
	}
	if len(resp.Documents) == 0 && resp.Selection != nil && !SelectionUsesBuilders(*resp.Selection) {
		resp.Documents = compileDraftSelection(*resp.Selection)
	}
	if len(resp.Documents) > 0 {
		if err := validateGeneratedDocuments(resp.Documents); err != nil {
			return GenerationResponse{}, err
		}
	}
	return resp, nil
}

func ParseEvidencePlan(raw string) (EvidencePlan, error) {
	cleaned := clean(raw)
	if cleaned == "" {
		return EvidencePlan{}, fmt.Errorf("model returned empty response")
	}
	var plan EvidencePlan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return EvidencePlan{}, fmt.Errorf("parse evidence plan: %w", err)
	}
	plan.Decision = normalizeEvidenceDecision(plan.Decision)
	if plan.Decision == "" {
		return EvidencePlan{}, fmt.Errorf("evidence plan is missing decision")
	}
	plan.Reason = strings.TrimSpace(plan.Reason)
	seen := map[string]bool{}
	normalized := make([]EvidenceEntity, 0, len(plan.Entities))
	for _, entity := range plan.Entities {
		entity.Name = strings.TrimSpace(entity.Name)
		entity.Kind = strings.TrimSpace(entity.Kind)
		if entity.Name == "" {
			continue
		}
		key := strings.ToLower(entity.Name) + "::" + strings.ToLower(entity.Kind)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, entity)
	}
	plan.Entities = normalized
	return plan, nil
}

func CompileDraftSelection(selection DraftSelection) []GeneratedDocument {
	return compileDraftSelection(selection)
}

func SelectionUsesBuilders(selection DraftSelection) bool {
	for _, target := range selection.Targets {
		if len(target.Builders) > 0 {
			return true
		}
	}
	return false
}

func validateGeneratedDocuments(documents []GeneratedDocument) error {
	for _, doc := range documents {
		if strings.TrimSpace(doc.Path) == "" {
			return fmt.Errorf("generated document path is empty")
		}
		action := strings.TrimSpace(doc.Action)
		if action == "" {
			action = inferredDocumentAction(doc)
		}
		switch action {
		case "preserve":
			continue
		case "delete":
			if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil || len(doc.Edits) > 0 {
				return fmt.Errorf("generated document %s delete action must not include content or edits", doc.Path)
			}
		case "edit":
			if len(doc.Edits) == 0 && len(doc.Transforms) == 0 && (doc.Workflow != nil || doc.Component != nil || doc.Vars != nil) {
				action = "replace"
			}
			if action == "edit" && len(doc.Edits) == 0 && len(doc.Transforms) == 0 {
				return fmt.Errorf("generated document %s edit action must include edits or transforms", doc.Path)
			}
			if action == "edit" && (doc.Workflow != nil || doc.Component != nil || doc.Vars != nil) && len(doc.Transforms) == 0 {
				return fmt.Errorf("generated document %s edit action must not include replacement content without transforms", doc.Path)
			}
			if action != "replace" {
				continue
			}
			fallthrough
		case "replace", "create":
			if err := validateDocumentPayload(doc); err != nil {
				return err
			}
		default:
			return fmt.Errorf("generated document %s uses unsupported action %q", doc.Path, action)
		}
	}
	return nil
}

func validateDocumentPayload(doc GeneratedDocument) error {
	count := 0
	if doc.Workflow != nil {
		count++
	}
	if doc.Component != nil {
		count++
	}
	if doc.Vars != nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("generated document %s must include exactly one of workflow, component, or vars", doc.Path)
	}
	return nil
}

func inferredDocumentAction(doc GeneratedDocument) string {
	if len(doc.Edits) > 0 {
		return "edit"
	}
	if doc.Workflow != nil || doc.Component != nil || doc.Vars != nil {
		return "replace"
	}
	return "preserve"
}

func compileDraftSelection(selection DraftSelection) []GeneratedDocument {
	documents := make([]GeneratedDocument, 0, len(selection.Targets)+1)
	for _, target := range selection.Targets {
		path := strings.TrimSpace(target.Path)
		if path == "" {
			continue
		}
		kind := normalizeDocumentKind(target.Kind)
		switch kind {
		case "vars":
			documents = append(documents, GeneratedDocument{Path: path, Kind: "vars", Vars: cloneMap(target.Vars)})
		case "component":
			documents = append(documents, GeneratedDocument{Path: path, Kind: "component", Component: &ComponentDocument{Steps: append([]WorkflowStep(nil), target.Steps...)}})
		default:
			workflow := &WorkflowDocument{Version: "v1alpha1", Vars: cloneMap(target.Vars), Steps: append([]WorkflowStep(nil), target.Steps...)}
			if len(target.Phases) > 0 {
				workflow.Phases = make([]WorkflowPhase, 0, len(target.Phases))
				for _, phase := range target.Phases {
					workflow.Phases = append(workflow.Phases, WorkflowPhase{Name: strings.TrimSpace(phase.Name), Imports: append([]PhaseImport(nil), phase.Imports...), Steps: append([]WorkflowStep(nil), phase.Steps...)})
				}
			}
			documents = append(documents, GeneratedDocument{Path: path, Kind: "workflow", Workflow: workflow})
		}
	}
	if len(selection.Vars) > 0 && !selectionHasVarsTarget(selection) {
		documents = append(documents, GeneratedDocument{Path: workspacepaths.CanonicalVarsWorkflow, Kind: "vars", Vars: cloneMap(selection.Vars)})
	}
	return documents
}

func selectionHasVarsTarget(selection DraftSelection) bool {
	for _, target := range selection.Targets {
		if normalizeDocumentKind(target.Kind) == "vars" || strings.TrimSpace(target.Path) == workspacepaths.CanonicalVarsWorkflow {
			return true
		}
	}
	return false
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func normalizeDocumentKind(kind string) string {
	trimmed := strings.ToLower(strings.TrimSpace(kind))
	switch trimmed {
	case "scenario":
		return "workflow"
	default:
		return trimmed
	}
}

func normalizeEditOp(op string) string {
	trimmed := strings.ToLower(strings.TrimSpace(op))
	if trimmed == "add" {
		return "insert"
	}
	if trimmed == "remove" {
		return "delete"
	}
	if trimmed == "replace" {
		return "set"
	}
	return trimmed
}

func normalizeDocumentAction(action string) string {
	trimmed := strings.ToLower(strings.TrimSpace(action))
	switch trimmed {
	case "update", "revise":
		return "edit"
	case "patch":
		return "edit"
	case "noop", "skip":
		return "preserve"
	default:
		return trimmed
	}
}

func normalizeTransformType(kind string) string {
	trimmed := strings.ToLower(strings.TrimSpace(kind))
	switch trimmed {
	case "extract_var", "extract-vars", "extractvar":
		return "extract-var"
	case "set_field", "set-field", "update-field", "update_field":
		return "set-field"
	case "delete_field", "delete-field", "remove-field", "remove_field":
		return "delete-field"
	case "extract_component", "extract-component", "extractcomponent":
		return "extract-component"
	default:
		return trimmed
	}
}

func normalizeEditPath(rawPath string, alias string, stepID string, target map[string]any) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		path = strings.TrimSpace(alias)
	}
	if strings.TrimSpace(stepID) == "" && len(target) > 0 {
		if id, ok := target["id"].(string); ok {
			stepID = id
		}
		if path == "" {
			if field, ok := target["field"].(string); ok {
				path = field
			}
		}
	}
	if strings.TrimSpace(stepID) != "" && path != "" {
		path = "steps." + strings.TrimSpace(stepID) + "." + strings.TrimPrefix(path, ".")
	}
	if strings.TrimSpace(path) == "" {
		return ""
	}
	canonical, err := structuredpath.Canonicalize(path)
	if err != nil {
		return path
	}
	return canonical
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeVarsDocumentRawPath(docPath string, rawPath string) string {
	if filepath.ToSlash(strings.TrimSpace(docPath)) != filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) {
		return strings.TrimSpace(rawPath)
	}
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "vars" {
		return ""
	}
	if strings.HasPrefix(rawPath, "vars.") {
		return strings.TrimPrefix(rawPath, "vars.")
	}
	return rawPath
}

func normalizeExtractVarShape(transform *RefineTransformAction) {
	if transform == nil || strings.TrimSpace(transform.Type) != "extract-var" {
		return
	}
	varsPath := strings.TrimSpace(transform.VarsPath)
	if varsPath == "" || strings.HasPrefix(filepath.ToSlash(varsPath), workspacepaths.WorkflowRootDir+"/") {
		return
	}
	if strings.TrimSpace(transform.VarName) == "" {
		transform.VarName = varsPath
	}
	transform.VarsPath = ""
}
