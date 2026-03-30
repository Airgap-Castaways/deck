package stepmeta

func propertyMap(node map[string]any) map[string]any {
	props, _ := node["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
		node["properties"] = props
	}
	return props
}

func specMap(root map[string]any) map[string]any {
	props := propertyMap(root)
	spec, _ := props["spec"].(map[string]any)
	if spec == nil {
		spec = map[string]any{"type": "object", "additionalProperties": false}
		props["spec"] = spec
	}
	return spec
}

func setMap(root map[string]any, key string, value map[string]any) {
	root[key] = value
}

func toAnySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func bundleRefSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"root", "path"},
		"properties": map[string]any{
			"root": enumStringSchema("files", "images", "packages"),
			"path": minLenStringSchema(),
		},
	}
}

func enumStringSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": toAnySlice(values)}
}

func minLenStringSchema() map[string]any {
	return map[string]any{"type": "string", "minLength": 1}
}

func durationStringSchema() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[0-9]+(ms|s|m|h)$"}
}

func modeSchema() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[0-7]{4}$"}
}

func sha256Schema() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[a-fA-F0-9]{64}$"}
}

func stringArraySchema(minItems int, minLen bool) map[string]any {
	item := map[string]any{"type": "string"}
	if minLen {
		item["minLength"] = 1
	}
	field := map[string]any{"type": "array", "items": item}
	if minItems > 0 {
		field["minItems"] = minItems
	}
	return field
}
