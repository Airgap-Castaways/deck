package mcpaugment

import (
	"fmt"
	"os"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
)

type capability string

const (
	capabilityEntityResolve     capability = "entity-resolve"
	capabilityOfficialDocSearch capability = "official-doc-search"
	capabilityDocFetch          capability = "doc-fetch"
	capabilityWebSearch         capability = "web-search"
	capabilityErrorLookup       capability = "error-lookup"
)

type transportCandidate struct {
	RunCommand string
	Args       []string
	Display    string
	Resolve    func() (transportCandidate, error)
}

type providerProfile struct {
	ID                string
	LegacyAliases     []string
	DefaultTransports []transportCandidate
	Capabilities      []capability
	Adapter           providerAdapter
}

type resolvedServer struct {
	Profile          providerProfile
	ConfiguredName   string
	RunCommand       string
	Args             []string
	TransportDisplay string
	TransportSource  string
}

func builtInProviderRegistry() map[string]providerProfile {
	return map[string]providerProfile{
		"context7": {
			ID:                "context7",
			DefaultTransports: []transportCandidate{{RunCommand: "npx", Args: []string{"-y", "@upstash/context7-mcp@latest"}, Display: "npx -y @upstash/context7-mcp@latest"}},
			Capabilities:      []capability{capabilityEntityResolve, capabilityDocFetch, capabilityOfficialDocSearch},
			Adapter:           context7ProviderAdapter{},
		},
		"web-search": {
			ID:                "web-search",
			LegacyAliases:     []string{"web-server"},
			DefaultTransports: []transportCandidate{{Resolve: resolveDeckBuiltInWebSearchTransport}},
			Capabilities:      []capability{capabilityOfficialDocSearch, capabilityWebSearch, capabilityErrorLookup},
			Adapter:           webSearchProviderAdapter{},
		},
	}
}

func lookupProviderProfile(name string) (providerProfile, bool) {
	registry := builtInProviderRegistry()
	profile, ok := registry[askconfig.NormalizeMCPProviderName(name)]
	return profile, ok
}

func resolveServer(server askconfig.MCPServer) (resolvedServer, string, bool) {
	name := strings.TrimSpace(server.Name)
	profile, ok := lookupProviderProfile(name)
	if !ok {
		if name == "" {
			name = "<unnamed>"
		}
		return resolvedServer{}, fmt.Sprintf("mcp:%s unknown built-in provider; ignoring config entry", name), false
	}
	transport := transportCandidate{}
	transportSource := "config-override"
	if strings.TrimSpace(server.RunCommand) != "" {
		transport.RunCommand = strings.TrimSpace(server.RunCommand)
		transport.Args = append([]string(nil), server.Args...)
		transport.Display = formatTransportDisplay(transport.RunCommand, transport.Args)
	} else if len(profile.DefaultTransports) > 0 {
		resolvedTransport, err := resolveTransport(profile.DefaultTransports[0])
		if err != nil {
			return resolvedServer{}, fmt.Sprintf("mcp:%s resolve transport failed: %v", profile.ID, err), false
		}
		transport = resolvedTransport
		transportSource = "built-in-default"
	}
	if transport.RunCommand == "" {
		return resolvedServer{}, fmt.Sprintf("mcp:%s has no configured transport", profile.ID), false
	}
	if transport.Display == "" {
		transport.Display = formatTransportDisplay(transport.RunCommand, transport.Args)
	}
	return resolvedServer{Profile: profile, ConfiguredName: strings.TrimSpace(server.Name), RunCommand: transport.RunCommand, Args: transport.Args, TransportDisplay: transport.Display, TransportSource: transportSource}, "", true
}

func resolveTransport(candidate transportCandidate) (transportCandidate, error) {
	if candidate.Resolve != nil {
		resolved, err := candidate.Resolve()
		if err != nil {
			return transportCandidate{}, err
		}
		if resolved.Display == "" {
			resolved.Display = formatTransportDisplay(resolved.RunCommand, resolved.Args)
		}
		return resolved, nil
	}
	if candidate.Display == "" {
		candidate.Display = formatTransportDisplay(candidate.RunCommand, candidate.Args)
	}
	return candidate, nil
}

func resolveDeckBuiltInWebSearchTransport() (transportCandidate, error) {
	if testCommand := strings.TrimSpace(os.Getenv("DECK_TEST_MCP_WEB_SEARCH_COMMAND")); testCommand != "" {
		return transportCandidate{RunCommand: testCommand, Args: strings.Fields(os.Getenv("DECK_TEST_MCP_WEB_SEARCH_ARGS")), Display: "deck ask mcp web-search"}, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return transportCandidate{}, fmt.Errorf("resolve current executable: %w", err)
	}
	return transportCandidate{RunCommand: exe, Args: []string{"ask", "mcp", "web-search"}, Display: "deck ask mcp web-search"}, nil
}

func formatTransportDisplay(command string, args []string) string {
	parts := []string{strings.TrimSpace(command)}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
