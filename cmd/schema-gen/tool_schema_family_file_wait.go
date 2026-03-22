package main

import "github.com/taedi90/deck/internal/stepspec"

func generateDownloadFileToolSchema() map[string]any {
	root := stepEnvelopeSchema("DownloadFile", "DownloadFileStep", "Downloads or stages a file into bundle output storage.", "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.DownloadFile{})
	if err != nil {
		panic(err)
	}
	delete(propertyMap(spec), "timeout")
	properties := propertyMap(spec)
	setMap(properties, "source", fileSourceSchema())
	setMap(properties, "fetch", fileFetchSchema())
	setMap(properties, "outputPath", minLenStringSchema())
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"source"}
	setMap(props, "spec", spec)
	return root
}

func generateWriteFileToolSchema() map[string]any {
	root := stepEnvelopeSchema("WriteFile", "WriteFileStep", "Writes inline or templated content to a node path.", "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.WriteFile{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "content", map[string]any{"type": "string"})
	setMap(properties, "template", map[string]any{"type": "string"})
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"path"}
	spec["oneOf"] = []any{
		map[string]any{"required": []any{"content"}, "not": map[string]any{"required": []any{"template"}}},
		map[string]any{"required": []any{"template"}, "not": map[string]any{"required": []any{"content"}}},
	}
	setMap(props, "spec", spec)
	return root
}

func generateCopyFileToolSchema() map[string]any {
	root := stepEnvelopeSchema("CopyFile", "CopyFileStep", "Copies a file already present on the node.", "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.CopyFile{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "source", fileSourceSchema())
	setMap(properties, "fetch", fileFetchSchema())
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"source", "path"}
	setMap(props, "spec", spec)
	return root
}

func generateExtractArchiveToolSchema() map[string]any {
	root := stepEnvelopeSchema("ExtractArchive", "ExtractArchiveStep", "Extracts an archive from a declared source into a destination directory.", "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.ExtractArchive{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "source", fileSourceSchema())
	setMap(properties, "fetch", fileFetchSchema())
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "include", stringArraySchema(0, false))
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"source", "path"}
	setMap(props, "spec", spec)
	return root
}

func generateEditFileToolSchema() map[string]any {
	root := stepEnvelopeSchema("EditFile", "EditFileStep", "Edits an existing file in place.", "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(&stepspec.EditFile{})
	if err != nil {
		panic(err)
	}
	properties := propertyMap(spec)
	setMap(properties, "path", minLenStringSchema())
	setMap(properties, "backup", map[string]any{"type": "boolean"})
	setMap(properties, "edits", fileEditsSchema())
	setMap(properties, "mode", modeSchema())
	spec["required"] = []any{"path", "edits"}
	setMap(props, "spec", spec)
	return root
}

func fileSourceSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"url":    minLenStringSchema(),
			"path":   minLenStringSchema(),
			"sha256": sha256Schema(),
			"bundle": bundleRefSchema(),
		},
		"anyOf": []any{
			map[string]any{"required": []any{"url"}},
			map[string]any{"required": []any{"path"}},
			map[string]any{"required": []any{"bundle"}},
		},
	}
}

func fileFetchSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"offlineOnly": map[string]any{"type": "boolean"},
			"sources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []any{"type"},
					"anyOf": []any{
						map[string]any{"required": []any{"path"}},
						map[string]any{"required": []any{"url"}},
					},
					"properties": map[string]any{
						"type": enumStringSchema("local", "bundle", "repo", "online"),
						"path": minLenStringSchema(),
						"url":  minLenStringSchema(),
					},
				},
			},
		},
	}
}

func fileEditsSchema() map[string]any {
	return map[string]any{
		"type":     "array",
		"minItems": 1,
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []any{"match"},
			"properties": map[string]any{
				"match":       map[string]any{"type": "string"},
				"replaceWith": map[string]any{"type": "string"},
				"op":          enumStringSchema("replace", "append"),
			},
		},
	}
}

func generateWaitForServiceToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForService", "WaitForServiceStep", "Waits until a systemd service reports active.", &stepspec.Wait{}, []string{"name"}, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"name":         minLenStringSchema(),
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	})
}

func generateWaitForCommandToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForCommand", "WaitForCommandStep", "Waits until a command exits successfully.", &stepspec.Wait{}, []string{"command"}, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"command":      stringArraySchema(1, false),
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	})
}

func generateWaitForFileToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForFile", "WaitForFileStep", "Waits until a file or directory exists.", &stepspec.Wait{}, []string{"path"}, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"path":         minLenStringSchema(),
		"type":         enumStringSchema("any", "file", "dir"),
		"nonEmpty":     map[string]any{"type": "boolean"},
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	})
}

func generateWaitForMissingFileToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForMissingFile", "WaitForMissingFileStep", "Waits until a file, set of files, or glob match is absent.", &stepspec.Wait{}, nil, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"path":         minLenStringSchema(),
		"paths":        stringArraySchema(1, false),
		"glob":         minLenStringSchema(),
		"type":         enumStringSchema("any", "file", "dir"),
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	}, []any{
		map[string]any{"required": []any{"path"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"paths"}}, map[string]any{"required": []any{"glob"}}}}},
		map[string]any{"required": []any{"paths"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"path"}}, map[string]any{"required": []any{"glob"}}}}},
		map[string]any{"required": []any{"glob"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []any{"path"}}, map[string]any{"required": []any{"paths"}}}}},
	})
}

func generateWaitForTCPPortToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForTCPPort", "WaitForTCPPortStep", "Waits until a TCP port becomes reachable.", &stepspec.Wait{}, []string{"port"}, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"address":      minLenStringSchema(),
		"port":         minLenStringSchema(),
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	})
}

func generateWaitForMissingTCPPortToolSchema() map[string]any {
	return generateWaitToolSchema("WaitForMissingTCPPort", "WaitForMissingTCPPortStep", "Waits until a TCP port becomes unreachable.", &stepspec.Wait{}, []string{"port"}, map[string]any{
		"interval":     durationStringSchema(),
		"initialDelay": durationStringSchema(),
		"address":      minLenStringSchema(),
		"port":         minLenStringSchema(),
		"timeout":      durationStringSchema(),
		"pollInterval": durationStringSchema(),
	})
}

func generateWaitToolSchema(kind, title, description string, specType any, required []string, properties map[string]any, extraConstraints ...[]any) map[string]any {
	root := stepEnvelopeSchema(kind, title, description, "public")
	props := propertyMap(root)
	spec, err := reflectedSpecSchema(specType)
	if err != nil {
		panic(err)
	}
	reflectedProps := propertyMap(spec)
	selected := make(map[string]any, len(properties))
	for key, override := range properties {
		if reflected, ok := reflectedProps[key].(map[string]any); ok {
			for rk, rv := range override.(map[string]any) {
				reflected[rk] = rv
			}
			selected[key] = reflected
			continue
		}
		selected[key] = override
	}
	spec["properties"] = selected
	if len(required) > 0 {
		spec["required"] = toAnySlice(required)
	}
	if len(extraConstraints) > 0 && len(extraConstraints[0]) > 0 {
		spec["oneOf"] = extraConstraints[0]
	}
	setMap(props, "spec", spec)
	return root
}
