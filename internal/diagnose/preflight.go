package diagnose

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/taedi90/deck/internal/config"
)

type RunOptions struct {
	WorkflowPath string
	BundleRoot   string
	OutputPath   string
	LookPath     func(file string) (string, error)
}

type Report struct {
	Timestamp string       `json:"timestamp"`
	Mode      string       `json:"mode"`
	Summary   Summary      `json:"summary"`
	Checks    []CheckEntry `json:"checks"`
}

type Summary struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type CheckEntry struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func Preflight(wf *config.Workflow, opts RunOptions) (*Report, error) {
	if wf == nil {
		return nil, fmt.Errorf("workflow is nil")
	}

	report := &Report{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Mode:      "preflight",
		Checks:    []CheckEntry{},
	}

	check := func(name string, ok bool, msg string) {
		status := "passed"
		if !ok {
			status = "failed"
			report.Summary.Failed++
		} else {
			report.Summary.Passed++
		}
		report.Checks = append(report.Checks, CheckEntry{Name: name, Status: status, Message: msg})
	}

	check("workflow.version", wf.Version == "v1", fmt.Sprintf("version=%s", wf.Version))
	check("phase.prepare.exists", hasPhase(wf, "prepare"), "prepare phase required")
	check("phase.install.exists", hasPhase(wf, "install"), "install phase required")
	checkPrepareBackendPrerequisites(wf, opts, check)

	bundleRoot := opts.BundleRoot
	if bundleRoot == "" {
		bundleRoot = wf.Context.BundleRoot
	}
	check("bundle.root.configured", bundleRoot != "", "bundle root should be provided")

	if bundleRoot != "" {
		manifestPath := filepath.Join(bundleRoot, "manifest.json")
		_, err := os.Stat(manifestPath)
		check("bundle.manifest.exists", err == nil, manifestPath)
	}

	statePath := wf.Context.StateFile
	check("state.path.configured", statePath != "", "state path should be configured")

	if opts.OutputPath != "" {
		if err := writeReport(opts.OutputPath, report); err != nil {
			return nil, err
		}
	}

	if report.Summary.Failed > 0 {
		return report, fmt.Errorf("preflight failed")
	}

	return report, nil
}

func checkPrepareBackendPrerequisites(wf *config.Workflow, opts RunOptions, check func(name string, ok bool, msg string)) {
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	prepare, found := findPhase(wf, "prepare")
	if !found {
		return
	}

	for _, step := range prepare.Steps {
		spec := step.Spec
		backend := nestedMap(spec, "backend")

		switch step.Kind {
		case "DownloadPackages", "DownloadK8sPackages":
			if stringField(backend, "mode") != "container" {
				continue
			}
			runtimeMode := stringFieldOrDefault(backend, "runtime", "auto")
			ok, msg := runtimeAvailable(lookPath, runtimeMode)
			check(fmt.Sprintf("prepare.runtime.%s", step.ID), ok, msg)

		case "DownloadImages":
			engine := stringFieldOrDefault(backend, "engine", "skopeo")
			if engine != "skopeo" {
				continue
			}
			sandbox := nestedMap(backend, "sandbox")
			if stringField(sandbox, "mode") == "container" {
				runtimeMode := stringFieldOrDefault(sandbox, "runtime", "auto")
				ok, msg := runtimeAvailable(lookPath, runtimeMode)
				check(fmt.Sprintf("prepare.image-sandbox-runtime.%s", step.ID), ok, msg)
				continue
			}

			_, err := lookPath("skopeo")
			ok := err == nil
			msg := "local skopeo binary required"
			if ok {
				msg = "skopeo found"
			}
			check(fmt.Sprintf("prepare.skopeo.%s", step.ID), ok, msg)
		}
	}
}

func runtimeAvailable(lookPath func(file string) (string, error), runtimeMode string) (bool, string) {
	runtimeMode = strings.TrimSpace(runtimeMode)
	if runtimeMode == "" {
		runtimeMode = "auto"
	}

	if runtimeMode == "auto" {
		if _, err := lookPath("docker"); err == nil {
			return true, "container runtime auto resolved: docker"
		}
		if _, err := lookPath("podman"); err == nil {
			return true, "container runtime auto resolved: podman"
		}
		return false, "container runtime auto resolution failed (docker/podman not found)"
	}

	if runtimeMode != "docker" && runtimeMode != "podman" {
		return false, fmt.Sprintf("unsupported runtime mode: %s", runtimeMode)
	}

	if _, err := lookPath(runtimeMode); err != nil {
		return false, fmt.Sprintf("runtime not found: %s", runtimeMode)
	}
	return true, fmt.Sprintf("runtime found: %s", runtimeMode)
}

func findPhase(wf *config.Workflow, name string) (config.Phase, bool) {
	for _, p := range wf.Phases {
		if p.Name == name {
			return p, true
		}
	}
	return config.Phase{}, false
}

func nestedMap(root map[string]any, key string) map[string]any {
	if root == nil {
		return map[string]any{}
	}
	v, ok := root[key]
	if !ok {
		return map[string]any{}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return m
}

func stringField(root map[string]any, key string) string {
	if root == nil {
		return ""
	}
	v, ok := root[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func stringFieldOrDefault(root map[string]any, key, fallback string) string {
	v := stringField(root, key)
	if v == "" {
		return fallback
	}
	return v
}

func hasPhase(wf *config.Workflow, name string) bool {
	for _, p := range wf.Phases {
		if p.Name == name {
			return true
		}
	}
	return false
}

func writeReport(path string, report *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create diagnose directory: %w", err)
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode diagnose report: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write diagnose report: %w", err)
	}
	return nil
}
