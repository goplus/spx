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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunSeparatesPreparationErrorFromChildExit(t *testing.T) {
	status, err := Run(context.Background(), exec.Command(filepath.Join(t.TempDir(), "missing")))
	if err == nil || status != (Status{}) {
		t.Fatalf("preparation result = (%+v, %v), want zero status and error", status, err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
	command.Env = append(os.Environ(), "SPX_PROCESS_SUPERVISOR_HELPER=exit1")
	status, err = Run(context.Background(), command)
	if err != nil || status != (Status{Code: 1}) {
		t.Fatalf("child exit result = (%+v, %v), want code 1 and nil error", status, err)
	}
}

func TestRunForwardsSignalCauseToChildGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
	command.Env = append(os.Environ(), "SPX_PROCESS_SUPERVISOR_HELPER=wait", "SPX_PROCESS_SUPERVISOR_MARKER="+marker)
	result := make(chan struct {
		status Status
		err    error
	}, 1)
	go func() {
		status, err := Run(ctx, command)
		result <- struct {
			status Status
			err    error
		}{status: status, err: err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel(&SignalCause{Signal: syscall.SIGINT})
	select {
	case result := <-result:
		if result.err != nil || result.status != (Status{Signal: int(syscall.SIGINT)}) {
			t.Fatalf("forwarded signal result = (%+v, %v), want SIGINT and nil error", result.status, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not finish after forwarded signal")
	}
}

func TestRunReturnsCancellationAfterGracefulExit(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
	command.Env = append(os.Environ(),
		"SPX_PROCESS_SUPERVISOR_HELPER=graceful-exit",
		"SPX_PROCESS_SUPERVISOR_MARKER="+marker,
	)
	result := make(chan struct {
		status Status
		err    error
	}, 1)
	go func() {
		status, err := Run(ctx, command)
		result <- struct {
			status Status
			err    error
		}{status: status, err: err}
	}()
	waitForFile(t, marker)
	cancel()
	select {
	case result := <-result:
		if result.err == nil || !errors.Is(result.err, context.Canceled) || result.status != (Status{}) {
			t.Fatalf("graceful cancellation result = (%+v, %v), want context.Canceled and zero status", result.status, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not finish after graceful cancellation")
	}
}

func TestRunCleansGrandchildAfterLeaderExit(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "grandchild")
	command := exec.Command(os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
	command.Env = append(os.Environ(),
		"SPX_PROCESS_SUPERVISOR_HELPER=spawn-grandchild",
		"SPX_PROCESS_SUPERVISOR_MARKER="+marker,
	)
	status, err := Run(context.Background(), command)
	if err != nil || status != (Status{}) {
		t.Fatalf("leader result = (%+v, %v), want successful status", status, err)
	}
	pid := readHelperPID(t, marker)
	waitForProcessGone(t, pid)
}

func TestRunContextEscalatesIgnoredTerm(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "leader")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
	command.Env = append(os.Environ(),
		"SPX_PROCESS_SUPERVISOR_HELPER=ignore-term",
		"SPX_PROCESS_SUPERVISOR_MARKER="+marker,
	)
	result := make(chan struct {
		status Status
		err    error
	}, 1)
	go func() {
		status, err := Run(ctx, command)
		result <- struct {
			status Status
			err    error
		}{status: status, err: err}
	}()
	waitForFile(t, marker)
	cancel()
	select {
	case result := <-result:
		if result.err != nil || result.status != (Status{Signal: int(syscall.SIGKILL)}) {
			t.Fatalf("canceled result = (%+v, %v), want SIGKILL after TERM grace", result.status, result.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not escalate ignored TERM")
	}
	waitForProcessGone(t, readHelperPID(t, marker))
}

func TestProcessSupervisorHelper(t *testing.T) {
	if os.Getenv("SPX_PROCESS_SUPERVISOR_GRANDCHILD") == "ignore-term" {
		runIgnoringTermHelper()
		return
	}
	switch os.Getenv("SPX_PROCESS_SUPERVISOR_HELPER") {
	case "exit1":
		os.Exit(1)
	case "wait":
		if marker := os.Getenv("SPX_PROCESS_SUPERVISOR_MARKER"); marker != "" {
			_ = os.WriteFile(marker, []byte("started"), 0o600)
		}
		for {
			time.Sleep(time.Second)
		}
	case "graceful-exit":
		runGracefulExitHelper()
	case "ignore-term":
		runIgnoringTermHelper()
	case "spawn-grandchild":
		command := exec.Command(os.Args[0], "-test.run=^TestProcessSupervisorHelper$", "--")
		command.Env = append(os.Environ(), "SPX_PROCESS_SUPERVISOR_GRANDCHILD=ignore-term")
		if err := command.Start(); err != nil {
			os.Exit(71)
		}
		if !waitForFilePath(os.Getenv("SPX_PROCESS_SUPERVISOR_MARKER"), 5*time.Second) {
			os.Exit(72)
		}
		os.Exit(0)
	}
}

func runGracefulExitHelper() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	marker := os.Getenv("SPX_PROCESS_SUPERVISOR_MARKER")
	if marker == "" || os.WriteFile(marker, []byte("started"), 0o600) != nil {
		os.Exit(74)
	}
	<-signals
	// Bypass os.Exit's race-detector finalization. Under -race, that finalizer
	// can exceed the supervisor's intentionally short grace period and turn a
	// graceful helper into a synthetic SIGKILL.
	syscall.Exit(0)
}

func runIgnoringTermHelper() {
	signal.Ignore(syscall.SIGTERM)
	marker := os.Getenv("SPX_PROCESS_SUPERVISOR_MARKER")
	if marker == "" || os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0o600) != nil {
		os.Exit(73)
	}
	for {
		time.Sleep(time.Second)
	}
}

func readHelperPID(t *testing.T, marker string) int {
	t.Helper()
	waitForFile(t, marker)
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("helper pid marker = %q: %v", data, err)
	}
	return pid
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	if !waitForFilePath(path, 5*time.Second) {
		t.Fatalf("file %q was not created", path)
	}
}

func waitForFilePath(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			t.Fatalf("inspect process %d: %v", pid, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d survived supervisor cleanup", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
