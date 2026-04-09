//go:build !windows

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
