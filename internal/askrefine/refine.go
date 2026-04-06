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

type UnknownCandidateError struct {
	Candidate string
	Path      string
	Type      string
	RawPath   string
	Ignorable bool
}

func (e UnknownCandidateError) Error() string {
	return fmt.Sprintf("unknown refine transform candidate %q for %s", strings.TrimSpace(e.Candidate), strings.TrimSpace(e.Path))
}

type candidateContext struct {
	recommendedVarNames map[string]bool
	repeatedValues      map[string]bool
	allowAnyExtractVar  bool
}

func CandidatesForDocuments(plan askcontract.PlanResponse, docs []askcontract.GeneratedDocument) []Candidate {
	allowed := allowedPaths(plan)
	ctx := newCandidateContext(plan, docs)
	out := []Candidate{}
	for _, doc := range docs {
		path := filepath.ToSlash(strings.TrimSpace(doc.Path))
		if path == "" || (len(allowed) > 0 && !allowed[path]) {
			continue
		}
		out = append(out, candidatesForDocument(doc, ctx)...)
	}
	return dedupeCandidates(out)
}

func ResolveCandidate(doc askcontract.GeneratedDocument, transform askcontract.RefineTransformAction) (askcontract.RefineTransformAction, error) {
	ctx := newCandidateContext(askcontract.PlanResponse{}, []askcontract.GeneratedDocument{doc})
	if strings.TrimSpace(transform.Candidate) == "" {
		if strings.TrimSpace(transform.Type) != "extract-var" {
			return transform, nil
		}
		if transform.Value != nil && strings.TrimSpace(transform.RawPath) != "" && strings.TrimSpace(transform.VarName) != "" {
			if strings.TrimSpace(transform.VarsPath) == "" {
				transform.VarsPath = "workflows/vars.yaml"
			}
			return transform, nil
		}
		for _, candidate := range candidatesForDocument(doc, ctx) {
			if candidate.Type == "extract-var" && candidate.RawPath == strings.TrimSpace(transform.RawPath) {
				if strings.TrimSpace(transform.VarName) == "" {
					transform.VarName = candidate.SuggestedVarName
				}
				if strings.TrimSpace(transform.VarsPath) == "" {
					transform.VarsPath = candidate.SuggestedVarsPath
				}
				return transform, nil
			}
		}
		return transform, UnknownCandidateError{Path: strings.TrimSpace(doc.Path), Type: "extract-var", RawPath: strings.TrimSpace(transform.RawPath), Ignorable: true}
	}
	if resolved, ok := resolveStepScopedCandidate(doc, transform); ok {
		transform = resolved
	}
	for _, candidate := range candidatesForDocument(doc, ctx) {
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
	if parsedType, parsedPath, _, ok := parseCandidateID(strings.TrimSpace(transform.Candidate)); ok && parsedType == "extract-var" && parsedPath == filepath.ToSlash(strings.TrimSpace(doc.Path)) {
		for _, candidate := range candidatesForDocument(doc, candidateContext{allowAnyExtractVar: true}) {
			if candidate.ID != strings.TrimSpace(transform.Candidate) {
				continue
			}
			transform.Type = candidate.Type
			if strings.TrimSpace(transform.RawPath) == "" {
				transform.RawPath = candidate.RawPath
			}
			if strings.TrimSpace(transform.VarName) == "" {
				transform.VarName = candidate.SuggestedVarName
			}
			if strings.TrimSpace(transform.VarsPath) == "" {
				transform.VarsPath = candidate.SuggestedVarsPath
			}
			return transform, nil
		}
	}
	parsedType, parsedPath, parsedRawPath, ok := parseCandidateID(strings.TrimSpace(transform.Candidate))
	if ok && parsedPath == filepath.ToSlash(strings.TrimSpace(doc.Path)) {
		return transform, UnknownCandidateError{Candidate: transform.Candidate, Path: strings.TrimSpace(doc.Path), Type: parsedType, RawPath: parsedRawPath, Ignorable: parsedType == "extract-var"}
	}
	return transform, UnknownCandidateError{Candidate: transform.Candidate, Path: strings.TrimSpace(doc.Path)}
}

func resolveStepScopedCandidate(doc askcontract.GeneratedDocument, transform askcontract.RefineTransformAction) (askcontract.RefineTransformAction, bool) {
	stepID := strings.TrimSpace(transform.Candidate)
	if stepID == "" || strings.Contains(stepID, "|") {
		return transform, false
	}
	relativePath := strings.TrimSpace(transform.Path)
	if relativePath == "" || strings.HasPrefix(relativePath, "workflows/") {
		return transform, false
	}
	if strings.TrimSpace(transform.Type) == "extract-component" {
		return transform, false
	}
	base := stepRawPathByID(doc, stepID)
	if base == "" {
		return transform, false
	}
	if strings.TrimSpace(transform.Type) == "set-field" || strings.TrimSpace(transform.Type) == "delete-field" || strings.TrimSpace(transform.Type) == "" {
		rawPath := base + "." + strings.TrimPrefix(strings.TrimSpace(relativePath), ".")
		transform.Candidate = candidateID(firstNonEmpty(strings.TrimSpace(transform.Type), "set-field"), doc.Path, rawPath)
		if strings.TrimSpace(transform.RawPath) == "" {
			transform.RawPath = rawPath
		}
		return transform, true
	}
	return transform, false
}

func stepRawPathByID(doc askcontract.GeneratedDocument, stepID string) string {
	stepID = strings.TrimSpace(stepID)
	if stepID == "" || doc.Workflow == nil {
		return ""
	}
	for i, step := range doc.Workflow.Steps {
		if strings.TrimSpace(step.ID) == stepID {
			return fmt.Sprintf("steps[%d]", i)
		}
	}
	for i, phase := range doc.Workflow.Phases {
		for j, step := range phase.Steps {
			if strings.TrimSpace(step.ID) == stepID {
				return fmt.Sprintf("phases[%d].steps[%d]", i, j)
			}
		}
	}
	return ""
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

func candidatesForDocument(doc askcontract.GeneratedDocument, ctx candidateContext) []Candidate {
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
		items = append(items, stepFieldCandidates(path, doc.Workflow.Steps, "steps", ctx)...)
		for i, phase := range doc.Workflow.Phases {
			items = append(items, stepFieldCandidates(path, phase.Steps, fmt.Sprintf("phases[%d].steps", i), ctx)...)
		}
	}
	if doc.Vars != nil {
		items = append(items, mapFieldCandidates(path, "", doc.Vars, ctx)...)
	}
	return items
}

func stepFieldCandidates(path string, steps []askcontract.WorkflowStep, prefix string, ctx candidateContext) []Candidate {
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
			items = append(items, scalarCandidates(path, rawPath, field.value)...)
		}
		items = append(items, mapFieldCandidatesWithCatalog(catalog, path, base+".spec", "spec", strings.TrimSpace(step.Kind), step.Spec, ctx)...)
	}
	return items
}

func mapFieldCandidates(path string, prefix string, values map[string]any, ctx candidateContext) []Candidate {
	return mapFieldCandidatesWithCatalog(askcatalog.Catalog{}, path, prefix, prefix, "", values, ctx)
}

func mapFieldCandidatesWithCatalog(catalog askcatalog.Catalog, path string, rawPrefix string, schemaPrefix string, kind string, values map[string]any, ctx candidateContext) []Candidate {
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
			items = append(items, mapFieldCandidatesWithCatalog(catalog, path, rawPath, schemaPath, kind, typed, ctx)...)
		case string:
			items = append(items, scalarCandidatesForField(catalog, kind, path, rawPath, schemaPath, key, typed, ctx)...)
		case int, int64, float64, bool:
			items = append(items, scalarCandidatesForField(catalog, kind, path, rawPath, schemaPath, key, typed, ctx)...)
		}
	}
	return items
}

func scalarCandidatesForField(catalog askcatalog.Catalog, kind string, path string, rawPath string, schemaPath string, leaf string, value any, ctx candidateContext) []Candidate {
	rendered := fmt.Sprint(value)
	field, ok := catalog.LookupField(kind, schemaPath)
	if !ok {
		return scalarCandidates(path, rawPath, rendered)
	}
	items := []Candidate{{ID: candidateID("set-field", path, rawPath), Path: path, Type: "set-field", RawPath: rawPath, Summary: fmt.Sprintf("update %s (current %q)", rawPath, rendered)}}
	if field.Requirement != "required" && field.Requirement != "conditional" {
		items = append(items, Candidate{ID: candidateID("delete-field", path, rawPath), Path: path, Type: "delete-field", RawPath: rawPath, Summary: fmt.Sprintf("delete %s", rawPath)})
	}
	if fieldSupportsExtractVar(field, value) && ctx.allowExtractVar(sanitizeName(leaf), value) {
		items = append(items, Candidate{ID: candidateID("extract-var", path, rawPath), Path: path, Type: "extract-var", RawPath: rawPath, Summary: fmt.Sprintf("extract %s into workflows/vars.yaml", rawPath), SuggestedVarName: sanitizeName(leaf), SuggestedVarsPath: "workflows/vars.yaml"})
	}
	return items
}

func fieldSupportsExtractVar(field askcatalog.Field, value any) bool {
	if field.ConstrainedLiteral || strings.TrimSpace(field.Pattern) != "" || len(field.Enum) > 0 || !strings.EqualFold(strings.TrimSpace(field.Type), "string") {
		return false
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	text = strings.TrimSpace(text)
	return text != "" && !strings.Contains(text, "{{") && !strings.Contains(text, "${{")
}

func scalarCandidates(path string, rawPath string, rendered string) []Candidate {
	return []Candidate{
		{ID: candidateID("set-field", path, rawPath), Path: path, Type: "set-field", RawPath: rawPath, Summary: fmt.Sprintf("update %s (current %q)", rawPath, rendered)},
		{ID: candidateID("delete-field", path, rawPath), Path: path, Type: "delete-field", RawPath: rawPath, Summary: fmt.Sprintf("delete %s", rawPath)},
	}
}

func newCandidateContext(plan askcontract.PlanResponse, docs []askcontract.GeneratedDocument) candidateContext {
	ctx := candidateContext{recommendedVarNames: recommendedVarNames(plan), repeatedValues: repeatedScalarValues(docs)}
	return ctx
}

func (ctx candidateContext) allowExtractVar(name string, value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if ctx.allowAnyExtractVar {
		return true
	}
	if len(ctx.repeatedValues) > 0 {
		return ctx.repeatedValues[text]
	}
	if len(ctx.recommendedVarNames) == 0 {
		return false
	}
	if ctx.recommendedVarNames[sanitizeName(name)] {
		return true
	}
	return false
}

func recommendedVarNames(plan askcontract.PlanResponse) map[string]bool {
	items := map[string]bool{}
	for _, entry := range plan.VarsRecommendation {
		for _, token := range strings.FieldsFunc(strings.ToLower(strings.TrimSpace(entry)), func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		}) {
			clean := sanitizeName(token)
			if clean != "" && clean != "vars" && clean != "yaml" && clean != "workflows" && clean != "values" && clean != "value" {
				items[clean] = true
			}
		}
	}
	return items
}

func repeatedScalarValues(docs []askcontract.GeneratedDocument) map[string]bool {
	counts := map[string]int{}
	for _, doc := range docs {
		collectRepeatedScalarValues(counts, doc)
	}
	out := map[string]bool{}
	for value, count := range counts {
		if count > 1 {
			out[value] = true
		}
	}
	return out
}

func collectRepeatedScalarValues(counts map[string]int, doc askcontract.GeneratedDocument) {
	if doc.Workflow != nil {
		collectScalarValuesFromMap(counts, doc.Workflow.Vars)
		for _, step := range doc.Workflow.Steps {
			collectScalarValuesFromStep(counts, step)
		}
		for _, phase := range doc.Workflow.Phases {
			for _, step := range phase.Steps {
				collectScalarValuesFromStep(counts, step)
			}
		}
	}
	if doc.Vars != nil {
		collectScalarValuesFromMap(counts, doc.Vars)
	}
}

func collectScalarValuesFromStep(counts map[string]int, step askcontract.WorkflowStep) {
	for _, value := range []string{strings.TrimSpace(step.When), strings.TrimSpace(step.Timeout)} {
		if value != "" {
			counts[value]++
		}
	}
	collectScalarValuesFromMap(counts, step.Spec)
}

func collectScalarValuesFromMap(counts map[string]int, values map[string]any) {
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			text := strings.TrimSpace(typed)
			if text != "" {
				counts[text]++
			}
		case map[string]any:
			collectScalarValuesFromMap(counts, typed)
		}
	}
}

func parseCandidateID(value string) (string, string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), "|", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return strings.TrimSpace(parts[0]), filepath.ToSlash(strings.TrimSpace(parts[1])), strings.TrimSpace(parts[2]), true
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
