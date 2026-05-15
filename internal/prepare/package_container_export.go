package prepare

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/Airgap-Castaways/deck/internal/errcode"
)

var packageContainerCounter uint64

func runPackageContainerWithExport(ctx context.Context, runner CommandRunner, runtimeSel, image, platform, script string) ([]byte, error) {
	containerName := fmt.Sprintf("deck-prepare-%d", atomic.AddUint64(&packageContainerCounter, 1))
	createStdout := &bytes.Buffer{}
	createStderr := &bytes.Buffer{}
	createArgs := []string{"create"}
	if strings.TrimSpace(platform) != "" {
		createArgs = append(createArgs, "--platform", platform)
	}
	createArgs = append(createArgs, "--name", containerName, image, "bash", "-lc", script)
	if err := runner.RunWithIO(ctx, createStdout, createStderr, runtimeSel, createArgs...); err != nil {
		return nil, formatContainerCommandError("create package download container", err, createStderr.String())
	}
	containerID := strings.TrimSpace(createStdout.String())
	if containerID == "" {
		containerID = containerName
	}
	defer func() {
		_ = runner.Run(detachedContext(ctx), runtimeSel, "rm", "-f", containerID)
	}()

	startStderr := &bytes.Buffer{}
	if err := runner.RunWithIO(ctx, nil, startStderr, runtimeSel, "start", "-a", containerID); err != nil {
		return nil, formatContainerCommandError("start package download container", err, startStderr.String())
	}

	exportStdout := &bytes.Buffer{}
	exportStderr := &bytes.Buffer{}
	if err := runner.RunWithIO(ctx, exportStdout, exportStderr, runtimeSel, "cp", containerID+":"+containerOutputRoot+"/.", "-"); err != nil {
		return nil, formatContainerCommandError("export package download artifacts", err, exportStderr.String())
	}
	return exportStdout.Bytes(), nil
}

func detachedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func formatContainerCommandError(action string, err error, stderr string) error {
	trimmed := strings.TrimSpace(stderr)
	if trimmed == "" {
		return errcode.New(action, err)
	}
	return errcode.New(action, fmt.Errorf("%w: %s", err, trimmed))
}
