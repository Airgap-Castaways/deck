package prepare

import (
	"context"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/operatorio"
)

type fakePrepareInteraction struct {
	messages []prepareMessageRecord
}

type prepareMessageRecord struct {
	level   string
	message string
	stream  string
}

func (f *fakePrepareInteraction) Message(level, message, stream string) error {
	f.messages = append(f.messages, prepareMessageRecord{level: level, message: message, stream: stream})
	return nil
}

func (f *fakePrepareInteraction) Confirm(context.Context, string, *bool) (bool, error) {
	return false, nil
}

func (f *fakePrepareInteraction) Input(context.Context, string, operatorio.InputOptions) (string, error) {
	return "", nil
}

func TestRun_MessageRendersDuringPrepare(t *testing.T) {
	interaction := &fakePrepareInteraction{}
	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "worker"},
		Phases: []config.Phase{{
			Name:  "prepare",
			Steps: []config.Step{{ID: "show", Kind: "Message", Spec: map[string]any{"message": "preparing {{ .vars.role }}", "stream": "stderr"}}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: t.TempDir(), Interaction: interaction}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(interaction.messages) != 1 {
		t.Fatalf("expected one message, got %d", len(interaction.messages))
	}
	if got := interaction.messages[0]; got.level != "info" || got.stream != "stderr" || got.message != "preparing worker" {
		t.Fatalf("unexpected message: %+v", got)
	}
}
