package askretrieve

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askintent"
)

func repoGroundingChunks(route askintent.Route, lowerPrompt string) []Chunk {
	if route != askintent.RouteDraft && route != askintent.RouteRefine && route != askintent.RouteExplain && route != askintent.RouteReview {
		return nil
	}
	root := repoRootFallback()
	if root == "" {
		return nil
	}
	chunks := []Chunk{}
	if chunk := repoGroundingChunk(root, "repo-grounding-stepmeta", "source-of-truth-stepmeta", filepath.Join("internal", "stepmeta", "registry.go"), buildStepmetaSummary(root)); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := repoGroundingChunk(root, "repo-grounding-stepspec", "stepspec-facts", filepath.Join("internal", "stepspec"), buildStepspecSummary(root, lowerPrompt)); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := repoGroundingChunk(root, "repo-grounding-askdraft", "askdraft-compiler", filepath.Join("internal", "askdraft"), buildDirectorySummary(root, filepath.Join("internal", "askdraft"), []string{
		"Local source-of-truth for draft builder selection compilation.",
		"Draft generation selects builders first; code compiles workflow documents afterwards.",
	})); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := repoGroundingChunk(root, "repo-grounding-askpolicy", "askpolicy-requirements", filepath.Join("internal", "askpolicy"), buildDirectorySummary(root, filepath.Join("internal", "askpolicy"), []string{
		"Local source-of-truth for authoring requirements, defaults, and plan shaping.",
		"Use policy-derived requirements to infer prepare/apply structure, topology, and validation expectations.",
	})); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	if chunk := repoGroundingChunk(root, "repo-grounding-askrepair", "askrepair-validation", filepath.Join("internal", "askrepair"), buildDirectorySummary(root, filepath.Join("internal", "askrepair"), []string{
		"Local source-of-truth for code-owned repair and auto-fix behavior.",
		"Repair follows validator and transform constraints instead of external docs.",
	})); chunk != nil {
		chunks = append(chunks, *chunk)
	}
	return chunks
}

func repoGroundingChunk(root string, id string, label string, path string, body string) *Chunk {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	clean := filepath.ToSlash(strings.TrimSpace(path))
	content := "Local repo grounding:\n- authoritative for deck workflow validity and ask behavior\n- path: " + clean + "\n" + strings.TrimSpace(body)
	return &Chunk{
		ID:      id,
		Source:  "repo-grounding",
		Label:   label,
		Topic:   askcontext.Topic(string(askcontext.TopicRepoGrounding) + ":" + id),
		Content: strings.TrimSpace(content),
		Score:   58,
	}
}

func buildStepmetaSummary(root string) string {
	metadata, err := parseStepspecMetadata(root)
	if err != nil {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("- file: internal/stepmeta/registry.go\n")
	b.WriteString("- role: central registry for typed step metadata, ask builder definitions, and schema-backed source-of-truth\n")
	_, _ = fmt.Fprintf(b, "- registered builder metadata blocks observed: %d\n", metadata.totalBuilders())
	_, _ = fmt.Fprintf(b, "- validation hints observed: %d\n", metadata.totalValidationHints())
	return strings.TrimSpace(b.String())
}

func buildStepspecSummary(root string, lowerPrompt string) string {
	metadata, err := parseStepspecMetadata(root)
	if err != nil {
		return ""
	}
	facts := collectStepspecFacts(metadata)
	if len(facts) == 0 {
		return ""
	}
	matched := requestedStepspecFacts(facts, lowerPrompt)
	kindCount, builderCount := stepspecFactCounts(facts)
	b := &strings.Builder{}
	b.WriteString("- directory: internal/stepspec/*_meta.go\n")
	b.WriteString("- role: typed step metadata and builder source-of-truth used by ask draft/refine compilation\n")
	_, _ = fmt.Fprintf(b, "- observed typed step kinds: %d\n", kindCount)
	_, _ = fmt.Fprintf(b, "- observed ask builders: %d\n", builderCount)
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

func collectStepspecFacts(metadata stepspecMetadata) []stepspecFact {
	type item struct {
		kind    string
		builder string
	}
	items := make([]item, 0)
	for _, entry := range metadata.entries {
		if len(entry.builders) == 0 {
			items = append(items, item{kind: entry.kind})
			continue
		}
		for _, builder := range entry.builders {
			items = append(items, item{kind: entry.kind, builder: builder})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].kind == items[j].kind {
			return items[i].builder < items[j].builder
		}
		return items[i].kind < items[j].kind
	})
	facts := make([]stepspecFact, 0, len(items))
	for _, item := range items {
		facts = append(facts, newStepspecFact(item.kind, item.builder))
	}
	return facts
}

type stepspecFact struct {
	kind        string
	builder     string
	normKind    string
	normBuilder string
}

func newStepspecFact(kind string, builder string) stepspecFact {
	kind = strings.TrimSpace(kind)
	builder = strings.TrimSpace(builder)
	return stepspecFact{
		kind:        kind,
		builder:     builder,
		normKind:    normalizeFactTerm(kind),
		normBuilder: normalizeFactTerm(builder),
	}
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

func stepspecFactCounts(facts []stepspecFact) (int, int) {
	kinds := map[string]bool{}
	builders := map[string]bool{}
	for _, fact := range facts {
		if fact.kind != "" {
			kinds[fact.kind] = true
		}
		if fact.builder != "" {
			builders[fact.builder] = true
		}
	}
	return len(kinds), len(builders)
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

type stepspecMetadata struct {
	entries []stepspecEntry
}

func (m stepspecMetadata) totalBuilders() int {
	total := 0
	for _, entry := range m.entries {
		total += len(entry.builders)
	}
	return total
}

func (m stepspecMetadata) totalValidationHints() int {
	total := 0
	for _, entry := range m.entries {
		total += entry.validationHints
	}
	return total
}

type stepspecEntry struct {
	kind            string
	builders        []string
	validationHints int
	lowerText       string
}

func parseStepspecMetadata(root string) (stepspecMetadata, error) {
	patternDir := filepath.Join(root, "internal", "stepspec")
	entries, err := os.ReadDir(patternDir)
	if err != nil {
		return stepspecMetadata{}, err
	}
	metadata := stepspecMetadata{entries: make([]stepspecEntry, 0, len(entries))}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_meta.go") {
			continue
		}
		fullPath := filepath.Join(patternDir, entry.Name())
		raw, readErr := os.ReadFile(fullPath) //nolint:gosec // repository-owned source file
		if readErr != nil {
			continue
		}
		file, parseErr := parser.ParseFile(fset, fullPath, raw, 0)
		if parseErr != nil {
			continue
		}
		lowerText := strings.ToLower(string(raw))
		for _, parsed := range parseMetadataFile(file, lowerText) {
			if parsed.kind == "" {
				continue
			}
			metadata.entries = append(metadata.entries, parsed)
		}
	}
	return metadata, nil
}

func parseMetadataFile(file *ast.File, lowerText string) []stepspecEntry {
	functionDefs := make(map[string]*ast.CompositeLit)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			continue
		}
		if lit := findReturnedDefinitionLiteral(fn.Body); lit != nil {
			functionDefs[fn.Name.Name] = lit
		}
	}
	entries := make([]stepspecEntry, 0)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range valueSpec.Values {
				lit := resolveDefinitionLiteral(value, functionDefs)
				if lit == nil {
					continue
				}
				entry := parseDefinitionLiteral(lit)
				entry.lowerText = lowerText
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func findReturnedDefinitionLiteral(body *ast.BlockStmt) *ast.CompositeLit {
	for _, stmt := range body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		lit, ok := ret.Results[0].(*ast.CompositeLit)
		if ok {
			return lit
		}
		return nil
	}
	return nil
}

func resolveDefinitionLiteral(expr ast.Expr, functionDefs map[string]*ast.CompositeLit) *ast.CompositeLit {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !isMustRegisterCall(call.Fun) {
		return nil
	}
	if lit, ok := call.Args[0].(*ast.CompositeLit); ok {
		return lit
	}
	ref, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return nil
	}
	ident, ok := ref.Fun.(*ast.Ident)
	if !ok || len(ref.Args) != 0 {
		return nil
	}
	return functionDefs[ident.Name]
}

func isMustRegisterCall(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.IndexExpr:
		selector, ok := typed.X.(*ast.SelectorExpr)
		return ok && selector.Sel != nil && selector.Sel.Name == "MustRegister"
	case *ast.IndexListExpr:
		selector, ok := typed.X.(*ast.SelectorExpr)
		return ok && selector.Sel != nil && selector.Sel.Name == "MustRegister"
	case *ast.SelectorExpr:
		return typed.Sel != nil && typed.Sel.Name == "MustRegister"
	default:
		return false
	}
}

func parseDefinitionLiteral(lit *ast.CompositeLit) stepspecEntry {
	entry := stepspecEntry{}
	for _, element := range lit.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key := exprIdentName(kv.Key)
		switch key {
		case "Kind":
			entry.kind = stringLiteralValue(kv.Value)
		case "Ask":
			builders, hints := parseAskMetadata(kv.Value)
			entry.builders = builders
			entry.validationHints = hints
		}
	}
	return entry
}

func parseAskMetadata(expr ast.Expr) ([]string, int) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, 0
	}
	builders := make([]string, 0)
	hints := 0
	for _, element := range lit.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch exprIdentName(kv.Key) {
		case "Builders":
			builders = append(builders, parseBuilderIDs(kv.Value)...)
		case "ValidationHints":
			hints = compositeLiteralLen(kv.Value)
		}
	}
	return builders, hints
}

func parseBuilderIDs(expr ast.Expr) []string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(lit.Elts))
	for _, element := range lit.Elts {
		builderLit, ok := element.(*ast.CompositeLit)
		if !ok {
			continue
		}
		for _, field := range builderLit.Elts {
			kv, ok := field.(*ast.KeyValueExpr)
			if !ok || exprIdentName(kv.Key) != "ID" {
				continue
			}
			if id := stringLiteralValue(kv.Value); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func compositeLiteralLen(expr ast.Expr) int {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return 0
	}
	return len(lit.Elts)
}

func exprIdentName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.SelectorExpr:
		if typed.Sel != nil {
			return typed.Sel.Name
		}
	}
	return ""
}

func stringLiteralValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func buildDirectorySummary(root string, rel string, notes []string) string {
	entries, err := os.ReadDir(filepath.Join(root, rel))
	if err != nil {
		return ""
	}
	b := &strings.Builder{}
	b.WriteString("- path: ")
	b.WriteString(filepath.ToSlash(rel))
	b.WriteString("\n")
	_, _ = fmt.Fprintf(b, "- source files observed: %d\n", len(entries))
	for _, note := range notes {
		if strings.TrimSpace(note) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(note))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
