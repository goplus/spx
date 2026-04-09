//go:build windows

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
