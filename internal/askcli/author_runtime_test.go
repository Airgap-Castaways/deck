package askcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

type testToolCall struct {
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

func agentToolCallsResponse(t *testing.T, _ string, calls ...testToolCall) askprovider.Response {
	t.Helper()
	toolCalls := make([]askprovider.ToolCall, 0, len(calls))
	for i, call := range calls {
		args := map[string]any{}
		if strings.TrimSpace(call.Path) != "" {
			args["path"] = call.Path
		}
		if len(call.Paths) > 0 {
			args["paths"] = append([]string(nil), call.Paths...)
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
		if call.Offset > 0 {
			args["offset"] = call.Offset
		}
		if call.Limit > 0 {
			args["limit"] = call.Limit
		}
		if call.OldString != "" || call.NewString != "" {
			args["old_string"] = call.OldString
			args["new_string"] = call.NewString
			args["replace_all"] = call.ReplaceAll
		}
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("marshal tool args: %v", err)
		}
		toolCalls = append(toolCalls, askprovider.ToolCall{ID: fmt.Sprintf("call-%d", i+1), Name: call.Name, Arguments: raw})
	}
	return askprovider.Response{ToolCalls: toolCalls}
}

func agentFinishResponse(t *testing.T, summary string) askprovider.Response {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"summary": summary, "reason": summary})
	if err != nil {
		t.Fatalf("marshal finish args: %v", err)
	}
	return askprovider.Response{ToolCalls: []askprovider.ToolCall{{ID: "finish-1", Name: authorToolFinish, Arguments: raw}}}
}

func agentWriteLintFinishResponses(t *testing.T, file askcontract.GeneratedFile) []askprovider.Response {
	t.Helper()
	return []askprovider.Response{
		agentToolCallsResponse(t, "stage candidate", testToolCall{Name: "file_write", Path: file.Path, Content: file.Content}, testToolCall{Name: "validate"}),
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

func invalidCommandWorkflow() string {
	return strings.TrimLeft(`
version: v1alpha1
steps:
  - id: run
    kind: Command
    spec:
      commands: ["true"]
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
	client := &stubClient{providerResponses: agentWriteLintFinishResponses(t, file)}
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
	client := &stubClient{providerResponses: agentWriteLintFinishResponses(t, file)}
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
	write := session.runFileWrite(authorToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})
	if !write.OK {
		t.Fatalf("expected candidate write to succeed, got %s", renderAgentPayload(write.Payload))
	}
	search := session.runGrep(authorToolCall{Name: "grep", Pattern: "wait-hosts"})
	if !search.OK || !strings.Contains(renderAgentPayload(search.Payload), workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("expected search to find candidate file, got %s", renderAgentPayload(search.Payload))
	}
	read := session.runRead(authorToolCall{Name: "read", Path: workspacepaths.CanonicalApplyWorkflow})
	if !read.OK || !strings.Contains(renderAgentPayload(read.Payload), "timeout: 5s") {
		t.Fatalf("expected read to return candidate content, got %s", renderAgentPayload(read.Payload))
	}
}

func TestAuthoringAgentGlobAndReadSupportCandidateState(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create a simple apply workflow", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	write := session.runFileWrite(authorToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})
	if !write.OK {
		t.Fatalf("expected candidate write to succeed, got %s", renderAgentPayload(write.Payload))
	}
	globResult := session.runGlob(authorToolCall{Name: "glob", Pattern: "*.yaml"})
	if !globResult.OK || !strings.Contains(renderAgentPayload(globResult.Payload), workspacepaths.CanonicalApplyWorkflow) {
		t.Fatalf("expected glob to find candidate workflow, got %s", renderAgentPayload(globResult.Payload))
	}
	read := session.runRead(authorToolCall{Name: "read", Path: workspacepaths.CanonicalApplyWorkflow, Offset: 7, Limit: 3})
	payload := renderAgentPayload(read.Payload)
	if !read.OK || !strings.Contains(payload, "7:         spec:") || !strings.Contains(payload, `"truncated": true`) {
		t.Fatalf("expected ranged read payload, got %s", payload)
	}
}

func TestAuthoringAgentFileEditUsesReadSnapshot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "workflows", "scenarios"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "workflows", "scenarios", "apply.yaml"), []byte(waitForHostsWorkflow("5s")), 0o600); err != nil {
		t.Fatalf("write apply workflow: %v", err)
	}
	session := newTestAgentSession(t, root, "refine apply workflow", askintent.Decision{Route: askintent.RouteRefine, Target: askintent.Target{Kind: "scenario", Path: workspacepaths.CanonicalApplyWorkflow}})
	editWithoutRead := session.runFileEdit(authorToolCall{Name: "file_edit", Path: workspacepaths.CanonicalApplyWorkflow, OldString: "timeout: 5s", NewString: "timeout: 10s"})
	if editWithoutRead.OK {
		t.Fatalf("expected file_edit to require a prior full read for existing files")
	}
	read := session.runRead(authorToolCall{Name: "read", Path: workspacepaths.CanonicalApplyWorkflow})
	if !read.OK {
		t.Fatalf("expected full read to succeed, got %s", renderAgentPayload(read.Payload))
	}
	edit := session.runFileEdit(authorToolCall{Name: "file_edit", Path: workspacepaths.CanonicalApplyWorkflow, OldString: "timeout: 5s", NewString: "timeout: 10s"})
	if !edit.OK {
		t.Fatalf("expected file_edit to succeed after read, got %s", renderAgentPayload(edit.Payload))
	}
	candidate := session.candidateByPath[workspacepaths.CanonicalApplyWorkflow]
	if !strings.Contains(candidate.Content, "timeout: 10s") {
		t.Fatalf("expected edited candidate content, got %q", candidate.Content)
	}
}

func TestAuthoringAgentStagesInvalidCandidateForRepair(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create a simple apply workflow", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	write := session.runFileWrite(authorToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: invalidCommandWorkflow()})
	if !write.OK {
		t.Fatalf("expected invalid workflow to stage for repair, got %s", renderAgentPayload(write.Payload))
	}
	read := session.runRead(authorToolCall{Name: "read", Path: workspacepaths.CanonicalApplyWorkflow})
	if !read.OK || !strings.Contains(renderAgentPayload(read.Payload), "commands:") {
		t.Fatalf("expected read to return staged invalid content, got %s", renderAgentPayload(read.Payload))
	}
	lint := session.runValidate(context.Background(), authorToolCall{Name: "validate"})
	if lint.OK {
		t.Fatalf("expected staged invalid workflow to fail lint")
	}
	payload := renderAgentPayload(lint.Payload)
	if !strings.Contains(payload, "command is required") || !strings.Contains(payload, "repairContext") || !strings.Contains(payload, "command.schema.json") {
		t.Fatalf("expected command repair context, got %s", payload)
	}
	if _, ok := session.candidateByPath[workspacepaths.CanonicalApplyWorkflow]; !ok {
		t.Fatalf("expected invalid candidate to remain available for repair")
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
	result := session.runInit()
	if result.OK {
		t.Fatalf("expected deck_init to be disabled when workflow tree exists")
	}
}

func TestAuthoringAgentRejectsWritesOutsideApprovedScope(t *testing.T) {
	root := t.TempDir()
	session := newTestAgentSession(t, root, "create a simple apply workflow", askintent.Decision{Route: askintent.RouteDraft, Target: askintent.Target{Kind: "workspace"}})
	result := session.runFileWrite(authorToolCall{Name: "file_write", Path: "workflows/components/helper.yaml", Content: "steps: []\n"})
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
	session.runFileWrite(authorToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")})
	result := session.runValidate(context.Background(), authorToolCall{Name: "validate"})
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
	invalid := agentToolCallsResponse(t, "write apply only", testToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")}, testToolCall{Name: "validate"})
	fix := agentToolCallsResponse(t, "add vars", testToolCall{Name: "file_write", Path: workspacepaths.CanonicalVarsWorkflow, Content: "waitPath: /etc/hosts\n"}, testToolCall{Name: "validate"})
	client := &stubClient{providerResponses: []askprovider.Response{invalid, fix, agentFinishResponse(t, "generated workflows")}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "create apply workflow with vars", Create: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	state, err := askstate.Load(root)
	if err != nil {
		t.Fatalf("load ask state: %v", err)
	}
	if len(state.LastToolCalls) < 4 || !strings.Contains(strings.Join(state.LastToolCalls, ","), "validate") {
		t.Fatalf("expected repeated lint calls in tool transcript, got %#v", state)
	}
}

func TestExecuteRepairsInvalidCandidateAfterLintFailure(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	invalid := agentToolCallsResponse(t, "stage invalid command step", testToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: invalidCommandWorkflow()}, testToolCall{Name: "validate"})
	fix := agentToolCallsResponse(t, "rewrite command step", testToolCall{Name: "file_write", Path: workspacepaths.CanonicalApplyWorkflow, Content: "version: v1alpha1\nsteps:\n  - id: run\n    kind: Command\n    spec:\n      command: [\"true\"]\n"}, testToolCall{Name: "validate"})
	client := &stubClient{providerResponses: []askprovider.Response{invalid, fix, agentFinishResponse(t, "generated workflows")}}
	if err := Execute(context.Background(), Options{Root: root, Prompt: "create apply workflow with a command step", Create: true, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: io.Discard}, client); err != nil {
		t.Fatalf("execute: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(workspacepaths.CanonicalApplyWorkflow)))
	if err != nil {
		t.Fatalf("read repaired workflow: %v", err)
	}
	if strings.Contains(string(content), "commands:") || !strings.Contains(string(content), "command: [\"true\"]") {
		t.Fatalf("expected repaired command workflow, got %q", string(content))
	}
}

func TestAuthoringAgentPersistsTranscriptFile(t *testing.T) {
	t.Setenv("DECK_ASK_API_KEY", "test-key")
	root := t.TempDir()
	stdout := &bytes.Buffer{}
	file := askcontract.GeneratedFile{Path: workspacepaths.CanonicalApplyWorkflow, Content: waitForHostsWorkflow("5s")}
	client := &stubClient{providerResponses: agentWriteLintFinishResponses(t, file)}
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
