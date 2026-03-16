package executil

import (
	"context"
	"os/exec"
)

func Command(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
