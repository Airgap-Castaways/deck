package mcpaugment

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
)

type ProviderHealth struct {
	ConfiguredName      string
	ProviderID          string
	Transport           string
	TransportSource     string
	Status              string
	Phase               string
	Message             string
	Tools               []string
	Capabilities        []string
	MissingCapabilities []string
}

func ProbeConfiguredProviders(ctx context.Context, cfg askconfig.MCP) []ProviderHealth {
	out := make([]ProviderHealth, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		out = append(out, probeServer(ctx, server))
	}
	return out
}

func probeServer(parent context.Context, server askconfig.MCPServer) ProviderHealth {
	resolved, warning, ok := resolveServer(server)
	if !ok {
		return ProviderHealth{
			ConfiguredName: strings.TrimSpace(server.Name),
			Status:         "skipped",
			Phase:          "resolve",
			Message:        warning,
		}
	}
	health := ProviderHealth{
		ConfiguredName:  resolved.ConfiguredName,
		ProviderID:      resolved.Profile.ID,
		Transport:       resolved.TransportDisplay,
		TransportSource: resolved.TransportSource,
		Capabilities:    capabilityStrings(resolved.Profile.Capabilities),
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	tr := transport.NewStdio(resolved.RunCommand, nil, resolved.Args...)
	c := client.NewClient(tr)
	if err := c.Start(ctx); err != nil {
		health.Status = "error"
		health.Phase = "start"
		health.Message = err.Error()
		return health
	}
	defer func() { _ = c.Close() }()
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "deck-ask-health", Version: "1.0.0"}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		health.Status = "error"
		health.Phase = "initialize"
		health.Message = err.Error()
		return health
	}
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		health.Status = "error"
		health.Phase = "list"
		health.Message = err.Error()
		return health
	}
	health.Tools = toolNames(tools)
	missing := missingCapabilities(resolved.Profile, tools)
	if len(missing) > 0 {
		health.Status = "error"
		health.Phase = "capability"
		health.MissingCapabilities = capabilityStrings(missing)
		health.Message = fmt.Sprintf("missing tools for capabilities: %s", strings.Join(health.MissingCapabilities, ","))
		return health
	}
	health.Status = "healthy"
	health.Phase = "ready"
	health.Message = "ok"
	return health
}

func toolNames(tools *mcp.ListToolsResult) []string {
	if tools == nil {
		return nil
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		if trimmed := strings.TrimSpace(tool.Name); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	sort.Strings(names)
	return names
}

func missingCapabilities(profile providerProfile, tools *mcp.ListToolsResult) []capability {
	available := map[string]bool{}
	for _, name := range toolNames(tools) {
		available[strings.ToLower(strings.TrimSpace(name))] = true
	}
	missing := make([]capability, 0)
	for _, capability := range profile.Capabilities {
		required := capabilityToolNames(profile.ID, capability)
		if len(required) == 0 {
			continue
		}
		matched := false
		for _, candidate := range required {
			if available[strings.ToLower(strings.TrimSpace(candidate))] {
				matched = true
				break
			}
		}
		if !matched {
			missing = append(missing, capability)
		}
	}
	return missing
}

func capabilityToolNames(providerID string, value capability) []string {
	switch value {
	case capabilityEntityResolve:
		return []string{"resolve-library-id"}
	case capabilityDocFetch:
		return []string{"get-library-docs", "query-docs"}
	case capabilityOfficialDocSearch:
		if strings.TrimSpace(providerID) == "context7" {
			return []string{"get-library-docs", "query-docs"}
		}
		return []string{"search", "web-search", "web_search"}
	case capabilityWebSearch, capabilityErrorLookup:
		return []string{"search", "web-search", "web_search"}
	default:
		return nil
	}
}
