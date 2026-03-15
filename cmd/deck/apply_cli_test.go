package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/taedi90/deck/internal/applycli"
	"github.com/taedi90/deck/internal/config"
	"github.com/taedi90/deck/internal/install"
)

func TestResolveInstallStatePathUsesHomeAndStateKey(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	t.Setenv("HOME", home)

	wf := &config.Workflow{StateKey: "abc123"}
	statePath, err := applycli.ResolveInstallStatePath(wf)
	if err != nil {
		t.Fatalf("resolveInstallStatePath failed: %v", err)
	}

	expected := filepath.Join(home, ".deck", "state", "abc123.json")
	if statePath != expected {
		t.Fatalf("state path mismatch: got %q want %q", statePath, expected)
	}
}

func TestRunApplyVarFlagLastWins(t *testing.T) {
	wfPath := filepath.Join(t.TempDir(), "apply-vars.yaml")
	content := `role: apply
version: v1alpha1
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

	out, err := runWithCapturedStdout([]string{"apply", "-f", wfPath, "--dry-run", "--var", "run=no", "--var", "run=yes", bundle})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(out, "run-with-vars Command PLAN") {
		t.Fatalf("expected PLAN status, got %q", out)
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
	workflowBody := fmt.Sprintf(`role: apply
version: v1alpha1
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

	if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--phase", "post", bundleRoot}); err != nil {
		t.Fatalf("first apply --phase post failed: %v", err)
	}
	if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--phase", "post", bundleRoot}); err != nil {
		t.Fatalf("second apply --phase post failed: %v", err)
	}
	dryRunOut, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--phase", "post", "--dry-run", bundleRoot})
	if err != nil {
		t.Fatalf("dry-run apply --phase post failed: %v", err)
	}
	if !strings.Contains(dryRunOut, "PHASE=post") {
		t.Fatalf("expected post phase line in dry-run output, got %q", dryRunOut)
	}
	if !strings.Contains(dryRunOut, "post-step Command SKIP (completed)") {
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

func TestApplyPrefetch(t *testing.T) {
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

	downloadedRelPath := filepath.ToSlash(filepath.Join("files", "prefetched.txt"))
	downloadedPath := filepath.Join(bundle, filepath.FromSlash(downloadedRelPath))
	workflowPath := filepath.Join(t.TempDir(), "apply-prefetch.yaml")

	var mu sync.Mutex
	downloadCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		downloadCount++
		mu.Unlock()
		_, _ = w.Write([]byte("prefetched\n"))
	}))
	defer srv.Close()

	workflowBody := fmt.Sprintf(`role: apply
version: v1alpha1
phases:
  - name: install
    steps:
      - id: requires-prefetch
        kind: Command
        spec:
          command:
            - sh
            - -c
            - 'test -f %s'
      - id: download-file
        kind: File
        spec:
          action: download
          source:
            url: '%s'
          output:
            path: '%s'
`, downloadedPath, srv.URL+"/payload", downloadedRelPath)
	if err := os.WriteFile(workflowPath, []byte(workflowBody), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	_, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, bundle})
	if err == nil {
		t.Fatalf("expected apply without --prefetch to fail")
	}

	if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--prefetch", bundle}); err != nil {
		t.Fatalf("apply with --prefetch failed: %v", err)
	}

	raw, err := os.ReadFile(downloadedPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(raw) != "prefetched\n" {
		t.Fatalf("unexpected downloaded content: %q", string(raw))
	}

	wf, err := config.Load(context.Background(), workflowPath)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	statePath, err := applycli.ResolveInstallStatePath(wf)
	if err != nil {
		t.Fatalf("resolve state path: %v", err)
	}
	stateRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state struct {
		CompletedSteps []string `json:"completedSteps"`
	}
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	if !sliceContains(state.CompletedSteps, "download-file") {
		t.Fatalf("expected download-file to be completed in state, got %#v", state.CompletedSteps)
	}

	if err := os.Remove(downloadedPath); err != nil {
		t.Fatalf("remove downloaded file before rerun: %v", err)
	}
	if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--prefetch", bundle}); err != nil {
		t.Fatalf("second apply with --prefetch failed: %v", err)
	}

	mu.Lock()
	gotDownloads := downloadCount
	mu.Unlock()
	if gotDownloads != 1 {
		t.Fatalf("expected exactly one download across prefetch reruns, got %d", gotDownloads)
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
		workflowBody := fmt.Sprintf(`role: apply
version: v1alpha1
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
		if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowURL}); err != nil {
			t.Fatalf("remote apply(1) failed: %v", err)
		}

		wf1, err := config.Load(context.Background(), workflowURL)
		if err != nil {
			t.Fatalf("load remote workflow(1): %v", err)
		}
		statePath1, err := applycli.ResolveInstallStatePath(wf1)
		if err != nil {
			t.Fatalf("resolve state path(1): %v", err)
		}
		if _, err := os.Stat(statePath1); err != nil {
			t.Fatalf("expected first state file: %v", err)
		}

		mu.Lock()
		varsBody = "mode: beta\n"
		mu.Unlock()

		if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowURL}); err != nil {
			t.Fatalf("remote apply(2) failed: %v", err)
		}

		wf2, err := config.Load(context.Background(), workflowURL)
		if err != nil {
			t.Fatalf("load remote workflow(2): %v", err)
		}
		statePath2, err := applycli.ResolveInstallStatePath(wf2)
		if err != nil {
			t.Fatalf("resolve state path(2): %v", err)
		}
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
		workflowBody := fmt.Sprintf(`role: apply
version: v1alpha1
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
		if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowURL, ignoredBundleArg}); err != nil {
			t.Fatalf("remote apply with ignored positional bundle(1) failed: %v", err)
		}
		if _, err := runWithCapturedStdout([]string{"apply", "--file", workflowURL, ignoredBundleArg}); err != nil {
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

	t.Run("role pack is rejected for remote apply", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/workflows/scenarios/apply.yaml":
				_, _ = w.Write([]byte("role: prepare\nversion: v1alpha1\nsteps:\n  - id: pack-step\n    kind: Command\n    spec:\n      command: [\"true\"]\n"))
			default:
				http.NotFound(w, r)
			}
		}))
		defer ts.Close()

		_, err := runWithCapturedStdout([]string{"apply", "--file", ts.URL + "/workflows/scenarios/apply.yaml"})
		if err == nil {
			t.Fatalf("expected role rejection error")
		}
		if !strings.Contains(err.Error(), "apply workflow role must be apply") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestPlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wfPath := filepath.Join(t.TempDir(), "apply.yaml")
	writeWorkflowYAML(t, wfPath, "role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: step-1\n        apiVersion: deck/v1alpha1\n        kind: Command\n        spec:\n          command: [\"true\"]\n")

	before, err := runWithCapturedStdout([]string{"plan", "--file", wfPath})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(before, "RUN") {
		t.Fatalf("expected RUN in plan output, got %q", before)
	}

	wf, err := config.Load(context.Background(), wfPath)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	execWf, err := applycli.BuildExecutionWorkflow(wf, "install")
	if err != nil {
		t.Fatalf("build execution workflow: %v", err)
	}
	if err := install.Run(context.Background(), execWf, install.RunOptions{BundleRoot: ""}); err != nil {
		t.Fatalf("install run: %v", err)
	}

	after, err := runWithCapturedStdout([]string{"plan", "--file", wfPath})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if before == after {
		t.Fatalf("expected plan output to change after apply run")
	}
	if !strings.Contains(after, "SKIP") {
		t.Fatalf("expected SKIP in plan output after apply run, got %q", after)
	}
}

func TestAssistedApplyUsesLocalEngine(t *testing.T) {
	assistedRoot := t.TempDir()
	t.Setenv("DECK_ASSISTED_ROOT", assistedRoot)
	operatorPath := filepath.Join(t.TempDir(), "etc", "deck", "node-id")
	t.Setenv("DECK_NODE_ID_OPERATOR_PATH", operatorPath)
	t.Setenv("DECK_NODE_ID_GENERATED_PATH", filepath.Join(t.TempDir(), "var", "lib", "deck", "node-id"))
	if err := os.MkdirAll(filepath.Dir(operatorPath), 0o755); err != nil {
		t.Fatalf("mkdir operator path: %v", err)
	}
	if err := os.WriteFile(operatorPath, []byte("node-1\n"), 0o644); err != nil {
		t.Fatalf("write node id: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "assisted-apply.log")
	bundleFilePath := filepath.Join(assistedRoot, "releases", "release-1", "bundle", "outputs", "files", "seed.txt")
	workflowBody := fmt.Sprintf("role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: assisted-apply\n        kind: Command\n        spec:\n          command: [\"sh\", \"-c\", \"test -f %s && echo assisted >> %s\"]\n", strings.ReplaceAll(bundleFilePath, "\\", "\\\\"), strings.ReplaceAll(logPath, "\\", "\\\\"))
	seedContent := []byte("seed\n")
	seedSum := sha256.Sum256(seedContent)
	manifestBody := fmt.Sprintf("{\n  \"entries\": [\n    {\"path\": %q, \"sha256\": %q, \"size\": %d}\n  ]\n}\n", "outputs/files/seed.txt", hex.EncodeToString(seedSum[:]), len(seedContent))

	var uploadedReport struct {
		Action string `json:"action"`
		Status string `json:"status"`
		NodeID string `json:"node_id"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/site/v1/") && r.Header.Get("Authorization") != "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1":
			_, _ = w.Write([]byte(`{"id":"session-1","release_id":"release-1","status":"open"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1/assignment":
			_, _ = w.Write([]byte(`{"id":"assign-1","session_id":"session-1","node_id":"node-1","role":"apply","workflow":"workflows/scenarios/apply.yaml"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/site/v1/sessions/session-1/reports":
			if err := json.NewDecoder(r.Body).Decode(&uploadedReport); err != nil {
				t.Fatalf("decode uploaded report: %v", err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/.deck/manifest.json":
			_, _ = w.Write([]byte(manifestBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/scenarios/apply.yaml":
			_, _ = w.Write([]byte(workflowBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/vars.yaml":
			_, _ = w.Write([]byte("{}\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/outputs/files/seed.txt":
			_, _ = w.Write(seedContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	out, err := runWithCapturedStdout([]string{"apply", "--server", srv.URL, "--session", "session-1", "--api-token", "token-1"})
	if err != nil {
		t.Fatalf("assisted apply failed: %v", err)
	}
	if out != "apply: ok\n" {
		t.Fatalf("unexpected apply output: %q", out)
	}
	if uploadedReport.Action != "apply" || uploadedReport.Status != "ok" || uploadedReport.NodeID != "node-1" {
		t.Fatalf("unexpected uploaded report: %#v", uploadedReport)
	}
	if raw, readErr := os.ReadFile(logPath); readErr != nil || !strings.Contains(string(raw), "assisted") {
		t.Fatalf("expected local engine execution log, err=%v raw=%q", readErr, string(raw))
	}
}

func TestAssistedDiffUsesLocalEngine(t *testing.T) {
	assistedRoot := t.TempDir()
	t.Setenv("DECK_ASSISTED_ROOT", assistedRoot)
	operatorPath := filepath.Join(t.TempDir(), "etc", "deck", "node-id")
	t.Setenv("DECK_NODE_ID_OPERATOR_PATH", operatorPath)
	t.Setenv("DECK_NODE_ID_GENERATED_PATH", filepath.Join(t.TempDir(), "var", "lib", "deck", "node-id"))
	_ = os.MkdirAll(filepath.Dir(operatorPath), 0o755)
	_ = os.WriteFile(operatorPath, []byte("node-1\n"), 0o644)

	workflowBody := "role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: assisted-diff\n        kind: Command\n        spec:\n          command: [\"true\"]\n"
	manifestBody := "{\n  \"entries\": []\n}\n"
	uploaded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1":
			_, _ = w.Write([]byte(`{"id":"session-1","release_id":"release-1","status":"open"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1/assignment":
			_, _ = w.Write([]byte(`{"id":"assign-diff","session_id":"session-1","node_id":"node-1","role":"diff","workflow":"workflows/scenarios/apply.yaml"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/site/v1/sessions/session-1/reports":
			uploaded = true
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/.deck/manifest.json":
			_, _ = w.Write([]byte(manifestBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/scenarios/apply.yaml":
			_, _ = w.Write([]byte(workflowBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/vars.yaml":
			_, _ = w.Write([]byte("{}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	out, err := runWithCapturedStdout([]string{"plan", "--server", srv.URL, "--session", "session-1", "--api-token", "deck-site-v1"})
	if err != nil {
		t.Fatalf("assisted plan failed: %v", err)
	}
	if !strings.Contains(out, "RUN") {
		t.Fatalf("expected RUN plan output, got %q", out)
	}
	if !uploaded {
		t.Fatalf("expected assisted plan report upload")
	}
}

func TestAssistedDoctorUsesLocalEngine(t *testing.T) {
	assistedRoot := t.TempDir()
	t.Setenv("DECK_ASSISTED_ROOT", assistedRoot)
	operatorPath := filepath.Join(t.TempDir(), "etc", "deck", "node-id")
	t.Setenv("DECK_NODE_ID_OPERATOR_PATH", operatorPath)
	t.Setenv("DECK_NODE_ID_GENERATED_PATH", filepath.Join(t.TempDir(), "var", "lib", "deck", "node-id"))
	_ = os.MkdirAll(filepath.Dir(operatorPath), 0o755)
	_ = os.WriteFile(operatorPath, []byte("node-1\n"), 0o644)

	localRepo := t.TempDir()
	reportPath := filepath.Join(t.TempDir(), "doctor-assist.json")
	workflowBody := "role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: doctor-check\n        kind: File\n        spec:\n          source:\n            path: files/dummy.txt\n          fetch:\n            sources:\n              - type: local\n                path: \"{{ .vars.localRepo }}\"\n          output:\n            path: files/dummy.txt\n"
	varsBody := fmt.Sprintf("localRepo: %q\n", localRepo)
	manifestBody := "{\n  \"entries\": []\n}\n"
	uploaded := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1":
			_, _ = w.Write([]byte(`{"id":"session-1","release_id":"release-1","status":"open"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1/assignment":
			_, _ = w.Write([]byte(`{"id":"assign-doctor","session_id":"session-1","node_id":"node-1","role":"doctor","workflow":"workflows/scenarios/apply.yaml"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/site/v1/sessions/session-1/reports":
			uploaded = true
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/.deck/manifest.json":
			_, _ = w.Write([]byte(manifestBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/scenarios/apply.yaml":
			_, _ = w.Write([]byte(workflowBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/vars.yaml":
			_, _ = w.Write([]byte(varsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	out, err := runWithCapturedStdout([]string{"doctor", "--server", srv.URL, "--session", "session-1", "--api-token", "deck-site-v1", "--out", reportPath})
	if err != nil {
		t.Fatalf("assisted doctor failed: %v", err)
	}
	if !strings.Contains(out, "doctor: wrote") {
		t.Fatalf("unexpected doctor output: %q", out)
	}
	if !uploaded {
		t.Fatalf("expected assisted doctor report upload")
	}
}

func TestAssistedModeNoAssignmentSkips(t *testing.T) {
	assistedRoot := t.TempDir()
	t.Setenv("DECK_ASSISTED_ROOT", assistedRoot)
	t.Setenv("DECK_SERVER_CONFIG_PATH", filepath.Join(t.TempDir(), "server.json"))
	operatorPath := filepath.Join(t.TempDir(), "etc", "deck", "node-id")
	t.Setenv("DECK_NODE_ID_OPERATOR_PATH", operatorPath)
	t.Setenv("DECK_NODE_ID_GENERATED_PATH", filepath.Join(t.TempDir(), "var", "lib", "deck", "node-id"))
	_ = os.MkdirAll(filepath.Dir(operatorPath), 0o755)
	_ = os.WriteFile(operatorPath, []byte("node-1\n"), 0o644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1":
			_, _ = w.Write([]byte(`{"id":"session-1","release_id":"release-1","status":"open"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1/assignment":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"no assignment matched session \"session-1\" node_id \"node-1\" role \"apply\""}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/site/v1/sessions/session-1/reports":
			t.Fatalf("no-assignment path must not upload report")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if _, err := runWithCapturedStdout([]string{"server", "set", srv.URL, "--api-token", "token-1"}); err != nil {
		t.Fatalf("server set failed: %v", err)
	}

	out, err := runWithCapturedStdout([]string{"apply", "--session", "session-1"})
	if err != nil {
		t.Fatalf("assisted apply no-assignment should succeed with skip, got %v", err)
	}
	if !strings.Contains(out, "apply: skipped (no-assignment)") {
		t.Fatalf("unexpected skip output: %q", out)
	}
	reportDir := filepath.Join(assistedRoot, "reports", "session-1", "node-1")
	entries, readErr := os.ReadDir(reportDir)
	if readErr != nil || len(entries) == 0 {
		t.Fatalf("expected local skipped report at %s: err=%v", reportDir, readErr)
	}
}

func TestAssistedApplyServerDownAfterAssignment(t *testing.T) {
	assistedRoot := t.TempDir()
	t.Setenv("DECK_ASSISTED_ROOT", assistedRoot)
	operatorPath := filepath.Join(t.TempDir(), "etc", "deck", "node-id")
	t.Setenv("DECK_NODE_ID_OPERATOR_PATH", operatorPath)
	t.Setenv("DECK_NODE_ID_GENERATED_PATH", filepath.Join(t.TempDir(), "var", "lib", "deck", "node-id"))
	_ = os.MkdirAll(filepath.Dir(operatorPath), 0o755)
	_ = os.WriteFile(operatorPath, []byte("node-1\n"), 0o644)

	logPath := filepath.Join(t.TempDir(), "assisted-upload-fail.log")
	bundleFilePath := filepath.Join(assistedRoot, "releases", "release-1", "bundle", "outputs", "files", "seed.txt")
	workflowBody := fmt.Sprintf("role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: assisted-apply\n        kind: Command\n        spec:\n          command: [\"sh\", \"-c\", \"test -f %s && echo assisted >> %s\"]\n", strings.ReplaceAll(bundleFilePath, "\\", "\\\\"), strings.ReplaceAll(logPath, "\\", "\\\\"))
	seedContent := []byte("seed\n")
	seedSum := sha256.Sum256(seedContent)
	manifestBody := fmt.Sprintf("{\n  \"entries\": [\n    {\"path\": %q, \"sha256\": %q, \"size\": %d}\n  ]\n}\n", "outputs/files/seed.txt", hex.EncodeToString(seedSum[:]), len(seedContent))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1":
			_, _ = w.Write([]byte(`{"id":"session-1","release_id":"release-1","status":"open"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/site/v1/sessions/session-1/assignment":
			_, _ = w.Write([]byte(`{"id":"assign-1","session_id":"session-1","node_id":"node-1","role":"apply","workflow":"workflows/scenarios/apply.yaml"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/site/v1/sessions/session-1/reports":
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"temporary_unavailable"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/.deck/manifest.json":
			_, _ = w.Write([]byte(manifestBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/scenarios/apply.yaml":
			_, _ = w.Write([]byte(workflowBody))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/workflows/vars.yaml":
			_, _ = w.Write([]byte("{}\n"))
		case r.Method == http.MethodGet && r.URL.Path == "/site/releases/release-1/bundle/outputs/files/seed.txt":
			_, _ = w.Write(seedContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	_, err := runWithCapturedStdout([]string{"apply", "--server", srv.URL, "--session", "session-1", "--api-token", "token-1"})
	if err == nil {
		t.Fatalf("expected upload transport failure")
	}
	if !strings.Contains(err.Error(), "report upload transport failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(assistedRoot, "reports", "session-1", "node-1")) {
		t.Fatalf("expected local report path in error: %v", err)
	}
	if raw, readErr := os.ReadFile(logPath); readErr != nil || !strings.Contains(string(raw), "assisted") {
		t.Fatalf("expected apply execution to complete before upload failure, err=%v raw=%q", readErr, string(raw))
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
	workflowBody := "role: apply\nversion: v1alpha1\nphases:\n  - name: install\n    steps:\n      - id: step-one\n        kind: Command\n        spec:\n          command: [\"true\"]\n"
	if err := os.WriteFile(workflowPath, []byte(workflowBody), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	_, err := runWithCapturedStdout([]string{"apply", "--file", workflowPath, "--phase", "post", bundle})
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

	resolved, err = applycli.ResolveBundleRoot("")
	if err != nil {
		t.Fatalf("resolve default bundle candidate: %v", err)
	}
	resolvedSlash := filepath.ToSlash(resolved)
	if !strings.Contains(resolvedSlash, "/.deck/extract/") || !strings.HasSuffix(resolvedSlash, "/bundle") {
		t.Fatalf("expected extracted bundle root, got %s", resolved)
	}

	if err := os.Remove(archivePath); err != nil {
		t.Fatalf("remove bundle.tar: %v", err)
	}
	resolved, err = applycli.ResolveBundleRoot("")
	if err != nil {
		t.Fatalf("resolve cwd candidate: %v", err)
	}
	if resolved != root {
		t.Fatalf("expected cwd bundle root, got %s", resolved)
	}
}
