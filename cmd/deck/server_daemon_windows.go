//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const detachedProcess = 0x00000008

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

func isDetachedServerProcessMissing(_ error) bool {
	return false
}
