package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askpolicy"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func agentToolCallsResponse(t *testing.T, summary string, calls ...askcontract.AgentToolCall) string {
	t.Helper()
	raw, err := json.Marshal(askcontract.AgentTurnResponse{Summary: summary, Review: nil, ToolCalls: calls})
	if err != nil {
		t.Fatalf("marshal agent tool calls: %v", err)
	}
	return string(raw)
}

func agentFinishResponse(t *testing.T, summary string) string {
	t.Helper()
	raw, err := json.Marshal(askcontract.AgentTurnResponse{Summary: summary, Review: nil, Finish: &askcontract.AgentFinish{Reason: summary}})
	if err != nil {
		t.Fatalf("marshal agent finish: %v", err)
	}
	return string(raw)
}

func agentWriteLintFinishResponses(t *testing.T, file askcontract.GeneratedFile) []string {
	t.Helper()
	return []string{
		agentToolCallsResponse(t, "stage candidate", askcontract.AgentToolCall{Name: "file_write", Path: file.Path, Content: file.Content}, askcontract.AgentToolCall{Name: "deck_lint"}),
		agentFinishResponse(t, "generated workflows"),
	}
}

func waitForHostsWorkflow(timeout string) string {
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

func kubernetesWaitWorkflow() string {
	return strings.TrimLeft(`
version: v1alpha1
phases:
  - name: apply
    steps:
      - id: wait-version
        kind: WaitForFile
        spec:
          path: /tmp/kubernetes-1.35.1
          interval: 1s
          timeout: 5s
`, "\n")
}

func newTestAgentSession(t *testing.T, root string, prompt string, decision askintent.Decision) *authoringAgentSession {
	t.Helper()
	workspace, err := askretrieve.InspectWorkspace(root)
	if err != nil {
		t.Fatalf("inspect workspace: %v", err)
	}
	plan, requirements := askpolicy.BuildAuthoringPreflight(prompt, askretrieve.RetrievalResult{}, workspace, decision, nil)
	return newAuthoringAgentSession(root, prompt, decision, plan, requirements, workspace, askstate.Context{}, askretrieve.RetrievalResult{}, askconfig.EffectiveSettings{Settings: askconfig.Settings{Provider: "openai", Model: "gpt-5.4", APIKey: "test-key"}}, askcontract.EvidencePlan{Decision: "unnecessary"}, newAskLogger(io.Discard, "trace"), 3, 6)
}

func TestExecuteUsesAgentDraftToolLoop(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	file := askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")}
	client := &stubClient{responses: agentWriteLintFinishResponses(t, file)}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "create a single apply workflow that waits for /etc/hosts", Create: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "ask write: ok") {
		t.Fatalf("expected successful write, got %q", stdout.String())
	}
	if client.calls != 2 {
		t.Fatalf("expected two llm turns, got %d", client.calls)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workspacepaths.CanonicalApplyWorkflow)))
	if err != nil {
		t.Fatalf("read apply workflow: %v", err)
	}
	if !strings.Contains(string(content), "kind: WaitForFile") {
		t.Fatalf("expected generated workflow, got %q", string(content))
	}
	state, err := askstate.Load(root)
	if err != nil {
		t.Fatalf("load ask state: %v", err)
	}
	if len(state.LastToolCalls) == 0 || state.LastToolTranscript == "" {
		t.Fatalf("expected persisted tool transcript metadata, got %#v", state)
	}
}

func TestExecuteUsesAgentRefineToolLoop(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(waitForHostsWorkflow("5s")), 0o600); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	stdout := &bytes.Buffer{}
	file := askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("30s")}
	client := &stubClient{responses: agentWriteLintFinishResponses(t, file)}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "refine workflows/scenarios/apply.yaml to wait longer for /etc/hosts", Edit: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "ask write: ok") {
		t.Fatalf("expected successful refine write, got %q", stdout.String())
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workspacepaths.CanonicalApplyWorkflow)))
	if err != nil {
		t.Fatalf("read apply workflow: %v", err)
	}
	if !strings.Contains(string(content), "timeout: 30s") {
		t.Fatalf("expected updated workflow content, got %q", string(content))
	}
}

func TestAuthoringAgentSearchAndReadUseCandidateState(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create a simple apply workflow", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	write := session.runFileWrite(askcontract.AgentToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})
	if !write.OK {
		t.Fatalf("expected candidate write to succeed, got %s", renderAgentPayload(write.Payload))
	}
	search := session.runFileSearch(askcontract.AgentToolCall{Name: "file_search", Query: "wait-hosts"})
	if !search.OK || !strings.Contains(renderAgentPayload(search.Payload), workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("expected search to find candidate file, got %s", renderAgentPayload(search.Payload))
	}
	read := session.runFileRead(askcontract.AgentToolCall{Name: "file_read", Path: workspacepaths.CanonicalApplyWorkflow})
	if !read.OK || !strings.Contains(renderAgentPayload(read.Payload), "timeout: 5s") {
		t.Fatalf("expected read to return candidate content, got %s", renderAgentPayload(read.Payload))
	}
}

func TestAuthoringAgentDeckInitIsDisabledWhenWorkflowTreeExists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(waitForHostsWorkflow("5s")), 0o600); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	session := newTestAgentSession(t, root, "refine workflows/scenarios/apply.yaml", askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: workspacepaths.CanonicalApplyWorkflow}})
	result := session.runDeckInit()
	if result.OK {
		t.Fatalf("expected deck_init to be disabled when workflow tree exists")
	}
}

func TestAuthoringAgentRejectsWritesOutsideApprovedScope(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create a simple apply workflow", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	result := session.runFileWrite(askcontract.AgentToolCall{Name: "file_write", Path: "workflows/components/helper.yaml", Content: "steps: []\n"})
	if result.OK {
		t.Fatalf("expected write outside scope to fail")
	}
	if !strings.Contains(renderAgentPayload(result.Payload), "outside the approved write scope") {
		t.Fatalf("expected scope rejection, got %s", renderAgentPayload(result.Payload))
	}
}

func TestAuthoringAgentLintReturnsStructuredDiagnostics(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create apply workflow with vars", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	session.runFileWrite(askcontract.AgentToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})
	result := session.runDeckLint(context.Background(), askcontract.AgentToolCall{Name: "deck_lint"})
	if result.OK {
		t.Fatalf("expected missing planned vars file to fail lint")
	}
	payload := renderAgentPayload(result.Payload)
	if !strings.Contains(payload, "diagnostics") || !strings.Contains(payload, workspacepaths.CanonicalVarsWorkflow) {
		t.Fatalf("expected structured diagnostics payload, got %s", payload)
	}
	if session.verificationFailure != 1 {
		t.Fatalf("expected verification failure count to increment, got %d", session.verificationFailure)
	}
}

func TestAuthoringAgentRepairsAfterLintFailure(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	invalid := agentToolCallsResponse(t, "write apply only", askcontract.AgentToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")}, askcontract.AgentToolCall{Name: "deck_lint"})
	fix := agentToolCallsResponse(t, "add vars", askcontract.AgentToolCall{Name: "file_write", Path: workspacepaths.CanonicalVarsWorkflow, Content: "waitPath: /etc/hosts\n"}, askcontract.AgentToolCall{Name: "deck_lint"})
	client := &stubClient{responses: []string{invalid, fix, agentFinishResponse(t, "generated workflows")}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "create apply workflow with vars", Create: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	state, err := askstate.Load(root)
	if err != nil {
		t.Fatalf("load ask state: %v", err)
	}
	if len(state.LastToolCalls) < 4 || !strings.Contains(strings.Join(state.LastToolCalls, ","), "deck_lint") {
		t.Fatalf("expected repeated lint calls in tool transcript, got %#v", state)
	}
}

func TestAuthoringAgentPersistsTranscriptFile(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	file := askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")}
	client := &stubClient{responses: agentWriteLintFinishResponses(t, file)}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "create a simple apply workflow", Create: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	session, err := askstate.LoadAgentSession(root)
	if err != nil {
		t.Fatalf("load agent session: %v", err)
	}
	if session.Route != string(askintent.RouteDraft) || len(session.ToolEvents) == 0 || len(session.CandidateFiles) == 0 {
		t.Fatalf("expected persisted agent session, got %#v", session)
	}
	if !strings.Contains(strings.Join(session.ApprovedPaths, ","), workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("expected approved apply path in session, got %#v", session.ApprovedPaths)
	}
}

func TestAgentSessionUsesCurrentAskContextBundle(t *testing.T) {
	if askcontext.CurrentBundle().Workflow.SupportedVersion == "" {
		t.Fatalf("expected ask context bundle to be available")
	}
}

var _ askprovider.Client = (*stubClient)(nil)
