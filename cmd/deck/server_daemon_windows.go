//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008
const windowsErrorInvalidParameter syscall.Errno = 87

func configureDetachedServerProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detachedProcess}
}

func terminateDetachedServerProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func isDetachedServerProcessMissing(err error) bool {
	if errors.Is(err, os.ErrProcessDone) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == windowsErrorInvalidParameter
	}
	return false
}
