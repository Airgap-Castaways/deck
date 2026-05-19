package workflowcontext

import "testing"

func TestRenderMapCoversFieldDefinitions(t *testing.T) {
	ctx := Context{
		Command:  CommandApply,
		Workflow: Workflow{Source: SourceServer, Path: "https://deck.example/workflows/apply.yaml", Scenario: "cluster-a"},
		Paths:    Paths{BundleRoot: "/srv/bundle", OutputRoot: "/srv/output", StateFile: "/var/lib/deck/state.json"},
	}
	rendered := ctx.RenderMap()

	for _, def := range FieldDefinitions() {
		path := trimContextPrefix(def.Path)
		if _, ok := valueAtPath(rendered, path); !ok {
			t.Fatalf("missing rendered context field %s", def.Path)
		}
	}
	if got := rendered["bundleRoot"]; got != "/srv/bundle" {
		t.Fatalf("legacy bundleRoot alias = %#v", got)
	}
	if got := rendered["stateFile"]; got != "/var/lib/deck/state.json" {
		t.Fatalf("legacy stateFile alias = %#v", got)
	}
}

func TestStateFingerprintExcludesDerivedAndStateFileFields(t *testing.T) {
	base := Context{
		Command:  CommandApply,
		Workflow: Workflow{Source: SourceServer, Path: "https://deck.example/workflows/apply.yaml", Scenario: "cluster-a"},
		Paths:    Paths{BundleRoot: "/srv/bundle", OutputRoot: "/srv/output", StateFile: "/var/lib/deck/state-a.json"},
	}
	changedStatePath := base
	changedStatePath.Paths.StateFile = "/var/lib/deck/state-b.json"
	if base.StateFingerprint() != changedStatePath.StateFingerprint() {
		t.Fatalf("state fingerprint must ignore context.paths.stateFile")
	}

	changedSource := base
	changedSource.Workflow.Source = SourceFilesystem
	if base.StateFingerprint() == changedSource.StateFingerprint() {
		t.Fatalf("state fingerprint must include context.workflow.source")
	}
}

func trimContextPrefix(path string) string {
	const prefix = "context."
	if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
		return path[len(prefix):]
	}
	return path
}

func valueAtPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i < len(path) && path[i] != '.' {
			continue
		}
		key := path[start:i]
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
		start = i + 1
	}
	return current, true
}
