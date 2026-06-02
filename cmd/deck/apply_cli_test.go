package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/applycli"
	"github.com/Airgap-Castaways/deck/internal/buildinfo"
	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestResolveInstallStatePathUsesLocalWorkspaceStateDir(t *testing.T) {
	root := t.TempDir()
	wfPath := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	wf := &config.Workflow{StateKey: "abc123"}

	statePath, err := applycli.ResolveInstallStatePathForWorkflowPath(wf, wfPath, "")
	if err != nil {
		t.Fatalf("resolve install state path failed: %v", err)
	}

	expected := filepath.Join(root, ".deck", "state", "apply", "abc123.json")
	if statePath != expected {
		t.Fatalf("state path mismatch: got %q want %q", statePath, expected)
	}
}

func TestResolveInstallStatePathUsesXDGForRemoteWorkflow(t *testing.T) {
	stateHome := filepath.Join(t.TempDir(), "state-home")
	t.Setenv("XDG_STATE_HOME", stateHome)
	wf := &config.Workflow{StateKey: "abc123"}

	statePath, err := applycli.ResolveInstallStatePathForWorkflowPath(wf, "https://example.invalid/apply.yaml", "")
	if err != nil {
		t.Fatalf("resolve install state path failed: %v", err)
	}

	expected := filepath.Join(stateHome, "deck", "state", "apply", "abc123.json")
	if statePath != expected {
		t.Fatalf("state path mismatch: got %q want %q", statePath, expected)
	}
}

func TestResolveInstallStatePathUsesExplicitStateDir(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	wf := &config.Workflow{StateKey: "abc123"}

	statePath, err := applycli.ResolveInstallStatePathForWorkflowPath(wf, "https://example.invalid/apply.yaml", stateDir)
	if err != nil {
		t.Fatalf("resolve install state path failed: %v", err)
	}

	expected := filepath.Join(stateDir, "abc123.json")
	if statePath != expected {
		t.Fatalf("state path mismatch: got %q want %q", statePath, expected)
	}
}

func statePathFromVerboseApply(t *testing.T, args []string) string {
	t.Helper()
	res := execute(append(args, "--v=1"))
	if res.err != nil {
		t.Fatalf("apply for state path: %v", res.err)
	}
	for _, field := range strings.Fields(res.stderr) {
		if strings.HasPrefix(field, "state=") {
			return strings.TrimPrefix(field, "state=")
		}
	}
	t.Fatalf("expected state in stderr, got %q", res.stderr)
	return ""
}

func TestApplyWritesRunRecordUnderXDGStateHome(t *testing.T) {
	stateHome := filepath.Join(t.TempDir(), "state-home")
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))

	wfPath := filepath.Join(t.TempDir(), "apply-runlog.yaml")
	content := "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: run-true\n        kind: Command\n        spec:\n          command: [\"true\"]\n"
	if err := os.WriteFile(wfPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}
	createValidBundleManifest(t, bundle)

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath, bundle}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	runsRoot := filepath.Join(stateHome, "deck", "runs")
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		t.Fatalf("read runs root: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one run directory, got %d", len(entries))
	}
	recordPath := filepath.Join(runsRoot, entries[0].Name(), "record.json")
	raw, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read run record: %v", err)
	}
	var record struct {
		Command string `json:"command"`
		Status  string `json:"status"`
		Steps   []struct {
			StepID string `json:"step_id"`
			Status string `json:"status"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("parse run record: %v", err)
	}
	if record.Command != "apply" || record.Status != "ok" {
		t.Fatalf("unexpected run record: %+v", record)
	}
	if len(record.Steps) == 0 || record.Steps[len(record.Steps)-1].Status != "succeeded" {
		t.Fatalf("expected succeeded step record, got %+v", record.Steps)
	}
}

func TestRunApplyVarFlagLastWins(t *testing.T) {
	wfPath := filepath.Join(t.TempDir(), "apply-vars.yaml")
	content := `version: v1alpha1
phases:
  - name: install
    steps:
      - id: run-with-vars
        kind: Command
        when: vars.run == "yes"
        spec:
          command: ["true"]
`
	if err := os.WriteFile(wfPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}

	out, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath, "--dry-run", "--var", "run=no", "--var", "run=yes", bundle})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(out, "run-with-vars Command PLAN") {
		t.Fatalf("expected PLAN status, got %q", out)
	}
}

func TestPlanAndApplyVarsFileOverlays(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	scenarioDir := filepath.Join(workflowDir, "scenarios")
	varsDir := filepath.Join(workflowDir, "vars")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.MkdirAll(varsDir, 0o755); err != nil {
		t.Fatalf("mkdir vars dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "vars.yaml"), []byte("cluster: base\nrole: base\nmode: base\n"), 0o644); err != nil {
		t.Fatalf("write vars.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varsDir, "site.yaml"), []byte("role: worker\nmode: site\n"), 0o644); err != nil {
		t.Fatalf("write site vars: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varsDir, "node.yaml"), []byte("mode: node\n"), 0o644); err != nil {
		t.Fatalf("write node vars: %v", err)
	}
	workflowPath := filepath.Join(scenarioDir, "apply.yaml")
	writeWorkflowYAML(t, workflowPath, `version: v1alpha1
vars:
  workflowOnly: present
phases:
  - name: install
    steps:
      - id: vars-file-match
        kind: Command
        when: vars.cluster == "base" && vars.role == "worker" && vars.mode == "node"
        spec:
          command: ["true"]
      - id: vars-file-miss
        kind: Command
        when: vars.role == "workflow"
        spec:
          command: ["true"]
`)

	planOut, err := runWithCapturedStdout([]string{"plan", "--workflow", workflowPath, "-f", "vars/site.yaml", "--vars-file", "vars/node.yaml"})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if !strings.Contains(planOut, "vars-file-match Command RUN") || !strings.Contains(planOut, "vars-file-miss Command SKIP") {
		t.Fatalf("expected plan to use vars-file overlays, got %q", planOut)
	}

	applyOut, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--dry-run", "-f", "vars/site.yaml", "--vars-file", "vars/node.yaml", root})
	if err != nil {
		t.Fatalf("apply dry-run failed: %v", err)
	}
	if !strings.Contains(applyOut, "vars-file-match Command PLAN") || !strings.Contains(applyOut, "vars-file-miss Command SKIP") {
		t.Fatalf("expected apply to use vars-file overlays, got %q", applyOut)
	}
}

func TestPlanVarsApplyShowsEffectiveInputs(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	scenarioDir := filepath.Join(workflowDir, "scenarios")
	varsDir := filepath.Join(workflowDir, "vars")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.MkdirAll(varsDir, 0o755); err != nil {
		t.Fatalf("mkdir vars dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "vars.yaml"), []byte("cluster: base\nmode: base\n"), 0o644); err != nil {
		t.Fatalf("write vars.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varsDir, "site.yaml"), []byte("mode: site\nrole: worker\n"), 0o644); err != nil {
		t.Fatalf("write site vars: %v", err)
	}
	workflowPath := filepath.Join(scenarioDir, "apply.yaml")
	writeWorkflowYAML(t, workflowPath, `version: v1alpha1
phases:
  - name: install
    steps:
      - id: capture
        kind: CheckHost
        register:
          captured: passed
        spec:
          checks: [os]
`)

	res := execute([]string{"plan", "vars", "--workflow", workflowPath, "-f", "vars/site.yaml", "--var", "mode=cli", "-o", "json"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	var payload struct {
		Command      string         `json:"command"`
		WorkflowPath string         `json:"workflowPath"`
		Vars         map[string]any `json:"vars"`
		Context      map[string]any `json:"context"`
		Runtime      struct {
			Initial map[string]any `json:"initial"`
			Planned []struct {
				Key    string `json:"key"`
				Step   string `json:"step"`
				Output string `json:"output"`
				Phase  string `json:"phase"`
			} `json:"planned"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &payload); err != nil {
		t.Fatalf("parse json: %v stdout=%q", err, res.stdout)
	}
	if payload.Command != "apply" || payload.WorkflowPath != workflowPath {
		t.Fatalf("unexpected command/workflow: %+v", payload)
	}
	if payload.Vars["cluster"] != "base" || payload.Vars["role"] != "worker" || payload.Vars["mode"] != "cli" {
		t.Fatalf("unexpected vars: %+v", payload.Vars)
	}
	if payload.Context["command"] != "apply" {
		t.Fatalf("unexpected context: %+v", payload.Context)
	}
	if _, ok := payload.Runtime.Initial["host"]; !ok {
		t.Fatalf("expected runtime.host in initial runtime: %+v", payload.Runtime.Initial)
	}
	expectedPlanned := []struct {
		Key    string `json:"key"`
		Step   string `json:"step"`
		Output string `json:"output"`
		Phase  string `json:"phase"`
	}{{Key: "captured", Step: "capture", Output: "passed", Phase: "install"}}
	if !reflect.DeepEqual(payload.Runtime.Planned, expectedPlanned) {
		t.Fatalf("unexpected planned runtime:\ngot:  %+v\nwant: %+v", payload.Runtime.Planned, expectedPlanned)
	}
}

func TestPlanVarsPrepareShowsEffectiveInputs(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	varsDir := filepath.Join(workflowDir, "vars")
	if err := os.MkdirAll(varsDir, 0o755); err != nil {
		t.Fatalf("mkdir vars dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "vars.yaml"), []byte("mode: base\n"), 0o644); err != nil {
		t.Fatalf("write vars.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(varsDir, "prepare.yaml"), []byte("mode: file\n"), 0o644); err != nil {
		t.Fatalf("write prepare vars: %v", err)
	}
	writeWorkflowYAML(t, filepath.Join(workflowDir, "prepare.yaml"), `version: v1alpha1
phases:
  - name: prepare
    steps:
      - id: check-host
        kind: CheckHost
        register:
          hostPassed: passed
        spec:
          checks: [os]
`)
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(originalCWD) }()

	res := execute([]string{"plan", "vars", "--command", "prepare", "-f", "vars/prepare.yaml", "--var", "mode=cli", "-o", "json"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	var payload struct {
		Command string         `json:"command"`
		Vars    map[string]any `json:"vars"`
		Context map[string]any `json:"context"`
		Runtime struct {
			Initial map[string]any `json:"initial"`
			Planned []struct {
				Key    string `json:"key"`
				Step   string `json:"step"`
				Output string `json:"output"`
			} `json:"planned"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &payload); err != nil {
		t.Fatalf("parse json: %v stdout=%q", err, res.stdout)
	}
	if payload.Command != "prepare" || payload.Vars["mode"] != "cli" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Context["command"] != "prepare" {
		t.Fatalf("unexpected context: %+v", payload.Context)
	}
	if _, ok := payload.Runtime.Initial["host"]; !ok {
		t.Fatalf("expected runtime.host in initial runtime: %+v", payload.Runtime.Initial)
	}
	expectedPlanned := []struct {
		Key    string `json:"key"`
		Step   string `json:"step"`
		Output string `json:"output"`
	}{{Key: "hostPassed", Step: "check-host", Output: "passed"}}
	if !reflect.DeepEqual(payload.Runtime.Planned, expectedPlanned) {
		t.Fatalf("unexpected planned runtime:\ngot:  %+v\nwant: %+v", payload.Runtime.Planned, expectedPlanned)
	}
}

func TestPlanAndApplySelectNodeScopedVarsByHostname(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		t.Skipf("hostname unavailable: %v", err)
	}

	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	scenarioDir := filepath.Join(workflowDir, "scenarios")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "vars.yaml"), []byte(fmt.Sprintf(`all:
  site: lab-a
hosts:
  %q:
    role: worker
`, hostname)), 0o644); err != nil {
		t.Fatalf("write vars.yaml: %v", err)
	}
	workflowPath := filepath.Join(scenarioDir, "apply.yaml")
	writeWorkflowYAML(t, workflowPath, `version: v1alpha1
phases:
  - name: install
    steps:
      - id: host-match
        kind: Command
        when: vars.role == "worker" && vars.site == "lab-a"
        spec:
          command: ["true"]
      - id: host-miss
        kind: Command
        when: vars.role == "control-plane"
        spec:
          command: ["true"]
`)

	planOut, err := runWithCapturedStdout([]string{"plan", "--workflow", workflowPath})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if !strings.Contains(planOut, "host-match Command RUN") || !strings.Contains(planOut, "host-miss Command SKIP") {
		t.Fatalf("expected plan to select hostname-scoped vars, got %q", planOut)
	}

	applyOut, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--dry-run", root})
	if err != nil {
		t.Fatalf("apply dry-run failed: %v", err)
	}
	if !strings.Contains(applyOut, "host-match Command PLAN") || !strings.Contains(applyOut, "host-miss Command SKIP") {
		t.Fatalf("expected apply dry-run to select hostname-scoped vars, got %q", applyOut)
	}
}

func TestApplyContextFieldsFromInferredBundleRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	createValidBundleManifest(t, root)
	workflowDir := filepath.Join(root, "workflows", "scenarios")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "context.txt")
	wfPath := filepath.Join(workflowDir, "apply.yaml")
	writeWorkflowYAML(t, wfPath, fmt.Sprintf(`version: v1alpha1
phases:
  - name: install
    steps:
      - id: context-write
        kind: Command
        when: context.command == "apply" && context.workflow.source == "filesystem" && context.workflow.isServer == false && context.paths.bundleRoot == %q
        spec:
          command: ["sh", "-c", "printf 'source={{ .context.workflow.source }} server={{ .context.workflow.isServer }} bundle={{ .context.paths.bundleRoot }} legacy={{ .context.bundleRoot }}' > %s"]
`, root, outPath))

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read context output: %v", err)
	}
	for _, want := range []string{"source=filesystem", "server=false", "bundle=" + root, "legacy=" + root} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("expected %q in context output, got %q", want, string(raw))
		}
	}
}

func TestPlanAllowsUnmatchedNodeScopedHostname(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		t.Skipf("hostname unavailable: %v", err)
	}

	root := t.TempDir()
	workflowDir := filepath.Join(root, "workflows")
	scenarioDir := filepath.Join(workflowDir, "scenarios")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "vars.yaml"), []byte(fmt.Sprintf(`all:
  site: lab-a
hosts:
  %q:
    role: worker
`, "not-"+hostname)), 0o644); err != nil {
		t.Fatalf("write vars.yaml: %v", err)
	}
	workflowPath := filepath.Join(scenarioDir, "apply.yaml")
	writeWorkflowYAML(t, workflowPath, `version: v1alpha1
phases:
  - name: install
    steps:
      - id: unmatched-host
        kind: Command
        when: vars.site == "lab-a"
        spec:
          command: ["true"]
`)

	planOut, err := runWithCapturedStdout([]string{"plan", "--workflow", workflowPath})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if !strings.Contains(planOut, "unmatched-host Command RUN") {
		t.Fatalf("expected unmatched hostname to remain runnable, got %q", planOut)
	}
}

func TestRunApplyPhaseSelectionAndSkip(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	root := t.TempDir()
	bundleRoot := root
	createValidBundleManifest(t, bundleRoot)
	if err := os.MkdirAll(filepath.Join(bundleRoot, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}

	installLogPath := filepath.Join(root, "install.log")
	postLogPath := filepath.Join(root, "post.log")
	workflowPath := filepath.Join(root, "apply.yaml")
	workflowBody := fmt.Sprintf(`version: v1alpha1
phases:
  - name: install
    steps:
      - id: install-step
        kind: Command
        spec:
          command: ["sh", "-c", "echo install >> %s"]
  - name: post
    steps:
      - id: post-step
        kind: Command
        spec:
          command: ["sh", "-c", "echo post >> %s"]
`, strings.ReplaceAll(installLogPath, "\\", "\\\\"), strings.ReplaceAll(postLogPath, "\\", "\\\\"))
	if err := os.WriteFile(workflowPath, []byte(workflowBody), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--phase", "post", bundleRoot}); err != nil {
		t.Fatalf("first apply --phase post failed: %v", err)
	}
	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--phase", "post", bundleRoot}); err != nil {
		t.Fatalf("second apply --phase post failed: %v", err)
	}
	dryRunOut, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--phase", "post", "--dry-run", bundleRoot})
	if err != nil {
		t.Fatalf("dry-run apply --phase post failed: %v", err)
	}
	if !strings.Contains(dryRunOut, "PHASE=post") {
		t.Fatalf("expected post phase line in dry-run output, got %q", dryRunOut)
	}
	if !strings.Contains(dryRunOut, "SKIP (completed phase)") {
		t.Fatalf("expected completed skip in dry-run output, got %q", dryRunOut)
	}
	if strings.Contains(dryRunOut, "install-step") {
		t.Fatalf("dry-run for phase post must not include install steps, got %q", dryRunOut)
	}

	postRaw, err := os.ReadFile(postLogPath)
	if err != nil {
		t.Fatalf("read post log: %v", err)
	}
	postLines := strings.Split(strings.TrimSpace(string(postRaw)), "\n")
	if len(postLines) != 1 {
		t.Fatalf("expected exactly one post execution, got %d (%q)", len(postLines), string(postRaw))
	}

	installRaw, err := os.ReadFile(installLogPath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("read install log: %v", err)
		}
	} else if strings.TrimSpace(string(installRaw)) != "" {
		t.Fatalf("expected install phase not to execute, got %q", string(installRaw))
	}
}

func TestApplyRemoteWorkflow(t *testing.T) {
	t.Run("vars.yaml 200 changes state key when vars changes", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		if err := os.MkdirAll(home, 0o755); err != nil {
			t.Fatalf("mkdir home: %v", err)
		}
		t.Setenv("HOME", home)

		logPath := filepath.Join(t.TempDir(), "remote-vars.log")
		workflowBody := fmt.Sprintf(`version: v1alpha1
phases:
  - name: install
    steps:
      - id: remote-step
        kind: Command
        spec:
          command: ["sh", "-c", "echo hit >> %s"]
`, strings.ReplaceAll(logPath, "\\", "\\\\"))

		var mu sync.Mutex
		varsBody := "mode: alpha\n"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/workflows/scenarios/apply.yaml":
				_, _ = w.Write([]byte(workflowBody))
			case "/workflows/vars.yaml":
				mu.Lock()
				current := varsBody
				mu.Unlock()
				_, _ = w.Write([]byte(current))
			default:
				http.NotFound(w, r)
			}
		}))
		defer ts.Close()

		workflowURL := ts.URL + "/workflows/scenarios/apply.yaml"
		statePath1 := statePathFromVerboseApply(t, []string{"apply", "--workflow", workflowURL})
		if _, err := os.Stat(statePath1); err != nil {
			t.Fatalf("expected first state file: %v", err)
		}

		mu.Lock()
		varsBody = "mode: beta\n"
		mu.Unlock()

		statePath2 := statePathFromVerboseApply(t, []string{"apply", "--workflow", workflowURL})
		if statePath1 == statePath2 {
			t.Fatalf("expected state path to change when vars.yaml changes")
		}
		if _, err := os.Stat(statePath2); err != nil {
			t.Fatalf("expected second state file: %v", err)
		}

		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read remote log: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected two executions with changed vars, got %d (%q)", len(lines), string(raw))
		}
	})

	t.Run("vars.yaml 404 is non-fatal and rerun skips with same state", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "home")
		if err := os.MkdirAll(home, 0o755); err != nil {
			t.Fatalf("mkdir home: %v", err)
		}
		t.Setenv("HOME", home)

		logPath := filepath.Join(t.TempDir(), "remote-404.log")
		workflowBody := fmt.Sprintf(`version: v1alpha1
phases:
  - name: install
    steps:
      - id: remote-step
        kind: Command
        spec:
          command: ["sh", "-c", "echo hit >> %s"]
`, strings.ReplaceAll(logPath, "\\", "\\\\"))

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/workflows/scenarios/apply.yaml":
				_, _ = w.Write([]byte(workflowBody))
			default:
				http.NotFound(w, r)
			}
		}))
		defer ts.Close()

		workflowURL := ts.URL + "/workflows/scenarios/apply.yaml"
		ignoredBundleArg := filepath.Join(t.TempDir(), "missing-bundle")
		if _, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowURL, ignoredBundleArg}); err != nil {
			t.Fatalf("remote apply with ignored positional bundle(1) failed: %v", err)
		}
		if _, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowURL, ignoredBundleArg}); err != nil {
			t.Fatalf("remote apply with ignored positional bundle(2) failed: %v", err)
		}

		raw, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read remote log: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		if len(lines) != 1 {
			t.Fatalf("expected one execution due to state reuse, got %d (%q)", len(lines), string(raw))
		}
	})

	t.Run("remote apply workflow with top-level steps uses implicit default phase", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/workflows/scenarios/apply.yaml":
				_, _ = w.Write([]byte("version: v1alpha1\nsteps:\n  - id: pack-step\n    kind: Command\n    spec:\n      command: [\"true\"]\n"))
			default:
				http.NotFound(w, r)
			}
		}))
		defer ts.Close()

		_, err := runWithCapturedStdout([]string{"apply", "--workflow", ts.URL + "/workflows/scenarios/apply.yaml"})
		if err != nil {
			t.Fatalf("expected success for implicit default phase, got %v", err)
		}
	})
}

func TestPlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: step-1\n        apiVersion: deck/v1alpha1\n        kind: Command\n        spec:\n          command: [\"true\"]\n")

	planRes := execute([]string{"plan", "--workflow", wfPath})
	if planRes.err != nil {
		t.Fatalf("expected success, got %v", planRes.err)
	}
	before := planRes.stdout
	if !strings.Contains(planRes.stderr, buildinfo.Summary()+" plan started") {
		t.Fatalf("expected plan startup line on stderr, got %q", planRes.stderr)
	}
	if !strings.Contains(before, "SUMMARY steps=1 run=1 skip=0") {
		t.Fatalf("expected summary in plan output, got %q", before)
	}
	if !strings.Contains(before, "RUN") {
		t.Fatalf("expected RUN in plan output, got %q", before)
	}

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath, "--phase", "install"}); err != nil {
		t.Fatalf("apply run: %v", err)
	}

	after, err := runWithCapturedStdout([]string{"plan", "--workflow", wfPath})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if before == after {
		t.Fatalf("expected plan output to change after apply run")
	}
	if !strings.Contains(after, "SKIP") {
		t.Fatalf("expected SKIP in plan output after apply run, got %q", after)
	}
	if !strings.Contains(after, "SUMMARY steps=1 run=0 skip=1 skipCompleted=1") {
		t.Fatalf("expected completed summary in plan output, got %q", after)
	}
}

func TestPlanJSONAndVerboseDiagnostics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-plan.json.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: guarded\n        apiVersion: deck/v1alpha1\n        kind: Command\n        when: vars.run == \"yes\"\n        retry: 2\n        timeout: 30s\n        spec:\n          command: [\"true\"]\n")

	res := execute([]string{"plan", "--workflow", wfPath, "-o", "json", "--v=2", "--var", "run=yes"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if !strings.Contains(res.stderr, "component=plan") || !strings.Contains(res.stderr, "event=plan_requested") || !strings.Contains(res.stderr, "workflow="+wfPath) {
		t.Fatalf("expected plan diagnostics on stderr, got %q", res.stderr)
	}
	if !strings.Contains(res.stderr, "component=plan") || !strings.Contains(res.stderr, "event=plan_step") || !strings.Contains(res.stderr, "step=guarded") {
		t.Fatalf("expected verbose step diagnostics on stderr, got %q", res.stderr)
	}
	var payload struct {
		WorkflowPath   string   `json:"workflowPath"`
		SelectedPhase  string   `json:"selectedPhase"`
		StatePath      string   `json:"statePath"`
		RuntimeVarKeys []string `json:"runtimeVarKeys"`
		Summary        struct {
			TotalSteps      int `json:"totalSteps"`
			RunSteps        int `json:"runSteps"`
			SkipSteps       int `json:"skipSteps"`
			CompletedPhases int `json:"completedPhases"`
		} `json:"summary"`
		Steps []struct {
			Phase   string `json:"phase"`
			ID      string `json:"id"`
			Kind    string `json:"kind"`
			Action  string `json:"action"`
			When    string `json:"when"`
			Retry   int    `json:"retry"`
			Timeout string `json:"timeout"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &payload); err != nil {
		t.Fatalf("parse plan json: %v stdout=%q", err, res.stdout)
	}
	if payload.WorkflowPath != wfPath {
		t.Fatalf("unexpected workflow path: %q", payload.WorkflowPath)
	}
	if payload.Summary.TotalSteps != 1 || payload.Summary.RunSteps != 1 || payload.Summary.SkipSteps != 0 {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}
	if len(payload.Steps) != 1 {
		t.Fatalf("unexpected steps: %+v", payload.Steps)
	}
	step := payload.Steps[0]
	if step.ID != "guarded" || step.Action != "run" || step.When != "vars.run == \"yes\"" || step.Retry != 2 || step.Timeout != "30s" {
		t.Fatalf("unexpected step payload: %+v", step)
	}

	res = execute([]string{"plan", "--workflow", wfPath, "--v=3", "--var", "run=yes"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	for _, want := range []string{"component=plan", "event=plan_context", "workflow_vars=run", "runtime_vars=host", "completed_phases=0", "event=plan_step_eval", "step=guarded", "when_evaluated=true", "register_keys=-"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
}

func TestApplyAndPlanFresh(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-fresh.yaml")
	logPath := filepath.Join(t.TempDir(), "fresh.log")
	writeWorkflowYAML(t, wfPath, fmt.Sprintf("version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: once\n        kind: Command\n        spec:\n          command: [\"sh\", \"-c\", \"echo run >> %s\"]\n", strings.ReplaceAll(logPath, "\\", "\\\\")))

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath}); err != nil {
		t.Fatalf("initial apply failed: %v", err)
	}
	planOut, err := runWithCapturedStdout([]string{"plan", "--workflow", wfPath})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if !strings.Contains(planOut, "SKIP (completed)") {
		t.Fatalf("expected completed phase skip in plan output, got %q", planOut)
	}
	freshPlan, err := runWithCapturedStdout([]string{"plan", "--workflow", wfPath, "--fresh"})
	if err != nil {
		t.Fatalf("fresh plan failed: %v", err)
	}
	if !strings.Contains(freshPlan, "once Command RUN") {
		t.Fatalf("expected fresh plan to rerun step, got %q", freshPlan)
	}
	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath, "--fresh"}); err != nil {
		t.Fatalf("fresh apply failed: %v", err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fresh log: %v", err)
	}
	if got := strings.Count(strings.TrimSpace(string(raw)), "run"); got != 2 {
		t.Fatalf("expected 2 executions after fresh apply, got %d (%q)", got, string(raw))
	}
}

func TestStateShowListAndClear(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	wfPath := filepath.Join(root, "workflows", "scenarios", "apply.yaml")
	if err := os.MkdirAll(filepath.Dir(wfPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	createValidBundleManifest(t, root)
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: once\n        kind: Command\n        spec:\n          command: [\"true\"]\n")

	if _, err := runWithCapturedStdout([]string{"apply", "--workflow", wfPath}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	showOut, err := runWithCapturedStdout([]string{"state", "show", "--workflow", wfPath})
	if err != nil {
		t.Fatalf("state show failed: %v", err)
	}
	for _, want := range []string{"version=2", "status=succeeded", "completedPhases=install", filepath.Join(root, ".deck", "state", "apply")} {
		if !strings.Contains(showOut, want) {
			t.Fatalf("expected %q in state show output, got %q", want, showOut)
		}
	}
	listOut, err := runWithCapturedStdout([]string{"state", "list", "--state-dir", filepath.Join(root, ".deck", "state", "apply")})
	if err != nil {
		t.Fatalf("state list failed: %v", err)
	}
	if !strings.Contains(listOut, "status=succeeded") || !strings.Contains(listOut, "completed=1") {
		t.Fatalf("unexpected state list output: %q", listOut)
	}
	if _, err := runWithCapturedStdout([]string{"state", "clear", "--workflow", wfPath, "--yes"}); err != nil {
		t.Fatalf("state clear failed: %v", err)
	}
	showAfterClear, err := runWithCapturedStdout([]string{"state", "show", "--workflow", wfPath})
	if err != nil {
		t.Fatalf("state show after clear failed: %v", err)
	}
	if !strings.Contains(showAfterClear, "status=running") || strings.Contains(showAfterClear, "completedPhases=install") {
		t.Fatalf("expected empty running state after clear, got %q", showAfterClear)
	}
}

func TestStateListSkipsInvalidStateFiles(t *testing.T) {
	stateDir := t.TempDir()
	validPath := filepath.Join(stateDir, "valid.json")
	validState := `{
  "version": 2,
  "kind": "deck.applyState",
  "stateKey": "valid",
  "status": "succeeded",
  "currentPhase": "completed",
  "createdAt": "2026-06-02T00:00:00Z",
  "updatedAt": "2026-06-02T00:00:00Z",
  "phases": [{"name":"install","status":"succeeded"}]
}`
	if err := os.WriteFile(validPath, []byte(validState), 0o600); err != nil {
		t.Fatalf("write valid state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write broken state: %v", err)
	}

	out, err := runWithCapturedStdout([]string{"state", "list", "--state-dir", stateDir})
	if err != nil {
		t.Fatalf("state list failed: %v", err)
	}
	if !strings.Contains(out, "valid status=succeeded") {
		t.Fatalf("expected valid state entry, got %q", out)
	}
	if strings.Contains(out, "broken") {
		t.Fatalf("expected broken state to be skipped, got %q", out)
	}
}

func TestApplyVerboseDiagnostics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-verbose.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: verbose-step\n        kind: Command\n        retry: 1\n        spec:\n          command: [\"true\"]\n")

	res := execute([]string{"apply", "--workflow", wfPath, "--v=2"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if res.stdout != "apply: ok\n" {
		t.Fatalf("unexpected stdout: %q", res.stdout)
	}
	for _, want := range []string{"component=apply event=run_requested", "workflow=" + wfPath, "component=apply event=execution_plan", "phases=1", "batches=1", "steps=1", "component=apply event=state_snapshot", "component=apply event=phase_plan", "phase=install", "component=apply event=batch_plan", "component=apply event=step_plan", "kind=Command", "component=apply event=runlog_created", "runlog=", "component=apply event=batch_started", "batch=install", "component=apply event=step_started", "step=verbose-step", "component=apply event=step_succeeded", "duration_ms=", "component=apply event=batch_succeeded", "component=apply event=run_completed", "status=ok"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
	if strings.Contains(res.stderr, "component=apply event=step_contract") || strings.Contains(res.stderr, "component=apply event=execution_context") {
		t.Fatalf("expected --v=2 to exclude v3 apply traces, got %q", res.stderr)
	}
}

func TestApplyTraceDiagnostics(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-trace.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nvars:\n  mode: test\nphases:\n  - name: install\n    steps:\n      - id: trace-step\n        kind: Command\n        metadata:\n          owner: test\n        when: vars.mode == \"test\"\n        spec:\n          command: [\"true\"]\n")

	res := execute([]string{"apply", "--workflow", wfPath, "--v=3"})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	for _, want := range []string{"component=apply event=execution_context", "workflow_sha256=", "state_key=", "component=apply event=context_keys", "component=apply event=workflow_vars", "keys=mode", "component=apply event=state_runtime", "component=apply event=step_contract", "api_version=-", "metadata_keys=owner", "spec_keys=command"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
}

func TestApplyDefaultShowsProgressLogs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-default-progress.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: progress-step\n        kind: Command\n        spec:\n          command: [\"true\"]\n")
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}
	createValidBundleManifest(t, bundle)

	res := execute([]string{"apply", "--workflow", wfPath, bundle})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	if res.stdout != "apply: ok\n" {
		t.Fatalf("unexpected stdout: %q", res.stdout)
	}
	if !strings.Contains(res.stderr, buildinfo.Summary()+" apply started") {
		t.Fatalf("expected apply startup line on stderr, got %q", res.stderr)
	}
	for _, want := range []string{"component=apply event=phase_started", "phase=install", "component=apply event=batch_started", "component=apply event=step_started", "step=progress-step", "component=apply event=step_succeeded", "component=apply event=batch_succeeded", "component=apply event=phase_succeeded"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected default verbosity to show progress log %q, got %q", want, res.stderr)
		}
	}
	for _, hidden := range []string{"component=apply event=run_requested", "component=apply event=run_completed", "component=apply event=execution_plan"} {
		if strings.Contains(res.stderr, hidden) {
			t.Fatalf("expected default verbosity to suppress diagnostic log %q, got %q", hidden, res.stderr)
		}
	}

	res = execute([]string{"apply", "--workflow", wfPath, bundle})
	if res.err != nil {
		t.Fatalf("expected second apply success, got %v", res.err)
	}
	if res.stdout != "apply: ok\n" {
		t.Fatalf("unexpected second stdout: %q", res.stdout)
	}
	for _, want := range []string{"component=apply event=phase_skipped", "phase=install", "reason=completed", "component=apply event=step_skipped", "step=progress-step"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected default verbosity to show completed skip log %q, got %q", want, res.stderr)
		}
	}
}

func TestApplyDefaultShowsFailureProgressLogs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-default-failure-progress.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: fail-step\n        kind: Command\n        spec:\n          command: [\"false\"]\n")
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}
	createValidBundleManifest(t, bundle)

	res := execute([]string{"apply", "--workflow", wfPath, bundle})
	if res.err == nil {
		t.Fatalf("expected apply failure")
	}
	for _, want := range []string{"component=apply event=phase_started", "phase=install", "component=apply event=step_started", "step=fail-step", "component=apply event=step_failed", "status=failed", "component=apply event=batch_failed", "component=apply event=phase_failed"} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected default verbosity to show failure progress log %q, got %q", want, res.stderr)
		}
	}
}

func TestApplyParallelBatchProgressLogs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply-parallel-progress.yaml")
	writeWorkflowYAML(t, wfPath, "version: v1alpha1\nphases:\n  - name: install\n    maxParallelism: 2\n    steps:\n      - id: first\n        parallelGroup: downloads\n        kind: Command\n        spec:\n          command: [\"true\"]\n      - id: second\n        parallelGroup: downloads\n        kind: Command\n        spec:\n          command: [\"true\"]\n")
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}
	createValidBundleManifest(t, bundle)

	res := execute([]string{"--v=2", "apply", "--workflow", wfPath, bundle})
	if res.err != nil {
		t.Fatalf("expected success, got %v", res.err)
	}
	for _, want := range []string{
		"component=apply event=batch_started",
		"batch=install:downloads",
		"parallel_group=downloads",
		"batch_size=2",
		"max_parallelism=2",
		"component=apply event=step_started",
		"step=first",
		"step=second",
		"component=apply event=batch_succeeded",
		"duration_ms=",
	} {
		if !strings.Contains(res.stderr, want) {
			t.Fatalf("expected %q in stderr, got %q", want, res.stderr)
		}
	}
}

func TestRunApplyPhaseNotFound(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	bundle := t.TempDir()
	createValidBundleManifest(t, bundle)
	if err := os.MkdirAll(filepath.Join(bundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir bundle workflows: %v", err)
	}

	workflowPath := filepath.Join(t.TempDir(), "apply.yaml")
	workflowBody := "version: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: step-one\n        kind: Command\n        spec:\n          command: [\"true\"]\n"
	if err := os.WriteFile(workflowPath, []byte(workflowBody), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	_, err := runWithCapturedStdout([]string{"apply", "--workflow", workflowPath, "--phase", "post", bundle})
	if err == nil {
		t.Fatalf("expected phase not found error")
	}
	if !strings.Contains(err.Error(), "post phase not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveApplyBundleRootPrecedence(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	root := t.TempDir()
	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalCWD) })

	positionalBundle := filepath.Join(root, "positional")
	if err := os.MkdirAll(filepath.Join(positionalBundle, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir positional workflows: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir cwd workflows: %v", err)
	}

	archivePath := filepath.Join(root, "bundle.tar")
	writeApplyBundleTarFixture(t, archivePath)

	resolved, err := applycli.ResolveBundleRoot(positionalBundle)
	if err != nil {
		t.Fatalf("resolve positional bundle: %v", err)
	}
	if resolved != positionalBundle {
		t.Fatalf("expected positional bundle, got %s", resolved)
	}

	resolved, err = applycli.ResolveBundleRoot(archivePath)
	if err != nil {
		t.Fatalf("resolve explicit bundle archive: %v", err)
	}
	resolvedSlash := filepath.ToSlash(resolved)
	if !strings.Contains(resolvedSlash, "/.cache/deck/extract/") || !strings.HasSuffix(resolvedSlash, "/bundle") {
		t.Fatalf("expected extracted bundle root, got %s", resolved)
	}

	resolved, err = applycli.ResolveBundleRoot("")
	if err != nil {
		t.Fatalf("resolve cwd candidate: %v", err)
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval symlinks on root: %v", err)
	}
	if resolved != rootResolved {
		t.Fatalf("expected cwd bundle root, got %s", resolved)
	}
}
