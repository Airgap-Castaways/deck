package workflowschema

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func stepEnvelopeSchema(kind, title, description, visibility string) map[string]any {
	apiVersion := workflowcontract.BuiltInStepAPIVersion
	if def, ok, err := workflowexec.BuiltInTypeDefinitionForKey(workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: kind}); err == nil && ok && def.Step.APIVersion != "" {
		apiVersion = def.Step.APIVersion
	}
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://deck.local/schemas/tools/" + schemaFileName(kind),
		"title":                title,
		"description":          description,
		"x-deck-visibility":    visibility,
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"id", "kind", "spec"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"apiVersion":    map[string]any{"type": "string", "const": apiVersion},
			"kind":          map[string]any{"const": kind},
			"metadata":      map[string]any{"type": "object", "additionalProperties": true},
			"when":          map[string]any{"type": "string"},
			"parallelGroup": map[string]any{"type": "string", "minLength": 1},
			"retry":         map[string]any{"type": "integer", "minimum": 0},
			"timeout":       durationStringSchema(),
			"register": map[string]any{
				"type":                 "object",
				"propertyNames":        map[string]any{"pattern": "^[A-Za-z_][A-Za-z0-9_]*$"},
				"additionalProperties": map[string]any{"type": "string"},
			},
		},
	}
}

func durationStringSchema() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[0-9]+(ms|s|m|h)$"}
}

func schemaFileName(kind string) string {
	if def, ok, err := workflowexec.BuiltInTypeDefinitionForKey(workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: kind}); err == nil && ok {
		return def.Step.SchemaFile
	}
	return strings.ToLower(kind) + ".schema.json"
}
