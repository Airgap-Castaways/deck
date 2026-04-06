package validate

import (
	"reflect"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestReferencedRuntimeVarsCollectsWhenAndTemplateReferences(t *testing.T) {
	step := config.Step{
		When: `runtime.host.os.family == "rhel" && vars.role == "worker"`,
		Spec: map[string]any{
			"source": map[string]any{"path": "{{ .runtime.downloaded }}"},
			"tags":   []any{"{{ index .runtime.images 0 }}"},
		},
	}
	got, err := referencedRuntimeVars(step)
	if err != nil {
		t.Fatalf("referencedRuntimeVars returned error: %v", err)
	}
	want := []string{"downloaded", "host", "images"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("referencedRuntimeVars = %#v, want %#v", got, want)
	}
}
