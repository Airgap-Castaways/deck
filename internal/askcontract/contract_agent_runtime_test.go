package askcontract

import "testing"

func TestParseAgentTurnAcceptsNestedArgsAndToolAlias(t *testing.T) {
	raw := `{"toolCalls":[{"tool":"file_read","arguments":{"path":"workflows/vars.yaml"}},{"tool":"file_search","args":{"path":".","query":"workflows/vars.yaml"}}]}`
	resp, err := ParseAgentTurn(raw)
	if err != nil {
		t.Fatalf("ParseAgentTurn returned error: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "file_read" || resp.ToolCalls[0].Path != "workflows/vars.yaml" {
		t.Fatalf("unexpected first tool call: %#v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].Name != "file_search" || resp.ToolCalls[1].Query != "workflows/vars.yaml" {
		t.Fatalf("unexpected second tool call: %#v", resp.ToolCalls[1])
	}
}

func TestParseAgentTurnAcceptsFlexibleFinishShapes(t *testing.T) {
	for _, raw := range []string{
		`{"finish":{"status":"ready","message":"lint passed"}}`,
		`{"summary":"done","finish":{"ok":true}}`,
		`{"finish":"completed"}`,
	} {
		resp, err := ParseAgentTurn(raw)
		if err != nil {
			t.Fatalf("ParseAgentTurn(%s) returned error: %v", raw, err)
		}
		if resp.Finish == nil {
			t.Fatalf("expected finish for %s", raw)
		}
	}
}
