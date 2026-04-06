package batchrun

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func TestExecuteRunsAllSequentialSteps(t *testing.T) {
	batch := workflowexec.StepBatch{
		PhaseName: "prepare",
		Steps: []config.Step{
			{ID: "first", Kind: "WriteFile"},
			{ID: "second", Kind: "WriteFile"},
			{ID: "third", Kind: "WriteFile"},
		},
	}

	seen := make([]string, 0, len(batch.Steps))
	results, err := Execute(context.Background(), batch, func(_ context.Context, step config.Step) (string, error) {
		seen = append(seen, step.ID)
		return step.ID, nil
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("unexpected execution order: got %v want %v", seen, want)
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("unexpected results: got %v want %v", results, want)
	}
}

func TestExecuteReturnsErrorFromSequentialStep(t *testing.T) {
	wantErr := errors.New("boom")
	batch := workflowexec.StepBatch{
		PhaseName: "prepare",
		Steps: []config.Step{
			{ID: "first", Kind: "WriteFile"},
			{ID: "second", Kind: "WriteFile"},
		},
	}

	seen := make([]string, 0, len(batch.Steps))
	_, err := Execute(context.Background(), batch, func(_ context.Context, step config.Step) (string, error) {
		seen = append(seen, step.ID)
		if step.ID == "second" {
			return "", wantErr
		}
		return step.ID, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	wantSeen := []string{"first", "second"}
	if !reflect.DeepEqual(seen, wantSeen) {
		t.Fatalf("unexpected executed steps: got %v want %v", seen, wantSeen)
	}
}
