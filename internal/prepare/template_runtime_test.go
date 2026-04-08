package prepare

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestTemplate_RenderVarsAndRuntime(t *testing.T) {
	wf := &config.Workflow{Vars: map[string]any{
		"kubernetesVersion": "v1.30.1",
		"registry":          map[string]any{"host": "registry.k8s.io"},
		"downloads": []any{
			map[string]any{"source": map[string]any{"path": "files/a.bin"}, "outputPath": "files/download-a.bin"},
			map[string]any{"source": map[string]any{"url": "https://example.invalid/b"}, "outputPath": "files/download-b.bin"},
		},
		"downloadSpec": map[string]any{"source": map[string]any{"path": "files/spec.bin"}, "outputPath": "files/spec-out.bin"},
	}}
	runtimeVars := map[string]any{"downloaded": "files/a.bin"}

	rendered, err := renderSpec(map[string]any{
		"source":     map[string]any{"path": "{{ .runtime.downloaded }}"},
		"outputPath": "files/{{ .vars.kubernetesVersion }}.bin",
		"downloads":  "{{ .vars.downloads }}",
		"download":   "{{ .vars.downloadSpec }}",
		"firstPath":  "{{ index .vars.downloads 0 \"outputPath\" }}",
		"images": []any{
			"{{ .vars.registry.host }}/kube-apiserver:{{ .vars.kubernetesVersion }}",
			map[string]any{"tag": "{{ .runtime.downloaded }}"},
			7,
		},
	}, wf, runtimeVars)
	if err != nil {
		t.Fatalf("renderSpec failed: %v", err)
	}

	source, ok := rendered["source"].(map[string]any)
	if !ok || source["path"] != "files/a.bin" {
		t.Fatalf("unexpected rendered source: %#v", rendered["source"])
	}
	outputPath, ok := rendered["outputPath"].(string)
	if !ok || outputPath != "files/v1.30.1.bin" {
		t.Fatalf("unexpected rendered output: %#v", rendered["outputPath"])
	}
	images, ok := rendered["images"].([]any)
	if !ok {
		t.Fatalf("images should be slice, got %#v", rendered["images"])
	}
	if got := images[0]; got != "registry.k8s.io/kube-apiserver:v1.30.1" {
		t.Fatalf("unexpected rendered images[0]: %#v", got)
	}
	imageMap, ok := images[1].(map[string]any)
	if !ok || imageMap["tag"] != "files/a.bin" {
		t.Fatalf("unexpected rendered images[1]: %#v", images[1])
	}
	if got := images[2]; got != 7 {
		t.Fatalf("unexpected rendered images[2]: %#v", got)
	}
	downloads, ok := rendered["downloads"].([]any)
	if !ok || len(downloads) != 2 {
		t.Fatalf("downloads should be structured slice, got %#v", rendered["downloads"])
	}
	firstDownload, ok := downloads[0].(map[string]any)
	if !ok || firstDownload["outputPath"] != "files/download-a.bin" {
		t.Fatalf("unexpected rendered downloads[0]: %#v", downloads[0])
	}
	download, ok := rendered["download"].(map[string]any)
	if !ok || download["outputPath"] != "files/spec-out.bin" {
		t.Fatalf("download should be structured map, got %#v", rendered["download"])
	}
	if got := rendered["firstPath"]; got != "files/download-a.bin" {
		t.Fatalf("unexpected indexed render value: %#v", got)
	}

	_, err = renderSpec(map[string]any{"content": "{{ .vars.missing }}"}, wf, runtimeVars)
	if err == nil {
		t.Fatalf("expected unresolved template reference error")
	}
	if !strings.Contains(err.Error(), "spec.content") {
		t.Fatalf("expected error to include spec path, got %v", err)
	}
}
