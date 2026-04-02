//go:build !windows

package engine

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureTrackedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func trackedSignals() []os.Signal {
	return []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGTSTP,
	}
}

func trackedProcessExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func terminateTrackedProcessGroup(process *os.Process) {
	if process == nil {
		return
	}
	pid := process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	time.Sleep(time.Second)
	if syscall.Kill(pid, 0) == nil {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
