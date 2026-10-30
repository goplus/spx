//go:build unix

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

package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/processsupervisor"
)

func TestRunCommandCancelsCleanupBeforeReturningSignal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			if runCommandTestInSubprocess(t) {
				return
			}
			cleaned := false
			var commandCtx context.Context
			status, err := RunCommand(t.Context(), func(ctx context.Context) (ProcessStatus, error) {
				commandCtx = ctx
				defer func() { cleaned = true }()
				if err := syscall.Kill(os.Getpid(), sig); err != nil {
					return ProcessStatus{}, err
				}
				select {
				case <-ctx.Done():
					// A later signal during cleanup must not replace the first one.
					second := syscall.SIGINT
					if second == sig {
						second = syscall.SIGTERM
					}
					if err := syscall.Kill(os.Getpid(), second); err != nil {
						return ProcessStatus{}, err
					}
					// Model cleanup that stays active while OS notifications arrive.
					time.Sleep(100 * time.Millisecond)
					return ProcessStatus{Code: 42}, ctx.Err()
				case <-time.After(5 * time.Second):
					return ProcessStatus{}, context.DeadlineExceeded
				}
			})
			if err != nil || status != (ProcessStatus{Signal: int(sig)}) {
				t.Fatalf("RunCommand() = (%+v, %v), want signal %v and nil error", status, err, sig)
			}
			if !cleaned {
				t.Fatal("RunCommand returned before deferred cleanup")
			}
			var cause *processsupervisor.SignalCause
			if !errors.As(context.Cause(commandCtx), &cause) || cause.Signal != sig {
				t.Fatalf("callback cancellation cause = %v, want SignalCause for %v", context.Cause(commandCtx), sig)
			}
		})
	}
}

func TestRunCommandIsSoleSignalOwnerForSupervisor(t *testing.T) {
	if os.Getenv("SPX_LAUNCHER_SIGNAL_HELPER") == "1" {
		runSignalRecordingHelper()
		return
	}
	if runCommandTestInSubprocess(t) {
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	root := t.TempDir()
	started := filepath.Join(root, "started")
	recorded := filepath.Join(root, "signals")
	type commandResult struct {
		status ProcessStatus
		err    error
	}
	result := make(chan commandResult, 1)
	finished := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("RunCommand did not finish child cleanup")
		}
	})
	go func() {
		defer close(finished)
		status, err := RunCommand(ctx, func(ctx context.Context) (ProcessStatus, error) {
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunCommandIsSoleSignalOwnerForSupervisor$", "-test.timeout=5s")
			command.Env = append(os.Environ(),
				"SPX_LAUNCHER_SIGNAL_HELPER=1",
				"SPX_LAUNCHER_SIGNAL_STARTED="+started,
				"SPX_LAUNCHER_SIGNAL_RECORDED="+recorded,
			)
			status, err := processsupervisor.Run(ctx, command)
			return ProcessStatus{Code: status.Code, Signal: status.Signal}, err
		})
		result <- commandResult{status: status, err: err}
	}()
	waitForLauncherMarker(t, started)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if got.err != nil || got.status != (ProcessStatus{Signal: int(syscall.SIGINT)}) {
			t.Fatalf("RunCommand supervisor result = (%+v, %v), want SIGINT", got.status, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunCommand did not finish after SIGINT")
	}
	data, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Fields(string(data)), []string{syscall.SIGINT.String()}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("child signals = %q, want exactly %q", got, want)
	}
}

func TestExitFallsBackForNonTerminatingSignal(t *testing.T) {
	if os.Getenv("SPX_LAUNCHER_EXIT_FALLBACK") == "1" {
		Exit(ProcessStatus{Code: 42, Signal: int(syscall.SIGCONT)})
		t.Fatal("Exit returned")
	}
	command := launcherTestCommand(t, "SPX_LAUNCHER_EXIT_FALLBACK=1")
	output, err := command.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 128+int(syscall.SIGCONT) {
		t.Fatalf("Exit(SIGCONT): %v, want exit code %d\n%s", err, 128+int(syscall.SIGCONT), output)
	}
}

func runCommandTestInSubprocess(t *testing.T) bool {
	t.Helper()
	if os.Getenv("SPX_LAUNCHER_COMMAND_TEST") == t.Name() {
		return false
	}
	command := launcherTestCommand(t, "SPX_LAUNCHER_COMMAND_TEST="+t.Name())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("command helper: %v\n%s", err, output)
	}
	return true
}

func runSignalRecordingHelper() {
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := os.WriteFile(os.Getenv("SPX_LAUNCHER_SIGNAL_STARTED"), []byte("started"), 0o600); err != nil {
		os.Exit(70)
	}
	received := []string{(<-signals).String()}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case current := <-signals:
			received = append(received, current.String())
		case <-timer.C:
			if err := os.WriteFile(os.Getenv("SPX_LAUNCHER_SIGNAL_RECORDED"), []byte(strings.Join(received, "\n")+"\n"), 0o600); err != nil {
				os.Exit(71)
			}
			return
		}
	}
}

func waitForLauncherMarker(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(name); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("launcher helper did not create %q", name)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
