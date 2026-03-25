//go:build windows

package main

import (
	"os"
	"os/exec"
)

func configureTrackedCommand(cmd *exec.Cmd) {}

func trackedSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func trackedProcessExists(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}

func terminateTrackedProcessGroup(process *os.Process) {
	if process == nil {
		return
	}
	_ = process.Kill()
}
