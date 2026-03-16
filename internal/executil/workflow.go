package executil

import (
	"context"
	"io"
	"os/exec"
)

func LookPathWorkflowBinary(file string) (string, error) {
	return exec.LookPath(file)
}

func RunWorkflowCommand(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	return command.Run()
}

func RunWorkflowCommandWithIO(ctx context.Context, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func CombinedOutputWorkflowCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	return command.CombinedOutput()
}
