package mcpaugment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/testutil/mcpfake"
)

func TestMCPFakeProcess(t *testing.T) {
	mode := mcpfake.ModeFromArgs()
	if mode == "" {
		return
	}
	mcpfake.Serve(mode, handleHelperRequest)
}

func TestGatherDisabledReturnsNothing(t *testing.T) {
	chunks, events := Gather(context.Background(), askconfig.MCP{}, askintent.RouteExplain, "explain apply")
	if len(chunks) != 0 || len(events) != 0 {
		t.Fatalf("expected disabled mcp gather to return nothing, got chunks=%v events=%v", chunks, events)
	}
}

func TestQueryServerContext7GenericExplainRouteSkipsProvider(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "context7", "context7-no-tool"), askintent.RouteExplain, "What is kubeadm?")
	if chunk != nil {
		t.Fatalf("expected no chunk, got %#v", chunk)
	}
	if event != "" {
		t.Fatalf("unexpected event: %q", event)
	}
}

func TestQueryServerContext7LibraryPromptResolveFlowReturnsToolError(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "context7", "context7-resolve-error"), askintent.RouteExplain, "Explain github.com/mark3labs/mcp-go")
	if chunk != nil {
		t.Fatalf("expected no chunk, got %#v", chunk)
	}
	if event != "mcp:context7 call resolve-library-id returned tool error" {
		t.Fatalf("unexpected event: %q", event)
	}
}

func TestQueryServerContext7LibraryPromptReturnsNormalizedEvidence(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "context7", "context7-success"), askintent.RouteExplain, "Explain github.com/mark3labs/mcp-go")
	if event != "mcp:context7 call get-library-docs ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Provider != "context7" || chunk.Evidence.SourceID != "github.com/mark3labs/mcp-go" {
		t.Fatalf("expected normalized context7 metadata, got %#v", chunk.Evidence)
	}
	if chunk.Evidence.Domain != "pkg.go.dev" || chunk.Evidence.Title != "github.com/mark3labs/mcp-go" {
		t.Fatalf("expected normalized source metadata, got %#v", chunk.Evidence)
	}
	if !chunk.Evidence.Official || !strings.Contains(chunk.Content, `"provider": "context7"`) || !strings.Contains(chunk.Content, `"domain": "pkg.go.dev"`) {
		t.Fatalf("expected structured evidence content, got %#v", chunk)
	}
	if !strings.Contains(chunk.Content, "MCP Go is a Go SDK for MCP") {
		t.Fatalf("expected excerpt in chunk content, got %q", chunk.Content)
	}
}

func TestQueryServerContext7QueryDocsFallbackReturnsNormalizedEvidence(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "context7", "context7-query-docs"), askintent.RouteExplain, "Explain github.com/mark3labs/mcp-go")
	if event != "mcp:context7 call query-docs ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Provider != "context7" || chunk.Evidence.Domain != "context7.com" {
		t.Fatalf("expected normalized context7 metadata, got %#v", chunk.Evidence)
	}
	if !strings.Contains(chunk.Content, "MCP Go lets Go programs expose MCP servers") {
		t.Fatalf("expected docs excerpt in chunk content, got %q", chunk.Content)
	}
}

func TestQueryServerContext7IgnoresGenericResultsNoiseInDocsResponse(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "context7", "context7-docs-results-noise"), askintent.RouteExplain, "Explain github.com/mark3labs/mcp-go")
	if event != "mcp:context7 call get-library-docs ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Domain != "pkg.go.dev" || chunk.Evidence.Title != "github.com/mark3labs/mcp-go" {
		t.Fatalf("expected context7 root document metadata to win, got %#v", chunk.Evidence)
	}
	if strings.Contains(chunk.Content, "Wrong result") {
		t.Fatalf("expected generic results noise to be ignored, got %q", chunk.Content)
	}
}

func TestBuildResolveArgsIncludesBothLibraryNameAndQueryWhenSchemaRequiresThem(t *testing.T) {
	tool := mcp.Tool{Name: "resolve-library-id", InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{"libraryName": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}}}}
	args := buildResolveArgs(tool, "Explain github.com/mark3labs/mcp-go")
	if got := strings.TrimSpace(fmt.Sprint(args["libraryName"])); got != "github.com/mark3labs/mcp-go" {
		t.Fatalf("expected libraryName arg, got %#v", args)
	}
	if got := strings.TrimSpace(fmt.Sprint(args["query"])); got != "Explain github.com/mark3labs/mcp-go" {
		t.Fatalf("expected query arg, got %#v", args)
	}
}

func TestExtractLibraryIDFromContext7ResolveText(t *testing.T) {
	text := "Available Libraries:\n\n- Title: MCP Go\n- Context7-compatible library ID: /mark3labs/mcp-go\n- Description: A Go implementation"
	if got := extractLibraryIDFromText(text); got != "/mark3labs/mcp-go" {
		t.Fatalf("expected library id from resolve text, got %q", got)
	}
}

func TestQueryServerWebSearchReturnsNormalizedEvidence(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "web-search-success"), askintent.RouteExplain, "How do I install kubeadm?")
	if event != "mcp:web-search call search ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Provider != "web-search" || chunk.Evidence.Domain != "kubernetes.io" {
		t.Fatalf("expected normalized web-search metadata, got %#v", chunk.Evidence)
	}
	if chunk.Evidence.Title != "Installing kubeadm" || !chunk.Evidence.Official {
		t.Fatalf("expected official search metadata, got %#v", chunk.Evidence)
	}
	if chunk.Evidence.DomainCategory != "official-docs" || chunk.Evidence.TrustLevel != "high" {
		t.Fatalf("expected trust metadata, got %#v", chunk.Evidence)
	}
	if !strings.Contains(chunk.Content, `"title": "Installing kubeadm"`) || !strings.Contains(chunk.Content, "Install kubeadm and kubelet from the official Kubernetes docs") {
		t.Fatalf("expected structured web-search evidence, got %q", chunk.Content)
	}
}

func TestQueryServerWebSearchPrefersOfficialResultAcrossMixedSources(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "web-search-mixed-trust"), askintent.RouteExplain, "How do I install kubeadm 1.35.1?")
	if event != "mcp:web-search call search ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Domain != "kubernetes.io" || chunk.Evidence.TrustLevel != "high" || !chunk.Evidence.Official {
		t.Fatalf("expected official high-trust evidence, got %#v", chunk.Evidence)
	}
	if chunk.Evidence.VersionSupport != "direct" {
		t.Fatalf("expected direct version support, got %#v", chunk.Evidence)
	}
}

func TestQueryServerWebSearchMarksWeakVersionSupport(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "web-search-community-version"), askintent.RouteExplain, "How do I install kubeadm 1.35.1?")
	if event != "mcp:web-search call search ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.TrustLevel != "low" || chunk.Evidence.DomainCategory != "community" || chunk.Evidence.Official {
		t.Fatalf("expected low-trust community evidence, got %#v", chunk.Evidence)
	}
	if chunk.Evidence.VersionSupport != "unknown" {
		t.Fatalf("expected unknown version support, got %#v", chunk.Evidence)
	}
}

func TestQueryServerWebSearchSupportsItemsShape(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "web-search-items-shape"), askintent.RouteExplain, "How do I install kubeadm?")
	if event != "mcp:web-search call search ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected evidence chunk, got %#v", chunk)
	}
	if chunk.Evidence.Domain != "kubernetes.io" || chunk.Evidence.Title != "Installing kubeadm from items" {
		t.Fatalf("expected items-based web search evidence, got %#v", chunk.Evidence)
	}
}

func TestDetectVersionSupportAvoidsSubstringFalsePositive(t *testing.T) {
	evidence := normalizedEvidence{Title: "Installing kubeadm v1.35", Excerpt: "Official Kubernetes documentation for kubeadm v1.35 installation."}
	if got := detectVersionSupport("1.3", evidence); got != "indirect" {
		t.Fatalf("expected substring mismatch to avoid direct match, got %q", got)
	}
}

func TestQueryServerReportsListToolsMismatch(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "list-tools-mismatch"), askintent.RouteExplain, "How do I install kubeadm?")
	if chunk != nil {
		t.Fatalf("expected no chunk, got %#v", chunk)
	}
	if event != "mcp:web-search no known tool for route explain" {
		t.Fatalf("unexpected event: %q", event)
	}
}

func TestQueryServerPartialEvidenceReturnsNormalizedFallbackChunk(t *testing.T) {
	chunk, event := queryServer(context.Background(), helperServer(t, "web-search", "partial-evidence"), askintent.RouteExplain, "How do I install kubeadm?")
	if event != "mcp:web-search call search ok" {
		t.Fatalf("unexpected event: %q", event)
	}
	if chunk == nil || chunk.Evidence == nil {
		t.Fatalf("expected fallback normalized chunk, got %#v", chunk)
	}
	if chunk.Evidence.Provider != "web-search" || !strings.Contains(chunk.Content, "resource_link") {
		t.Fatalf("expected fallback resource evidence, got %#v", chunk)
	}
}

func TestGatherInitializeFailureReturnsProviderEvent(t *testing.T) {
	chunks, events := Gather(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-server", RunCommand: "/bin/sh", Args: []string{"-c", "exit 0"}}}}, askintent.RouteExplain, "search kubernetes docs")
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %#v", chunks)
	}
	if len(events) != 1 {
		t.Fatalf("expected one initialize failure event, got %#v", events)
	}
	if !strings.Contains(events[0], "mcp:web-search initialize failed:") || !strings.Contains(events[0], "transport closed") {
		t.Fatalf("expected initialize failure event, got %q", events[0])
	}
}

func TestResolveServerUsesBuiltInDefaultTransport(t *testing.T) {
	resolved, warning, ok := resolveServer(askconfig.MCPServer{Name: "context7"})
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if !ok {
		t.Fatalf("expected built-in provider resolution")
	}
	if resolved.Profile.ID != "context7" || resolved.RunCommand != "npx" {
		t.Fatalf("unexpected resolved provider: %#v", resolved)
	}
	if len(resolved.Args) != 2 || resolved.Args[0] != "-y" || resolved.Args[1] != "@upstash/context7-mcp@latest" {
		t.Fatalf("unexpected default transport args: %#v", resolved.Args)
	}
	if resolved.TransportDisplay != "npx -y @upstash/context7-mcp@latest" || resolved.TransportSource != "built-in-default" {
		t.Fatalf("unexpected transport metadata: %#v", resolved)
	}
}

func TestResolveServerUsesBuiltInDeckWebSearchTransport(t *testing.T) {
	resolved, warning, ok := resolveServer(askconfig.MCPServer{Name: "web-search"})
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if !ok {
		t.Fatalf("expected built-in provider resolution")
	}
	if resolved.Profile.ID != "web-search" || resolved.RunCommand == "" {
		t.Fatalf("unexpected resolved provider: %#v", resolved)
	}
	if len(resolved.Args) != 3 || resolved.Args[0] != "ask" || resolved.Args[1] != "mcp" || resolved.Args[2] != "web-search" {
		t.Fatalf("unexpected built-in web-search args: %#v", resolved.Args)
	}
	if resolved.TransportDisplay != "deck ask mcp web-search" || resolved.TransportSource != "built-in-default" {
		t.Fatalf("unexpected transport metadata: %#v", resolved)
	}
}

func TestResolveServerNormalizesLegacyAliasAndPreservesTransportOverride(t *testing.T) {
	resolved, warning, ok := resolveServer(askconfig.MCPServer{Name: "web-server", RunCommand: "node", Args: []string{"custom-web-search.js"}})
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if !ok {
		t.Fatalf("expected alias to resolve")
	}
	if resolved.Profile.ID != "web-search" {
		t.Fatalf("expected alias to normalize to web-search, got %#v", resolved)
	}
	if resolved.RunCommand != "node" || len(resolved.Args) != 1 || resolved.Args[0] != "custom-web-search.js" {
		t.Fatalf("expected transport override to win, got %#v", resolved)
	}
}

func TestGatherUnknownProviderWarnsAndSkips(t *testing.T) {
	chunks, events := Gather(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "custom-provider", RunCommand: "custom-mcp"}}}, askintent.RouteExplain, "search kubernetes docs")
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %#v", chunks)
	}
	if len(events) != 1 || events[0] != "mcp:custom-provider unknown built-in provider; ignoring config entry" {
		t.Fatalf("unexpected warning events: %#v", events)
	}
}

func TestCapabilityRequestForRoute(t *testing.T) {
	context7, ok := lookupProviderProfile("context7")
	if !ok {
		t.Fatalf("expected context7 profile")
	}
	request := capabilityRequestForRoute(context7, askintent.RouteExplain, "Explain github.com/mark3labs/mcp-go")
	if !hasCapability(request, capabilityEntityResolve) || !hasCapability(request, capabilityDocFetch) {
		t.Fatalf("expected library explain request to require entity resolve and doc fetch, got %#v", request)
	}
	webSearch, ok := lookupProviderProfile("web-search")
	if !ok {
		t.Fatalf("expected web-search profile")
	}
	request = capabilityRequestForRoute(webSearch, askintent.RouteReview, "review this workspace")
	if len(request.Capabilities) != 0 {
		t.Fatalf("expected review route to avoid external evidence capabilities, got %#v", request)
	}
}

func TestCapabilityRequestForRouteSkipsContext7ForInstallPrompt(t *testing.T) {
	context7, ok := lookupProviderProfile("context7")
	if !ok {
		t.Fatalf("expected context7 profile")
	}
	request := capabilityRequestForRoute(context7, askintent.RouteExplain, "How do I install kubeadm on Ubuntu 24.04?")
	if len(request.Capabilities) != 0 {
		t.Fatalf("expected install prompt to skip context7, got %#v", request)
	}
}

func TestGatherPrefersWebSearchForInstallPrompt(t *testing.T) {
	chunks, events := Gather(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{
		helperServer(t, "context7", "context7-success"),
		helperServer(t, "web-search", "web-search-success"),
	}}, askintent.RouteExplain, "How do I install kubeadm on Ubuntu 24.04?")
	if len(chunks) != 1 {
		t.Fatalf("expected only one evidence chunk, got %#v", chunks)
	}
	if chunks[0].Evidence == nil || chunks[0].Evidence.Provider != "web-search" {
		t.Fatalf("expected web-search evidence, got %#v", chunks[0].Evidence)
	}
	joined := strings.Join(events, "\n")
	if strings.Contains(joined, "mcp:context7") {
		t.Fatalf("expected install prompt to avoid context7 activity, got %#v", events)
	}
	if !strings.Contains(joined, "mcp:web-search call search ok") {
		t.Fatalf("expected web-search success event, got %#v", events)
	}
}

func TestProbeConfiguredProvidersReportsHealthyContext7Provider(t *testing.T) {
	health := ProbeConfiguredProviders(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{helperServer(t, "context7", "context7-success")}})
	if len(health) != 1 {
		t.Fatalf("expected one provider health result, got %#v", health)
	}
	if health[0].Status != "healthy" || health[0].Phase != "ready" {
		t.Fatalf("expected healthy provider, got %#v", health[0])
	}
	if len(health[0].Tools) != 2 || health[0].MissingCapabilities != nil {
		t.Fatalf("expected discovered tools and no missing capabilities, got %#v", health[0])
	}
}

func TestProbeConfiguredProvidersAcceptsContext7QueryDocsCapabilityShape(t *testing.T) {
	health := ProbeConfiguredProviders(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{helperServer(t, "context7", "context7-query-docs")}})
	if len(health) != 1 {
		t.Fatalf("expected one provider health result, got %#v", health)
	}
	if health[0].Status != "healthy" || health[0].Phase != "ready" {
		t.Fatalf("expected healthy provider, got %#v", health[0])
	}
}

func TestProbeConfiguredProvidersReportsCapabilityFailure(t *testing.T) {
	health := ProbeConfiguredProviders(context.Background(), askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{helperServer(t, "context7", "context7-no-tool")}})
	if len(health) != 1 {
		t.Fatalf("expected one provider health result, got %#v", health)
	}
	if health[0].Status != "error" || health[0].Phase != "capability" {
		t.Fatalf("expected capability failure, got %#v", health[0])
	}
	if len(health[0].MissingCapabilities) == 0 || !strings.Contains(health[0].Message, "missing tools for capabilities") {
		t.Fatalf("expected missing capability detail, got %#v", health[0])
	}
}

func helperServer(t *testing.T, name string, mode string) askconfig.MCPServer {
	return mcpfake.Server(t, name, mode)
}

func handleHelperRequest(mode string, req mcpfake.Request) (*mcpfake.Response, bool) {
	if mode == "initialize-close" && req.Method == string(mcp.MethodInitialize) {
		return nil, true
	}
	resp := &mcpfake.Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case string(mcp.MethodInitialize):
		resp.Result = map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"serverInfo": map[string]any{
				"name":    "test-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}
		return resp, false
	case string(mcp.MethodToolsList):
		resp.Result = map[string]any{"tools": helperTools(mode)}
		return resp, false
	case string(mcp.MethodToolsCall):
		resp.Result = helperToolResult(mode, req)
		return resp, false
	case "notifications/initialized":
		return nil, false
	default:
		return nil, false
	}
}

func helperTools(mode string) []map[string]any {
	switch mode {
	case "context7-no-tool":
		return []map[string]any{{
			"name": "resolve-library-id",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"libraryName": map[string]any{"type": "string"}},
			},
		}}
	case "context7-resolve-error":
		return []map[string]any{
			{
				"name": "resolve-library-id",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"libraryName": map[string]any{"type": "string"}},
				},
			},
			{
				"name": "get-library-docs",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"libraryID": map[string]any{"type": "string"},
						"query":     map[string]any{"type": "string"},
					},
				},
			},
		}
	case "context7-success", "context7-docs-results-noise":
		return []map[string]any{
			{
				"name": "resolve-library-id",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"libraryName": map[string]any{"type": "string"}},
				},
			},
			{
				"name": "get-library-docs",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"context7CompatibleLibraryID": map[string]any{"type": "string"},
						"topic":                       map[string]any{"type": "string"},
						"tokens":                      map[string]any{"type": "integer"},
					},
				},
			},
		}
	case "context7-query-docs":
		return []map[string]any{
			{
				"name": "resolve-library-id",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"libraryName": map[string]any{"type": "string"}},
				},
			},
			{
				"name": "query-docs",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		}
	case "web-search-success", "web-search-mixed-trust", "web-search-community-version", "web-search-items-shape":
		return []map[string]any{{
			"name": "search",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "integer"},
				},
			},
		}}
	case "list-tools-mismatch":
		return []map[string]any{{
			"name": "lookup",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"query": map[string]any{"type": "string"}},
			},
		}}
	case "partial-evidence":
		return []map[string]any{{
			"name": "search",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "integer"},
				},
			},
		}}
	default:
		return []map[string]any{}
	}
}

func helperToolResult(mode string, req mcpfake.Request) map[string]any {
	toolName, args := mcpfake.ToolCall(req)
	switch mode {
	case "context7-resolve-error":
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "lookup failed"}},
			"isError": true,
		}
	case "context7-query-docs":
		switch toolName {
		case "resolve-library-id":
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "lookup failed"}},
				"isError": true,
			}
		case "query-docs":
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "MCP Go lets Go programs expose MCP servers and clients."}},
				"structuredContent": map[string]any{
					"url":         "https://context7.com/github.com/mark3labs/mcp-go",
					"title":       "github.com/mark3labs/mcp-go docs",
					"description": "MCP Go lets Go programs expose MCP servers and clients.",
				},
			}
		}
	case "context7-docs-results-noise":
		switch toolName {
		case "resolve-library-id":
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "Resolved github.com/mark3labs/mcp-go"}},
				"structuredContent": map[string]any{
					"libraryId": "github.com/mark3labs/mcp-go",
					"title":     "github.com/mark3labs/mcp-go",
				},
			}
		case "get-library-docs":
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "MCP Go is a Go SDK for MCP."}},
				"structuredContent": map[string]any{
					"url":     "https://pkg.go.dev/github.com/mark3labs/mcp-go",
					"title":   "github.com/mark3labs/mcp-go",
					"excerpt": "MCP Go is a Go SDK for MCP.",
					"results": []map[string]any{{
						"title":   "Wrong result",
						"url":     "https://example.invalid/wrong",
						"excerpt": "Noise from a generic results key.",
					}},
				},
			}
		}
	case "context7-success":
		switch toolName {
		case "resolve-library-id":
			if mcpfake.StringArg(args, "libraryName") != "github.com/mark3labs/mcp-go" {
				return map[string]any{"content": []map[string]any{{"type": "text", "text": "bad library query"}}, "isError": true}
			}
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "Resolved github.com/mark3labs/mcp-go"}},
				"structuredContent": map[string]any{
					"libraryId": "github.com/mark3labs/mcp-go",
					"title":     "github.com/mark3labs/mcp-go",
				},
			}
		case "get-library-docs":
			if mcpfake.StringArg(args, "context7CompatibleLibraryID") != "github.com/mark3labs/mcp-go" {
				return map[string]any{"content": []map[string]any{{"type": "text", "text": "missing resolved library id"}}, "isError": true}
			}
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "MCP Go is a Go SDK for MCP."}},
				"structuredContent": map[string]any{
					"url":     "https://pkg.go.dev/github.com/mark3labs/mcp-go",
					"title":   "github.com/mark3labs/mcp-go",
					"excerpt": "MCP Go is a Go SDK for MCP.",
				},
			}
		}
	case "web-search-success":
		if toolName != "search" || mcpfake.StringArg(args, "query") == "" {
			return map[string]any{"content": []map[string]any{{"type": "text", "text": "missing search query"}}, "isError": true}
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Install kubeadm and kubelet from the official Kubernetes docs."}},
			"structuredContent": map[string]any{
				"results": []map[string]any{{
					"title":   "Installing kubeadm",
					"url":     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/",
					"snippet": "Install kubeadm and kubelet from the official Kubernetes docs.",
				}},
			},
		}
	case "web-search-mixed-trust":
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Multiple install guides found."}},
			"structuredContent": map[string]any{
				"results": []map[string]any{
					{
						"title":   "Kubeadm install thread",
						"url":     "https://reddit.com/r/kubernetes/comments/example/kubeadm_install/",
						"snippet": "Community discussion about installing kubeadm.",
					},
					{
						"title":   "Installing kubeadm v1.35.1",
						"url":     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/",
						"snippet": "Official Kubernetes documentation for kubeadm v1.35.1 installation.",
					},
				},
			},
		}
	case "web-search-community-version":
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Community installation notes for kubeadm."}},
			"structuredContent": map[string]any{
				"results": []map[string]any{{
					"title":   "Kubeadm install notes",
					"url":     "https://stackoverflow.com/questions/example/kubeadm-install-notes",
					"snippet": "A user-described kubeadm installation flow without explicit version coverage.",
				}},
			},
		}
	case "web-search-items-shape":
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "Install kubeadm from the official Kubernetes docs."}},
			"structuredContent": map[string]any{
				"items": []map[string]any{{
					"title":   "Installing kubeadm from items",
					"url":     "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/",
					"summary": "Official Kubernetes installation documentation.",
				}},
			},
		}
	case "partial-evidence":
		return map[string]any{
			"content": []map[string]any{{"type": "resource_link", "name": "kubernetes", "uri": "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/"}},
		}
	default:
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
		}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": "unknown tool"}},
		"isError": true,
	}
}
