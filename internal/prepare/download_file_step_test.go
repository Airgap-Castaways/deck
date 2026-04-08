package prepare

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestRunPrepareStep_DownloadFileItems(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	firstRel := filepath.ToSlash(filepath.Join("files", "first.bin"))
	secondRel := filepath.ToSlash(filepath.Join("files", "second.bin"))
	for rel, content := range map[string]string{firstRel: "first", secondRel: "second"} {
		abs := filepath.Join(localCache, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir source dir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
	}

	step := config.Step{Kind: "DownloadFile"}
	key := workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "DownloadFile"}
	rendered := map[string]any{
		"items": []any{
			map[string]any{
				"source":     map[string]any{"path": firstRel},
				"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
				"outputPath": "files/out-first.bin",
			},
			map[string]any{
				"source":     map[string]any{"path": secondRel},
				"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
				"outputPath": "files/out-second.bin",
			},
		},
	}

	files, outputs, err := runPrepareRenderedStepWithKey(context.Background(), nil, bundle, step, rendered, key, nil, RunOptions{})
	if err != nil {
		t.Fatalf("runPrepareRenderedStepWithKey failed: %v", err)
	}
	if len(files) != 2 || files[0] != "files/out-first.bin" || files[1] != "files/out-second.bin" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, ok := outputs["outputPath"]; ok {
		t.Fatalf("did not expect single outputPath for multi-item download: %#v", outputs)
	}
	paths, ok := outputs["outputPaths"].([]string)
	if !ok || len(paths) != 2 {
		t.Fatalf("expected outputPaths list, got %#v", outputs["outputPaths"])
	}
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(bundle, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

func TestRunPrepareStep_DownloadFileDecodeError(t *testing.T) {
	step := config.Step{Kind: "DownloadFile", Spec: map[string]any{"source": 42}}
	key := workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "DownloadFile"}
	_, _, err := runPrepareRenderedStepWithKey(context.Background(), &fakeRunner{}, t.TempDir(), step, step.Spec, key, nil, RunOptions{})
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode prepare File spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadURLToFile_RejectsNilContext(t *testing.T) {
	target, err := os.CreateTemp(t.TempDir(), "download-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = target.Close() }()
	if _, err := downloadURLToFile(nilContextForPrepareTest(), target, "https://example.invalid/file", nil); err == nil {
		t.Fatalf("expected nil context error")
	} else if !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}
