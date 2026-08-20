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

// Package processsupervisor runs one already-prepared child command while
// keeping process lifetime and process-status handling at a library boundary.
// It never changes the caller's cwd/environment and never terminates the
// calling process.
package processsupervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Status is the terminal status of a child. A non-zero Signal means the child
// was terminated by that signal; Code is used only for normal termination.
// Windows cannot preserve POSIX signal identity and reports only Code.
type Status struct {
	Code   int
	Signal int
}

// SignalCause carries a signal intercepted by the outer command boundary
// through context cancellation. On Unix, Run consumes this cause when stopping
// a child group instead of subscribing to process-wide signals itself.
type SignalCause struct {
	Signal os.Signal
}

func (c *SignalCause) Error() string {
	if c == nil || c.Signal == nil {
		return "processsupervisor: canceled by signal"
	}
	return fmt.Sprintf("processsupervisor: canceled by signal %s", c.Signal)
}

func signalFromContext(ctx context.Context) (os.Signal, bool) {
	var cause *SignalCause
	if !errors.As(context.Cause(ctx), &cause) || cause == nil || cause.Signal == nil {
		return nil, false
	}
	return cause.Signal, true
}

func cancellationError(ctx context.Context, observed bool) error {
	if !observed && ctx.Err() == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return context.Canceled
}

// Success reports whether the child exited normally with status zero.
func (s Status) Success() bool { return s.Signal == 0 && s.Code == 0 }

// Run starts and waits for cmd. Preparation/start failures and wait failures
// are returned as errors. A child exit (including a non-zero exit code or a
// signal termination) is represented by Status and is not an error. On Unix,
// host applications own signal subscription and may cancel ctx with
// SignalCause.
func Run(ctx context.Context, cmd *exec.Cmd) (Status, error) {
	if ctx == nil {
		return Status{}, errors.New("processsupervisor: nil context")
	}
	if cmd == nil {
		return Status{}, errors.New("processsupervisor: nil command")
	}
	if cmd.Process != nil {
		return Status{}, errors.New("processsupervisor: command has already started")
	}
	return run(ctx, cmd)
}
