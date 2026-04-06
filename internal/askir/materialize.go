package askir

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askdraft"
	"github.com/Airgap-Castaways/deck/internal/workflowrefs"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
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
	gen.Documents = pruneUnusedVarsDocumentTransforms(root, gen.Documents, base)
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

func pruneUnusedVarsDocumentTransforms(root string, documents []askcontract.GeneratedDocument, base []askcontract.GeneratedFile) []askcontract.GeneratedDocument {
	used, previewOK := referencedVarNamesAfterMaterialization(root, documents, base)
	if !previewOK {
		used, previewOK = referencedVarNames(documents, base)
	}
	if !previewOK {
		return documents
	}
	if len(used) == 0 && !documentsContainNonVarsEdits(documents) {
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

func documentsContainNonVarsEdits(documents []askcontract.GeneratedDocument) bool {
	varsPath := filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel))
	for _, doc := range documents {
		if filepath.ToSlash(strings.TrimSpace(doc.Path)) != varsPath {
			return true
		}
	}
	return false
}

func referencedVarNamesAfterMaterialization(root string, documents []askcontract.GeneratedDocument, base []askcontract.GeneratedFile) (map[string]bool, bool) {
	baseContent := renderedFileContentMap(base)
	finalFiles := map[string]askcontract.GeneratedFile{}
	for _, file := range base {
		path := filepath.ToSlash(strings.TrimSpace(file.Path))
		if path == "" || file.Delete {
			continue
		}
		finalFiles[path] = askcontract.GeneratedFile{Path: path, Content: file.Content}
	}
	for _, doc := range documents {
		files, err := materializeDocument(root, baseContent, doc)
		if err != nil {
			return nil, false
		}
		for _, file := range files {
			path := filepath.ToSlash(strings.TrimSpace(file.Path))
			if path == "" || path == filepath.ToSlash(filepath.Join(workspacepaths.WorkflowRootDir, workspacepaths.WorkflowVarsRel)) {
				continue
			}
			if file.Delete {
				delete(finalFiles, path)
				continue
			}
			finalFiles[path] = askcontract.GeneratedFile{Path: path, Content: file.Content}
		}
	}
	used := map[string]bool{}
	for _, file := range finalFiles {
		if !collectReferencedVarsFromGeneratedFile(used, file) {
			return nil, false
		}
	}
	return used, true
}

func referencedVarNames(documents []askcontract.GeneratedDocument, base []askcontract.GeneratedFile) (map[string]bool, bool) {
	used := map[string]bool{}
	for _, file := range base {
		if file.Delete {
			continue
		}
		if !collectReferencedVarsFromGeneratedFile(used, file) {
			return nil, false
		}
	}
	for _, doc := range documents {
		for _, transform := range doc.Transforms {
			if strings.TrimSpace(transform.Type) == "extract-var" && strings.TrimSpace(transform.VarName) != "" {
				used[strings.TrimSpace(transform.VarName)] = true
			}
		}
		if !collectReferencedVarsFromDocument(used, doc) {
			return nil, false
		}
	}
	return used, true
}

func collectReferencedVarsFromGeneratedFile(used map[string]bool, file askcontract.GeneratedFile) bool {
	parsed, err := ParseDocument(file.Path, []byte(file.Content))
	if err != nil {
		return false
	}
	return collectReferencedVarsFromDocument(used, parsed)
}

func collectReferencedVarsFromDocument(used map[string]bool, doc askcontract.GeneratedDocument) bool {
	if doc.Workflow != nil {
		collectReferencedVarsFromMap(used, doc.Workflow.Vars)
		for _, phase := range doc.Workflow.Phases {
			for _, item := range phase.Imports {
				if !collectReferencedVarsFromWhen(used, item.When) {
					return false
				}
			}
		}
		for _, step := range doc.Workflow.Steps {
			if !collectReferencedVarsFromStep(used, step) {
				return false
			}
		}
		for _, phase := range doc.Workflow.Phases {
			for _, step := range phase.Steps {
				if !collectReferencedVarsFromStep(used, step) {
					return false
				}
			}
		}
	}
	if doc.Component != nil {
		for _, step := range doc.Component.Steps {
			if !collectReferencedVarsFromStep(used, step) {
				return false
			}
		}
	}
	return true
}

func collectReferencedVarsFromWhen(used map[string]bool, expr string) bool {
	refs, err := workflowrefs.WhenReferences(expr)
	if err != nil {
		return false
	}
	for _, ref := range refs {
		if ref.Namespace == workflowrefs.NamespaceVars {
			used[ref.Path] = true
		}
	}
	return true
}

func collectReferencedVarsFromStep(used map[string]bool, step askcontract.WorkflowStep) bool {
	if !collectReferencedVarsFromWhen(used, step.When) {
		return false
	}
	collectReferencedVarsFromString(used, step.Timeout)
	collectReferencedVarsFromMap(used, step.Metadata)
	collectReferencedVarsFromMap(used, step.Spec)
	return true
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
	for _, ref := range workflowrefs.TemplateReferences(text) {
		if ref.Namespace == workflowrefs.NamespaceVars {
			used[ref.Path] = true
		}
	}
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
