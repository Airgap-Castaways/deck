package install

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestTemplate_RenderVarsAndRuntime(t *testing.T) {
	wf := &config.Workflow{Vars: map[string]any{"kubernetesVersion": "v1.30.1", "registry": map[string]any{"host": "registry.k8s.io"}}}
	runtimeVars := map[string]any{"joinFile": "/tmp/join.txt"}

	rendered, err := renderSpec(map[string]any{
		"path": "{{ .runtime.joinFile }}",
		"nested": map[string]any{
			"image": "{{ .vars.registry.host }}/kube-apiserver:{{ .vars.kubernetesVersion }}",
		},
		"items": []any{
			"{{ .vars.kubernetesVersion }}",
			map[string]any{"join": "{{ .runtime.joinFile }}"},
			123,
		},
	}, wf, runtimeVars)
	if err != nil {
		t.Fatalf("renderSpec failed: %v", err)
	}

	if got := rendered["path"]; got != "/tmp/join.txt" {
		t.Fatalf("unexpected rendered path: %#v", got)
	}
	nested, ok := rendered["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested should be map, got %#v", rendered["nested"])
	}
	if got := nested["image"]; got != "registry.k8s.io/kube-apiserver:v1.30.1" {
		t.Fatalf("unexpected rendered image: %#v", got)
	}
	items, ok := rendered["items"].([]any)
	if !ok {
		t.Fatalf("items should be slice, got %#v", rendered["items"])
	}
	if got := items[0]; got != "v1.30.1" {
		t.Fatalf("unexpected rendered items[0]: %#v", got)
	}
	itemMap, ok := items[1].(map[string]any)
	if !ok || itemMap["join"] != "/tmp/join.txt" {
		t.Fatalf("unexpected rendered items[1]: %#v", items[1])
	}
	if got := items[2]; got != 123 {
		t.Fatalf("unexpected rendered items[2]: %#v", got)
	}

	_, err = renderSpec(map[string]any{"content": "{{ .runtime.missing }}"}, wf, runtimeVars)
	if err == nil {
		t.Fatalf("expected unresolved template reference error")
	}
	if !strings.Contains(err.Error(), "spec.content") {
		t.Fatalf("expected error to include spec path, got %v", err)
	}
}
