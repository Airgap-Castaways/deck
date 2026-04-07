package mcpaugment

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

type providerAdapter interface {
	Fetch(ctx context.Context, server resolvedServer, c *client.Client, route askintent.Route, prompt string, tools *mcp.ListToolsResult) (*askretrieve.Chunk, string)
}

type capabilityRequest struct {
	Query        string
	Capabilities []capability
}

type context7ProviderAdapter struct{}

type webSearchProviderAdapter struct{}

type normalizedEvidence struct {
	Provider       string
	ToolName       string
	SourceURL      string
	SourceID       string
	Domain         string
	DomainCategory string
	Title          string
	Excerpt        string
	Freshness      string
	Official       bool
	TrustLevel     string
	VersionSupport string
	ArtifactKinds  []string
	InstallHints   []string
	OfflineHints   []string
}

type context7Entity struct {
	LibraryID string
	Title     string
}

var context7LibraryIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)context7-compatible\s+library\s+id\s*[:=]\s*(/[\w./-]+)`),
	regexp.MustCompile(`(?i)library\s*id\s*[:=]\s*(/[\w./-]+)`),
	regexp.MustCompile(`(?i)library\s*[:=]\s*([\w./-]+)`),
	regexp.MustCompile(`(?i)(/[a-z0-9_.-]+/[a-z0-9_.-]+(?:/[a-z0-9_.-]+)?)`),
	regexp.MustCompile(`(?i)(github\.com/[\w./-]+)`),
	regexp.MustCompile(`(?i)(golang\.org/[\w./-]+)`),
}

var requestedVersionPattern = regexp.MustCompile(`(?i)\bv?\d+\.\d+(?:\.\d+)?\b`)

var communityDomains = map[string]struct{}{
	"stackoverflow.com": {},
	"serverfault.com":   {},
	"superuser.com":     {},
	"reddit.com":        {},
	"medium.com":        {},
	"dev.to":            {},
	"substack.com":      {},
	"blogspot.com":      {},
	"wordpress.com":     {},
}

var aggregatorDomains = map[string]struct{}{
	"wikipedia.org":      {},
	"geeksforgeeks.org":  {},
	"baeldung.com":       {},
	"tutorialspoint.com": {},
	"w3schools.com":      {},
}

var sourceHostDomains = map[string]struct{}{
	"github.com":            {},
	"gitlab.com":            {},
	"pkg.go.dev":            {},
	"hub.docker.com":        {},
	"registry.terraform.io": {},
	"pypi.org":              {},
	"npmjs.com":             {},
}

const (
	defaultContext7DocsTokenBudget = 1800
	draftContext7DocsTokenBudget   = 2200
	maxContext7DocsTokenBudget     = 2400
	context7LongPromptRuneLimit    = 240
	context7LongPromptBudgetBump   = 200
)

func (context7ProviderAdapter) Fetch(ctx context.Context, server resolvedServer, c *client.Client, route askintent.Route, prompt string, tools *mcp.ListToolsResult) (*askretrieve.Chunk, string) {
	request := capabilityRequestForRoute(server.Profile, route, prompt)
	if len(request.Capabilities) == 0 {
		return nil, ""
	}
	docTool, ok := findTool(tools, "get-library-docs", "query-docs")
	if !ok {
		return nil, fmt.Sprintf("mcp:%s no known tool for route %s", server.Profile.ID, route)
	}
	if hasCapability(request, capabilityEntityResolve) && strings.EqualFold(strings.TrimSpace(docTool.Name), "query-docs") {
		result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, route, prompt, ""))
		if failure == "" {
			evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, Freshness: "external-docs"})
			return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
		}
	}
	var entity context7Entity
	if hasCapability(request, capabilityEntityResolve) {
		tool, ok := findTool(tools, "resolve-library-id")
		if !ok {
			result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, route, prompt, ""))
			if failure != "" {
				return nil, failure
			}
			evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, Freshness: "external-docs"})
			return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
		}
		resolved, failure := callTool(ctx, c, server.Profile.ID, tool, buildResolveArgs(tool, prompt))
		if failure != "" {
			result, docFailure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, route, prompt, ""))
			if docFailure != "" {
				return nil, failure
			}
			evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, Freshness: "external-docs"})
			return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
		}
		entity = parseContext7Entity(resolved, prompt)
		if entity.LibraryID == "" && !strings.EqualFold(docTool.Name, "query-docs") {
			return nil, fmt.Sprintf("mcp:%s call %s returned no library id", server.Profile.ID, tool.Name)
		}
	}
	result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, route, prompt, entity.LibraryID))
	if failure != "" {
		return nil, failure
	}
	evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, SourceID: entity.LibraryID, Title: entity.Title, Freshness: "external-docs"})
	return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
}

func (webSearchProviderAdapter) Fetch(ctx context.Context, server resolvedServer, c *client.Client, route askintent.Route, prompt string, tools *mcp.ListToolsResult) (*askretrieve.Chunk, string) {
	request := capabilityRequestForRoute(server.Profile, route, prompt)
	if len(request.Capabilities) == 0 {
		return nil, ""
	}
	tool, ok := findTool(tools, "search", "web-search", "web_search")
	if !ok {
		return nil, fmt.Sprintf("mcp:%s no known tool for route %s", server.Profile.ID, route)
	}
	result, failure := callTool(ctx, c, server.Profile.ID, tool, buildSearchArgs(tool, prompt))
	if failure != "" {
		return nil, failure
	}
	evidence := normalizeEvidence(server.Profile.ID, tool.Name, prompt, result, normalizedEvidence{Official: hasCapability(request, capabilityOfficialDocSearch), Freshness: "external-docs"})
	return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, tool.Name)
}

func capabilityRequestForRoute(profile providerProfile, route askintent.Route, prompt string) capabilityRequest {
	switch route {
	case askintent.RouteQuestion, askintent.RouteExplain, askintent.RouteDraft:
	default:
		return capabilityRequest{}
	}
	request := capabilityRequest{Query: strings.TrimSpace(prompt)}
	switch profile.ID {
	case "context7":
		if isLibraryPrompt(prompt) {
			request.Capabilities = []capability{capabilityEntityResolve, capabilityDocFetch}
		}
	case "web-search":
		request.Capabilities = []capability{capabilityOfficialDocSearch, capabilityWebSearch}
	}
	return request
}

func hasCapability(request capabilityRequest, target capability) bool {
	for _, capability := range request.Capabilities {
		if capability == target {
			return true
		}
	}
	return false
}

func isLibraryPrompt(prompt string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return false
	}
	for _, hint := range []string{
		"github.com/",
		"gitlab.com/",
		"golang.org/",
		"pkg.go.dev/",
		"library",
		"package",
		"module",
		"sdk",
		"client-go",
		"godoc",
		"api docs",
		"api documentation",
		"api reference",
		"reference docs",
		"crate",
	} {
		if strings.Contains(prompt, hint) {
			return true
		}
	}
	return false
}

func findTool(tools *mcp.ListToolsResult, names ...string) (mcp.Tool, bool) {
	if tools == nil {
		return mcp.Tool{}, false
	}
	for _, candidate := range names {
		for _, tool := range tools.Tools {
			if strings.EqualFold(tool.Name, candidate) {
				return tool, true
			}
		}
	}
	return mcp.Tool{}, false
}

func callTool(ctx context.Context, c *client.Client, providerID string, tool mcp.Tool, args map[string]any) (*mcp.CallToolResult, string) {
	result, err := c.CallTool(ctx, mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool.Name, Arguments: args}})
	if err != nil {
		return nil, fmt.Sprintf("mcp:%s call %s failed: %v", providerID, tool.Name, err)
	}
	if result != nil && result.IsError {
		return nil, fmt.Sprintf("mcp:%s call %s returned tool error", providerID, tool.Name)
	}
	if result == nil || len(result.Content) == 0 {
		return nil, fmt.Sprintf("mcp:%s call %s returned empty", providerID, tool.Name)
	}
	return result, ""
}

func buildResolveArgs(tool mcp.Tool, prompt string) map[string]any {
	args := map[string]any{}
	query := strings.TrimSpace(prompt)
	libraryName := libraryQueryFromPrompt(prompt)
	setToolArg(tool, args, []string{"libraryName", "library", "name"}, libraryName)
	setToolArg(tool, args, []string{"query", "question", "prompt"}, query)
	if len(args) == 0 {
		args["libraryName"] = libraryName
		args["query"] = query
	}
	return args
}

func buildContext7DocsArgs(tool mcp.Tool, route askintent.Route, prompt string, libraryID string) map[string]any {
	args := map[string]any{}
	query := strings.TrimSpace(prompt)
	budget := context7DocsTokenBudget(route, query)
	if strings.EqualFold(strings.TrimSpace(tool.Name), "query-docs") && strings.TrimSpace(libraryID) == "" {
		query = libraryQueryFromPrompt(prompt)
	}
	if libraryID != "" {
		setToolArg(tool, args, []string{"context7CompatibleLibraryID", "libraryID", "libraryId", "id"}, libraryID)
	}
	setToolArg(tool, args, []string{"topic", "query", "question", "prompt"}, query)
	setToolArg(tool, args, []string{"libraryName", "library", "name"}, query)
	setToolArg(tool, args, []string{"tokens", "maxTokens", "tokenLimit"}, budget)
	if len(args) == 0 {
		if libraryID != "" {
			args["libraryID"] = libraryID
		}
		args["query"] = query
		args["tokens"] = budget
	}
	return args
}

func context7DocsTokenBudget(route askintent.Route, query string) int {
	budget := defaultContext7DocsTokenBudget
	if route == askintent.RouteDraft {
		budget = draftContext7DocsTokenBudget
	}
	if len([]rune(query)) > context7LongPromptRuneLimit {
		budget += context7LongPromptBudgetBump
	}
	if budget > maxContext7DocsTokenBudget {
		return maxContext7DocsTokenBudget
	}
	return budget
}

func buildSearchArgs(tool mcp.Tool, prompt string) map[string]any {
	args := map[string]any{}
	setToolArg(tool, args, []string{"query", "q", "search"}, strings.TrimSpace(prompt))
	setToolArg(tool, args, []string{"limit", "maxResults", "topK"}, 3)
	if len(args) == 0 {
		args["query"] = strings.TrimSpace(prompt)
		args["limit"] = 3
	}
	return args
}

func setToolArg(tool mcp.Tool, args map[string]any, keys []string, value any) {
	if args == nil {
		return
	}
	properties := tool.InputSchema.Properties
	for _, key := range keys {
		if _, ok := properties[key]; ok {
			args[key] = value
			return
		}
	}
}

func libraryQueryFromPrompt(prompt string) string {
	for _, token := range strings.Fields(prompt) {
		clean := strings.Trim(token, " \t\r\n\"'`(),.:;")
		if strings.Contains(clean, "/") || strings.Contains(clean, ".") {
			return clean
		}
	}
	return strings.TrimSpace(prompt)
}

func parseContext7Entity(result *mcp.CallToolResult, prompt string) context7Entity {
	structured := context7StructuredValue("resolve-library-id", result)
	index := caseInsensitiveStringIndex(structured)
	entity := context7Entity{
		LibraryID: indexedString(index, []string{"context7CompatibleLibraryID", "libraryID", "libraryId", "id"}),
		Title:     indexedString(index, []string{"title", "name", "libraryName"}),
	}
	if entity.LibraryID == "" {
		entity.LibraryID = extractLibraryIDFromText(extractText(result))
	}
	if entity.Title == "" {
		entity.Title = libraryQueryFromPrompt(prompt)
	}
	return entity
}

func extractLibraryIDFromText(text string) string {
	for _, pattern := range context7LibraryIDPatterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		candidate := strings.TrimSpace(match[1])
		candidate = strings.Trim(candidate, " \t\r\n\"'")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func normalizeEvidence(providerID string, toolName string, prompt string, result *mcp.CallToolResult, seed normalizedEvidence) normalizedEvidence {
	normalizedToolName := normalizeToolName(toolName)
	if providerID == "web-search" {
		return normalizeWebSearchEvidence(normalizedToolName, prompt, result, seed)
	}
	text := extractText(result)
	structured := context7StructuredValue(normalizedToolName, result)
	structuredStrings := caseInsensitiveStringIndex(structured)
	evidence := seed
	evidence.Provider = providerID
	evidence.ToolName = toolName
	if evidence.SourceURL == "" {
		evidence.SourceURL = indexedString(structuredStrings, []string{"url", "uri", "href", "link", "source"})
	}
	if evidence.Domain == "" {
		evidence.Domain = domainFromURL(evidence.SourceURL)
	}
	if evidence.Title == "" {
		evidence.Title = indexedString(structuredStrings, []string{"title", "name", "libraryName"})
	}
	if evidence.Title == "" {
		evidence.Title = firstNonEmptyLine(text)
	}
	if evidence.Excerpt == "" {
		evidence.Excerpt = indexedString(structuredStrings, []string{"excerpt", "snippet", "summary", "description", "text", "content"})
	}
	if evidence.Excerpt == "" {
		evidence.Excerpt = compactExcerpt(text, 600)
	}
	artifacts := summarizeEvidence(text, prompt)
	if artifacts != nil {
		evidence.ArtifactKinds = append([]string(nil), artifacts.ArtifactKinds...)
		evidence.InstallHints = append([]string(nil), artifacts.InstallHints...)
		evidence.OfflineHints = append([]string(nil), artifacts.OfflineHints...)
	}
	return evidence
}

func normalizeWebSearchEvidence(toolName string, prompt string, result *mcp.CallToolResult, seed normalizedEvidence) normalizedEvidence {
	text := extractText(result)
	candidates := webSearchStructuredCandidates(toolName, result)
	version := requestedVersion(prompt)
	best := seed
	best.Provider = "web-search"
	best.ToolName = toolName
	bestScore := -1
	for _, candidate := range candidates {
		candidateStrings := caseInsensitiveStringIndex(candidate)
		evidence := seed
		evidence.Provider = "web-search"
		evidence.ToolName = toolName
		evidence.SourceURL = indexedString(candidateStrings, []string{"url", "uri", "href", "link", "source"})
		evidence.Domain = domainFromURL(evidence.SourceURL)
		evidence.Title = indexedString(candidateStrings, []string{"title", "name"})
		evidence.Excerpt = indexedString(candidateStrings, []string{"excerpt", "snippet", "summary", "description", "text", "content"})
		if evidence.Title == "" {
			evidence.Title = firstNonEmptyLine(text)
		}
		if evidence.Excerpt == "" {
			evidence.Excerpt = compactExcerpt(text, 600)
		}
		annotateEvidenceTrust(&evidence, version)
		score := scoreWebSearchEvidence(evidence, version)
		if score > bestScore {
			best = evidence
			bestScore = score
		}
	}
	if bestScore < 0 {
		structured := webSearchStructuredFallback(toolName, result)
		structuredStrings := caseInsensitiveStringIndex(structured)
		best.SourceURL = indexedString(structuredStrings, []string{"url", "uri", "href", "link", "source"})
		best.Domain = domainFromURL(best.SourceURL)
		best.Title = indexedString(structuredStrings, []string{"title", "name"})
		if best.Title == "" {
			best.Title = firstNonEmptyLine(text)
		}
		best.Excerpt = indexedString(structuredStrings, []string{"excerpt", "snippet", "summary", "description", "text", "content"})
		if best.Excerpt == "" {
			best.Excerpt = compactExcerpt(text, 600)
		}
		annotateEvidenceTrust(&best, version)
	}
	artifacts := summarizeEvidence(text, prompt)
	if artifacts != nil {
		best.ArtifactKinds = append([]string(nil), artifacts.ArtifactKinds...)
		best.InstallHints = append([]string(nil), artifacts.InstallHints...)
		best.OfflineHints = append([]string(nil), artifacts.OfflineHints...)
	}
	return best
}

func webSearchStructuredCandidates(toolName string, result *mcp.CallToolResult) []any {
	if result == nil {
		return nil
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		return nil
	}
	if items, ok := caseInsensitiveSliceValue(structured, webSearchCandidateKeys(toolName)); ok && len(items) > 0 {
		return items
	}
	return nil
}

func webSearchStructuredFallback(toolName string, result *mcp.CallToolResult) any {
	if result == nil {
		return nil
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		return result.StructuredContent
	}
	if nested, ok := caseInsensitiveValue(structured, webSearchSingleObjectKeys(toolName)); ok {
		return nested
	}
	return structured
}

func webSearchCandidateKeys(toolName string) []string {
	switch toolName {
	case "search", "web-search", "web_search":
		return []string{"results", "items", "sources"}
	default:
		return nil
	}
}

func webSearchSingleObjectKeys(toolName string) []string {
	switch toolName {
	case "search", "web-search", "web_search":
		return []string{"result", "item", "source"}
	default:
		return nil
	}
}

func context7StructuredValue(toolName string, result *mcp.CallToolResult) any {
	if result == nil {
		return nil
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		return result.StructuredContent
	}
	switch toolName {
	case "resolve-library-id":
		if hasCaseInsensitiveKey(structured, []string{"context7CompatibleLibraryID", "libraryID", "libraryId", "id"}) {
			return structured
		}
		if nested, ok := caseInsensitiveValue(structured, []string{"library", "match", "resolved"}); ok {
			return nested
		}
		if items, ok := caseInsensitiveSliceValue(structured, []string{"matches", "libraries", "candidates"}); ok && len(items) > 0 {
			return items[0]
		}
	case "get-library-docs", "query-docs":
		if hasCaseInsensitiveKey(structured, []string{"url", "uri", "href", "link", "source", "title", "name", "libraryName", "excerpt", "snippet", "summary", "description", "text", "content"}) {
			return structured
		}
		if nested, ok := caseInsensitiveValue(structured, []string{"document", "doc", "page"}); ok {
			return nested
		}
	}
	return structured
}

func scoreWebSearchEvidence(evidence normalizedEvidence, version string) int {
	score := 0
	switch evidence.TrustLevel {
	case "high":
		score += 300
	case "medium":
		score += 200
	case "low":
		score += 100
	}
	if evidence.Official {
		score += 50
	}
	switch evidence.VersionSupport {
	case "direct":
		score += 30
	case "indirect":
		score += 10
	case "unknown":
		if strings.TrimSpace(version) != "" {
			score -= 10
		}
	}
	if strings.TrimSpace(evidence.Excerpt) != "" {
		score += 5
	}
	if strings.TrimSpace(evidence.Title) != "" {
		score += 3
	}
	return score
}

func annotateEvidenceTrust(evidence *normalizedEvidence, version string) {
	if evidence == nil {
		return
	}
	domain := strings.ToLower(strings.TrimSpace(evidence.Domain))
	path := strings.ToLower(strings.TrimSpace(evidence.SourceURL))
	var category string
	var trust string
	official := evidence.Official
	switch {
	case domain == "":
		category = "unknown"
		trust = "medium"
	case isCommunityDomain(domain):
		category = "community"
		trust = "low"
		official = false
	case isAggregatorDomain(domain):
		category = "aggregator"
		trust = "low"
		official = false
	case isSourceHostDomain(domain):
		category = "source-host"
		trust = "medium"
		official = false
	case strings.HasPrefix(domain, "docs.") || strings.HasPrefix(domain, "developer.") || strings.HasPrefix(domain, "learn.") || strings.HasPrefix(domain, "support.") || strings.HasPrefix(domain, "help.") || strings.Contains(path, "/docs/") || strings.HasSuffix(path, "/docs") || strings.Contains(path, "/documentation/"):
		category = "official-docs"
		trust = "high"
		official = true
	default:
		category = "vendor-site"
		trust = "medium"
	}
	evidence.DomainCategory = category
	evidence.TrustLevel = trust
	evidence.Official = official
	evidence.VersionSupport = detectVersionSupport(version, *evidence)
}

func requestedVersion(prompt string) string {
	match := requestedVersionPattern.FindString(strings.TrimSpace(prompt))
	return strings.TrimSpace(strings.TrimPrefix(strings.ToLower(match), "v"))
}

func detectVersionSupport(version string, evidence normalizedEvidence) string {
	version = strings.TrimSpace(strings.ToLower(version))
	if version == "" {
		return ""
	}
	text := strings.ToLower(strings.Join([]string{evidence.Title, evidence.Excerpt, evidence.SourceURL}, " "))
	if versionBoundaryPattern(version).MatchString(text) {
		return "direct"
	}
	if requestedVersionPattern.MatchString(text) {
		return "indirect"
	}
	return "unknown"
}

func isCommunityDomain(domain string) bool {
	for token := range communityDomains {
		if domain == token || strings.HasSuffix(domain, "."+token) {
			return true
		}
	}
	return false
}

func isAggregatorDomain(domain string) bool {
	for token := range aggregatorDomains {
		if domain == token || strings.HasSuffix(domain, "."+token) {
			return true
		}
	}
	return false
}

func isSourceHostDomain(domain string) bool {
	for token := range sourceHostDomains {
		if domain == token || strings.HasSuffix(domain, "."+token) {
			return true
		}
	}
	return false
}

func versionBoundaryPattern(version string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\bv?` + regexp.QuoteMeta(strings.TrimSpace(version)) + `\b`)
}

func evidenceChunk(evidence normalizedEvidence) *askretrieve.Chunk {
	summary := &askretrieve.EvidenceSummary{
		Provider:       evidence.Provider,
		SourceURL:      evidence.SourceURL,
		SourceID:       evidence.SourceID,
		Domain:         evidence.Domain,
		DomainCategory: evidence.DomainCategory,
		Title:          evidence.Title,
		Excerpt:        evidence.Excerpt,
		Freshness:      evidence.Freshness,
		Official:       evidence.Official,
		TrustLevel:     evidence.TrustLevel,
		VersionSupport: evidence.VersionSupport,
		ArtifactKinds:  append([]string(nil), evidence.ArtifactKinds...),
		InstallHints:   append([]string(nil), evidence.InstallHints...),
		OfflineHints:   append([]string(nil), evidence.OfflineHints...),
	}
	label := evidence.Provider
	switch {
	case strings.TrimSpace(summary.Title) != "":
		label += ":" + strings.TrimSpace(summary.Title)
	case strings.TrimSpace(summary.Domain) != "":
		label += ":" + strings.TrimSpace(summary.Domain)
	case strings.TrimSpace(evidence.ToolName) != "":
		label += ":" + strings.TrimSpace(evidence.ToolName)
	}
	content := renderEvidence(*summary)
	if strings.TrimSpace(summary.Excerpt) != "" {
		content += "\n\nSource excerpt:\n" + strings.TrimSpace(summary.Excerpt)
	}
	topicKey := evidence.Provider
	switch {
	case summary.Domain != "":
		topicKey += ":" + summary.Domain
	case summary.Title != "":
		topicKey += ":" + summary.Title
	case evidence.ToolName != "":
		topicKey += ":" + evidence.ToolName
	}
	idKey := evidence.Provider + "-" + evidence.ToolName
	if summary.Domain != "" {
		idKey += "-" + summary.Domain
	} else if summary.Title != "" {
		idKey += "-" + summary.Title
	}
	return &askretrieve.Chunk{
		ID:       "mcp-" + sanitize(idKey),
		Source:   "mcp",
		Label:    label,
		Topic:    askcontext.Topic("mcp:" + sanitize(topicKey)),
		Content:  content,
		Score:    70,
		Evidence: summary,
	}
}

func extractText(result *mcp.CallToolResult) string {
	if result == nil {
		return ""
	}
	b := &strings.Builder{}
	for _, content := range result.Content {
		text := strings.TrimSpace(mcp.GetTextFromContent(content))
		if text == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(text)
	}
	return strings.TrimSpace(b.String())
}

func hasCaseInsensitiveKey(mapped map[string]any, keys []string) bool {
	_, ok := caseInsensitiveValue(mapped, keys)
	return ok
}

func normalizeToolName(toolName string) string {
	return strings.ToLower(strings.TrimSpace(toolName))
}

func caseInsensitiveStringIndex(value any) map[string]string {
	mapped, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	indexed := make(map[string]string, len(mapped))
	for candidate, raw := range mapped {
		text, ok := raw.(string)
		if !ok {
			continue
		}
		indexed[strings.ToLower(strings.TrimSpace(candidate))] = strings.TrimSpace(text)
	}
	return indexed
}

func caseInsensitiveValueIndex(mapped map[string]any) map[string]any {
	if len(mapped) == 0 {
		return nil
	}
	indexed := make(map[string]any, len(mapped))
	for candidate, raw := range mapped {
		indexed[strings.ToLower(strings.TrimSpace(candidate))] = raw
	}
	return indexed
}

func indexedString(index map[string]string, keys []string) string {
	if len(index) == 0 || len(keys) == 0 {
		return ""
	}
	for _, key := range keys {
		if text, ok := index[strings.ToLower(strings.TrimSpace(key))]; ok {
			return text
		}
	}
	return ""
}

func caseInsensitiveValue(mapped map[string]any, keys []string) (any, bool) {
	if len(keys) == 0 || len(mapped) == 0 {
		return nil, false
	}
	index := caseInsensitiveValueIndex(mapped)
	for _, key := range keys {
		if raw, ok := index[strings.ToLower(strings.TrimSpace(key))]; ok {
			return raw, true
		}
	}
	return nil, false
}

func caseInsensitiveSliceValue(mapped map[string]any, keys []string) ([]any, bool) {
	raw, ok := caseInsensitiveValue(mapped, keys)
	if !ok {
		return nil, false
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	return items, true
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func compactExcerpt(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "..."
}

func domainFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}
