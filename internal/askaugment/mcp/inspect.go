package mcpaugment

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
)

type ProviderDescriptor struct {
	ConfiguredName  string
	ProviderID      string
	Transport       string
	TransportSource string
	Capabilities    []string
	Warning         string
}

func DescribeConfiguredProviders(cfg askconfig.MCP) []ProviderDescriptor {
	providers := make([]ProviderDescriptor, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		resolved, warning, ok := resolveServer(server)
		descriptor := ProviderDescriptor{ConfiguredName: strings.TrimSpace(server.Name), Warning: warning}
		if ok {
			descriptor.ProviderID = resolved.Profile.ID
			descriptor.Transport = resolved.TransportDisplay
			descriptor.TransportSource = resolved.TransportSource
			descriptor.Capabilities = capabilityStrings(resolved.Profile.Capabilities)
		}
		providers = append(providers, descriptor)
	}
	return providers
}

func capabilityStrings(values []capability) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(string(value))
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
