package askretrieve

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Airgap-Castaways/deck/internal/askcatalog"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askintent"
)

func localFactChunks(route askintent.Route, lowerPrompt string) []Chunk {
	if route != askintent.RouteDraft && route != askintent.RouteRefine && route != askintent.RouteExplain && route != askintent.RouteReview {
		return nil
	}
	root := repoRootFallback()
	if root == "" {
		return nil
	}
	chunks := []Chunk{}
	if chunk := localFactChunk(root, "local-facts-stepmeta", "source-of-truth-stepmeta", filepath.Join("internal", "stepmeta", "registry.go"), buildStepmetaSummary(root)); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := localFactChunk(root, "local-facts-stepspec", "stepspec-facts", filepath.Join("internal", "stepspec"), buildStepspecSummary(root, lowerPrompt)); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := localFactChunk(root, "local-facts-askdraft", "askdraft-compiler", filepath.Join("internal", "askdraft", "draft.go"), buildAskdraftSummary()); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := localFactChunk(root, "local-facts-askpolicy", "askpolicy-requirements", filepath.Join("internal", "askpolicy"), buildPackageProvenanceSummary("internal/askpolicy", []string{
		"Local source-of-truth for authoring requirements, defaults, and plan shaping.",
		"Use policy-derived requirements to infer prepare/apply structure, topology, and validation expectations.",
	})); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := localFactChunk(root, "local-facts-askrepair", "askrepair-validation", filepath.Join("internal", "askrepair"), buildPackageProvenanceSummary("internal/askrepair", []string{
		"Local source-of-truth for code-owned repair and auto-fix behavior.",
		"Repair follows validator and transform constraints instead of external docs.",
	})); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	return chunks
}

func buildAskdraftSummary() string {
	lines := []string{
		"- file: internal/askdraft/draft.go",
		"- role: draft builder selection compiler and workflow assembly path",
		"- function: CompileWithProgram - entrypoint that turns builder selection plus authoring program into generated documents",
		"- function: buildWorkflowTarget - assembles each workflow target from selected builders",
		"- function: buildStep - materializes a typed workflow step from builder metadata and bindings",
		"- function: resolveBindings - maps authoring program fields into builder binding values before step assembly",
	}
	return strings.Join(lines, "\n")
}

func localFactChunk(root string, id string, label string, path string, body string) *Chunk {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	clean := filepath.ToSlash(strings.TrimSpace(path))
	content := "Local facts:\n- authoritative for deck workflow validity and ask behavior\n- path: " + clean + "\n" + strings.TrimSpace(body)
	return &Chunk{ID: id, Source: "local-facts", Label: label, Topic: askcontext.Topic(string(askcontext.TopicLocalFacts) + ":" + id), Content: strings.TrimSpace(content), Score: 58}
}

func buildStepmetaSummary(root string) string {
	_ = root
	catalog := askcatalog.Current()
	builderCount := 0
	hintCount := 0
	for _, step := range catalog.StepKinds() {
		builderCount += len(step.Builders)
		hintCount += len(step.ValidationHints)
	}
	lines := []string{
		"- file: internal/stepmeta/registry.go",
		"- role: central registry for typed step metadata, ask builder definitions, and schema-backed source-of-truth",
		fmt.Sprintf("- registered builder metadata blocks observed: %d", builderCount),
		fmt.Sprintf("- validation hints observed: %d", hintCount),
	}
	return strings.Join(lines, "\n")
}

func buildStepspecSummary(root string, lowerPrompt string) string {
	_ = root
	catalog := askcatalog.Current()
	facts := collectStepspecFacts(catalog)
	if len(facts) == 0 {
		return ""
	}
	matched := requestedStepspecFacts(facts, lowerPrompt)
	kindCount := len(catalog.StepKinds())
	builderCount := 0
	sourceFiles := map[string]bool{}
	for _, step := range catalog.StepKinds() {
		builderCount += len(step.Builders)
		for _, ref := range step.SourceRefs {
			file := sourceFileFromRef(ref)
			if strings.Contains(file, "internal/stepspec/") {
				sourceFiles[file] = true
			}
		}
	}
	b := &strings.Builder{}
	b.WriteString("- directory: internal/stepspec/*_meta.go\n")
	b.WriteString("- role: typed step metadata and builder source-of-truth used by ask draft/refine compilation\n")
	_, _ = fmt.Fprintf(b, "- observed typed step kinds: %d\n", kindCount)
	_, _ = fmt.Fprintf(b, "- observed ask builders: %d\n", builderCount)
	if len(sourceFiles) > 0 {
		_, _ = fmt.Fprintf(b, "- source files observed: %d\n", len(sourceFiles))
	}
	if len(matched) == 0 {
		for _, fact := range sampleStepspecFacts(facts, 5) {
			appendStepspecFactLine(b, "observed step fact", fact)
		}
		return strings.TrimSpace(b.String())
	}
	for _, fact := range matched {
		appendStepspecFactLine(b, "step fact", fact)
	}
	return strings.TrimSpace(b.String())
}

type stepspecFact struct {
	kind        string
	builder     string
	normKind    string
	normBuilder string
}

func collectStepspecFacts(catalog askcatalog.Catalog) []stepspecFact {
	items := make([]stepspecFact, 0)
	for _, step := range catalog.StepKinds() {
		if len(step.Builders) == 0 {
			items = append(items, newStepspecFact(step.Kind, ""))
			continue
		}
		for _, builder := range step.Builders {
			items = append(items, newStepspecFact(step.Kind, builder.ID))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].kind == items[j].kind {
			return items[i].builder < items[j].builder
		}
		return items[i].kind < items[j].kind
	})
	return items
}

func newStepspecFact(kind string, builder string) stepspecFact {
	kind = strings.TrimSpace(kind)
	builder = strings.TrimSpace(builder)
	return stepspecFact{kind: kind, builder: builder, normKind: normalizeFactTerm(kind), normBuilder: normalizeFactTerm(builder)}
}

func requestedStepspecFacts(facts []stepspecFact, lowerPrompt string) []stepspecFact {
	promptTerms := promptFactTerms(lowerPrompt)
	if len(promptTerms) == 0 {
		return nil
	}
	matches := make([]stepspecFact, 0)
	for _, fact := range facts {
		if fact.normKind != "" && promptTerms[fact.normKind] {
			matches = append(matches, fact)
			continue
		}
		if fact.normBuilder != "" && promptTerms[fact.normBuilder] {
			matches = append(matches, fact)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].kind == matches[j].kind {
			return matches[i].builder < matches[j].builder
		}
		return matches[i].kind < matches[j].kind
	})
	return matches
}

func sampleStepspecFacts(facts []stepspecFact, limit int) []stepspecFact {
	if len(facts) == 0 || limit <= 0 {
		return nil
	}
	sorted := append([]stepspecFact(nil), facts...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].kind == sorted[j].kind {
			return sorted[i].builder < sorted[j].builder
		}
		return sorted[i].kind < sorted[j].kind
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func appendStepspecFactLine(b *strings.Builder, prefix string, fact stepspecFact) {
	b.WriteString("- ")
	b.WriteString(prefix)
	b.WriteString(": ")
	if fact.kind != "" {
		b.WriteString(fact.kind)
	} else {
		b.WriteString("unknown")
	}
	if fact.builder != "" {
		b.WriteString(" builders=")
		b.WriteString(fact.builder)
	}
	b.WriteString("\n")
}

func promptFactTerms(prompt string) map[string]bool {
	terms := map[string]bool{}
	for _, raw := range strings.Fields(prompt) {
		normalized := normalizeFactTerm(raw)
		if normalized == "" {
			continue
		}
		terms[normalized] = true
	}
	return terms
}

func normalizeFactTerm(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	b := strings.Builder{}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func buildPackageProvenanceSummary(path string, notes []string) string {
	b := &strings.Builder{}
	b.WriteString("- package: ")
	b.WriteString(strings.TrimSpace(path))
	b.WriteString("\n")
	for _, note := range notes {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(note))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func sourceFileFromRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if idx := strings.LastIndex(ref, ":"); idx > 0 {
		return ref[:idx]
	}
	return ref
}
