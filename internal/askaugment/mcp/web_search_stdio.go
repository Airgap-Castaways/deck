package mcpaugment

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	xhtml "golang.org/x/net/html"
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
	// DuckDuckGo's HTML endpoint is a pragmatic fallback because we do not have a
	// stable structured search API in this built-in provider path. Parsing these
	// result pages is inherently brittle, so keep the extraction logic simple and
	// treat any future layout change as a signal to revisit the transport.
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
	doc, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	containers := findDuckDuckGoResultContainers(doc)
	results := make([]builtInSearchResult, 0, min(limit, len(containers)))
	for _, container := range containers {
		if len(results) >= limit {
			break
		}
		anchor := findFirstNode(container, func(node *xhtml.Node) bool {
			return node.Type == xhtml.ElementNode && node.Data == "a" && hasClass(node, "result__a", "result-link")
		})
		if anchor == nil {
			continue
		}
		title := strings.TrimSpace(nodeText(anchor))
		if title == "" {
			continue
		}
		snippetNode := findFirstNode(container, func(node *xhtml.Node) bool {
			return node.Type == xhtml.ElementNode && hasClass(node, "result__snippet")
		})
		result := builtInSearchResult{Title: title, URL: normalizeDuckDuckGoURL(attr(anchor, "href"))}
		if snippetNode != nil {
			result.Snippet = strings.TrimSpace(nodeText(snippetNode))
		}
		results = append(results, result)
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

func findDuckDuckGoResultContainers(root *xhtml.Node) []*xhtml.Node {
	results := make([]*xhtml.Node, 0)
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node == nil {
			return
		}
		if node.Type == xhtml.ElementNode && hasClass(node, "result") {
			results = append(results, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return results
}

func findFirstNode(root *xhtml.Node, match func(*xhtml.Node) bool) *xhtml.Node {
	if root == nil {
		return nil
	}
	if match(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstNode(child, match); found != nil {
			return found
		}
	}
	return nil
}

func hasClass(node *xhtml.Node, classes ...string) bool {
	if node == nil {
		return false
	}
	classValue := attr(node, "class")
	if classValue == "" {
		return false
	}
	parts := strings.Fields(classValue)
	for _, className := range classes {
		for _, part := range parts {
			if strings.TrimSpace(part) == strings.TrimSpace(className) {
				return true
			}
		}
	}
	return false
}

func attr(node *xhtml.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return html.UnescapeString(strings.TrimSpace(attribute.Val))
		}
	}
	return ""
}

func nodeText(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	b := &strings.Builder{}
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			text := strings.TrimSpace(html.UnescapeString(current.Data))
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(b.String()), " ")
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
