package batchrun

import (
	"context"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type EventContext struct {
	BatchID        string
	ParallelGroup  string
	Parallel       bool
	BatchSize      int
	MaxParallelism int
	StartedAt      string
}

func NewEventContext(batch workflowexec.StepBatch) EventContext {
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	limit := batch.MaxParallelism
	if limit <= 0 || limit > len(batch.Steps) {
		limit = len(batch.Steps)
	}
	batchID := strings.TrimSpace(batch.PhaseName)
	if group := strings.TrimSpace(batch.ParallelGroup); group != "" {
		batchID = batchID + ":" + group
	}
	return EventContext{
		BatchID:        batchID,
		ParallelGroup:  strings.TrimSpace(batch.ParallelGroup),
		Parallel:       batch.Parallel(),
		BatchSize:      len(batch.Steps),
		MaxParallelism: limit,
		StartedAt:      startedAt,
	}
}

func CloneRuntimeVars(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func Execute[T any](ctx context.Context, batch workflowexec.StepBatch, run func(context.Context, config.Step) (T, error)) ([]T, error) {
	results := make([]T, len(batch.Steps))
	if len(batch.Steps) == 0 {
		return results, nil
	}
	group, groupCtx := errgroup.WithContext(ctx)
	limit := batch.MaxParallelism
	if !batch.Parallel() {
		limit = 1
	}
	if limit <= 0 || limit > len(batch.Steps) {
		limit = len(batch.Steps)
	}
	group.SetLimit(limit)
	for i, step := range batch.Steps {
		i := i
		step := step
		group.Go(func() error {
			result, err := run(groupCtx, step)
			if err != nil {
				return err
			}
			results[i] = result
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return results, err
	}
	return results, nil
}
