//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const processQueryLimitedInformation = 0x1000

func configureTrackedCommand(cmd *exec.Cmd) {}

func trackedSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func trackedProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	waitStatus, err := syscall.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return waitStatus == syscall.WAIT_TIMEOUT
}

func terminateTrackedProcessGroup(process *os.Process) {
	if process == nil {
		return
	}
	_ = process.Kill()
}
