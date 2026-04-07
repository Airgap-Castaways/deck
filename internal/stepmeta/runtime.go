package stepmeta

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RuntimeOutputOptions struct {
	FileExists func(path string) bool
}

func ProjectRuntimeOutputsForKind(kind string, rendered map[string]any, runtime map[string]any, opts RuntimeOutputOptions) (map[string]any, error) {
	entry, ok, err := LookupCatalogEntry(kind)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("missing stepmeta registration for %s", strings.TrimSpace(kind))
	}
	return ProjectRuntimeOutputs(entry, rendered, runtime, opts), nil
}

func ProjectRuntimeOutputs(entry Entry, rendered map[string]any, runtime map[string]any, opts RuntimeOutputOptions) map[string]any {
	projected := map[string]any{}
	for _, output := range entry.Definition.Outputs {
		switch output {
		case "artifacts":
			if artifacts := projectedArtifacts(rendered, runtime); len(artifacts) > 0 {
				projected[output] = artifacts
			}
		case "failedChecks":
			if checks := stringListValue(runtime, output); len(checks) > 0 {
				projected[output] = checks
			}
		case "joinFile":
			if joinFile := projectedJoinFile(rendered, opts); joinFile != "" {
				projected[output] = joinFile
			}
		case "name":
			if name := stringValue(rendered, output); name != "" {
				projected[output] = name
			}
		case "names":
			if names := stringListValue(rendered, output); len(names) > 0 {
				projected[output] = names
			}
		case "outputPath":
			if path := projectedOutputPath(rendered, runtime); path != "" {
				projected[output] = path
			}
		case "outputPaths":
			if paths := projectedOutputPaths(rendered, runtime); len(paths) > 0 {
				projected[output] = paths
			}
		case "passed":
			if value, ok := runtime[output]; ok {
				projected[output] = value
			}
		case "path":
			if path := projectedPath(rendered); path != "" {
				projected[output] = path
			}
		}
	}
	for key, value := range runtime {
		projected[key] = value
	}
	return projected
}

func projectedPath(rendered map[string]any) string {
	if path := stringValue(mapValue(rendered, "output"), "path"); path != "" {
		return path
	}
	return stringValue(rendered, "path")
}

func projectedArtifacts(rendered map[string]any, runtime map[string]any) []string {
	if artifacts := stringListValue(runtime, "artifacts"); len(artifacts) > 0 {
		return artifacts
	}
	if paths := stringListValue(runtime, "outputPaths"); len(paths) > 0 {
		return paths
	}
	if path := stringValue(runtime, "outputPath"); path != "" {
		return []string{path}
	}
	return inferDownloadFileOutputs(rendered)
}

func projectedOutputPaths(rendered map[string]any, runtime map[string]any) []string {
	if paths := stringListValue(runtime, "outputPaths"); len(paths) > 0 {
		return paths
	}
	return projectedArtifacts(rendered, runtime)
}

func projectedOutputPath(rendered map[string]any, runtime map[string]any) string {
	if path := stringValue(runtime, "outputPath"); path != "" {
		return path
	}
	paths := projectedOutputPaths(rendered, runtime)
	if len(paths) == 1 {
		return paths[0]
	}
	return ""
}

func projectedJoinFile(rendered map[string]any, opts RuntimeOutputOptions) string {
	joinFile := stringValue(rendered, "outputJoinFile")
	if joinFile == "" {
		return ""
	}
	fileExists := opts.FileExists
	if fileExists == nil {
		fileExists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}
	if !fileExists(joinFile) {
		return ""
	}
	return joinFile
}

func inferDownloadFileOutputs(rendered map[string]any) []string {
	if items := mapItems(rendered["items"]); len(items) > 0 {
		outputs := make([]string, 0, len(items))
		for _, item := range items {
			if path := inferDownloadFileOutputPath(item); path != "" {
				outputs = append(outputs, path)
			}
		}
		return outputs
	}
	if path := inferDownloadFileOutputPath(rendered); path != "" {
		return []string{path}
	}
	return nil
}

func inferDownloadFileOutputPath(rendered map[string]any) string {
	if path := stringValue(rendered, "outputPath"); path != "" {
		return path
	}
	source := mapValue(rendered, "source")
	if len(source) == 0 {
		return ""
	}
	return filepath.ToSlash(filepath.Join("files", inferDownloadFileName(stringValue(source, "path"), stringValue(source, "url"))))
}

func inferDownloadFileName(sourcePath, sourceURL string) string {
	if strings.TrimSpace(sourcePath) != "" {
		base := filepath.Base(filepath.FromSlash(strings.TrimSpace(sourcePath)))
		if base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	if strings.TrimSpace(sourceURL) != "" {
		trimmed := strings.TrimSpace(sourceURL)
		if idx := strings.Index(trimmed, "?"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		base := filepath.Base(filepath.FromSlash(trimmed))
		if base != "" && base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return "downloaded.bin"
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func stringListValue(values map[string]any, key string) []string {
	if values == nil {
		return nil
	}
	return stringList(values[key])
}

func stringList(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	default:
		return nil
	}
}

func mapValue(values map[string]any, key string) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	raw, ok := values[key]
	if !ok {
		return map[string]any{}
	}
	mapped, ok := raw.(map[string]any)
	if !ok || mapped == nil {
		return map[string]any{}
	}
	return mapped
}

func mapItems(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mapped, ok := item.(map[string]any)
		if !ok || mapped == nil {
			continue
		}
		out = append(out, mapped)
	}
	return out
}
