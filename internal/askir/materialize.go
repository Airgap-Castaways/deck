package askir

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func Materialize(root string, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	return MaterializeWithBase(root, nil, gen)
}

func MaterializeWithBase(root string, base []askcontract.GeneratedFile, gen askcontract.GenerationResponse) ([]askcontract.GeneratedFile, error) {
	if len(gen.Documents) == 0 {
		return nil, nil
	}
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
