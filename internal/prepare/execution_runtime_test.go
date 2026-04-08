package prepare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRun_WhenAndRegisterSemantics(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	sourceRel := filepath.ToSlash(filepath.Join("files", "a.bin"))
	sourceAbs := filepath.Join(localCache, filepath.FromSlash(sourceRel))
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "download-a",
					Kind: "DownloadFile",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/a-out.bin",
					},
					Register: map[string]string{"downloaded": "outputPath"},
				},
				{
					ID:   "download-b",
					Kind: "DownloadFile",
					When: "vars.role == \"control-plane\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": "{{ .runtime.downloaded }}"},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "bundle", "path": bundle}}},
						"outputPath": "files/b-out.bin",
					},
				},
				{
					ID:   "skip-worker-only",
					Kind: "DownloadFile",
					When: "vars.role == \"worker\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/skip.bin",
					},
				},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "files", "a-out.bin")); err != nil {
		t.Fatalf("expected a-out artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "files", "b-out.bin")); err != nil {
		t.Fatalf("expected b-out artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "files", "skip.bin")); err == nil {
		t.Fatalf("expected skipped artifact to not exist")
	}
}

func TestRun_EmitsStepEvents(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	sourceRel := filepath.ToSlash(filepath.Join("files", "a.bin"))
	sourceAbs := filepath.Join(localCache, filepath.FromSlash(sourceRel))
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "download-a",
					Kind: "DownloadFile",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/a-out.bin",
					},
				},
				{
					ID:   "skip-worker-only",
					Kind: "DownloadFile",
					When: "vars.role == \"worker\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/skip.bin",
					},
				},
			},
		}},
	}

	var (
		events []StepEvent
		mu     sync.Mutex
	)
	if err := Run(context.Background(), wf, RunOptions{
		BundleRoot: bundle,
		EventSink: func(event StepEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	stepEvents := make([]StepEvent, 0, len(events))
	for _, event := range events {
		if event.StepID != "" {
			stepEvents = append(stepEvents, event)
		}
	}
	if len(stepEvents) != 3 {
		t.Fatalf("expected 3 step events, got %#v", events)
	}
	if stepEvents[0].StepID != "download-a" || stepEvents[0].Status != "started" || stepEvents[0].Phase != "prepare" || stepEvents[0].Attempt != 1 {
		t.Fatalf("unexpected first event: %+v", stepEvents[0])
	}
	if stepEvents[1].StepID != "download-a" || stepEvents[1].Status != "succeeded" || stepEvents[1].Phase != "prepare" || stepEvents[1].Attempt != 1 {
		t.Fatalf("unexpected second event: %+v", stepEvents[1])
	}
	if stepEvents[2].StepID != "skip-worker-only" || stepEvents[2].Status != "skipped" || stepEvents[2].Reason != "when" || stepEvents[2].Phase != "prepare" {
		t.Fatalf("unexpected third event: %+v", stepEvents[2])
	}
}

func TestRun_RetrySemantics(t *testing.T) {
	t.Run("retry succeeds on second attempt", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		bundle := t.TempDir()
		runner := &failOnceRunner{}
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:    "retry-packages",
					Kind:  "DownloadPackage",
					Retry: 1,
					Spec: map[string]any{
						"packages": []any{"containerd"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "docker",
							"image":   "ubuntu:22.04",
						},
					},
				}},
			}},
		}

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: runner}); err != nil {
			t.Fatalf("expected retry success, got %v", err)
		}
		if runner.attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", runner.attempts)
		}
	})

	t.Run("retry exhausted keeps failure", func(t *testing.T) {
		bundle := t.TempDir()

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:    "retry-fail",
					Kind:  "DownloadFile",
					Retry: 1,
					Spec: map[string]any{
						"source":     map[string]any{"path": "files/missing.bin"},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": t.TempDir()}}},
						"outputPath": "files/retry-fail.bin",
					},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
		if err == nil {
			t.Fatalf("expected failure after retry exhaustion")
		}
		if !strings.Contains(err.Error(), "E_PREPARE_SOURCE_NOT_FOUND") {
			t.Fatalf("expected E_PREPARE_SOURCE_NOT_FOUND, got %v", err)
		}
	})
}

func TestRun_WhenInvalidExpression(t *testing.T) {
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "worker"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "bad-when",
				Kind: "DownloadPackage",
				When: "vars.role = \"worker\"",
				Spec: map[string]any{"packages": []any{"containerd"}},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
	if err == nil {
		t.Fatalf("expected condition eval error")
	}
	if !strings.Contains(err.Error(), "E_CONDITION_EVAL") {
		t.Fatalf("expected E_CONDITION_EVAL, got %v", err)
	}
}

func TestWhen_NamespaceEnforced(t *testing.T) {
	vars := map[string]any{"nodeRole": "worker"}
	runtimeVars := map[string]any{"hostPassed": true}
	ok, err := EvaluateWhen("vars.nodeRole == \"worker\"", vars, runtimeVars)
	if err != nil {
		t.Fatalf("expected vars namespace expression to pass, got %v", err)
	}
	if !ok {
		t.Fatalf("expected vars namespace expression to be true")
	}

	_, err = EvaluateWhen("nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected bare identifier to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"nodeRole\"; use vars.nodeRole") {
		t.Fatalf("expected bare identifier guidance, got %v", err)
	}

	_, err = EvaluateWhen("context.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected context namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"context.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}

	_, err = EvaluateWhen("other.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected unknown dotted namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"other.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}
}
