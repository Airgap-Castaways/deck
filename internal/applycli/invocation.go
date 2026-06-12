package applycli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InvocationOptions struct {
	WorkflowPath     string
	Scenario         string
	Source           string
	Root             string
	Server           string
	PositionalArgs   []string
	ResolveScenario  func(source, scenario, localRoot, server string) (string, error)
	DefaultLocalRoot string
}

func ResolvePlanWorkflowPath(ctx context.Context, opts InvocationOptions) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context is nil")
	}
	resolvedWorkflow := strings.TrimSpace(opts.WorkflowPath)
	resolvedScenario := strings.TrimSpace(opts.Scenario)
	resolvedSource := strings.TrimSpace(opts.Source)
	server := strings.TrimRight(strings.TrimSpace(opts.Server), "/")
	localRoot := strings.TrimSpace(opts.DefaultLocalRoot)
	if strings.TrimSpace(opts.Root) != "" {
		localRoot = strings.TrimSpace(opts.Root)
	}
	if localRoot == "" {
		localRoot = "."
	}
	if err := validateSourceLocator(strings.TrimSpace(opts.Root), server); err != nil {
		return "", err
	}
	if server != "" {
		resolvedSource = "server"
	} else if strings.TrimSpace(opts.Root) != "" {
		resolvedSource = "local"
	}
	if resolvedWorkflow != "" && resolvedScenario != "" {
		return "", fmt.Errorf("plan accepts either --workflow or --scenario, not both")
	}
	if resolvedWorkflow != "" {
		return resolvedWorkflow, nil
	}
	if resolvedScenario != "" {
		if opts.ResolveScenario == nil {
			return "", fmt.Errorf("scenario resolver is nil")
		}
		return opts.ResolveScenario(resolvedSource, resolvedScenario, localRoot, server)
	}
	if server != "" {
		return "", fmt.Errorf("plan with --server requires --scenario or --workflow")
	}
	if resolvedSource == "server" {
		return "", fmt.Errorf("plan with --source server requires --scenario or --workflow")
	}
	return DiscoverApplyWorkflow(ctx, localRoot)
}

func ResolveApplyWorkflowAndBundle(ctx context.Context, opts InvocationOptions) (string, string, error) {
	if ctx == nil {
		return "", "", fmt.Errorf("context is nil")
	}
	resolvedWorkflow := strings.TrimSpace(opts.WorkflowPath)
	resolvedScenario := strings.TrimSpace(opts.Scenario)
	resolvedSource := strings.TrimSpace(opts.Source)
	server := strings.TrimRight(strings.TrimSpace(opts.Server), "/")
	root := strings.TrimSpace(opts.Root)
	positionalWorkflow, positionalBundle, err := parseApplyPositionals(opts.PositionalArgs)
	if err != nil {
		return "", "", err
	}
	if err := validateSourceLocator(root, server); err != nil {
		return "", "", err
	}
	if root != "" && strings.TrimSpace(positionalBundle) != "" {
		return "", "", fmt.Errorf("apply accepts either --root or a positional bundle path, not both")
	}
	if server != "" {
		resolvedSource = "server"
	} else if root != "" {
		resolvedSource = "local"
	}
	if resolvedWorkflow != "" && resolvedScenario != "" {
		return "", "", fmt.Errorf("apply accepts either --workflow or --scenario, not both")
	}
	if resolvedWorkflow != "" && positionalWorkflow != "" {
		return "", "", fmt.Errorf("apply accepts at most one workflow reference")
	}
	if resolvedWorkflow == "" && resolvedScenario == "" && resolvedSource == "server" {
		if server != "" {
			return "", "", fmt.Errorf("apply with --server requires --scenario or --workflow")
		}
		return "", "", fmt.Errorf("apply with --source server requires --scenario or --workflow")
	}
	if (resolvedWorkflow != "" || resolvedScenario != "") && len(opts.PositionalArgs) > 1 {
		return "", "", fmt.Errorf("apply accepts at most one positional bundle path when --workflow or --scenario is set")
	}

	if resolvedWorkflow == "" {
		resolvedWorkflow = positionalWorkflow
	}
	if resolvedWorkflow != "" {
		bundleRoot := ""
		if strings.TrimSpace(positionalBundle) != "" {
			bundleRoot, err = ResolveBundleRoot(positionalBundle)
			if err != nil {
				if !IsHTTPWorkflowPath(resolvedWorkflow) {
					return "", "", err
				}
				bundleRoot = ""
			}
		}
		return resolvedWorkflow, bundleRoot, nil
	}

	if resolvedSource == "server" && resolvedScenario != "" {
		bundleRoot := ""
		if strings.TrimSpace(positionalBundle) != "" {
			bundleRoot, err = ResolveBundleRoot(positionalBundle)
			if err != nil {
				return "", "", err
			}
		}
		if opts.ResolveScenario == nil {
			return "", "", fmt.Errorf("scenario resolver is nil")
		}
		workflowPath, err := opts.ResolveScenario(resolvedSource, resolvedScenario, ".", server)
		if err != nil {
			return "", "", err
		}
		return workflowPath, bundleRoot, nil
	}

	bundleRoot := ""
	if root != "" {
		bundleRoot, err = ResolveBundleRoot(root)
		if err != nil {
			return "", "", err
		}
	} else {
		bundleRoot, err = ResolveBundleRoot(positionalBundle)
		if err != nil {
			return "", "", err
		}
	}
	if resolvedScenario != "" {
		if opts.ResolveScenario == nil {
			return "", "", fmt.Errorf("scenario resolver is nil")
		}
		localRoot := "."
		if resolvedSource == "local" {
			localRoot = bundleRoot
		}
		workflowPath, err := opts.ResolveScenario(resolvedSource, resolvedScenario, localRoot, server)
		if err != nil {
			return "", "", err
		}
		return workflowPath, bundleRoot, nil
	}
	workflowPath, err := DiscoverApplyWorkflow(ctx, bundleRoot)
	if err != nil {
		return "", "", err
	}
	return workflowPath, bundleRoot, nil
}

func validateSourceLocator(root string, server string) error {
	if strings.TrimSpace(root) != "" && strings.TrimSpace(server) != "" {
		return fmt.Errorf("--root and --server are mutually exclusive")
	}
	return nil
}

func parseApplyPositionals(positionalArgs []string) (string, string, error) {
	positionalWorkflow := ""
	positionalBundle := ""
	if len(positionalArgs) == 1 {
		arg0 := strings.TrimSpace(positionalArgs[0])
		if IsHTTPWorkflowPath(arg0) || looksLikeWorkflowArgument(arg0) {
			positionalWorkflow = arg0
		} else {
			positionalBundle = arg0
		}
	}
	if len(positionalArgs) == 2 {
		arg0 := strings.TrimSpace(positionalArgs[0])
		arg1 := strings.TrimSpace(positionalArgs[1])
		if !IsHTTPWorkflowPath(arg0) && !looksLikeWorkflowArgument(arg0) {
			return "", "", fmt.Errorf("apply with two positional arguments requires [workflow] [bundle]")
		}
		positionalWorkflow = arg0
		positionalBundle = arg1
	}
	return positionalWorkflow, positionalBundle, nil
}

func looksLikeWorkflowArgument(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return true
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return false
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
