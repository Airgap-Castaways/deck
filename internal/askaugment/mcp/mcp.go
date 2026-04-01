package mcpaugment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
)

func Gather(ctx context.Context, cfg askconfig.MCP, route askintent.Route, prompt string) ([]askretrieve.Chunk, []string) {
	if !cfg.Enabled || len(cfg.Servers) == 0 {
		return nil, nil
	}
	chunks := make([]askretrieve.Chunk, 0)
	events := make([]string, 0)
	for _, server := range cfg.Servers {
		resolved, warning, ok := resolveServer(server)
		if warning != "" {
			events = append(events, warning)
		}
		if !ok {
			continue
		}
		chunk, event := queryResolvedServer(ctx, resolved, route, prompt)
		if event != "" {
			events = append(events, event)
		}
		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
	}
	return chunks, events
}

func queryServer(parent context.Context, server askconfig.MCPServer, route askintent.Route, prompt string) (*askretrieve.Chunk, string) {
	resolved, warning, ok := resolveServer(server)
	if warning != "" && !ok {
		return nil, warning
	}
	if !ok {
		return nil, ""
	}
	return queryResolvedServer(parent, resolved, route, prompt)
}

func queryResolvedServer(parent context.Context, server resolvedServer, route askintent.Route, prompt string) (*askretrieve.Chunk, string) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	tr := transport.NewStdio(server.RunCommand, nil, server.Args...)
	c := client.NewClient(tr)
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Sprintf("mcp:%s start failed: %v", server.Profile.ID, err)
	}
	defer func() { _ = c.Close() }()
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "deck-ask", Version: "1.0.0"}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Sprintf("mcp:%s initialize failed: %v", server.Profile.ID, err)
	}
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Sprintf("mcp:%s list tools failed: %v", server.Profile.ID, err)
	}
	if server.Profile.Adapter == nil {
		return nil, fmt.Sprintf("mcp:%s no adapter configured", server.Profile.ID)
	}
	return server.Profile.Adapter.Fetch(ctx, server, c, route, prompt, tools)
}

func sanitize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func summarizeEvidence(content string, prompt string) *askretrieve.EvidenceSummary {
	lower := strings.ToLower(strings.TrimSpace(content + "\n" + prompt))
	artifactKinds := []string{}
	addArtifact := func(kind string) {
		for _, existing := range artifactKinds {
			if existing == kind {
				return
			}
		}
		artifactKinds = append(artifactKinds, kind)
	}
	for _, token := range []string{"rpm", "package", "packages", "dnf", "apt"} {
		if strings.Contains(lower, token) {
			addArtifact("package")
			break
		}
	}
	for _, token := range []string{"image", "images", "registry", "container image"} {
		if strings.Contains(lower, token) {
			addArtifact("image")
			break
		}
	}
	for _, token := range []string{"binary", "tarball", "archive", "bundle"} {
		if strings.Contains(lower, token) {
			addArtifact("binary")
			break
		}
	}
	hints := []string{}
	if strings.Contains(lower, "air-gapped") || strings.Contains(lower, "offline") {
		hints = append(hints, "Treat gathered installation artifacts as offline bundle inputs for prepare before apply.")
	}
	if len(artifactKinds) == 0 && len(hints) == 0 {
		return nil
	}
	return &askretrieve.EvidenceSummary{ArtifactKinds: artifactKinds, OfflineHints: hints}
}

func renderEvidence(evidence askretrieve.EvidenceSummary) string {
	raw, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return "Typed MCP evidence JSON:\n{}"
	}
	return "Typed MCP evidence JSON:\n" + string(raw)
}
