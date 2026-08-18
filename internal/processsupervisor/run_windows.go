//go:build windows

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package processsupervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsWaitDelay = 400 * time.Millisecond

// Windows has no POSIX process-group signal semantics. The child is created
// suspended, assigned to a KILL_ON_JOB_CLOSE Job Object, and only then resumed.
// This closes the previous assignment race in which the child could create an
// untracked descendant before job ownership was established.
func run(ctx context.Context, cmd *exec.Cmd) (Status, error) {
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	} else {
		copy := *attr
		attr = &copy
	}
	attr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED
	cmd.SysProcAttr = attr

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return Status{}, fmt.Errorf("processsupervisor: create Windows job: %w", err)
	}
	jobOpen := true
	defer func() {
		if jobOpen {
			_ = windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return Status{}, fmt.Errorf("processsupervisor: configure Windows job: %w", err)
	}

	// CommandContext's default Cancel targets only the leader. Queue a whole-job
	// termination request instead. Leave nil Cancel untouched for exec.Command,
	// which has no internal context and rejects a non-nil Cancel during Start.
	cancelRequests := make(chan struct{}, 1)
	if cmd.Cancel != nil {
		cmd.Cancel = func() error {
			select {
			case cancelRequests <- struct{}{}:
			default:
			}
			return nil
		}
	}
	cmd.WaitDelay = windowsWaitDelay

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return Status{}, err
	}
	abort := func(cause error) (Status, error) {
		// Every failure after suspended creation must reap the child. Terminate
		// the job if assignment happened, directly kill as the fail-closed
		// fallback, then call Cmd.Wait to release os/exec resources.
		_ = windows.TerminateJobObject(job, 1)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Status{}, cause
	}

	processHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		return abort(fmt.Errorf("processsupervisor: open suspended Windows child: %w", err))
	}
	assignErr := windows.AssignProcessToJobObject(job, processHandle)
	closeProcessErr := windows.CloseHandle(processHandle)
	if assignErr != nil {
		return abort(fmt.Errorf("processsupervisor: assign suspended child to Windows job: %w", assignErr))
	}
	if closeProcessErr != nil {
		return abort(fmt.Errorf("processsupervisor: close Windows child handle: %w", closeProcessErr))
	}

	thread, err := openSuspendedProcessThread(uint32(cmd.Process.Pid))
	if err != nil {
		return abort(fmt.Errorf("processsupervisor: open suspended Windows child thread: %w", err))
	}
	resumeErr := resumeWindowsThread(thread)
	closeThreadErr := windows.CloseHandle(thread)
	if resumeErr != nil {
		return abort(fmt.Errorf("processsupervisor: resume Windows child: %w", resumeErr))
	}
	if closeThreadErr != nil {
		return abort(fmt.Errorf("processsupervisor: close Windows child thread handle: %w", closeThreadErr))
	}

	type waitResult struct {
		state *os.ProcessState
		err   error
	}
	wait := make(chan waitResult, 1)
	go func() {
		err := cmd.Wait()
		wait <- waitResult{state: cmd.ProcessState, err: err}
	}()
	ctxDone := ctx.Done()
	for {
		select {
		case result := <-wait:
			// Closing a KILL_ON_JOB_CLOSE job terminates descendants that outlive
			// the leader. Do it before returning so caller cleanup cannot race a
			// surviving Engine child tree.
			if err := windows.CloseHandle(job); err != nil {
				return Status{}, fmt.Errorf("processsupervisor: close Windows job: %w", err)
			}
			jobOpen = false
			if result.err == nil {
				return Status{}, nil
			}
			var exitError *exec.ExitError
			if !errors.As(result.err, &exitError) || result.state == nil {
				return Status{}, result.err
			}
			return statusFromProcessState(result.state), nil
		case <-signals:
			// CTRL_BREAK is the closest console-level equivalent to forwarding
			// an interrupt. If unavailable, terminate the complete job. Windows
			// status still reports an exit code, never a fabricated POSIX signal.
			if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil {
				_ = windows.TerminateJobObject(job, 1)
			}
		case <-cancelRequests:
			_ = windows.TerminateJobObject(job, 1)
		case <-ctxDone:
			ctxDone = nil
			_ = windows.TerminateJobObject(job, 1)
		}
	}
}

func openSuspendedProcessThread(pid uint32) (windows.Handle, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	if err := windows.Thread32First(snapshot, &entry); err != nil {
		return 0, err
	}
	for {
		if entry.OwnerProcessID == pid {
			return windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		if err := windows.Thread32Next(snapshot, &entry); err != nil {
			if errors.Is(err, syscall.ERROR_NO_MORE_FILES) {
				return 0, fmt.Errorf("no thread found for process %d", pid)
			}
			return 0, err
		}
	}
}

func resumeWindowsThread(thread windows.Handle) error {
	for {
		previous, err := windows.ResumeThread(thread)
		if err != nil {
			return err
		}
		if previous == 0 {
			return errors.New("processsupervisor: Windows child thread was not suspended")
		}
		if previous == 1 {
			return nil
		}
	}
}

func statusFromProcessState(state *os.ProcessState) Status {
	if state == nil {
		return Status{Code: 1}
	}
	return Status{Code: state.ExitCode()}
}
