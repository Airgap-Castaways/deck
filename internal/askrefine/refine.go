package askrefine

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

type Candidate struct {
	ID                     string
	Path                   string
	Type                   string
	RawPath                string
	Summary                string
	SuggestedVarName       string
	SuggestedVarsPath      string
	SuggestedComponentPath string
}

func CandidatesForDocuments(plan askcontract.PlanResponse, docs []askcontract.GeneratedDocument) []Candidate {
	allowed := allowedPaths(plan)
	out := []Candidate{}
	for _, doc := range docs {
		path := filepath.ToSlash(strings.TrimSpace(doc.Path))
		if path == "" || (len(allowed) > 0 && !allowed[path]) {
			continue
		}
		out = append(out, candidatesForDocument(doc)...)
	}
	return dedupeCandidates(out)
}

func ResolveCandidate(doc askcontract.GeneratedDocument, transform askcontract.RefineTransformAction) (askcontract.RefineTransformAction, error) {
	if strings.TrimSpace(transform.Candidate) == "" {
		return transform, nil
	}
	for _, candidate := range candidatesForDocument(doc) {
		if candidate.ID != strings.TrimSpace(transform.Candidate) {
			continue
		}
		transform.Type = candidate.Type
		if strings.TrimSpace(transform.RawPath) == "" {
			transform.RawPath = candidate.RawPath
		}
		if candidate.Type == "extract-var" {
			if strings.TrimSpace(transform.VarName) == "" {
				transform.VarName = candidate.SuggestedVarName
			}
			if strings.TrimSpace(transform.VarsPath) == "" {
				transform.VarsPath = candidate.SuggestedVarsPath
			}
		}
		if candidate.Type == "extract-component" && strings.TrimSpace(transform.Path) == "" {
			transform.Path = candidate.SuggestedComponentPath
		}
		return transform, nil
	}
	return transform, fmt.Errorf("unknown refine transform candidate %q for %s", transform.Candidate, strings.TrimSpace(doc.Path))
}

func PromptBlock(plan askcontract.PlanResponse, docs []askcontract.GeneratedDocument) string {
	candidates := CandidatesForDocuments(plan, docs)
	if len(candidates) == 0 {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("Current refine transform candidates:\n")
	b.WriteString("- Select transform candidate ids and optional override values; do not invent raw paths when a candidate id already exists.\n")
	for _, candidate := range candidates {
		b.WriteString("- id: ")
		b.WriteString(candidate.ID)
		b.WriteString(" type=")
		b.WriteString(candidate.Type)
		b.WriteString(" path=")
		b.WriteString(candidate.Path)
		b.WriteString(" summary=")
		b.WriteString(candidate.Summary)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func candidatesForDocument(doc askcontract.GeneratedDocument) []Candidate {
	path := filepath.ToSlash(strings.TrimSpace(doc.Path))
	items := []Candidate{}
	if doc.Workflow != nil {
		for i, phase := range doc.Workflow.Phases {
			if len(phase.Steps) == 0 {
				continue
			}
			rawPath := fmt.Sprintf("phases[%d]", i)
			phaseName := strings.TrimSpace(phase.Name)
			componentPath := "workflows/components/" + sanitizeName(firstNonEmpty(phaseName, "phase")) + ".yaml"
			items = append(items, Candidate{ID: candidateID("extract-component", path, rawPath), Path: path, Type: "extract-component", RawPath: rawPath, Summary: fmt.Sprintf("extract phase %q into %s", firstNonEmpty(phaseName, "phase"), componentPath), SuggestedComponentPath: componentPath})
		}
		items = append(items, stepFieldCandidates(path, doc.Workflow.Steps, "steps")...)
		for i, phase := range doc.Workflow.Phases {
			items = append(items, stepFieldCandidates(path, phase.Steps, fmt.Sprintf("phases[%d].steps", i))...)
		}
	}
	if doc.Vars != nil {
		items = append(items, mapFieldCandidates(path, "", doc.Vars)...)
	}
	return items
}

func stepFieldCandidates(path string, steps []askcontract.WorkflowStep, prefix string) []Candidate {
	items := []Candidate{}
	catalog := askcatalog.Current()
	for i, step := range steps {
		base := fmt.Sprintf("%s[%d]", prefix, i)
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "when", value: strings.TrimSpace(step.When)},
			{name: "timeout", value: strings.TrimSpace(step.Timeout)},
		} {
			if field.value == "" {
				continue
			}
			rawPath := base + "." + field.name
			items = append(items, scalarCandidates(path, rawPath, field.name, field.value)...)
		}
		items = append(items, mapFieldCandidatesWithCatalog(catalog, path, base+".spec", "spec", strings.TrimSpace(step.Kind), step.Spec)...)
	}
	return items
}

func mapFieldCandidates(path string, prefix string, values map[string]any) []Candidate {
	return mapFieldCandidatesWithCatalog(askcatalog.Catalog{}, path, prefix, prefix, "", values)
}

func mapFieldCandidatesWithCatalog(catalog askcatalog.Catalog, path string, rawPrefix string, schemaPrefix string, kind string, values map[string]any) []Candidate {
	items := []Candidate{}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		rawPath := key
		if strings.TrimSpace(rawPrefix) != "" {
			rawPath = rawPrefix + "." + key
		}
		schemaPath := key
		if strings.TrimSpace(schemaPrefix) != "" {
			schemaPath = schemaPrefix + "." + key
		}
		switch typed := values[key].(type) {
		case map[string]any:
			items = append(items, mapFieldCandidatesWithCatalog(catalog, path, rawPath, schemaPath, kind, typed)...)
		case string:
			items = append(items, scalarCandidatesForField(catalog, kind, path, rawPath, schemaPath, key, typed)...)
		case int, int64, float64, bool:
			items = append(items, scalarCandidatesForField(catalog, kind, path, rawPath, schemaPath, key, fmt.Sprint(typed))...)
		}
	}
	return items
}

func scalarCandidatesForField(catalog askcatalog.Catalog, kind string, path string, rawPath string, schemaPath string, leaf string, rendered string) []Candidate {
	field, ok := catalog.LookupField(kind, schemaPath)
	if !ok {
		return scalarCandidates(path, rawPath, leaf, rendered)
	}
	items := []Candidate{{ID: candidateID("set-field", path, rawPath), Path: path, Type: "set-field", RawPath: rawPath, Summary: fmt.Sprintf("update %s (current %q)", rawPath, rendered)}}
	if field.Requirement != "required" && field.Requirement != "conditional" {
		items = append(items, Candidate{ID: candidateID("delete-field", path, rawPath), Path: path, Type: "delete-field", RawPath: rawPath, Summary: fmt.Sprintf("delete %s", rawPath)})
	}
	if !field.ConstrainedLiteral && len(field.Enum) == 0 {
		items = append(items, Candidate{ID: candidateID("extract-var", path, rawPath), Path: path, Type: "extract-var", RawPath: rawPath, Summary: fmt.Sprintf("extract %s into workflows/vars.yaml", rawPath), SuggestedVarName: sanitizeName(leaf), SuggestedVarsPath: "workflows/vars.yaml"})
	}
	return items
}

func scalarCandidates(path string, rawPath string, leaf string, rendered string) []Candidate {
	leaf = sanitizeName(leaf)
	return []Candidate{
		{ID: candidateID("set-field", path, rawPath), Path: path, Type: "set-field", RawPath: rawPath, Summary: fmt.Sprintf("update %s (current %q)", rawPath, rendered)},
		{ID: candidateID("delete-field", path, rawPath), Path: path, Type: "delete-field", RawPath: rawPath, Summary: fmt.Sprintf("delete %s", rawPath)},
		{ID: candidateID("extract-var", path, rawPath), Path: path, Type: "extract-var", RawPath: rawPath, Summary: fmt.Sprintf("extract %s into workflows/vars.yaml", rawPath), SuggestedVarName: leaf, SuggestedVarsPath: "workflows/vars.yaml"},
	}
}

func candidateID(kind string, path string, rawPath string) string {
	return kind + "|" + filepath.ToSlash(strings.TrimSpace(path)) + "|" + strings.TrimSpace(rawPath)
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, ".", "-")
	value = strings.ReplaceAll(value, "[", "-")
	value = strings.ReplaceAll(value, "]", "")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "value"
	}
	return value
}

func dedupeCandidates(items []Candidate) []Candidate {
	seen := map[string]bool{}
	out := make([]Candidate, 0, len(items))
	for _, item := range items {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func allowedPaths(plan askcontract.PlanResponse) map[string]bool {
	allowed := map[string]bool{}
	for _, path := range plan.AuthoringBrief.AnchorPaths {
		if clean := filepath.ToSlash(strings.TrimSpace(path)); clean != "" {
			allowed[clean] = true
		}
	}
	for _, path := range plan.AuthoringBrief.AllowedCompanionPaths {
		if clean := filepath.ToSlash(strings.TrimSpace(path)); clean != "" {
			allowed[clean] = true
		}
	}
	for _, path := range plan.AuthoringBrief.TargetPaths {
		if clean := filepath.ToSlash(strings.TrimSpace(path)); clean != "" {
			allowed[clean] = true
		}
	}
	for _, file := range plan.Files {
		if clean := filepath.ToSlash(strings.TrimSpace(file.Path)); clean != "" {
			allowed[clean] = true
		}
	}
	return allowed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
