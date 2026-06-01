//go:build windows

package main

import (
	"os"
	"os/exec"
)

func configureDetachedServerProcess(_ *exec.Cmd) {}

func terminateDetachedServerProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func isDetachedServerProcessMissing(_ error) bool {
	return false
}
