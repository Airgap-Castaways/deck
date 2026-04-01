package mcpaugment

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type builtInSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

var builtInWebSearchHTTPClient = &http.Client{Timeout: 10 * time.Second}

func ServeBuiltInWebSearchMCP() error {
	srv := server.NewMCPServer("deck-web-search", "1.0.0", server.WithToolCapabilities(false))
	srv.AddTool(mcp.NewTool("search",
		mcp.WithDescription("Search the web for current upstream documentation and troubleshooting pages."),
		mcp.WithString("query", mcp.Description("Search query"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return"), mcp.DefaultNumber(3)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := strings.TrimSpace(req.GetString("query", ""))
		if query == "" {
			return mcp.NewToolResultError("query is required"), nil
		}
		limit := req.GetInt("limit", 3)
		if limit <= 0 {
			limit = 3
		}
		if limit > 5 {
			limit = 5
		}
		results, err := runBuiltInWebSearch(ctx, query, limit)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("web search failed", err), nil
		}
		return mcp.NewToolResultStructured(map[string]any{"results": results}, renderBuiltInWebSearchResults(results)), nil
	})
	return server.ServeStdio(srv)
}

func runBuiltInWebSearch(ctx context.Context, query string, limit int) ([]builtInSearchResult, error) {
	requestURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(strings.TrimSpace(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("User-Agent", "deck-web-search/1.0")
	resp, err := builtInWebSearchHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("search request failed with status %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read search response: %w", err)
	}
	return parseDuckDuckGoResults(string(body), limit), nil
}

func parseDuckDuckGoResults(body string, limit int) []builtInSearchResult {
	anchorRe := regexp.MustCompile(`(?is)<a[^>]*class="[^"]*(?:result__a|result-link)[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	snippetRe := regexp.MustCompile(`(?is)<(?:a|div)[^>]*class="[^"]*result__snippet[^"]*"[^>]*>(.*?)</(?:a|div)>`)
	matches := anchorRe.FindAllStringSubmatchIndex(body, limit)
	results := make([]builtInSearchResult, 0, len(matches))
	for idx, match := range matches {
		if len(match) < 6 {
			continue
		}
		rawURL := body[match[2]:match[3]]
		title := cleanSearchHTML(body[match[4]:match[5]])
		if title == "" {
			continue
		}
		segmentEnd := len(body)
		if idx+1 < len(matches) && len(matches[idx+1]) > 0 {
			segmentEnd = matches[idx+1][0]
		}
		snippet := ""
		if snippetMatch := snippetRe.FindStringSubmatch(body[match[1]:segmentEnd]); len(snippetMatch) > 1 {
			snippet = cleanSearchHTML(firstNonEmptyString(snippetMatch[1:]...))
		}
		results = append(results, builtInSearchResult{Title: title, URL: normalizeDuckDuckGoURL(rawURL), Snippet: snippet})
	}
	return results
}

func normalizeDuckDuckGoURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Host == "duckduckgo.com" && strings.HasPrefix(parsed.Path, "/l/") {
		if target := strings.TrimSpace(parsed.Query().Get("uddg")); target != "" {
			decoded, err := url.QueryUnescape(target)
			if err == nil {
				return decoded
			}
			return target
		}
	}
	return parsed.String()
}

func cleanSearchHTML(raw string) string {
	tagRe := regexp.MustCompile(`(?is)<[^>]+>`)
	cleaned := html.UnescapeString(tagRe.ReplaceAllString(raw, " "))
	return strings.Join(strings.Fields(cleaned), " ")
}

func renderBuiltInWebSearchResults(results []builtInSearchResult) string {
	if len(results) == 0 {
		return "No search results found."
	}
	b := &strings.Builder{}
	for _, result := range results {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(result.Title))
		if strings.TrimSpace(result.URL) != "" {
			b.WriteString(" (")
			b.WriteString(strings.TrimSpace(result.URL))
			b.WriteString(")")
		}
		if strings.TrimSpace(result.Snippet) != "" {
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(result.Snippet))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
