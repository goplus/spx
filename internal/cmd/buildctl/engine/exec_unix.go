//go:build unix

/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package engine

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var trackedSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGHUP,
	syscall.SIGQUIT,
	syscall.SIGTSTP,
}

func configureTrackedCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func trackedProcessExists(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	// Permission and other probe errors cannot establish that the owner is dead.
	return err == nil, err
}

func terminateTrackedProcessGroup(process *os.Process) {
	if process == nil {
		return
	}
	pid := process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	time.Sleep(time.Second)
	// The leader may have exited while descendants still need SIGKILL.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
