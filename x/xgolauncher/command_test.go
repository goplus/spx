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

package xgolauncher

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	t.Run("nil command", func(t *testing.T) {
		status, err := RunCommand(nil, nil)
		if status != (ProcessStatus{}) || err == nil || err.Error() != "xgolauncher: nil command" {
			t.Fatalf("RunCommand(nil, nil) = (%+v, %v), want zero status and nil command error", status, err)
		}
	})

	type contextKey struct{}
	parent, cancel := context.WithTimeout(context.WithValue(t.Context(), contextKey{}, "value"), 5*time.Second)
	defer cancel()
	wantErr := errors.New("command failed")
	for _, tt := range []struct {
		name   string
		parent context.Context
		status ProcessStatus
		err    error
	}{
		{name: "nil parent"},
		{name: "exit status", parent: parent, status: ProcessStatus{Code: 42}},
		{name: "signal status", parent: parent, status: ProcessStatus{Signal: 15}},
		{name: "callback error", parent: parent, status: ProcessStatus{Code: 7}, err: wantErr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var commandCtx context.Context
			calls := 0
			status, err := RunCommand(tt.parent, func(ctx context.Context) (ProcessStatus, error) {
				commandCtx = ctx
				calls++
				if ctx.Err() != nil {
					t.Errorf("callback context error = %v, want nil", ctx.Err())
				}
				if tt.parent != nil {
					if ctx.Value(contextKey{}) != "value" {
						t.Error("callback context lost parent value")
					}
					gotDeadline, gotOK := ctx.Deadline()
					wantDeadline, wantOK := tt.parent.Deadline()
					if gotDeadline != wantDeadline || gotOK != wantOK {
						t.Errorf("callback deadline = (%v, %v), want (%v, %v)", gotDeadline, gotOK, wantDeadline, wantOK)
					}
				}
				return tt.status, tt.err
			})
			if status != tt.status || err != tt.err || calls != 1 {
				t.Fatalf("RunCommand() = (%+v, %v), calls = %d; want (%+v, %v), one call", status, err, calls, tt.status, tt.err)
			}
			if commandCtx.Err() != context.Canceled {
				t.Errorf("callback context error after return = %v, want context.Canceled", commandCtx.Err())
			}
			if parent.Err() != nil {
				t.Errorf("parent context error after return = %v, want nil", parent.Err())
			}
		})
	}
}

func TestRunCommandParentCancellation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		before bool
	}{
		{name: "before callback", before: true},
		{name: "during callback"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parent, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			cause := errors.New("parent canceled")
			if tt.before {
				cancel(cause)
			}
			status, err := RunCommand(parent, func(ctx context.Context) (ProcessStatus, error) {
				if !tt.before {
					cancel(cause)
				}
				if ctx.Err() != context.Canceled {
					t.Errorf("callback context error = %v, want context.Canceled", ctx.Err())
				}
				return ProcessStatus{Code: 7}, context.Cause(ctx)
			})
			if status != (ProcessStatus{Code: 7}) || err != cause {
				t.Fatalf("RunCommand() = (%+v, %v), want code 7 and parent cancellation cause", status, err)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	if value := os.Getenv("SPX_LAUNCHER_EXIT_CODE"); value != "" {
		code, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		Exit(ProcessStatus{Code: code})
		t.Fatal("Exit returned")
	}
	for _, code := range []int{0, 42} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			command := launcherTestCommand(t, "SPX_LAUNCHER_EXIT_CODE="+strconv.Itoa(code))
			output, err := command.CombinedOutput()
			if code == 0 {
				if err != nil {
					t.Fatalf("Exit(0): %v\n%s", err, output)
				}
				return
			}
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) || exitError.ExitCode() != code {
				t.Fatalf("Exit(%d): %v, want exit code %d\n%s", code, err, code, output)
			}
		})
	}
}

func launcherTestCommand(t *testing.T, env ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)
	testName := "^" + strings.ReplaceAll(regexp.QuoteMeta(t.Name()), "/", "$/^") + "$"
	command := exec.CommandContext(ctx, os.Args[0], "-test.run="+testName, "-test.timeout=10s")
	command.Env = append(os.Environ(), env...)
	// Include helper execution, including os.Exit, in go test's coverage report.
	if coverDir := flag.Lookup("test.gocoverdir"); coverDir != nil && coverDir.Value.String() != "" {
		command.Args = append(command.Args, "-test.gocoverdir="+coverDir.Value.String())
		command.Env = append(command.Env, "GOCOVERDIR="+coverDir.Value.String())
	}
	return command
}
