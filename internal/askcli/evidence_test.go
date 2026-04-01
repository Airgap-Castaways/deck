package askcli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpaugment "github.com/Airgap-Castaways/deck/internal/askaugment/mcp"
	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/testutil/mcpfake"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPFakeProcess(t *testing.T) {
	mode := mcpfake.ModeFromArgs()
	if mode == "" {
		return
	}
	mcpfake.Serve(mode, handleEvidenceHelperRequest)
}

func TestBuildEvidencePlanUsesLLMFallbackForAmbiguousExternalRequest(t *testing.T) {
	client := &stubClient{responses: []string{`{"decision":"required","reason":"fresh install guidance needs upstream docs","freshnessSensitive":true,"installEvidence":true,"entities":[{"name":"Talos","kind":"technology"}]}`}}
	plan, events, err := buildEvidencePlan(context.Background(), client, askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}}, "Install the latest release on Debian 12", askintent.Decision{Route: askintent.RouteDraft}, askretrieve.WorkspaceSummary{}, newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("build evidence plan: %v", err)
	}
	if plan.Decision != "required" || len(plan.Entities) != 1 || plan.Entities[0].Name != "Talos" {
		t.Fatalf("expected llm-enriched evidence plan, got %#v", plan)
	}
	if len(events) != 2 || !strings.Contains(events[0], "source=heuristic") || !strings.Contains(events[1], "source=llm") {
		t.Fatalf("expected heuristic and llm evidence plan events, got %#v", events)
	}
	if len(client.prompts) != 1 || client.prompts[0].Kind != "evidence-plan" {
		t.Fatalf("expected evidence-plan llm request, got %#v", client.prompts)
	}
}

func TestExecuteSkipsMCPWhenEvidencePlanIsUnnecessary(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if err := askconfig.SaveStored(askconfig.Settings{MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-search", RunCommand: "/bin/sh", Args: []string{"-c", "exit 0"}}}}}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"), 0o600); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	client := &stubClient{responses: []string{`{"summary":"reviewed workspace","answer":"ok","findings":[],"suggestedChanges":[]}`}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "Review workflows/scenarios/apply.yaml in this workspace", Review: true, Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute review: %v", err)
	}
	state, err := askstate.Load(root)
	if err != nil {
		t.Fatalf("load ask state: %v", err)
	}
	if state.LastRoute == "" {
		t.Fatalf("expected saved route, got %#v", state)
	}
	joined := strings.Join(state.LastAugmentEvents, "\n")
	for _, want := range []string{"evidence-plan: source=heuristic decision=unnecessary", "mcp: skipped by evidence plan (unnecessary)"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected augment event %q, got %#v", want, state.LastAugmentEvents)
		}
	}
	if strings.Contains(joined, "initialize failed") {
		t.Fatalf("expected mcp to be skipped before startup, got %#v", state.LastAugmentEvents)
	}
}

func TestExecuteAuthoringBlocksWhenRequiredExternalEvidenceFails(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if err := askconfig.SaveStored(askconfig.Settings{MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-search", RunCommand: "/bin/sh", Args: []string{"-c", "exit 0"}}}}}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	client := &stubClient{responses: []string{`{"version":1,"request":"create Kubernetes 1.35.1 workflow","intent":"draft","complexity":"complex","authoringProgram":{"verification":{"expectedNodeCount":1,"expectedReadyCount":1,"expectedControlPlaneReady":1}},"blockers":[],"targetOutcome":"generate files","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry"}],"validationChecklist":["lint"]}`,
		`{"summary":"ok","blocking":[],"advisory":[],"missingContracts":[],"suggestedFixes":[],"findings":[]}`,
		`{"summary":"generated","review":[],"selection":{"targets":[{"path":"workflows/scenarios/apply.yaml","kind":"workflow","builders":[{"id":"apply.check-cluster","overrides":{"nodeCount":1}}]}]}}`}}
	root := t.TempDir()
	err := Execute(context.Background(), Options{Root: root, Prompt: "Create a Kubernetes 1.35.1 workflow", Create: true, Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: io.Discard}, client)
	if err == nil || !strings.Contains(err.Error(), "required external evidence could not be fetched") {
		t.Fatalf("expected required evidence failure, got %v", err)
	}
	state, err := askstate.Load(root)
	if err != nil {
		t.Fatalf("load ask state: %v", err)
	}
	if len(state.LastAugmentEvents) != 0 {
		t.Fatalf("expected failed authoring request not to persist ask state, got %#v", state.LastAugmentEvents)
	}
}

func TestQuestionPromptIncludesExternalEvidenceFailureGuidance(t *testing.T) {
	prompt := questionSystemPrompt(askintent.Target{}, askretrieve.RetrievalResult{Chunks: []askretrieve.Chunk{{ID: "ext-status", Source: "external-evidence", Label: "required-evidence-status", Topic: askcontext.TopicExternalEvidence, Content: "External evidence status:\n- required upstream evidence could not be fetched", Score: 90}, {ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Topic: "mcp:web-search:kubernetes.io", Content: "Typed MCP evidence JSON:\n{}", Score: 80, Evidence: &askretrieve.EvidenceSummary{Title: "Installing kubeadm", Domain: "kubernetes.io", Freshness: "external-docs", Official: true}}}})
	for _, want := range []string{"Evidence boundaries:", "external source: Installing kubeadm [domain=kubernetes.io, freshness=external-docs, official=true]", "External evidence status:", "required upstream evidence could not be fetched"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected %q in question prompt, got %q", want, prompt)
		}
	}
}

func TestExternalEvidenceWarningEventsAreAdvisoryOnly(t *testing.T) {
	events := externalEvidenceWarningEvents([]askretrieve.Chunk{{ID: "mcp-1", Source: "mcp", Label: "web-search:kubernetes.io", Evidence: &askretrieve.EvidenceSummary{ArtifactKinds: []string{"package"}, OfflineHints: []string{"prepare before apply"}}}})
	if len(events) != 1 {
		t.Fatalf("expected one advisory event, got %#v", events)
	}
	for _, want := range []string{"evidence-warning: external summaries are advisory only", "artifactKinds=package", "offlineHints=1"} {
		if !strings.Contains(events[0], want) {
			t.Fatalf("expected advisory event to contain %q, got %q", want, events[0])
		}
	}
}

func TestAnswerWithLLMUsesHealthyExternalEvidence(t *testing.T) {
	effective := askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key", MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{mcpfake.Server(t, "web-search", "web-search-success")}}}}
	probe := mcpaugment.ProbeConfiguredProviders(context.Background(), effective.MCP)
	if len(probe) != 1 || probe[0].Status != "healthy" {
		t.Fatalf("expected helper provider to probe healthy, got %#v", probe)
	}
	mcpChunks, events := mcpaugment.Gather(context.Background(), effective.MCP, askintent.RouteExplain, "Explain how to install kubeadm")
	if len(mcpChunks) == 0 || len(events) == 0 || !strings.Contains(strings.Join(events, "\n"), "mcp:web-search call search ok") {
		t.Fatalf("expected direct mcp gather to succeed, got chunks=%#v events=%#v", mcpChunks, events)
	}
	retrieval := askretrieve.Retrieve(askintent.RouteExplain, "Explain how to install kubeadm", askintent.Target{}, askretrieve.WorkspaceSummary{}, askstate.Context{}, mcpChunks)
	client := &stubClient{responses: []string{`{"summary":"explained kubeadm","answer":"Use the official kubeadm install guide.","suggestions":["Pin the exact version."]}`}}
	resp, err := answerWithLLM(context.Background(), client, effective, askintent.Decision{Route: askintent.RouteExplain, Target: askintent.Target{Kind: "workspace"}}, retrieval, "Explain how to install kubeadm", newAskLogger(io.Discard, "trace"))
	if err != nil {
		t.Fatalf("answer with llm: %v", err)
	}
	if resp.Summary != "explained kubeadm" || !strings.Contains(resp.Answer, "official kubeadm install guide") {
		t.Fatalf("unexpected llm answer: %#v", resp)
	}
	if len(client.prompts) != 1 || !strings.Contains(client.prompts[0].SystemPrompt, "external source: Installing kubeadm [domain=kubernetes.io, freshness=external-docs, official=true]") {
		t.Fatalf("expected answer prompt to include normalized external evidence, got %#v", client.prompts)
	}
}

func handleEvidenceHelperRequest(mode string, req mcpfake.Request) (*mcpfake.Response, bool) {
	resp := &mcpfake.Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case string(mcp.MethodInitialize):
		resp.Result = map[string]any{"protocolVersion": mcp.LATEST_PROTOCOL_VERSION, "serverInfo": map[string]any{"name": "test-mcp", "version": "1.0.0"}, "capabilities": map[string]any{"tools": map[string]any{}}}
		return resp, false
	case string(mcp.MethodToolsList):
		if mode == "web-search-success" {
			resp.Result = map[string]any{"tools": []map[string]any{{"name": "search", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}}}}}
			return resp, false
		}
		return resp, false
	case string(mcp.MethodToolsCall):
		name, _ := mcpfake.ToolCall(req)
		if mode == "web-search-success" && name == "search" {
			resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": "Install kubeadm and kubelet from the official Kubernetes docs."}}, "structuredContent": map[string]any{"results": []map[string]any{{"title": "Installing kubeadm", "url": "https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/", "snippet": "Install kubeadm and kubelet from the official Kubernetes docs."}}}}
			return resp, false
		}
		resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": "unknown tool"}}, "isError": true}
		return resp, false
	case "notifications/initialized":
		return nil, false
	default:
		return nil, false
	}
}
