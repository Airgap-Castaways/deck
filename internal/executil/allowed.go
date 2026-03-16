package executil

import (
	"context"
	"os/exec"
)

func LookPath(cmd AllowedCommand) (string, error) {
	return exec.LookPath(string(cmd))
}

func Run(ctx context.Context, cmd AllowedCommand, args ...string) error {
	command := exec.CommandContext(ctx, string(cmd), args...)
	return command.Run()
}

func CombinedOutput(ctx context.Context, cmd AllowedCommand, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, string(cmd), args...)
	return command.CombinedOutput()
}
