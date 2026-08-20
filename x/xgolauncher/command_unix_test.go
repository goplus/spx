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

package xgolauncher

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/processsupervisor"
)

func TestRunCommandCancelsCleanupBeforeReturningSignal(t *testing.T) {
	cleaned := false
	status, err := RunCommand(context.Background(), func(ctx context.Context) (ProcessStatus, error) {
		defer func() { cleaned = true }()
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			return ProcessStatus{}, err
		}
		select {
		case <-ctx.Done():
			return ProcessStatus{}, ctx.Err()
		case <-time.After(5 * time.Second):
			return ProcessStatus{}, context.DeadlineExceeded
		}
	})
	if err != nil || status.Signal != int(syscall.SIGTERM) {
		t.Fatalf("RunCommand() = (%+v, %v), want SIGTERM", status, err)
	}
	if !cleaned {
		t.Fatal("RunCommand returned before deferred cleanup")
	}
}

func TestRunCommandIsSoleSignalOwnerForSupervisor(t *testing.T) {
	if os.Getenv("SPX_LAUNCHER_SIGNAL_HELPER") == "1" {
		runSignalRecordingHelper()
		return
	}
	root := t.TempDir()
	started := filepath.Join(root, "started")
	recorded := filepath.Join(root, "signals")
	type commandResult struct {
		status ProcessStatus
		err    error
	}
	result := make(chan commandResult, 1)
	go func() {
		status, err := RunCommand(context.Background(), func(ctx context.Context) (ProcessStatus, error) {
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunCommandIsSoleSignalOwnerForSupervisor$")
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
