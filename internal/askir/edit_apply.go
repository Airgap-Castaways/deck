package askir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/structurededit"
)

func applyDocumentEdits(root string, baseContent map[string]string, path string, doc askcontract.GeneratedDocument) (string, error) {
	if len(doc.Edits) == 0 {
		return "", fmt.Errorf("document %s requested edit action without edits", path)
	}
	resolved, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(path), "/")...)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(resolved) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			if existing, ok := baseContent[path]; ok {
				raw = []byte(existing)
			} else {
				return "", fmt.Errorf("read refine target %s: %w", path, err)
			}
		} else {
			return "", fmt.Errorf("read refine target %s: %w", path, err)
		}
	}
	parsedDoc, err := ParseDocument(path, raw)
	if err != nil {
		return "", err
	}
	edits := make([]stepspec.StructuredEdit, 0, len(doc.Edits))
	for _, edit := range doc.Edits {
		edits = append(edits, stepspec.StructuredEdit{Op: edit.Op, RawPath: resolveStructuredEditPath(edit.RawPath, parsedDoc), Value: edit.Value})
	}
	applied, err := applyStructuredEdits(raw, edits)
	if err != nil {
		return "", fmt.Errorf("apply structured edits to %s: %w", path, err)
	}
	return normalizeRenderedContent(applied), nil
}

func renderedFileContentMap(files []askcontract.GeneratedFile) map[string]string {
	out := make(map[string]string, len(files))
	for _, file := range files {
		if file.Delete {
			continue
		}
		out[filepath.ToSlash(strings.TrimSpace(file.Path))] = file.Content
	}
	return out
}

func applyStructuredEdits(raw []byte, edits []stepspec.StructuredEdit) ([]byte, error) {
	return structurededit.Apply(structurededit.FormatYAML, raw, edits)
}
