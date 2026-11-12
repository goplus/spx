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
	"runtime"
	"strings"
	"testing"
)

func TestRunRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name string
		ctx  context.Context
		cmd  *exec.Cmd
		want string
	}{
		{name: "nil context", cmd: &exec.Cmd{}, want: "processsupervisor: nil context"},
		{name: "nil command", ctx: context.Background(), want: "processsupervisor: nil command"},
		{name: "started command", ctx: context.Background(), cmd: &exec.Cmd{Process: &os.Process{}}, want: "processsupervisor: command has already started"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			status, err := Run(tt.ctx, tt.cmd)
			if status != (Status{}) || err == nil || err.Error() != tt.want {
				t.Fatalf("Run() = (%+v, %v), want zero status and %q", status, err, tt.want)
			}
		})
	}
}

func TestRunUnsupportedPlatform(t *testing.T) {
	switch runtime.GOOS {
	case "js", "wasip1", "plan9":
	default:
		t.Skip("process supervision is supported on Unix and Windows")
	}
	cmd := exec.Command("child")
	status, err := Run(context.Background(), cmd)
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Run() = (%+v, %v), want unsupported error", status, err)
	}
	if !strings.Contains(err.Error(), runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("Run() error lacks platform: %v", err)
	}
	if status != (Status{}) || cmd.Process != nil || cmd.SysProcAttr != nil {
		t.Fatalf("Run() changed command or status: status=%+v, command=%+v", status, cmd)
	}
}

func TestRunRejectsStartedCommand(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestStatusSuccess$")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	status, err := Run(context.Background(), command)
	if status != (Status{}) || err == nil || err.Error() != "processsupervisor: command has already started" {
		t.Fatalf("Run() = (%+v, %v), want zero status and already-started error", status, err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("caller could not wait for rejected command: %v", err)
	}
}

func TestStatusSuccess(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "zero exit", want: true},
		{name: "nonzero exit", status: Status{Code: 1}},
		{name: "signal", status: Status{Signal: 2}},
		{name: "signal and code", status: Status{Code: 1, Signal: 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Success(); got != tt.want {
				t.Fatalf("Status(%+v).Success() = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestSignalCauseError(t *testing.T) {
	for _, tt := range []struct {
		name  string
		cause *SignalCause
		want  string
	}{
		{name: "nil cause", want: "processsupervisor: canceled by signal"},
		{name: "nil signal", cause: &SignalCause{}, want: "processsupervisor: canceled by signal"},
		{name: "interrupt", cause: &SignalCause{Signal: os.Interrupt}, want: "processsupervisor: canceled by signal interrupt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cause.Error(); got != tt.want {
				t.Fatalf("SignalCause.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCancellationError(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	cause := errors.New("custom cancellation cause")
	causedCtx, cancelCause := context.WithCancelCause(context.Background())
	cancelCause(cause)

	tests := []struct {
		name     string
		ctx      context.Context
		observed bool
		want     error
	}{
		{name: "not canceled", ctx: context.Background()},
		{name: "observed command cancellation", ctx: context.Background(), observed: true, want: context.Canceled},
		{name: "context canceled before observation", ctx: canceledCtx, want: context.Canceled},
		{name: "context cancellation cause", ctx: causedCtx, want: cause},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cancellationError(tt.ctx, tt.observed)
			if !errors.Is(got, tt.want) {
				t.Fatalf("cancellationError() = %v, want %v", got, tt.want)
			}
		})
	}
}
