package askir

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdraft"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

var (
	varsTemplateRefRE   = regexp.MustCompile(`\$?\{\{\s*\.?vars\.([a-zA-Z0-9_.\[\]-]+)\s*\}\}`)
	varsExpressionRefRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])\.?vars\.([a-zA-Z0-9_-]+(?:\[[^\]]+\]|\.[a-zA-Z0-9_-]+)*)`)
)

func Materialize(root string, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	return MaterializeWithBase(root, nil, gen)
}

func MaterializeWithBase(root string, base []askcontract.GeneratedFile, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	if gen.Selection != nil {
		switch {
		case askcontract.SelectionUsesBuilders(*gen.Selection):
			docs, err := askdraft.CompileWithProgram(derefProgram(gen.Program), *gen.Selection)
			if err != nil {
				return nil, err
			}
			gen.Documents = append(gen.Documents, docs...)
		case len(gen.Documents) == 0:
			gen.Documents = askcontract.CompileDraftSelection(*gen.Selection)
		}
	}
	if len(gen.Documents) == 0 {
		return nil, nil
	}
	gen.Documents = pruneUnusedVarsDocumentTransforms(gen.Documents, base)
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

func pruneUnusedVarsDocumentTransforms(documents []askcontract.GeneratedDocument, base []askcontract.GeneratedFile) []askcontract.GeneratedDocument {
	used := referencedVarNames(documents, base)
	if len(used) == 0 {
		return documents
	}
	out := append([]askcontract.GeneratedDocument(nil), documents...)
	for i := range out {
		if filepath.ToSlash(strings.TrimSpace(out[i].Path)) != filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(out[i].Action)) != "edit" {
			continue
		}
		filtered := make([]askcontract.RefineTransformAction, 0, len(out[i].Transforms))
		for _, transform := range out[i].Transforms {
			rawPath := varsRootKey(transform)
			if strings.TrimSpace(transform.Type) != "extract-component" && rawPath == "" {
				continue
			}
			if strings.TrimSpace(transform.Type) != "set-field" {
				filtered = append(filtered, transform)
				continue
			}
			if rawPath == "" || used[rawPath] {
				filtered = append(filtered, transform)
			}
		}
		out[i].Transforms = filtered
	}
	filteredDocs := make([]askcontract.GeneratedDocument, 0, len(out))
	for _, doc := range out {
		if filepath.ToSlash(strings.TrimSpace(doc.Path)) == filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) && strings.ToLower(strings.TrimSpace(doc.Action)) == "edit" && len(doc.Transforms) == 0 && len(doc.Edits) == 0 {
			continue
		}
		filteredDocs = append(filteredDocs, doc)
	}
	return filteredDocs
}

func referencedVarNames(documents []askcontract.GeneratedDocument, base []askcontract.GeneratedFile) map[string]bool {
	used := map[string]bool{}
	for _, file := range base {
		if file.Delete {
			continue
		}
		for _, match := range varTemplateMatches(file.Content) {
			used[match] = true
		}
	}
	for _, doc := range documents {
		for _, transform := range doc.Transforms {
			if strings.TrimSpace(transform.Type) == "extract-var" && strings.TrimSpace(transform.VarName) != "" {
				used[strings.TrimSpace(transform.VarName)] = true
			}
		}
		collectReferencedVarsFromDocument(used, doc)
	}
	return used
}

func collectReferencedVarsFromDocument(used map[string]bool, doc askcontract.GeneratedDocument) {
	if doc.Workflow != nil {
		collectReferencedVarsFromMap(used, doc.Workflow.Vars)
		for _, step := range doc.Workflow.Steps {
			collectReferencedVarsFromStep(used, step)
		}
		for _, phase := range doc.Workflow.Phases {
			for _, step := range phase.Steps {
				collectReferencedVarsFromStep(used, step)
			}
		}
	}
	if doc.Component != nil {
		for _, step := range doc.Component.Steps {
			collectReferencedVarsFromStep(used, step)
		}
	}
}

func collectReferencedVarsFromStep(used map[string]bool, step askcontract.WorkflowStep) {
	collectReferencedVarsFromString(used, step.When)
	collectReferencedVarsFromString(used, step.Timeout)
	collectReferencedVarsFromMap(used, step.Metadata)
	collectReferencedVarsFromMap(used, step.Spec)
}

func collectReferencedVarsFromMap(used map[string]bool, values map[string]any) {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			collectReferencedVarsFromString(used, typed)
		case map[string]any:
			collectReferencedVarsFromMap(used, typed)
		case []any:
			for _, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					collectReferencedVarsFromMap(used, nested)
				} else if text, ok := item.(string); ok {
					collectReferencedVarsFromString(used, text)
				}
			}
		}
	}
}

func collectReferencedVarsFromString(used map[string]bool, text string) {
	for _, match := range varTemplateMatches(text) {
		used[match] = true
	}
}

func varTemplateMatches(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, match := range varsTemplateRefRE.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 {
			out = appendVarTemplateMatch(out, seen, match[1])
		}
	}
	for _, match := range varsExpressionRefRE.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 {
			out = appendVarTemplateMatch(out, seen, match[1])
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendVarTemplateMatch(out []string, seen map[string]bool, name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return out
	}
	if !seen[name] {
		seen[name] = true
		out = append(out, name)
	}
	if idx := strings.IndexAny(name, ".["); idx > 0 {
		root := strings.TrimSpace(name[:idx])
		if root != "" && !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	return out
}

func varsRootKey(transform askcontract.RefineTransformAction) string {
	raw := strings.TrimSpace(transform.RawPath)
	if raw == "" {
		raw = strings.TrimSpace(transform.Path)
	}
	for i, r := range raw {
		if r == '.' || r == '[' {
			return strings.TrimSpace(raw[:i])
		}
	}
	return raw
}

func derefProgram(program *askcontract.AuthoringProgram) askcontract.AuthoringProgram {
	if program == nil {
		return askcontract.AuthoringProgram{}
	}
	return *program
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
		content, extraFiles, err := applyDocumentEdits(root, baseContent, path, doc)
		if err != nil {
			return nil, err
		}
		files := []askcontract.GeneratedFile{{Path: path, Content: content}}
		files = append(files, extraFiles...)
		return files, nil
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
