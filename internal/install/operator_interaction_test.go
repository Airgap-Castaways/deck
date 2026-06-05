package install

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/operatorio"
)

type fakeInteraction struct {
	messages   []recordedMessage
	confirm    bool
	input      string
	inputs     []string
	inputCalls int
}

type recordedMessage struct {
	level   string
	message string
	stream  string
}

func (f *fakeInteraction) Message(level, message, stream string) error {
	f.messages = append(f.messages, recordedMessage{level: level, message: message, stream: stream})
	return nil
}

func (f *fakeInteraction) Confirm(context.Context, string, *bool) (bool, error) {
	return f.confirm, nil
}

func (f *fakeInteraction) Input(context.Context, string, operatorio.InputOptions) (string, error) {
	f.inputCalls++
	if len(f.inputs) > 0 {
		value := f.inputs[0]
		f.inputs = f.inputs[1:]
		return value, nil
	}
	return f.input, nil
}

func TestRun_MessageRendersTemplateAndWritesStream(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	interaction := &fakeInteraction{}
	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name:  "apply",
			Steps: []config.Step{{ID: "show", Kind: "Message", Spec: map[string]any{"level": "warn", "stream": "stderr", "message": "role={{ .vars.role }}\nnext"}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(interaction.messages) != 1 {
		t.Fatalf("expected one message, got %d", len(interaction.messages))
	}
	got := interaction.messages[0]
	if got.level != "warn" || got.stream != "stderr" || got.message != "role=control-plane\nnext" {
		t.Fatalf("unexpected message: %+v", got)
	}
}

func TestRun_ConfirmRegistersFalseWhenOnNoContinues(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	interaction := &fakeInteraction{confirm: false}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "apply",
			Steps: []config.Step{{ID: "ask", Kind: "Confirm", Register: map[string]string{"doReset": "confirmed"}, Spec: map[string]any{"message": "Reset?", "onNo": "continue"}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	var st State
	readStateForInteractionTest(t, statePath, &st)
	if got, ok := st.RuntimeVars["doReset"].(bool); !ok || got {
		t.Fatalf("expected doReset=false, got %#v", st.RuntimeVars["doReset"])
	}
}

func TestRun_ConfirmNoFailsByDefault(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	interaction := &fakeInteraction{confirm: false}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "apply",
			Steps: []config.Step{{ID: "ask", Kind: "Confirm", Spec: map[string]any{"message": "Continue?"}}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction})
	if err == nil {
		t.Fatalf("expected interaction error")
	}
	if !errcode.Is(err, errCodeInstallInteraction) && !strings.Contains(err.Error(), errCodeInstallInteraction) {
		t.Fatalf("expected interaction error, got %v", err)
	}
}

func TestRun_InputRegistersValue(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	interaction := &fakeInteraction{input: "192.0.2.10"}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "apply",
			Steps: []config.Step{{ID: "ask", Kind: "Input", Register: map[string]string{"nodeIP": "value"}, Spec: map[string]any{"message": "Node IP"}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	var st State
	readStateForInteractionTest(t, statePath, &st)
	if got := st.RuntimeVars["nodeIP"]; got != "192.0.2.10" {
		t.Fatalf("expected nodeIP runtime value, got %#v", got)
	}
}

func TestRun_NonInteractiveDefaults(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "apply",
			Steps: []config.Step{
				{ID: "confirm", Kind: "Confirm", Register: map[string]string{"approved": "confirmed"}, Spec: map[string]any{"message": "Continue?", "default": true}},
				{ID: "input", Kind: "Input", Register: map[string]string{"answer": "value"}, Spec: map[string]any{"message": "Value", "default": "from-default"}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, NonInteractive: true}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	var st State
	readStateForInteractionTest(t, statePath, &st)
	if got := st.RuntimeVars["approved"]; got != true {
		t.Fatalf("expected approved=true, got %#v", got)
	}
	if got := st.RuntimeVars["answer"]; got != "from-default" {
		t.Fatalf("expected answer default, got %#v", got)
	}
}

func TestRun_SecretInputRegistersInMemoryButNotState(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	outputPath := filepath.Join(t.TempDir(), "out.txt")
	interaction := &fakeInteraction{input: "s3cr3t-token"}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "apply",
			Steps: []config.Step{
				{ID: "secret", Kind: "Input", Register: map[string]string{"token": "value"}, Spec: map[string]any{"message": "Token", "secret": true}},
				{ID: "write", Kind: "WriteFile", Spec: map[string]any{"path": outputPath, "content": "{{ .runtime.token }}"}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(string(written)) != "s3cr3t-token" {
		t.Fatalf("expected secret to be available in-memory, got %q", string(written))
	}
	var st State
	readStateForInteractionTest(t, statePath, &st)
	if _, ok := st.RuntimeVars["token"]; ok {
		t.Fatalf("secret runtime value was persisted: %#v", st.RuntimeVars["token"])
	}
	if got := st.RuntimeSecrets["token"]; got.Phase != "apply" || got.StepID != "secret" || got.Output != "value" {
		t.Fatalf("unexpected runtime secret metadata: %+v", got)
	}
	if !contains(st.CompletedPhases, "apply") {
		t.Fatalf("completed secret-producing workflow should keep completed phase: %#v", st.CompletedPhases)
	}
	rawState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read raw state: %v", err)
	}
	if strings.Contains(string(rawState), "s3cr3t-token") {
		t.Fatalf("raw state contains secret: %s", rawState)
	}
}

func TestRun_CompletedSecretInputDoesNotReprompt(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	outputPath := filepath.Join(dir, "out.txt")
	interaction := &fakeInteraction{inputs: []string{"first-secret", "second-secret"}}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "collect",
			Steps: []config.Step{
				{ID: "secret", Kind: "Input", Register: map[string]string{"token": "value"}, Spec: map[string]any{"message": "Token", "secret": true}},
				{ID: "write", Kind: "WriteFile", Spec: map[string]any{"path": outputPath, "content": "{{ .runtime.token }}"}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if interaction.inputCalls != 1 {
		t.Fatalf("expected completed workflow to skip re-prompt, got %d input calls", interaction.inputCalls)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(string(written)) != "first-secret" {
		t.Fatalf("expected original output after completed rerun, got %q", string(written))
	}
}

func TestRun_IncompleteSecretInputRepromptsOnResume(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	outputPath := filepath.Join(dir, "out.txt")
	interaction := &fakeInteraction{inputs: []string{"first-secret", "second-secret"}}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{
			{
				Name: "collect",
				Steps: []config.Step{
					{ID: "secret", Kind: "Input", Register: map[string]string{"token": "value"}, Spec: map[string]any{"message": "Token", "secret": true}},
					{ID: "write", Kind: "WriteFile", Spec: map[string]any{"path": outputPath, "content": "{{ .runtime.token }}"}},
				},
			},
			{
				Name:  "fail",
				Steps: []config.Step{{ID: "fail-command", Kind: "Command", Spec: map[string]any{"command": []any{"false"}}}},
			},
		},
	}

	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err == nil {
		t.Fatalf("expected first run to fail")
	}
	if err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction}); err == nil {
		t.Fatalf("expected second run to fail")
	}
	if interaction.inputCalls != 2 {
		t.Fatalf("expected re-prompt on incomplete resume, got %d input calls", interaction.inputCalls)
	}
	written, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(string(written)) != "second-secret" {
		t.Fatalf("expected second secret after incomplete resume, got %q", string(written))
	}
}

func TestRun_MessageRejectsSecretRuntimeValue(t *testing.T) {
	statePath := t.TempDir() + "/state.json"
	interaction := &fakeInteraction{input: "do-not-print"}
	var events []StepEvent
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "apply",
			Steps: []config.Step{
				{ID: "secret", Kind: "Input", Register: map[string]string{"token": "value"}, Spec: map[string]any{"message": "Token", "secret": true}},
				{ID: "show", Kind: "Message", Spec: map[string]any{"message": "token={{ .runtime.token }}"}},
			},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{StatePath: statePath, Interaction: interaction, EventSink: func(event StepEvent) { events = append(events, event) }})
	if err == nil {
		t.Fatalf("expected message secret rejection")
	}
	if strings.Contains(err.Error(), "do-not-print") {
		t.Fatalf("error leaked secret: %v", err)
	}
	for _, event := range events {
		if strings.Contains(fmt.Sprintf("%+v", event), "do-not-print") {
			t.Fatalf("event leaked secret: %+v", event)
		}
	}
	rawState, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatalf("read raw state: %v", readErr)
	}
	if strings.Contains(string(rawState), "do-not-print") {
		t.Fatalf("state leaked secret: %s", rawState)
	}
}

func readStateForInteractionTest(t *testing.T, path string, st *State) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if err := json.Unmarshal(raw, st); err != nil {
		t.Fatalf("parse state: %v", err)
	}
}
