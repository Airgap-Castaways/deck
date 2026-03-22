package workflowcontract

import (
	"fmt"
	"sort"
	"strings"
)

const (
	SupportedWorkflowVersion = "v1alpha1"
	BuiltInStepAPIVersion    = "deck/v1alpha1"
)

var workflowToStepAPIVersion = map[string]string{
	SupportedWorkflowVersion: BuiltInStepAPIVersion,
}

func SupportedWorkflowVersions() []string {
	out := make([]string, 0, len(workflowToStepAPIVersion))
	for version := range workflowToStepAPIVersion {
		out = append(out, version)
	}
	sort.Strings(out)
	return out
}

func DefaultStepAPIVersionForWorkflowVersion(version string) (string, bool) {
	apiVersion, ok := workflowToStepAPIVersion[strings.TrimSpace(version)]
	return apiVersion, ok
}

func IsSupportedStepAPIVersion(apiVersion string) bool {
	return strings.TrimSpace(apiVersion) == BuiltInStepAPIVersion
}

func ResolveStepAPIVersion(workflowVersion, stepAPIVersion string) (string, error) {
	if trimmed := strings.TrimSpace(stepAPIVersion); trimmed != "" {
		if !IsSupportedStepAPIVersion(trimmed) {
			return "", fmt.Errorf("unsupported step apiVersion: %s", trimmed)
		}
		return trimmed, nil
	}
	resolved, ok := DefaultStepAPIVersionForWorkflowVersion(workflowVersion)
	if !ok {
		return "", fmt.Errorf("unsupported workflow version: %s", strings.TrimSpace(workflowVersion))
	}
	return resolved, nil
}
