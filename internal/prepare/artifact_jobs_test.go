package prepare

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRunArtifactJobGroupParallelism(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	group := artifactJobGroup{
		Kind:      "image",
		Name:      "parallel",
		Execution: artifactExecution{Parallelism: 2},
		Jobs: []artifactJob{
			{
				Label: "job-a",
				Run: func(ctx context.Context) ([]string, error) {
					started <- "a"
					<-release
					return []string{"images/a.tar"}, nil
				},
			},
			{
				Label: "job-b",
				Run: func(ctx context.Context) ([]string, error) {
					started <- "b"
					<-release
					return []string{"images/b.tar"}, nil
				},
			},
		},
	}

	resultCh := make(chan error, 1)
	go func() {
		_, err := runArtifactJobGroup(context.Background(), group)
		resultCh <- err
	}()

	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected both jobs to start before release")
		}
	}
	close(release)
	if err := <-resultCh; err != nil {
		t.Fatalf("runArtifactJobGroup failed: %v", err)
	}
}

func TestRunArtifactJobGroupCancelsSiblingsOnFailure(t *testing.T) {
	canceled := make(chan struct{}, 1)
	started := make(chan struct{}, 1)
	releaseFailure := make(chan struct{})
	group := artifactJobGroup{
		Kind:      "package",
		Name:      "cancel",
		Execution: artifactExecution{Parallelism: 2},
		Jobs: []artifactJob{
			{
				Label: "fail-fast",
				Run: func(ctx context.Context) ([]string, error) {
					<-releaseFailure
					return nil, fmt.Errorf("boom")
				},
			},
			{
				Label: "blocked",
				Run: func(ctx context.Context) ([]string, error) {
					started <- struct{}{}
					<-ctx.Done()
					canceled <- struct{}{}
					return nil, ctx.Err()
				},
			},
		},
	}

	err := make(chan error, 1)
	go func() {
		_, runErr := runArtifactJobGroup(context.Background(), group)
		err <- runErr
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected sibling job to start")
	}
	close(releaseFailure)
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected sibling job to see cancellation")
	}
	if runErr := <-err; runErr == nil || runErr.Error() == "" {
		t.Fatalf("expected group failure, got %v", runErr)
	}
}

func TestRunArtifactJobWithRetry(t *testing.T) {
	attempts := 0
	job := artifactJob{
		Label: "retry",
		Run: func(ctx context.Context) ([]string, error) {
			attempts++
			if attempts == 1 {
				return nil, fmt.Errorf("first failure")
			}
			return []string{"files/out.bin"}, nil
		},
	}
	files, err := runArtifactJobWithRetry(context.Background(), 1, job)
	if err != nil {
		t.Fatalf("runArtifactJobWithRetry failed: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if len(files) != 1 || files[0] != "files/out.bin" {
		t.Fatalf("unexpected files: %v", files)
	}
}
