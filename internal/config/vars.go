package config

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Airgap-Castaways/deck/internal/cloneutil"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func loadBaseVars(ctx context.Context, origin workflowOrigin) (map[string]any, error) {
	if origin.localPath != "" {
		varsPath := localVarsPath(origin)
		root, err := fsutil.NewRoot(filepath.Dir(varsPath))
		if err != nil {
			return nil, err
		}
		b, _, err := root.ReadFile(filepath.Base(varsPath))
		if err != nil {
			if os.IsNotExist(err) {
				return map[string]any{}, nil
			}
			return nil, fmt.Errorf("read vars file: %w", err)
		}
		return parseVarsYAML(b)
	}

	if origin.remoteURL != nil {
		varsURL := remoteVarsURL(origin)
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

func loadSharedVars(ctx context.Context, origin workflowOrigin, baseVars map[string]any, opts LoadOptions) (map[string]any, error) {
	sharedVars := cloneVars(baseVars)
	for _, rawPath := range opts.VarsFiles {
		vars, err := loadVarsFile(ctx, origin, rawPath)
		if err != nil {
			return nil, err
		}
		mergeVars(sharedVars, vars)
	}
	return sharedVars, nil
}

func loadVarsFile(ctx context.Context, origin workflowOrigin, rawPath string) (map[string]any, error) {
	varsFile, err := normalizeVarsFileRef(rawPath)
	if err != nil {
		return nil, err
	}
	if origin.localPath != "" {
		varsPath := localVarsPath(origin)
		root, err := fsutil.NewRoot(filepath.Dir(varsPath))
		if err != nil {
			return nil, err
		}
		b, _, err := root.ReadFile(filepath.FromSlash(varsFile))
		if err != nil {
			return nil, fmt.Errorf("read vars file %q: %w", rawPath, err)
		}
		return parseVarsYAML(b)
	}
	if origin.remoteURL != nil {
		varsURL := remoteVarsURL(origin)
		varsURL.Path = path.Join(path.Dir(varsURL.Path), varsFile)
		b, err := getRequiredHTTP(ctx, varsURL.String())
		if err != nil {
			return nil, fmt.Errorf("read vars file %q: %w", rawPath, err)
		}
		return parseVarsYAML(b)
	}
	return map[string]any{}, nil
}

func localVarsPath(origin workflowOrigin) string {
	workflowRoot, err := WorkflowRootForPath(origin.localPath)
	if err == nil {
		return filepath.Join(workflowRoot, workspacepaths.WorkflowVarsRel)
	}
	return filepath.Join(filepath.Dir(origin.localPath), workspacepaths.WorkflowVarsRel)
}

func remoteVarsURL(origin workflowOrigin) url.URL {
	workflowRoot, err := remoteWorkflowRoot(origin.remoteURL)
	varsURL := *siblingURL(origin.remoteURL, workspacepaths.WorkflowVarsRel)
	if err == nil {
		varsURL = *workflowRoot
		varsURL.Path = path.Join(varsURL.Path, workspacepaths.WorkflowVarsRel)
	}
	return varsURL
}

func normalizeVarsFileRef(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/"))
	if trimmed == "" {
		return "", fmt.Errorf("vars file path is empty")
	}
	if strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "://") {
		return "", fmt.Errorf("vars file path must be relative to %s/: %s", workspacepaths.WorkflowRootDir, rawPath)
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "" || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("vars file path must stay under %s/: %s", workspacepaths.WorkflowRootDir, rawPath)
	}
	return cleaned, nil
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
		if existing, ok := dst[k]; ok {
			merged, didMerge := mergeVarValue(existing, v)
			if didMerge {
				dst[k] = merged
				continue
			}
		}
		dst[k] = cloneutil.DeepValue(v)
	}
}

func mergeVarValue(dst, src any) (any, bool) {
	srcMap, ok := src.(map[string]any)
	if !ok {
		return nil, false
	}
	dstMap, ok := dst.(map[string]any)
	if !ok {
		return nil, false
	}
	merged := cloneutil.DeepMap(dstMap)
	mergeVars(merged, srcMap)
	return merged, true
}
