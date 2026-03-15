package config

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func loadBaseVars(ctx context.Context, origin workflowOrigin) (map[string]any, error) {
	if origin.localPath != "" {
		workflowRoot, err := localWorkflowRoot(origin.localPath)
		varsPath := ""
		if err == nil {
			varsPath = filepath.Join(workflowRoot, "vars.yaml")
		} else {
			varsPath = filepath.Join(filepath.Dir(origin.localPath), "vars.yaml")
		}
		b, err := os.ReadFile(varsPath)
		if err != nil {
			if os.IsNotExist(err) {
				return map[string]any{}, nil
			}
			return nil, fmt.Errorf("read vars file: %w", err)
		}
		return parseVarsYAML(b)
	}

	if origin.remoteURL != nil {
		workflowRoot, err := remoteWorkflowRoot(origin.remoteURL)
		varsURL := *siblingURL(origin.remoteURL, "vars.yaml")
		if err == nil {
			varsURL = *workflowRoot
			varsURL.Path = path.Join(varsURL.Path, "vars.yaml")
		}
		b, ok, err := getOptionalHTTP(ctx, varsURL.String())
		if err != nil {
			return nil, err
		}
		if !ok {
			return map[string]any{}, nil
		}
		return parseVarsYAML(b)
	}

	return map[string]any{}, nil
}

func parseVarsYAML(content []byte) (map[string]any, error) {
	if len(content) == 0 {
		return map[string]any{}, nil
	}

	vars := map[string]any{}
	if err := yaml.Unmarshal(content, &vars); err != nil {
		return nil, fmt.Errorf("parse vars yaml: %w", err)
	}
	if vars == nil {
		return map[string]any{}, nil
	}
	return vars, nil
}

func mergeVars(dst map[string]any, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}
