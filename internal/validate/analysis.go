package validate

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
)

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Path     string `json:"path,omitempty"`
	Phase    string `json:"phase,omitempty"`
	StepID   string `json:"stepId,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

func AnalyzeFiles(paths []string) ([]Finding, error) {
	return AnalyzeFilesWithContext(context.Background(), paths)
}

func AnalyzeFilesWithContext(ctx context.Context, paths []string) ([]Finding, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	findings := make([]Finding, 0)
	for _, path := range dedupeAndSort(paths) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		displayPath := relativeOrOriginal(path)
		content, err := fsutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read workflow file: %w", err)
		}
		kind := detectDocumentKind(path)
		if kind == documentKindComponentFragment {
			fragment, err := parseComponentFragment(content)
			if err != nil {
				return nil, withWorkflowName(path, err)
			}
			findings = append(findings, analyzeStepsWithContext(ctx, displayPath, "", fragment.Steps)...)
			continue
		}
		wf, err := parseWorkflow(content)
		if err != nil {
			return nil, withWorkflowName(path, err)
		}
		findings = append(findings, analyzeWorkflowWithContext(ctx, displayPath, wf)...)
	}
	slices.SortFunc(findings, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := strings.Compare(a.Phase, b.Phase); c != 0 {
			return c
		}
		if c := strings.Compare(a.StepID, b.StepID); c != 0 {
			return c
		}
		return strings.Compare(a.Code, b.Code)
	})
	return findings, nil
}

func analyzeWorkflowWithContext(ctx context.Context, path string, wf *config.Workflow) []Finding {
	if wf == nil {
		return nil
	}
	findings := make([]Finding, 0)
	for _, step := range wf.Steps {
		if ctx.Err() != nil {
			return findings
		}
		findings = append(findings, analyzeStep(path, "", step)...)
	}
	for _, phase := range wf.Phases {
		if ctx.Err() != nil {
			return findings
		}
		findings = append(findings, analyzeStepsWithContext(ctx, path, phase.Name, phase.Steps)...)
	}
	return findings
}

func analyzeStepsWithContext(ctx context.Context, path string, phase string, steps []config.Step) []Finding {
	findings := make([]Finding, 0, len(steps))
	for _, step := range steps {
		if ctx.Err() != nil {
			return findings
		}
		findings = append(findings, analyzeStep(path, phase, step)...)
	}
	return findings
}

func analyzeStep(path string, phase string, step config.Step) []Finding {
	if !strings.EqualFold(strings.TrimSpace(step.Kind), "Command") {
		return nil
	}
	findings := []Finding{{
		Severity: "warning",
		Code:     "W_COMMAND_OPAQUE",
		Message:  "Command step relies on opaque shell behavior; deck cannot infer idempotency or side effects.",
		Hint:     "Reserve Command for vendor tools, custom probes, or one-off local commands with clear side effects. Prefer typed steps when available.",
		Path:     path,
		Phase:    strings.TrimSpace(phase),
		StepID:   strings.TrimSpace(step.ID),
		Kind:     step.Kind,
	}}
	if replacement, ok := typedCommandReplacementHint(step.Spec); ok {
		findings = append(findings, Finding{
			Severity: "warning",
			Code:     "W_COMMAND_TYPED_PREFERRED",
			Message:  fmt.Sprintf("Command step matches a built-in %s pattern and should use the typed step instead.", replacement.kind),
			Hint:     replacement.hint,
			Path:     path,
			Phase:    strings.TrimSpace(phase),
			StepID:   strings.TrimSpace(step.ID),
			Kind:     step.Kind,
		})
	}
	return findings
}

type commandReplacement struct {
	kind string
	hint string
}

func typedCommandReplacementHint(spec map[string]any) (commandReplacement, bool) {
	command := commandVector(spec)
	if len(command) == 0 {
		return commandReplacement{}, false
	}
	name := filepath.Base(strings.TrimSpace(command[0]))
	if isShellCommand(name) {
		return commandReplacement{}, false
	}
	switch name {
	case "systemctl":
		if len(command) >= 3 && isServiceLifecycleAction(command[1]) {
			return commandReplacement{kind: "ManageService", hint: "Use `ManageService` with `spec.name`, `spec.state`, and `spec.enabled` for service lifecycle changes instead of wrapping `systemctl`."}, true
		}
	case "mkdir":
		if hasAnyArg(command[1:], "-p", "--parents") {
			return commandReplacement{kind: "EnsureDirectory", hint: "Use `EnsureDirectory` for directory creation and mode management instead of shelling out to `mkdir -p`."}, true
		}
	case "cp":
		if len(command) >= 3 {
			return commandReplacement{kind: "CopyFile", hint: "Use `CopyFile` for direct file copy operations instead of wrapping `cp`."}, true
		}
	case "tar":
		if hasExtractFlag(command[1:]) {
			return commandReplacement{kind: "ExtractArchive", hint: "Use `ExtractArchive` for archive unpacking instead of wrapping `tar` extraction flags."}, true
		}
	case "modprobe":
		return commandReplacement{kind: "KernelModule", hint: "Use `KernelModule` to load or persist kernel modules instead of wrapping `modprobe`."}, true
	case "sysctl":
		return commandReplacement{kind: "Sysctl", hint: "Use `Sysctl` for kernel parameter changes instead of wrapping `sysctl`."}, true
	case "swapoff", "swapon":
		return commandReplacement{kind: "Swap", hint: "Use `Swap` to enable, disable, or persist swap state instead of wrapping swap commands directly."}, true
	case "ln":
		if hasAnyArg(command[1:], "-s", "-sf", "-snf", "--symbolic") {
			return commandReplacement{kind: "CreateSymlink", hint: "Use `CreateSymlink` for symlink management instead of wrapping `ln -s`."}, true
		}
	}
	return commandReplacement{}, false
}

func commandVector(spec map[string]any) []string {
	if spec == nil {
		return nil
	}
	raw, ok := spec["command"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			s, ok := value.(string)
			if !ok {
				return nil
			}
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	default:
		return nil
	}
}

func isShellCommand(name string) bool {
	switch name {
	case "sh", "bash", "ash", "dash":
		return true
	default:
		return false
	}
}

func isServiceLifecycleAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "start", "stop", "restart", "reload", "enable", "disable":
		return true
	default:
		return false
	}
}

func hasAnyArg(args []string, candidates ...string) bool {
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		for _, candidate := range candidates {
			if trimmed == candidate {
				return true
			}
		}
	}
	return false
}

func hasExtractFlag(args []string) bool {
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "--extract" || strings.HasPrefix(trimmed, "--extract=") {
			return true
		}
		if strings.HasPrefix(trimmed, "-") && strings.Contains(trimmed, "x") {
			return true
		}
	}
	return false
}

func relativeOrOriginal(path string) string {
	if rel, err := filepath.Rel(".", path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}
