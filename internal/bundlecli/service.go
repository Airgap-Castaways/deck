package bundlecli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/bundle"
	"github.com/Airgap-Castaways/deck/internal/logs"
)

type VerifyOptions struct {
	FilePath       string
	PositionalArgs []string
	Output         string
	Verbosef       func(level int, format string, args ...any) error
	JSONEncoder    func(any) error
	StdoutPrintf   func(format string, args ...any) error
}

type BuildOptions struct {
	Root         string
	Out          string
	Verbosef     func(level int, format string, args ...any) error
	StdoutPrintf func(format string, args ...any) error
}

type verifyReport struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type manifestSummary struct {
	Entries  int
	Files    int
	Images   int
	Packages int
	Other    int
}

func Verify(opts VerifyOptions) error {
	resolvedPath, err := resolveBundlePathArg(opts.FilePath, opts.PositionalArgs, "bundle verify accepts a single <path>")
	if err != nil {
		return err
	}
	if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "bundle", Event: "verify_requested", Attrs: map[string]any{"path": resolvedPath}}); err != nil {
		return err
	}
	if err := bundle.VerifyManifest(resolvedPath); err != nil {
		_ = verboseEvent(opts.Verbosef, 2, logs.CLIEvent{Level: "debug", Component: "bundle", Event: "verify_failed", Attrs: map[string]any{"error": err}})
		return err
	}
	entries, err := bundle.InspectManifest(resolvedPath)
	if err != nil {
		return err
	}
	summary := summarizeBundleManifest(entries)
	if err := verboseEvent(opts.Verbosef, 2, logs.CLIEvent{Level: "debug", Component: "bundle", Event: "verify_manifest", Attrs: map[string]any{"manifest_entries": summary.Entries, "files": summary.Files, "images": summary.Images, "packages": summary.Packages, "other": summary.Other}}); err != nil {
		return err
	}
	report := verifyReport{Status: "ok", Path: resolvedPath}
	if strings.TrimSpace(opts.Output) == "json" {
		if opts.JSONEncoder == nil {
			return nil
		}
		return opts.JSONEncoder(report)
	}
	if opts.StdoutPrintf == nil {
		return nil
	}
	return opts.StdoutPrintf("bundle verify: ok (%s)\n", report.Path)
}

func Build(opts BuildOptions) error {
	resolvedRoot := strings.TrimSpace(opts.Root)
	if resolvedRoot == "" {
		resolvedRoot = "."
	}
	if strings.TrimSpace(opts.Out) == "" {
		return errors.New("--out is required")
	}
	if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "bundle", Event: "build_requested", Attrs: map[string]any{"root": resolvedRoot, "out": strings.TrimSpace(opts.Out)}}); err != nil {
		return err
	}
	manifestPath := filepath.Join(resolvedRoot, ".deck", "manifest.json")
	entries, err := bundle.InspectManifest(resolvedRoot)
	if err != nil {
		if err := verboseEvent(opts.Verbosef, 2, logs.CLIEvent{Level: "debug", Component: "bundle", Event: "manifest_inspect_failed", Attrs: map[string]any{"error": err}}); err != nil {
			return err
		}
	} else {
		summary := summarizeBundleManifest(entries)
		if err := verboseEvent(opts.Verbosef, 1, logs.CLIEvent{Component: "bundle", Event: "manifest_loaded", Attrs: map[string]any{"manifest": manifestPath, "entries": summary.Entries}}); err != nil {
			return err
		}
		if err := verboseEvent(opts.Verbosef, 2, logs.CLIEvent{Level: "debug", Component: "bundle", Event: "manifest_summary", Attrs: map[string]any{"files": summary.Files, "images": summary.Images, "packages": summary.Packages, "other": summary.Other}}); err != nil {
			return err
		}
	}
	if err := bundle.CollectArchive(resolvedRoot, opts.Out); err != nil {
		return err
	}
	if info, err := os.Stat(opts.Out); err == nil {
		if err := verboseEvent(opts.Verbosef, 2, logs.CLIEvent{Level: "debug", Component: "bundle", Event: "archive_written", Attrs: map[string]any{"archive_size": info.Size()}}); err != nil {
			return err
		}
	}
	if opts.StdoutPrintf == nil {
		return nil
	}
	return opts.StdoutPrintf("bundle build: ok (%s -> %s)\n", resolvedRoot, opts.Out)
}

func resolveBundlePathArg(filePath string, positionalArgs []string, tooManyArgsErr string) (string, error) {
	if len(positionalArgs) > 1 {
		return "", errors.New(tooManyArgsErr)
	}
	resolvedPath := strings.TrimSpace(filePath)
	if resolvedPath == "" && len(positionalArgs) == 1 {
		resolvedPath = strings.TrimSpace(positionalArgs[0])
	}
	if resolvedPath == "" {
		return "", errors.New("bundle path is required")
	}
	return resolvedPath, nil
}

func summarizeBundleManifest(entries []bundle.ManifestEntry) manifestSummary {
	summary := manifestSummary{Entries: len(entries)}
	for _, entry := range entries {
		path := strings.TrimSpace(entry.Path)
		switch {
		case strings.HasPrefix(path, "outputs/files/") || strings.HasPrefix(path, "files/"):
			summary.Files++
		case strings.HasPrefix(path, "outputs/images/") || strings.HasPrefix(path, "images/"):
			summary.Images++
		case strings.HasPrefix(path, "outputs/packages/") || strings.HasPrefix(path, "packages/"):
			summary.Packages++
		default:
			summary.Other++
		}
	}
	return summary
}

func verboseEvent(fn func(level int, format string, args ...any) error, level int, event logs.CLIEvent) error {
	return logs.EmitCLIEventf(fn, level, event)
}
