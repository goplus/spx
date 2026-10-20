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
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows"
)

func configureTrackedCommand(cmd *exec.Cmd) {}

var trackedSignals = []os.Signal{os.Interrupt}

func trackedProcessExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
			return false, nil
		}
		return false, err
	}
	defer windows.CloseHandle(handle)

	waitStatus, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false, err
	}
	switch waitStatus {
	case uint32(windows.WAIT_TIMEOUT):
		return true, nil
	case windows.WAIT_OBJECT_0:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected wait status for process %d: %d", pid, waitStatus)
	}
}

func terminateTrackedProcessGroup(process *os.Process) {
	if process == nil {
		return
	}
	_ = process.Kill()
}
