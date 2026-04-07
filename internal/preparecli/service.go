package preparecli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
	"github.com/Airgap-Castaways/deck/internal/hostfs"
	"github.com/Airgap-Castaways/deck/internal/logs"
	"github.com/Airgap-Castaways/deck/internal/prepare"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type Options struct {
	PreparedRoot      string
	DryRun            bool
	Refresh           bool
	Clean             bool
	BinarySource      string
	BinaryDir         string
	BinaryVer         string
	Binaries          []string
	BinaryExcludes    []string
	VarOverrides      map[string]any
	Stdout            io.Writer
	Diagnosticf       func(level int, format string, args ...any) error
	EventSink         prepare.StepEventSink
	runtimeBinaryDeps runtimeBinaryDeps
}

type preparedManifest struct {
	Entries []preparedManifestEntry `json:"entries"`
}

type preparedManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func Run(ctx context.Context, opts Options) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	prepareWorkflowPath, err := discoverPrepareWorkflow(ctx)
	if err != nil {
		return err
	}
	if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "workflow_selected", Attrs: map[string]any{"workflow": filepath.ToSlash(prepareWorkflowPath)}}); err != nil {
		return err
	}
	workflowRootDirPath := filepath.Dir(prepareWorkflowPath)
	varsWorkflowPath, err := resolveOptionalVarsWorkflowPath(workflowRootDirPath)
	if err != nil {
		return err
	}
	if varsWorkflowPath != "" {
		if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "vars_selected", Attrs: map[string]any{"vars": filepath.ToSlash(varsWorkflowPath)}}); err != nil {
			return err
		}
	}
	applyWorkflowPath, err := resolveOptionalApplyWorkflowPath(workflowRootDirPath)
	if err != nil {
		return err
	}
	if applyWorkflowPath != "" {
		if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "apply_selected", Attrs: map[string]any{"apply": filepath.ToSlash(applyWorkflowPath)}}); err != nil {
			return err
		}
	}
	resolvedPreparedRoot := strings.TrimSpace(opts.PreparedRoot)
	if resolvedPreparedRoot == "" {
		resolvedPreparedRoot = workspacepaths.DefaultPreparedRoot(".")
	}
	resolvedPreparedRootAbs, err := filepath.Abs(resolvedPreparedRoot)
	if err != nil {
		return fmt.Errorf("resolve --root: %w", err)
	}
	if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "prepared_root", Attrs: map[string]any{"prepared_root": filepath.ToSlash(resolvedPreparedRootAbs)}}); err != nil {
		return err
	}
	preparedRoot, err := fsutil.NewPreparedRoot(resolvedPreparedRootAbs)
	if err != nil {
		return err
	}
	preparedHostPath, err := hostfs.NewHostPath(preparedRoot.Abs())
	if err != nil {
		return err
	}
	prepareWorkflow, err := config.LoadWithOptions(ctx, prepareWorkflowPath, config.LoadOptions{VarOverrides: opts.VarOverrides})
	if err != nil {
		return err
	}
	if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "run_config", Attrs: map[string]any{"refresh": opts.Refresh, "clean": opts.Clean}}); err != nil {
		return err
	}
	planDiagnostics, err := prepare.InspectPlan(prepareWorkflow, preparedRoot.Abs(), prepare.RunOptions{BundleRoot: preparedRoot.Abs(), ForceRedownload: opts.Refresh})
	if err != nil {
		return err
	}
	if len(planDiagnostics.CachePlan.Artifact) > 0 {
		fetchCount := 0
		reuseCount := 0
		for _, artifact := range planDiagnostics.CachePlan.Artifact {
			switch strings.TrimSpace(artifact.Action) {
			case "REUSE":
				reuseCount++
			default:
				fetchCount++
			}
			if err := emitDiagnosticEvent(opts, 2, logs.CLIEvent{Level: "debug", Component: "prepare", Event: "cache_artifact", Attrs: map[string]any{"step": artifact.StepID, "type": artifact.Type, "action": artifact.Action}}); err != nil {
				return err
			}
		}
		if err := emitDiagnosticEvent(opts, 2, logs.CLIEvent{Level: "debug", Component: "prepare", Event: "cache_plan", Attrs: map[string]any{"fetch": fetchCount, "reuse": reuseCount}}); err != nil {
			return err
		}
	}

	if opts.DryRun {
		runtimeWrites, err := dryRunRuntimeBinaryWrites(preparedRoot.Abs(), opts)
		if err != nil {
			return err
		}
		if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "dry_run", Attrs: map[string]any{"outputs_root": filepath.ToSlash(preparedRoot.Abs())}}); err != nil {
			return err
		}
		if err := emitDiagnosticEvent(opts, 2, logs.CLIEvent{Level: "debug", Component: "prepare", Event: "workflow_includes", Attrs: map[string]any{"count": workflowIncludeCount(prepareWorkflowPath, varsWorkflowPath, applyWorkflowPath)}}); err != nil {
			return err
		}
		for _, line := range []string{
			fmt.Sprintf("PREPARE_WORKFLOW=%s", filepath.ToSlash(prepareWorkflowPath)),
			fmt.Sprintf("WORKFLOW_INCLUDE=%s", filepath.ToSlash(prepareWorkflowPath)),
			fmt.Sprintf("PREPARED_ROOT=%s", filepath.ToSlash(preparedRoot.Abs())),
			fmt.Sprintf("WRITE=%s", filepath.ToSlash(filepath.Join(preparedRoot.Abs(), "packages"))),
			fmt.Sprintf("WRITE=%s", filepath.ToSlash(filepath.Join(preparedRoot.Abs(), "images"))),
			fmt.Sprintf("WRITE=%s", filepath.ToSlash(filepath.Join(preparedRoot.Abs(), "files"))),
			fmt.Sprintf("WRITE=%s", filepath.ToSlash(filepath.Join(filepath.Dir(preparedRoot.Abs()), "deck"))),
			fmt.Sprintf("WRITE=%s", filepath.ToSlash(filepath.Join(filepath.Dir(preparedRoot.Abs()), ".deck", "manifest.json"))),
		} {
			if err := printLine(opts.Stdout, line); err != nil {
				return err
			}
		}
		for _, path := range runtimeWrites {
			if err := printLine(opts.Stdout, fmt.Sprintf("WRITE=%s", filepath.ToSlash(path))); err != nil {
				return err
			}
		}
		if varsWorkflowPath != "" {
			if err := printLine(opts.Stdout, fmt.Sprintf("WORKFLOW_INCLUDE=%s", filepath.ToSlash(varsWorkflowPath))); err != nil {
				return err
			}
		}
		if applyWorkflowPath != "" {
			if err := printLine(opts.Stdout, fmt.Sprintf("WORKFLOW_INCLUDE=%s", filepath.ToSlash(applyWorkflowPath))); err != nil {
				return err
			}
		}
		return nil
	}

	if opts.Clean {
		if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "cleaning", Attrs: map[string]any{"prepared_root": filepath.ToSlash(preparedRoot.Abs())}}); err != nil {
			return err
		}
		if err := preparedHostPath.RemoveAll(); err != nil {
			return fmt.Errorf("reset prepared root: %w", err)
		}
	}
	if err := preparedHostPath.EnsureDir(filemode.PublishedArtifact); err != nil {
		return fmt.Errorf("create prepared root: %w", err)
	}

	if err := prepare.Run(ctx, prepareWorkflow, prepare.RunOptions{BundleRoot: preparedRoot.Abs(), ForceRedownload: opts.Refresh, EventSink: opts.EventSink}); err != nil {
		return err
	}
	if err := emitDiagnosticEvent(opts, 2, logs.CLIEvent{Level: "debug", Component: "prepare", Event: "bundle_root", Attrs: map[string]any{"bundle_root": filepath.ToSlash(preparedRoot.Abs())}}); err != nil {
		return err
	}

	preparedWorkspaceRoot := filepath.Dir(preparedRoot.Abs())
	if err := stageRuntimeBinariesWithContext(ctx, preparedRoot.Abs(), opts); err != nil {
		return err
	}
	if err := writeBytes(filepath.Join(preparedWorkspaceRoot, "deck"), []byte(renderLauncherScript()), 0o755); err != nil {
		return err
	}

	manifest, err := buildPreparedManifest(preparedRoot)
	if err != nil {
		return err
	}
	if err := writePreparedManifest(filepath.Join(preparedWorkspaceRoot, ".deck", "manifest.json"), manifest); err != nil {
		return err
	}
	if err := emitDiagnosticEvent(opts, 1, logs.CLIEvent{Component: "prepare", Event: "manifest_written", Attrs: map[string]any{"manifest_entries": len(manifest.Entries), "workspace_root": filepath.ToSlash(preparedWorkspaceRoot)}}); err != nil {
		return err
	}
	if err := emitDiagnosticEvent(opts, 2, logs.CLIEvent{Level: "debug", Component: "prepare", Event: "manifest_path", Attrs: map[string]any{"manifest_path": filepath.ToSlash(filepath.Join(preparedWorkspaceRoot, ".deck", "manifest.json"))}}); err != nil {
		return err
	}

	return printLine(opts.Stdout, fmt.Sprintf("prepare: ok (%s)", preparedRoot.Abs()))
}

func emitDiagnosticEvent(opts Options, level int, event logs.CLIEvent) error {
	return logs.EmitCLIEventf(opts.Diagnosticf, level, event)
}

func workflowIncludeCount(prepareWorkflowPath, varsWorkflowPath, applyWorkflowPath string) int {
	count := 1
	if strings.TrimSpace(varsWorkflowPath) != "" {
		count++
	}
	if strings.TrimSpace(applyWorkflowPath) != "" {
		count++
	}
	return count
}

func printLine(w io.Writer, line string) error {
	if w == nil {
		w = os.Stdout
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

func discoverPrepareWorkflow(ctx context.Context) (string, error) {
	workflowDir := filepath.Join(".", workspacepaths.WorkflowRootDir)
	absWorkflowDir, err := filepath.Abs(workflowDir)
	if err != nil {
		return "", fmt.Errorf("resolve workflow directory: %w", err)
	}
	info, err := os.Stat(absWorkflowDir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("workflow directory not found: %s", absWorkflowDir)
	}

	preferred := workspacepaths.CanonicalPrepareWorkflowPath(filepath.Dir(absWorkflowDir))
	preferredInfo, statErr := os.Stat(preferred)
	if statErr != nil || preferredInfo.IsDir() {
		return "", fmt.Errorf("prepare workflow not found: %s", preferred)
	}
	if _, loadErr := config.Load(ctx, preferred); loadErr != nil {
		return "", loadErr
	}
	return preferred, nil
}

func resolveOptionalApplyWorkflowPath(workflowRootPath string) (string, error) {
	path := workspacepaths.CanonicalApplyWorkflowPath(filepath.Dir(workflowRootPath))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("apply workflow path is a directory: %s", path)
	}
	return path, nil
}

func resolveOptionalVarsWorkflowPath(workflowRootPath string) (string, error) {
	varsPath := workspacepaths.CanonicalVarsPath(filepath.Dir(workflowRootPath))
	if info, err := os.Stat(varsPath); err == nil && !info.IsDir() {
		return varsPath, nil
	}
	return "", nil
}

func writeBytes(path string, data []byte, mode os.FileMode) error {
	hostPath, err := hostfs.NewHostPath(path)
	if err != nil {
		return err
	}
	if err := hostPath.WriteFileMode(data, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
