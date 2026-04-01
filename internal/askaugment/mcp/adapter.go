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
	Provider      string
	ToolName      string
	SourceURL     string
	SourceID      string
	Domain        string
	Title         string
	Excerpt       string
	Freshness     string
	Official      bool
	ArtifactKinds []string
	InstallHints  []string
	OfflineHints  []string
}

type context7Entity struct {
	LibraryID string
	Title     string
}

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
		result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, prompt, ""))
		if failure == "" {
			evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, Freshness: "external-docs"})
			return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
		}
	}
	var entity context7Entity
	if hasCapability(request, capabilityEntityResolve) {
		tool, ok := findTool(tools, "resolve-library-id")
		if !ok {
			result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, prompt, ""))
			if failure != "" {
				return nil, failure
			}
			evidence := normalizeEvidence(server.Profile.ID, docTool.Name, prompt, result, normalizedEvidence{Official: true, Freshness: "external-docs"})
			return evidenceChunk(evidence), fmt.Sprintf("mcp:%s call %s ok", server.Profile.ID, docTool.Name)
		}
		resolved, failure := callTool(ctx, c, server.Profile.ID, tool, buildResolveArgs(tool, prompt))
		if failure != "" {
			result, docFailure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, prompt, ""))
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
	result, failure := callTool(ctx, c, server.Profile.ID, docTool, buildContext7DocsArgs(docTool, prompt, entity.LibraryID))
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
		request.Capabilities = []capability{capabilityDocFetch}
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
	for _, hint := range []string{"library", "package", "module", "sdk", "api", "golang.org/", "github.com/", "npm", "pip", "crate"} {
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

func buildContext7DocsArgs(tool mcp.Tool, prompt string, libraryID string) map[string]any {
	args := map[string]any{}
	query := strings.TrimSpace(prompt)
	if strings.EqualFold(strings.TrimSpace(tool.Name), "query-docs") && strings.TrimSpace(libraryID) == "" {
		query = libraryQueryFromPrompt(prompt)
	}
	if libraryID != "" {
		setToolArg(tool, args, []string{"context7CompatibleLibraryID", "libraryID", "libraryId", "id"}, libraryID)
	}
	setToolArg(tool, args, []string{"topic", "query", "question", "prompt"}, query)
	setToolArg(tool, args, []string{"libraryName", "library", "name"}, query)
	setToolArg(tool, args, []string{"tokens", "maxTokens", "tokenLimit"}, 1800)
	if len(args) == 0 {
		if libraryID != "" {
			args["libraryID"] = libraryID
		}
		args["query"] = query
		args["tokens"] = 1800
	}
	return args
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
	properties := map[string]any(tool.InputSchema.Properties)
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
	structured := primaryStructuredValue(result.StructuredContent)
	entity := context7Entity{
		LibraryID: firstString(structured, []string{"context7CompatibleLibraryID", "libraryID", "libraryId", "id"}),
		Title:     firstString(structured, []string{"title", "name", "libraryName"}),
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
	pattern := regexp.MustCompile(`(?i)(/[a-z0-9_.-]+/[a-z0-9_.-]+(?:/[a-z0-9_.-]+)?|github\.com/[\w./-]+|golang\.org/[\w./-]+|context7-compatible\s+library\s+id\s*[:=]\s*(/[\w./-]+)|library\s*id\s*[:=]\s*(/[\w./-]+)|library\s*[:=]\s*([\w./-]+))`)
	match := pattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return ""
	}
	for _, candidate := range match[1:] {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(strings.ToLower(candidate), "context7-compatible library id:")
		candidate = strings.TrimPrefix(candidate, "library id:")
		candidate = strings.TrimPrefix(candidate, "library:")
		candidate = strings.Trim(candidate, " \t\r\n\"'")
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func normalizeEvidence(providerID string, toolName string, prompt string, result *mcp.CallToolResult, seed normalizedEvidence) normalizedEvidence {
	text := extractText(result)
	structured := primaryStructuredValue(result.StructuredContent)
	evidence := seed
	evidence.Provider = providerID
	evidence.ToolName = toolName
	if evidence.SourceURL == "" {
		evidence.SourceURL = firstString(structured, []string{"url", "uri", "href", "link", "source"})
	}
	if evidence.Domain == "" {
		evidence.Domain = domainFromURL(evidence.SourceURL)
	}
	if evidence.Title == "" {
		evidence.Title = firstString(structured, []string{"title", "name", "libraryName"})
	}
	if evidence.Title == "" {
		evidence.Title = firstNonEmptyLine(text)
	}
	if evidence.Excerpt == "" {
		evidence.Excerpt = firstString(structured, []string{"excerpt", "snippet", "summary", "description", "text", "content"})
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

func evidenceChunk(evidence normalizedEvidence) *askretrieve.Chunk {
	summary := &askretrieve.EvidenceSummary{
		Provider:      evidence.Provider,
		SourceURL:     evidence.SourceURL,
		SourceID:      evidence.SourceID,
		Domain:        evidence.Domain,
		Title:         evidence.Title,
		Excerpt:       evidence.Excerpt,
		Freshness:     evidence.Freshness,
		Official:      evidence.Official,
		ArtifactKinds: append([]string(nil), evidence.ArtifactKinds...),
		InstallHints:  append([]string(nil), evidence.InstallHints...),
		OfflineHints:  append([]string(nil), evidence.OfflineHints...),
	}
	label := evidence.Provider
	if strings.TrimSpace(summary.Title) != "" {
		label += ":" + strings.TrimSpace(summary.Title)
	} else if strings.TrimSpace(summary.Domain) != "" {
		label += ":" + strings.TrimSpace(summary.Domain)
	} else if strings.TrimSpace(evidence.ToolName) != "" {
		label += ":" + strings.TrimSpace(evidence.ToolName)
	}
	content := renderEvidence(*summary)
	if strings.TrimSpace(summary.Excerpt) != "" {
		content += "\n\nSource excerpt:\n" + strings.TrimSpace(summary.Excerpt)
	}
	topicKey := evidence.Provider
	if summary.Domain != "" {
		topicKey += ":" + summary.Domain
	} else if summary.Title != "" {
		topicKey += ":" + summary.Title
	} else if evidence.ToolName != "" {
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

func primaryStructuredValue(value any) any {
	current := value
	for {
		mapped, ok := current.(map[string]any)
		if !ok {
			return current
		}
		for _, key := range []string{"results", "items", "documents", "sources"} {
			items, ok := mapped[key].([]any)
			if ok && len(items) > 0 {
				current = items[0]
				goto next
			}
		}
		return mapped
	next:
	}
}

func firstString(value any, keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	for _, key := range keys {
		if out := firstStringForKey(value, key); out != "" {
			return out
		}
	}
	return ""
}

func firstStringForKey(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		for candidate, raw := range typed {
			if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(key)) {
				if text, ok := raw.(string); ok {
					return strings.TrimSpace(text)
				}
			}
			if nested := firstStringForKey(raw, key); nested != "" {
				return nested
			}
		}
	case []any:
		for _, item := range typed {
			if nested := firstStringForKey(item, key); nested != "" {
				return nested
			}
		}
	}
	return ""
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
