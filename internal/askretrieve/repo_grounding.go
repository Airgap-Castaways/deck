package askretrieve

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	if chunk := repoGroundingChunk(root, "repo-grounding-stepspec", "typed-step-builders", filepath.Join("internal", "stepspec"), buildStepspecSummary(root, lowerPrompt)); chunk != nil {
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
	path := filepath.Join(root, "internal", "stepmeta", "registry.go")
	raw, err := os.ReadFile(path) //nolint:gosec // repository-owned source file
	if err != nil {
		return ""
	}
	text := string(raw)
	b := &strings.Builder{}
	b.WriteString("- file: internal/stepmeta/registry.go\n")
	b.WriteString("- role: central registry for typed step metadata, ask builder definitions, and schema-backed source-of-truth\n")
	_, _ = fmt.Fprintf(b, "- registered builder metadata blocks observed: %d\n", strings.Count(text, "Builders"))
	_, _ = fmt.Fprintf(b, "- validation hints observed: %d\n", strings.Count(text, "ValidationHints"))
	return strings.TrimSpace(b.String())
}

func buildStepspecSummary(root string, lowerPrompt string) string {
	patternDir := filepath.Join(root, "internal", "stepspec")
	entries, err := os.ReadDir(patternDir)
	if err != nil {
		return ""
	}
	type item struct {
		kind    string
		builder string
		score   int
	}
	items := make([]item, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_meta.go") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(patternDir, entry.Name())) //nolint:gosec // repository-owned source file
		if readErr != nil {
			continue
		}
		text := string(raw)
		kind := extractQuotedValue(text, "Kind:")
		builder := extractQuotedValue(text, "ID:")
		score := 0
		for _, token := range strings.Fields(lowerPrompt) {
			if token != "" && strings.Contains(strings.ToLower(text), strings.ToLower(token)) {
				score++
			}
		}
		items = append(items, item{kind: kind, builder: builder, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			if items[i].kind == items[j].kind {
				return items[i].builder < items[j].builder
			}
			return items[i].kind < items[j].kind
		}
		return items[i].score > items[j].score
	})
	if len(items) > 5 {
		items = items[:5]
	}
	b := &strings.Builder{}
	b.WriteString("- directory: internal/stepspec/*_meta.go\n")
	b.WriteString("- role: typed step metadata and builder source-of-truth used by ask draft/refine compilation\n")
	for _, item := range items {
		if item.kind == "" && item.builder == "" {
			continue
		}
		b.WriteString("- candidate step kind: ")
		if item.kind != "" {
			b.WriteString(item.kind)
		} else {
			b.WriteString("unknown")
		}
		if item.builder != "" {
			b.WriteString(" builder=")
			b.WriteString(item.builder)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
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

func extractQuotedValue(text string, marker string) string {
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(marker):]
	start := strings.Index(rest, `"`)
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}
