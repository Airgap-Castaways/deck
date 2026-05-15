package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/cloneutil"
)

const (
	nodeScopedAllKey   = "all"
	nodeScopedHostsKey = "hosts"
)

func resolveNodeScopedVars(baseVars map[string]any, opts LoadOptions) (map[string]any, map[string]any, map[string]any, bool, error) {
	hostsRaw, hasHosts := baseVars[nodeScopedHostsKey]
	if !hasHosts {
		return cloneVars(baseVars), nil, nil, false, nil
	}
	hosts, ok := asStringAnyMap(hostsRaw)
	if !ok {
		return nil, nil, nil, false, fmt.Errorf("vars.%s must be a map when node-scoped vars are enabled", nodeScopedHostsKey)
	}

	ordinary := make(map[string]any, len(baseVars))
	for key, value := range baseVars {
		if key == nodeScopedAllKey || key == nodeScopedHostsKey {
			continue
		}
		ordinary[key] = cloneutil.DeepValue(value)
	}

	allVars := map[string]any(nil)
	if allRaw, ok := baseVars[nodeScopedAllKey]; ok {
		allMap, ok := asStringAnyMap(allRaw)
		if !ok {
			return nil, nil, nil, false, fmt.Errorf("vars.%s must be a map when node-scoped vars are enabled", nodeScopedAllKey)
		}
		allVars = cloneVars(allMap)
	}

	hostname, err := resolveNodeScopedHostname(opts)
	if err != nil {
		return nil, nil, nil, false, err
	}
	hostOverlay := map[string]any(nil)
	if hostKey, hostRaw, ok := selectHostOverlay(hosts, hostname); ok {
		hostVars, ok := asStringAnyMap(hostRaw)
		if !ok {
			return nil, nil, nil, false, fmt.Errorf("vars.%s.%s must be a map", nodeScopedHostsKey, hostKey)
		}
		hostOverlay = cloneVars(hostVars)
	}

	return ordinary, allVars, hostOverlay, true, nil
}

func resolveNodeScopedHostname(opts LoadOptions) (string, error) {
	if hostname := strings.TrimSpace(opts.Hostname); hostname != "" {
		return hostname, nil
	}
	detect := opts.DetectHostname
	if detect == nil {
		detect = os.Hostname
	}
	hostname, err := detect()
	if err != nil {
		return "", fmt.Errorf("detect hostname for node-scoped vars: %w", err)
	}
	return strings.TrimSpace(hostname), nil
}

func selectHostOverlay(hosts map[string]any, hostname string) (string, any, bool) {
	trimmed := strings.TrimSpace(hostname)
	if trimmed == "" {
		return "", nil, false
	}
	if value, ok := hosts[trimmed]; ok {
		return trimmed, value, true
	}
	short := shortHostname(trimmed)
	if short != trimmed {
		if value, ok := hosts[short]; ok {
			return short, value, true
		}
	}
	return "", nil, false
}

func shortHostname(hostname string) string {
	trimmed := strings.TrimSpace(hostname)
	if idx := strings.Index(trimmed, "."); idx > 0 {
		return trimmed[:idx]
	}
	return trimmed
}

func asStringAnyMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	return typed, ok
}

func cloneVars(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	return cloneutil.DeepMap(input)
}
