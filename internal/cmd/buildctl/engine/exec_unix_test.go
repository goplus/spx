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

package engine

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTerminateTrackedProcessGroupKillsDescendantAfterLeaderExit(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "grandchild")
	lifetime, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer lifetime.Close()
	defer writer.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestTrackedProcessHelper$", "--")
	cmd.Env = append(os.Environ(), "SPX_TRACKED_PROCESS_HELPER=leader", "SPX_TRACKED_PROCESS_MARKER="+marker)
	cmd.ExtraFiles = []*os.File{writer}
	configureTrackedCommand(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid != cmd.Process.Pid {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper process group = %d, %v; want private group %d", pgid, err, cmd.Process.Pid)
	}
	defer func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) }()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// EOF does not depend on how quickly the system reaps an orphaned worker.
	workersExited := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, lifetime)
		workersExited <- err
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(marker)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("grandchild did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	terminateTrackedProcessGroup(cmd.Process)
	<-done
	select {
	case err := <-workersExited:
		if err != nil {
			t.Fatalf("wait for worker pipe EOF: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("grandchild kept its pipe open after tracked group termination")
	}
}

func TestTrackedProcessHelper(t *testing.T) {
	switch os.Getenv("SPX_TRACKED_PROCESS_HELPER") {
	case "leader":
		lifetime := os.NewFile(3, "tracked-worker-lifetime")
		defer lifetime.Close()
		cmd := exec.Command(os.Args[0], "-test.run=^TestTrackedProcessHelper$", "--")
		cmd.Env = append(os.Environ(), "SPX_TRACKED_PROCESS_HELPER=child")
		cmd.ExtraFiles = []*os.File{lifetime}
		if err := cmd.Start(); err != nil {
			os.Exit(71)
		}
	case "child":
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(os.Getenv("SPX_TRACKED_PROCESS_MARKER"), []byte("ready"), 0o600); err != nil {
			os.Exit(72)
		}
	default:
		return
	}
	for {
		time.Sleep(time.Second)
	}
}
