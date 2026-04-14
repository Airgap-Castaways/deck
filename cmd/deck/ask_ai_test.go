package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
)

type mockAskClient struct {
	responses         []string
	providerResponses []askprovider.Response
	responseIndex     int
	providerIndex     int
	calls             int
}

type mcpHelperRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *mcp.RequestId  `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpHelperResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      *mcp.RequestId `json:"id,omitempty"`
	Result  any            `json:"result,omitempty"`
}

func enableBuiltInWebSearchTransportHelper(t *testing.T) {
	t.Helper()
	t.Setenv("DECK_TEST_MCP_WEB_SEARCH_COMMAND", os.Args[0])
	t.Setenv("DECK_TEST_MCP_WEB_SEARCH_ARGS", "-test.run=TestAskMCPWebSearchHelperProcess --")
	t.Setenv("DECK_TEST_MCP_WEB_SEARCH_HELPER", "1")
}

func TestAskMCPWebSearchHelperProcess(t *testing.T) {
	if os.Getenv("DECK_TEST_MCP_WEB_SEARCH_HELPER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			os.Exit(0)
		}
		line = strings.TrimRight(line, "\r\n")
		var req mcpHelperRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		resp := handleWebSearchHelperRequest(req)
		if resp == nil {
			continue
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s\n", raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}

func handleWebSearchHelperRequest(req mcpHelperRequest) *mcpHelperResponse {
	resp := &mcpHelperResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case string(mcp.MethodInitialize):
		resp.Result = map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"serverInfo":      map[string]any{"name": "deck-web-search", "version": "1.0.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
		return resp
	case string(mcp.MethodToolsList):
		resp.Result = map[string]any{"tools": []map[string]any{{
			"name": "search",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "integer"},
				},
			},
		}}}
		return resp
	case "notifications/initialized":
		return nil
	default:
		return nil
	}
}

func (m *mockAskClient) Generate(_ context.Context, req askprovider.Request) (askprovider.Response, error) {
	m.calls++
	if req.Kind == "classify" {
		if m.responseIndex < len(m.responses) && strings.Contains(strings.TrimSpace(m.responses[m.responseIndex]), `"route"`) {
			resp := m.responses[m.responseIndex]
			m.responseIndex++
			return askprovider.Response{Content: resp}, nil
		}
		return askprovider.Response{Content: synthesizeClassification(req.Prompt)}, nil
	}
	if len(m.providerResponses) > 0 && strings.HasPrefix(strings.TrimSpace(req.Kind), "generate") {
		if m.providerIndex >= len(m.providerResponses) {
			return m.providerResponses[len(m.providerResponses)-1], nil
		}
		resp := m.providerResponses[m.providerIndex]
		m.providerIndex++
		return resp, nil
	}
	if m.responseIndex >= len(m.responses) {
		return askprovider.Response{Content: m.responses[len(m.responses)-1]}, nil
	}
	resp := m.responses[m.responseIndex]
	m.responseIndex++
	return askprovider.Response{Content: resp}, nil
}

func synthesizeClassification(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "review flag: true"):
		return `{"route":"review","confidence":0.99,"reason":"explicit review flag","target":{"kind":"workspace"},"generationAllowed":false}`
	case strings.Contains(lower, "what is this workspace"):
		return `{"route":"question","confidence":0.9,"reason":"workspace question","target":{"kind":"workspace"},"generationAllowed":false}`
	case strings.Contains(lower, "explain"):
		return `{"route":"explain","confidence":0.9,"reason":"explain request","target":{"kind":"workspace"},"generationAllowed":false}`
	case strings.Contains(lower, "refactor") || strings.Contains(lower, "repair") || strings.Contains(lower, "edit"):
		return `{"route":"refine","confidence":0.9,"reason":"edit request","target":{"kind":"workspace"},"generationAllowed":true}`
	default:
		return `{"route":"draft","confidence":0.9,"reason":"create request","target":{"kind":"workspace"},"generationAllowed":true}`
	}
}

func TestAskConfigCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	out, err := runWithCapturedStdout([]string{"ask", "config", "set", "--provider", "openrouter", "--model", "anthropic/claude-3.5-sonnet", "--endpoint", "https://openrouter.ai/api/v1", "--api-key", "secret-token", "--oauth-token", "oauth-token", "--log-level", "debug"})
	if err != nil {
		t.Fatalf("config set: %v", err)
	}
	if !strings.Contains(out, "ask config saved") {
		t.Fatalf("unexpected config set output: %q", out)
	}
	out, err = runWithCapturedStdout([]string{"ask", "config", "show"})
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	for _, want := range []string{"provider=openrouter", "model=anthropic/claude-3.5-sonnet", "endpoint=https://openrouter.ai/api/v1", "endpointSource=config", "logLevel=debug", "mcpEnabled=false", "apiKey=secr****oken", "apiKeySource=config", "oauthToken=oaut***oken", "oauthTokenSource=config", "authStatus="} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in config show output, got %q", want, out)
		}
	}
	out, err = runWithCapturedStdout([]string{"ask", "config", "unset"})
	if err != nil {
		t.Fatalf("config unset: %v", err)
	}
	if !strings.Contains(out, "ask config cleared") {
		t.Fatalf("unexpected config unset output: %q", out)
	}
}

func TestAskLoginStatusLogoutHeadless(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	out, err := runWithCapturedStdout([]string{"ask", "login", "--provider", "openai", "--headless", "--oauth-token", "oauth-token", "--refresh-token", "refresh-token", "--account-email", "user@example.com", "--expires-at", "2030-01-02T03:04:05Z"})
	if err != nil {
		t.Fatalf("ask login: %v", err)
	}
	if !strings.Contains(out, "ask login saved provider=openai") {
		t.Fatalf("unexpected login output: %q", out)
	}
	out, err = runWithCapturedStdout([]string{"ask", "status", "--provider", "openai"})
	if err != nil {
		t.Fatalf("ask status: %v", err)
	}
	for _, want := range []string{"provider=openai", "authenticated=true", "status=valid", "accountEmail=user@example.com", "hasRefreshToken=true"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in status output, got %q", want, out)
		}
	}
	out, err = runWithCapturedStdout([]string{"ask", "logout", "--provider", "openai"})
	if err != nil {
		t.Fatalf("ask logout: %v", err)
	}
	if !strings.Contains(out, "ask logout removed provider=openai") {
		t.Fatalf("unexpected logout output: %q", out)
	}
	out, err = runWithCapturedStdout([]string{"ask", "status", "--provider", "openai"})
	if err != nil {
		t.Fatalf("ask status after logout: %v", err)
	}
	if !strings.Contains(out, "authenticated=false") {
		t.Fatalf("expected missing auth after logout, got %q", out)
	}
}

func TestAskLoginRejectsNonOpenAIProvider(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if _, err := runWithCapturedStdout([]string{"ask", "login", "--provider", "gemini", "--headless", "--oauth-token", "token"}); err == nil {
		t.Fatalf("expected non-openai provider to fail")
	} else if !strings.Contains(err.Error(), "supports only provider") {
		t.Fatalf("expected provider guard, got %v", err)
	}
}

func TestAskCommandMetadataMatchesAskContext(t *testing.T) {
	cmd := newAskCommand()
	meta := askcontext.AskCommandMeta()
	if cmd.Short != meta.Short {
		t.Fatalf("unexpected ask short help: %q", cmd.Short)
	}
	plan, _, err := cmd.Find([]string{"plan"})
	if err != nil {
		t.Fatalf("find ask plan: %v", err)
	}
	if plan == nil || plan.Short != meta.Plan.Short || plan.Long != meta.Plan.Long {
		t.Fatalf("unexpected ask plan metadata")
	}
}

func TestAskConfigShowIncludesStoredMCPSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if err := askconfig.SaveStored(askconfig.Settings{
		Provider:   "openai",
		Model:      "gpt-5.4",
		APIKey:     testAPIKey(),
		OAuthToken: testOAuthToken(),
		LogLevel:   "trace",
		MCP:        askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "context7", RunCommand: "context7-mcp"}}},
	}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	out, err := runWithCapturedStdout([]string{"ask", "config", "show"})
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	for _, want := range []string{"logLevel=trace", "mcpEnabled=true", "mcpProviderCount=1", "mcpProvider[0].name=context7", "mcpProvider[0].id=context7", "mcpProvider[0].transport=context7-mcp", "mcpProvider[0].transportSource=config-override", "mcpProvider[0].capabilities=entity-resolve,doc-fetch,official-doc-search"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in config show output, got %q", want, out)
		}
	}
}

func TestAskConfigShowIncludesBuiltInWebSearchTransport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	if err := askconfig.SaveStored(askconfig.Settings{MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-server"}}}}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	out, err := runWithCapturedStdout([]string{"ask", "config", "show"})
	if err != nil {
		t.Fatalf("config show: %v", err)
	}
	for _, want := range []string{"mcpProviderCount=1", "mcpProvider[0].name=web-search", "mcpProvider[0].id=web-search", "mcpProvider[0].transport=deck ask mcp web-search", "mcpProvider[0].transportSource=built-in-default"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in config show output, got %q", want, out)
		}
	}
}

func TestAskConfigHealthReportsBuiltInWebSearchHealthy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	enableBuiltInWebSearchTransportHelper(t)
	if err := askconfig.SaveStored(askconfig.Settings{MCP: askconfig.MCP{Enabled: true, Servers: []askconfig.MCPServer{{Name: "web-search"}}}}); err != nil {
		t.Fatalf("save stored config: %v", err)
	}
	out, err := runWithCapturedStdout([]string{"ask", "config", "health"})
	if err != nil {
		t.Fatalf("config health: %v", err)
	}
	for _, want := range []string{"mcpEnabled=true", "mcpProviderCount=1", "mcpProvider[0].name=web-search", "mcpProvider[0].id=web-search", "mcpProvider[0].transport=deck ask mcp web-search", "mcpProvider[0].status=healthy", "mcpProvider[0].phase=ready", "mcpProvider[0].tools=search", "mcpProvider[0].capabilities=official-doc-search,web-search,error-lookup", "mcpProvider[0].message=ok"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in config health output, got %q", want, out)
		}
	}
}

func TestAskAuthoringWritesFiles(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{providerResponses: validAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--create", "create a specific single-node apply workflow"})
	if err != nil {
		t.Fatalf("ask authoring: %v", err)
	}
	if !strings.Contains(out, "ask write: ok") || !strings.Contains(out, "wrote:") {
		t.Fatalf("expected write output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(root, "workflows", "scenarios", "apply.yaml")); err != nil {
		t.Fatalf("expected written workflow file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".deck", "ask", "context.json")); err != nil {
		t.Fatalf("expected ask context state: %v", err)
	}
}

func TestAskClarifyDoesNotGenerate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	client := &mockAskClient{responses: []string{}}
	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return client
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "test"})
	if err != nil {
		t.Fatalf("ask clarify: %v", err)
	}
	if !strings.Contains(out, "could not safely determine") {
		t.Fatalf("expected clarification output, got %q", out)
	}
	if client.calls != 0 {
		t.Fatalf("clarify route should not invoke the answer llm path")
	}
}

func TestAskRepairLoop(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{providerResponses: repairAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--create", "--max-iterations", "2", "create apply workflow with vars"})
	if err != nil {
		t.Fatalf("ask authoring with repair: %v", err)
	}
	if !strings.Contains(out, "lint: lint ok") {
		t.Fatalf("expected lint success after repair, got %q", out)
	}
}

func TestAskReviewMode(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir scenarios: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte("version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"), 0o644); err != nil {
		t.Fatalf("write apply: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "prepare.yaml"), []byte("version: v1alpha1\nphases:\n  - name: collect\n    steps: []\n"), 0o644); err != nil {
		t.Fatalf("write prepare: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	client := &mockAskClient{responses: []string{`{"summary":"reviewed workspace","answer":"The apply scenario currently uses a Command step and would benefit from typed steps.","suggestions":["Replace generic Command usage with typed steps where possible."]}`}}
	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client { return client }
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--review"})
	if err != nil {
		t.Fatalf("ask review: %v", err)
	}
	if !strings.Contains(out, "reviewed workspace") || !strings.Contains(out, "local-findings:") {
		t.Fatalf("unexpected review output: %q", out)
	}
	if client.calls != 1 {
		t.Fatalf("expected a single review call, got %d", client.calls)
	}
}

func TestAskPlanWritesArtifact(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{validPlanJSON()}}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "plan", "create multi-node cluster workflow"})
	if err != nil {
		t.Fatalf("ask plan: %v", err)
	}
	for _, want := range []string{"plan:", "plan-json:", "next:", "deck ask --from"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".deck", "plan", "latest.md")); err != nil {
		t.Fatalf("expected latest markdown plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".deck", "plan", "latest.json")); err != nil {
		t.Fatalf("expected latest json plan: %v", err)
	}
}

func TestAskPlanRejectsNonAuthoringRoute(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{`{"route":"question","confidence":0.9,"reason":"question","target":{"kind":"workspace"},"generationAllowed":false}`}}
	}
	defer func() { newAskBackend = originalFactory }()

	if _, err := runWithCapturedStdout([]string{"ask", "plan", "what is this workspace"}); err == nil {
		t.Fatalf("expected non-authoring ask plan to fail")
	} else if !strings.Contains(err.Error(), "Try `deck ask") {
		t.Fatalf("expected helpful guidance, got %v", err)
	}
}

func TestAskFromPlanPrefersJSONArtifact(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	planDir := filepath.Join(root, ".deck", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	mdPath := filepath.Join(planDir, "sample.md")
	jsonPath := filepath.Join(planDir, "sample.json")
	if err := os.WriteFile(mdPath, []byte("human plan text"), 0o600); err != nil {
		t.Fatalf("write md: %v", err)
	}
	if err := os.WriteFile(jsonPath, []byte(validPlanJSON()), 0o600); err != nil {
		t.Fatalf("write json: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{providerResponses: validAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--from", ".deck/plan/sample.md", "implement this plan"})
	if err != nil {
		t.Fatalf("ask from plan: %v", err)
	}
	if !strings.Contains(out, "ask write: ok") {
		t.Fatalf("expected generation write, got %q", out)
	}
}

func TestAskComplexDirectAuthoringUsesReviewedPlan(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{complexDirectPlanJSON(), complexDirectPlanCriticJSON()}, providerResponses: complexDirectAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--create", "create a 3-node kubeadm cluster apply workflow with one control-plane and two workers"})
	if err != nil {
		t.Fatalf("ask complex direct authoring: %v", err)
	}
	for _, want := range []string{"plan:", "plan-json:", "plan-review: plan review highlighted worker join sequencing", "ask write: ok", "wrote:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".deck", "plan", "latest.json")); err != nil {
		t.Fatalf("expected latest json plan artifact: %v", err)
	}
}

func TestAskComplexDirectEditUsesReviewedPlan(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(waitForHostsAskWorkflow("5s")), 0o644); err != nil {
		t.Fatalf("write apply: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{complexDirectEditPlanJSON(), complexDirectEditPlanCriticJSON()}, providerResponses: complexDirectEditAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--edit", "refactor this 3-node kubeadm apply workflow for rhel 9 with kubernetes v1.30.0, tar archive image loading, one control-plane and two workers; add a prepare workflow for offline package and image staging while preserving role-aware control-plane and worker behavior"})
	if err != nil {
		t.Fatalf("ask complex direct edit: %v", err)
	}
	for _, want := range []string{"plan:", "plan-json:", "plan-review: plan review approved explicit refine contract", "ask write: ok", "wrote:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
	applyContent, err := os.ReadFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"))
	if err != nil {
		t.Fatalf("read refined apply: %v", err)
	}
	if !strings.Contains(string(applyContent), "kind: InitKubeadm") || !strings.Contains(string(applyContent), "kind: JoinKubeadm") || strings.Contains(string(applyContent), "timeout: 5s") {
		t.Fatalf("expected refined apply content, got %q", string(applyContent))
	}
	if _, err := os.Stat(filepath.Join(root, "workflows", "prepare.yaml")); err != nil {
		t.Fatalf("expected prepare workflow: %v", err)
	}
}

func TestAskTerseComplexEditUsesReviewedPlan(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(strings.TrimLeft(`
version: v1alpha1
phases:
  - name: cluster
    steps:
      - id: init-control-plane
        kind: InitKubeadm
        when: vars.role == "control-plane"
        spec:
          outputJoinFile: /tmp/deck/join.txt
          podNetworkCIDR: 10.244.0.0/16
      - id: join-worker
        kind: JoinKubeadm
        when: vars.role == "worker"
        spec:
          joinFile: /tmp/deck/join.txt
  - name: verify
    steps:
      - id: verify-cluster
        kind: CheckKubernetesCluster
        when: vars.role == "control-plane"
        spec:
          interval: 5s
          timeout: 15m
          nodes:
            total: 3
            ready: 3
            controlPlaneReady: 1
`, "\n")), 0o644); err != nil {
		t.Fatalf("write apply: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{terseLocalRepoEditPlanJSON(), terseLocalRepoEditPlanCriticJSON()}, providerResponses: complexDirectEditAskResponses()}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--edit", "switch this apply workflow to local repo packages"})
	if err != nil {
		t.Fatalf("ask terse complex edit: %v", err)
	}
	for _, want := range []string{"plan:", "plan-json:", "plan-review: plan review approved local repo refine contract", "ask: plan generated with review blockers"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func TestAskComplexDirectEditStopsOnReviewedPlanBlockers(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	original := waitForHostsAskWorkflow("5s")
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(original), 0o644); err != nil {
		t.Fatalf("write apply: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{complexDirectEditPlanJSON(), complexDirectEditBlockingCriticJSON(), complexDirectEditPlanJSON(), complexDirectEditBlockingCriticJSON()}}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "--edit", "refactor this 3-node kubeadm apply workflow but leave topology details ambiguous"})
	if err != nil {
		t.Fatalf("ask complex direct blocker edit: %v", err)
	}
	if !strings.Contains(out, "plan-review: plan review found blocking review issues") || strings.Contains(out, "ask write: ok") {
		t.Fatalf("expected blocker stop output, got %q", out)
	}
	applyContent, err := os.ReadFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"))
	if err != nil {
		t.Fatalf("read blocked apply: %v", err)
	}
	if string(applyContent) != original {
		t.Fatalf("expected blocker path to leave apply unchanged")
	}
}

func TestAskPlanShowsBlockers(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{`{"version":1,"request":"create cluster workflow","intent":"draft","complexity":"complex","blockers":["missing os image details"],"targetOutcome":"Generate workflows","assumptions":[],"openQuestions":["blocking: choose base image"],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry scenario"}],"validationChecklist":["lint"]}`}}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "plan", "create air-gapped cluster workflow"})
	if err != nil {
		t.Fatalf("ask plan: %v", err)
	}
	for _, want := range []string{"plan:", "blocker: missing os image details", "next:", "deck ask --from"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in plan output, got %q", want, out)
		}
	}
}

func TestAskComplexPromptShowsJudgeFindingsAndRepairsLoosePlanJSON(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "env-key")
	root := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	originalFactory := newAskBackend
	newAskBackend = func() askprovider.Client {
		return &mockAskClient{responses: []string{
			`{"version":1,"request":"create an air-gapped rhel9 3-node kubeadm workflow","intent":"draft","complexity":"complex","authoringBrief":{"routeIntent":"draft","targetScope":"workspace","targetPaths":["workflows/prepare.yaml","workflows/scenarios/apply.yaml",],"modeIntent":"prepare+apply","connectivity":"offline","completenessTarget":"complete","topology":"multi-node","nodeCount":3,"requiredCapabilities":["prepare-artifacts","package-staging","image-staging","kubeadm-bootstrap","kubeadm-join",]},"authoringProgram":{"platform":{"family":"rhel","release":"9","repoType":"rpm"},"artifacts":{"packages":["kubeadm","kubelet","kubectl"],"images":["registry.k8s.io/kube-apiserver:v1.30.0"],"packageOutputDir":"packages/rpm/9","imageOutputDir":"images/control-plane"},"cluster":{"joinFile":"/tmp/deck/join.txt","roleSelector":"vars.role","controlPlaneCount":1,"workerCount":2},"verification":{"expectedNodeCount":3,"expectedReadyCount":3,"expectedControlPlaneReady":1}},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"},{"kind":"image","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline image flow"}],"sharedStateContracts":[{"name":"join-file","producerPath":"/tmp/deck/join.txt","consumerPaths":["/tmp/deck/join.txt"],"availabilityModel":"published-for-worker-consumption","description":"publish join file for workers"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"expectedNodeCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane"}},"artifactKinds":["package","image"],"blockers":[],"targetOutcome":"Generate workflows","assumptions":[],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"apply"},],"validationChecklist":["lint",]}`,
			`{"summary":"plan review found unresolved role layout","blocking":["topology role layout still needs clarification"],"advisory":[],"missingContracts":[],"suggestedFixes":["choose a concrete role model before generation"],"findings":[{"code":"role_cardinality_gap","severity":"blocking","message":"topology role layout still needs clarification","path":"executionModel.roleExecution","recoverable":false}]}`,
		}}
	}
	defer func() { newAskBackend = originalFactory }()

	out, err := runWithCapturedStdout([]string{"ask", "plan", "create an air-gapped rhel9 3-node kubeadm cluster workflow with prepare and apply workflows for offline package and image staging"})
	if err != nil {
		t.Fatalf("ask complex prompt: %v", err)
	}
	for _, want := range []string{"generated plan artifact", "plan:", "plan-json:", "topology.roleModel", "plan-review: plan review found unresolved role layout", "deck ask plan --from", "deck ask --from"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output, got %q", want, out)
		}
	}
}

func waitForHostsAskWorkflow(timeout string) string {
	return strings.TrimLeft(`
version: v1alpha1
phases:
  - name: apply
    steps:
      - id: wait-hosts
        kind: WaitForFile
        spec:
          path: /etc/hosts
          interval: 1s
          timeout: `+timeout+`
`, "\n")
}

func validAskWorkflow() string {
	return strings.TrimLeft(`
version: v1alpha1
phases:
  - name: verify
    steps:
      - id: verify-cluster
        kind: CheckKubernetesCluster
        spec:
          interval: 5s
          timeout: 5m
          nodes:
            total: 1
            ready: 1
            controlPlaneReady: 1
`, "\n")
}

func validAskResponses() []askprovider.Response {
	return []askprovider.Response{validAskToolResponse(), validAskFinishResponse()}
}

func repairAskResponses() []askprovider.Response {
	return []askprovider.Response{repairAskInitialResponse(), repairAskFixResponse(), validAskFinishResponse()}
}

type mockToolCall struct {
	Name       string
	Path       string
	Paths      []string
	Query      string
	Content    string
	Include    []string
	Intent     string
	Pattern    string
	Glob       string
	Offset     int
	Limit      int
	OldString  string
	NewString  string
	ReplaceAll bool
	Topic      string
	Kind       string
}

func toolResponse(calls ...mockToolCall) askprovider.Response {
	toolCalls := make([]askprovider.ToolCall, 0, len(calls))
	for i, call := range calls {
		args := map[string]any{}
		if strings.TrimSpace(call.Path) != "" {
			args["path"] = call.Path
		}
		if strings.TrimSpace(call.Query) != "" {
			args["query"] = call.Query
		}
		if strings.TrimSpace(call.Pattern) != "" {
			args["pattern"] = call.Pattern
		}
		if strings.TrimSpace(call.Glob) != "" {
			args["glob"] = call.Glob
		}
		if strings.TrimSpace(call.Content) != "" {
			args["content"] = call.Content
		}
		if len(call.Paths) > 0 {
			args["paths"] = append([]string(nil), call.Paths...)
		}
		if len(call.Include) > 0 {
			args["include"] = append([]string(nil), call.Include...)
		}
		if strings.TrimSpace(call.Intent) != "" {
			args["intent"] = call.Intent
		}
		if strings.TrimSpace(call.Topic) != "" {
			args["topic"] = call.Topic
		}
		if strings.TrimSpace(call.Kind) != "" {
			args["kind"] = call.Kind
		}
		raw, err := json.Marshal(args)
		if err != nil {
			panic(err)
		}
		toolCalls = append(toolCalls, askprovider.ToolCall{ID: fmt.Sprintf("call-%d", i+1), Name: call.Name, Arguments: raw})
	}
	return askprovider.Response{ToolCalls: toolCalls}
}

func finishResponse(summary string) askprovider.Response {
	raw, err := json.Marshal(map[string]any{"summary": summary, "reason": "deck_lint passed"})
	if err != nil {
		panic(err)
	}
	return askprovider.Response{ToolCalls: []askprovider.ToolCall{{ID: "finish-1", Name: "author_finish", Arguments: raw}}}
}

func validAskToolResponse() askprovider.Response {
	return toolResponse(
		mockToolCall{Name: "file_write", Path: "workflows/scenarios/apply.yaml", Content: validAskWorkflow()},
		mockToolCall{Name: "validate"},
	)
}

func validAskFinishResponse() askprovider.Response {
	return finishResponse("generated starter workflows")
}

func repairAskInitialResponse() askprovider.Response {
	return toolResponse(
		mockToolCall{Name: "file_write", Path: "workflows/scenarios/apply.yaml", Content: validAskWorkflow()},
		mockToolCall{Name: "validate"},
	)
}

func repairAskFixResponse() askprovider.Response {
	return toolResponse(
		mockToolCall{Name: "file_write", Path: "workflows/vars.yaml", Content: "waitPath: /etc/hosts\n"},
		mockToolCall{Name: "validate"},
	)
}

func testAPIKey() string {
	return "test-" + "api-key"
}

func testOAuthToken() string {
	return "test-" + "oauth-token"
}

func complexDirectPlanJSON() string {
	return `{"version":1,"request":"create a 3-node kubeadm cluster apply workflow","intent":"draft","complexity":"complex","authoringBrief":{"routeIntent":"draft","targetScope":"workspace","targetPaths":["workflows/scenarios/apply.yaml"],"allowedCompanionPaths":["workflows/vars.yaml"],"modeIntent":"apply-only","connectivity":"offline","completenessTarget":"complete","topology":"multi-node","nodeCount":3,"requiredCapabilities":["kubeadm-bootstrap","kubeadm-join","cluster-verification"]},"authoringProgram":{"cluster":{"joinFile":"/tmp/deck/join.txt","roleSelector":"vars.role","controlPlaneCount":1,"workerCount":2},"verification":{"expectedNodeCount":3,"expectedReadyCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane","interval":"5s","timeout":"15m"}},"executionModel":{"sharedStateContracts":[{"name":"join-file","producerPath":"/tmp/deck/join.txt","consumerPaths":["/tmp/deck/join.txt"],"availabilityModel":"published-for-worker-consumption","description":"publish join file for workers"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"expectedNodeCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane"}},"blockers":[],"targetOutcome":"Generate reviewed workflows","assumptions":[],"openQuestions":[],"clarifications":[{"id":"topology.roleModel","question":"Which role layout should the plan use?","kind":"topology","decision":"defaulted","options":["1cp-2workers"],"recommendedDefault":"1cp-2workers","answer":"1cp-2workers","blocksGeneration":false,"affects":["authoringProgram.cluster.controlPlaneCount","authoringProgram.cluster.workerCount"]}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry scenario"},{"path":"workflows/vars.yaml","kind":"vars","action":"create","purpose":"role selector values"}],"validationChecklist":["lint"]}`
}

func complexDirectPlanCriticJSON() string {
	return `{"summary":"plan review highlighted worker join sequencing","blocking":[],"advisory":["worker join publication needs explicit sequencing"],"missingContracts":[],"suggestedFixes":["publish the join contract before worker consumption"],"findings":[{"code":"worker_join_fanout_gap","severity":"advisory","message":"worker join publication needs explicit sequencing","path":"workflows/scenarios/apply.yaml","recoverable":true}]}`
}

func terseLocalRepoEditPlanJSON() string {
	return `{"version":1,"request":"switch this apply workflow to local repo packages","intent":"refine","complexity":"medium","authoringBrief":{"routeIntent":"refine","targetScope":"workspace","targetPaths":["workflows/prepare.yaml","workflows/scenarios/apply.yaml"],"anchorPaths":["workflows/scenarios/apply.yaml"],"allowedCompanionPaths":["workflows/vars.yaml"],"modeIntent":"prepare+apply","connectivity":"offline","completenessTarget":"refine","topology":"multi-node","nodeCount":3,"platformFamily":"rhel","requiredCapabilities":["package-staging","prepare-artifacts","kubeadm-bootstrap","kubeadm-join","cluster-verification"]},"authoringProgram":{"platform":{"family":"rhel","release":"9","repoType":"local-repo"},"artifacts":{"packages":["kubeadm","kubelet","kubectl","containerd"]},"cluster":{"joinFile":"/tmp/deck/join.txt","roleSelector":"vars.role","controlPlaneCount":1,"workerCount":2},"verification":{"expectedNodeCount":3,"expectedReadyCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane","interval":"5s","timeout":"15m"}},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"},{"kind":"repository-setup","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"repo metadata flow"}],"sharedStateContracts":[{"name":"join-file","producerPath":"/tmp/deck/join.txt","consumerPaths":["/tmp/deck/join.txt"],"availabilityModel":"published-for-worker-consumption","description":"publish join file for workers"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"expectedNodeCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane"}},"artifactKinds":["package"],"blockers":["Platform family/release and local repository delivery method are not specified, so repo configuration cannot be generated safely.","Exact package set/version pinning for Kubernetes and container runtime is not specified, so prepare package staging cannot be made deterministic."],"targetOutcome":"Refine reviewed workflows","assumptions":[],"openQuestions":[],"clarifications":[{"id":"runtime.platformFamily","question":"This request depends on distro-specific package or repository behavior, but the target platform family is not explicit. Which platform family should the plan target?","kind":"enum","decision":"runtime","options":["rhel","debian","custom"],"recommendedDefault":"rhel","blocksGeneration":true,"affects":["authoringBrief.platformFamily","validationChecklist"]},{"id":"repo-delivery","question":"How will apply hosts access the local repository content?","kind":"choice","decision":"scope","options":["http mirror served inside environment","filesystem-mounted repo on each node","copied repo payload unpacked locally on each node"],"recommendedDefault":"http mirror served inside environment","blocksGeneration":true,"affects":["executionModel.artifactContracts"]},{"id":"kubernetes-version","question":"What Kubernetes version should package staging and install target?","kind":"string","decision":"version","recommendedDefault":"v1.30.0","blocksGeneration":true,"affects":["authoringProgram.cluster.kubernetesVersion"]}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare workflow"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"update","purpose":"refined apply workflow"}],"validationChecklist":["lint"]}`
}

func terseLocalRepoEditPlanCriticJSON() string {
	return `{"summary":"plan review approved local repo refine contract","blocking":[],"advisory":["worker join publication should remain explicit"],"missingContracts":[],"suggestedFixes":["preserve join publication semantics during refine"],"findings":[{"code":"worker_join_fanout_gap","severity":"warning","message":"worker join publication should remain explicit","path":"workflows/scenarios/apply.yaml","recoverable":true}]}`
}

func complexDirectEditPlanJSON() string {
	return `{"version":1,"request":"refine the existing 3-node kubeadm apply workflow for rhel9","intent":"refine","complexity":"complex","authoringBrief":{"routeIntent":"refine","targetScope":"workspace","targetPaths":["workflows/prepare.yaml","workflows/scenarios/apply.yaml"],"anchorPaths":["workflows/scenarios/apply.yaml"],"modeIntent":"prepare+apply","connectivity":"offline","completenessTarget":"refine","topology":"multi-node","nodeCount":3,"platformFamily":"rhel","requiredCapabilities":["prepare-artifacts","package-staging","image-staging","kubeadm-bootstrap","kubeadm-join","cluster-verification"]},"authoringProgram":{"platform":{"family":"rhel","release":"9","repoType":"offline-local"},"artifacts":{"packages":["kubeadm","kubelet","kubectl"],"images":["registry.k8s.io/kube-apiserver:v1.30.0"],"packageOutputDir":"/tmp/deck/artifacts/packages","imageOutputDir":"/tmp/deck/artifacts/images"},"cluster":{"joinFile":"/tmp/deck/join.txt","kubernetesVersion":"v1.30.0","roleSelector":"vars.role","controlPlaneCount":1,"workerCount":2},"verification":{"expectedNodeCount":3,"expectedReadyCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane","interval":"5s","timeout":"15m"}},"executionModel":{"artifactContracts":[{"kind":"package","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline package flow"},{"kind":"image","producerPath":"workflows/prepare.yaml","consumerPath":"workflows/scenarios/apply.yaml","description":"offline image flow"}],"sharedStateContracts":[{"name":"join-file","producerPath":"/tmp/deck/join.txt","consumerPaths":["/tmp/deck/join.txt"],"availabilityModel":"published-for-worker-consumption","description":"publish join file for workers"}],"roleExecution":{"roleSelector":"vars.role","controlPlaneFlow":"bootstrap","workerFlow":"join","perNodeInvocation":true},"verification":{"expectedNodeCount":3,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane"}},"blockers":[],"targetOutcome":"Refine reviewed workflows","assumptions":[],"openQuestions":[],"clarifications":[{"id":"runtime.platformFamily","question":"Which platform family should the plan target?","kind":"environment","decision":"defaulted","options":["rhel"],"recommendedDefault":"rhel","answer":"rhel","blocksGeneration":false,"affects":["authoringBrief.platformFamily","validationChecklist"]}],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/prepare.yaml","kind":"workflow","action":"create","purpose":"prepare workflow"},{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"update","purpose":"refined apply workflow"}],"validationChecklist":["lint"]}`
}

func complexDirectEditPlanCriticJSON() string {
	return `{"summary":"plan review approved explicit refine contract","blocking":[],"advisory":["worker join publication needs explicit sequencing"],"missingContracts":[],"suggestedFixes":["preserve join publication semantics during refine"],"findings":[{"code":"worker_join_fanout_gap","severity":"advisory","message":"worker join publication needs explicit sequencing","path":"workflows/scenarios/apply.yaml","recoverable":true}]}`
}

func complexDirectEditBlockingCriticJSON() string {
	return `{"summary":"plan review found blocking review issues","blocking":["refine plan still needs manual review"],"advisory":[],"missingContracts":[],"suggestedFixes":["tighten the refine contract before execution"],"findings":[{"code":"ask_unclassified_critic_finding","severity":"blocking","message":"refine plan still needs manual review","path":"executionModel","recoverable":false}]}`
}

func complexDirectEditAskResponses() []askprovider.Response {
	return []askprovider.Response{
		toolResponse(
			mockToolCall{Name: "file_write", Path: "workflows/prepare.yaml", Content: strings.TrimLeft(`
version: v1alpha1
steps:
  - id: stage-packages
    kind: DownloadPackage
    spec:
      packages: [kubeadm, kubelet, kubectl]
      distro:
        family: rhel
        release: "9"
      repo:
        type: rpm
  - id: stage-images
    kind: DownloadImage
    spec:
      images:
        - registry.k8s.io/kube-apiserver:v1.30.0
      outputDir: images/control-plane
`, "\n")},
			mockToolCall{Name: "file_write", Path: "workflows/scenarios/apply.yaml", Content: strings.TrimLeft(`
version: v1alpha1
phases:
  - name: runtime
    steps:
      - id: install-k8s-packages
        kind: InstallPackage
        spec:
          packages: [kubeadm, kubelet, kubectl]
          source:
            type: local-repo
            path: /tmp/deck/artifacts/packages
      - id: init-control-plane
        kind: InitKubeadm
        when: vars.role == "control-plane"
        spec:
          outputJoinFile: /tmp/deck/join.txt
          kubernetesVersion: v1.30.0
          podNetworkCIDR: 10.244.0.0/16
      - id: join-worker
        kind: JoinKubeadm
        when: vars.role == "worker"
        spec:
          joinFile: /tmp/deck/join.txt
  - name: verify
    steps:
      - id: verify-cluster
        kind: CheckKubernetesCluster
        when: vars.role == "control-plane"
        spec:
          interval: 5s
          timeout: 15m
          nodes:
            total: 3
            ready: 3
            controlPlaneReady: 1
`, "\n")},
			mockToolCall{Name: "validate"},
		),
		finishResponse("generated reviewed workflows"),
	}
}

func complexDirectAskResponses() []askprovider.Response {
	return []askprovider.Response{
		toolResponse(
			mockToolCall{Name: "file_write", Path: "workflows/scenarios/apply.yaml", Content: strings.TrimLeft(`
version: v1alpha1
phases:
  - name: cluster
    steps:
      - id: init-control-plane
        kind: InitKubeadm
        when: vars.role == "control-plane"
        spec:
          outputJoinFile: /tmp/deck/join.txt
          podNetworkCIDR: 10.244.0.0/16
      - id: join-worker
        kind: JoinKubeadm
        when: vars.role == "worker"
        spec:
          joinFile: /tmp/deck/join.txt
  - name: verify
    steps:
      - id: verify-cluster
        kind: CheckKubernetesCluster
        when: vars.role == "control-plane"
        spec:
          interval: 5s
          timeout: 15m
          nodes:
            total: 3
            ready: 3
            controlPlaneReady: 1
`, "\n")},
			mockToolCall{Name: "file_write", Path: "workflows/vars.yaml", Content: "role: control-plane\njoinFile: /tmp/deck/join.txt\n"},
			mockToolCall{Name: "validate"},
		),
		finishResponse("generated reviewed workflows"),
	}
}

func validPlanJSON() string {
	return `{"version":1,"request":"create single-node cluster workflow","intent":"draft","complexity":"medium","authoringBrief":{"routeIntent":"draft","targetScope":"workspace","targetPaths":["workflows/scenarios/apply.yaml"],"modeIntent":"apply-only","connectivity":"offline","completenessTarget":"complete","topology":"single-node","nodeCount":1,"requiredCapabilities":["kubeadm-bootstrap","cluster-verification"]},"authoringProgram":{"cluster":{"joinFile":"/tmp/deck/join.txt","controlPlaneCount":1},"verification":{"expectedNodeCount":1,"expectedReadyCount":1,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane","interval":"5s","timeout":"5m"}},"executionModel":{"verification":{"expectedNodeCount":1,"expectedControlPlaneReady":1,"finalVerificationRole":"control-plane"}},"blockers":[],"targetOutcome":"Generate workflows","assumptions":["Use v1alpha1"],"openQuestions":[],"entryScenario":"workflows/scenarios/apply.yaml","files":[{"path":"workflows/scenarios/apply.yaml","kind":"scenario","action":"create","purpose":"entry scenario"}],"validationChecklist":["lint"]}`
}
