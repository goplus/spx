//go:build !windows

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
	"os"
	"os/exec"
	"syscall"
	"time"
)

const (
	unixShutdownGrace = 200 * time.Millisecond
	unixGroupPoll     = 10 * time.Millisecond
)

func run(ctx context.Context, cmd *exec.Cmd) (Status, error) {
	// Keep any caller-provided attributes, but make the child the leader of a
	// private process group. Forwarding to -pid then reaches the Engine and
	// descendants without ever signalling this wrapper's process group.
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	} else {
		copy := *attr
		attr = &copy
	}
	attr.Setpgid = true
	cmd.SysProcAttr = attr

	// CommandContext's default Cancel kills only the leader. Replace it with a
	// request that this supervisor turns into TERM -> grace -> KILL for the
	// whole group. A nil Cancel identifies a plain exec.Command and must remain
	// nil because os/exec rejects Cancel on commands without an internal context.
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
	// Bound pipe-copy goroutines if a descendant inherits stdout/stderr after
	// the leader exits. Group termination is scheduled sooner than this direct-
	// leader os/exec fallback, so supervisor semantics remain authoritative.
	cmd.WaitDelay = 2 * unixShutdownGrace

	if err := cmd.Start(); err != nil {
		return Status{}, err
	}
	pgid := cmd.Process.Pid

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
	var shutdownDeadline time.Time
	var killTimer *time.Timer
	var killTimerC <-chan time.Time
	var shutdownErr error
	var cancellationObserved bool
	beginShutdown := func() {
		if !shutdownDeadline.IsZero() {
			return
		}
		shutdownDeadline = time.Now().Add(unixShutdownGrace)
		shutdownSignal := syscall.SIGTERM
		if signal, ok := signalFromContext(ctx); ok {
			if unixSignal, ok := signal.(syscall.Signal); ok {
				shutdownSignal = unixSignal
			}
		}
		if err := signalProcessGroup(pgid, shutdownSignal); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		killTimer = time.NewTimer(unixShutdownGrace)
		killTimerC = killTimer.C
	}

	for {
		select {
		case result := <-wait:
			if killTimer != nil {
				killTimer.Stop()
			}
			// Waiting for the leader is not enough: descendants remain in its
			// process group. Give them the same short graceful shutdown window,
			// then kill any remainder before returning to caller cleanup.
			cleanupErr := cleanupProcessGroup(pgid, shutdownDeadline)
			if err := errors.Join(shutdownErr, cleanupErr); err != nil {
				return Status{}, err
			}
			if result.err == nil {
				if err := cancellationError(ctx, cancellationObserved); err != nil {
					return Status{}, err
				}
				return Status{}, nil
			}
			var exitError *exec.ExitError
			if !errors.As(result.err, &exitError) || result.state == nil {
				return Status{}, result.err
			}
			return statusFromProcessState(result.state), nil
		case <-cancelRequests:
			cancellationObserved = true
			beginShutdown()
		case <-ctxDone:
			ctxDone = nil
			cancellationObserved = true
			beginShutdown()
		case <-killTimerC:
			killTimerC = nil
			if err := signalProcessGroup(pgid, syscall.SIGKILL); err != nil {
				shutdownErr = errors.Join(shutdownErr, err)
			}
		}
	}
}

func cleanupProcessGroup(pgid int, deadline time.Time) error {
	alive, err := processGroupAlive(pgid)
	if err != nil || !alive {
		return err
	}
	if deadline.IsZero() {
		if err := signalProcessGroup(pgid, syscall.SIGTERM); err != nil {
			return err
		}
		deadline = time.Now().Add(unixShutdownGrace)
	}
	for time.Now().Before(deadline) {
		alive, err = processGroupAlive(pgid)
		if err != nil || !alive {
			return err
		}
		remaining := time.Until(deadline)
		if remaining > unixGroupPoll {
			remaining = unixGroupPoll
		}
		time.Sleep(remaining)
	}
	return signalProcessGroup(pgid, syscall.SIGKILL)
}

func processGroupAlive(pgid int) (bool, error) {
	err := syscall.Kill(-pgid, 0)
	switch {
	case err == nil, errors.Is(err, syscall.EPERM):
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, err
	}
}

func signalProcessGroup(pgid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func statusFromProcessState(state *os.ProcessState) Status {
	if state == nil {
		return Status{Code: 1}
	}
	if wait, ok := state.Sys().(syscall.WaitStatus); ok && wait.Signaled() {
		return Status{Signal: int(wait.Signal())}
	}
	return Status{Code: state.ExitCode()}
}
