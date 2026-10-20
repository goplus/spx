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

package engine

import (
	"errors"
	"io"
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

func TestEngineBuildLockPreservesPermissionDeniedOwner(t *testing.T) {
	// PID 1 belongs to the system; a null signal only checks access.
	if err := syscall.Kill(1, 0); !errors.Is(err, syscall.EPERM) {
		t.Skipf("PID 1 is not protected from this user: %v", err)
	}
	assertEngineBuildLockPreservedOnProbeError(t, 1, syscall.EPERM)
}

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
	switch mode := os.Getenv("SPX_TRACKED_PROCESS_HELPER"); mode {
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
	case "interrupt-clean", "interrupt-error":
		termination := make(chan os.Signal, 1)
		if mode == "interrupt-clean" {
			signal.Notify(termination, syscall.SIGTERM)
			defer signal.Stop(termination)
		} else {
			signal.Ignore(syscall.SIGTERM)
		}
		if err := os.WriteFile(os.Getenv("SPX_TRACKED_PROCESS_MARKER"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			t.Fatal(err)
		}
		if mode == "interrupt-clean" {
			<-termination
			// Avoid emitting a child test summary into the parent command output.
			os.Exit(0)
		}
	default:
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestTrackedEngineCommandInterrupted(t *testing.T) {
	for _, mode := range []string{"interrupt-clean", "interrupt-error"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "ready")
			// The race runtime otherwise sleeps for the entire termination grace period.
			t.Setenv("GORACE", "atexit_sleep_ms=0")
			t.Setenv("SPX_TRACKED_PROCESS_HELPER", mode)
			t.Setenv("SPX_TRACKED_PROCESS_MARKER", marker)
			// Keep SIGINT intercepted even in the short intervals before and after
			// the command installs its own handler. No unrelated process is signaled.
			guard := make(chan os.Signal, 1)
			signal.Notify(guard, os.Interrupt)
			defer signal.Stop(guard)
			done := make(chan error, 1)
			go func() { done <- runTrackedEngineCommand("", os.Args[0], "-test.run=^TestTrackedProcessHelper$", "--") }()
			deadline := time.Now().Add(5 * time.Second)
			var pid int
			for pid == 0 {
				if data, err := os.ReadFile(marker); err == nil {
					pid, _ = strconv.Atoi(string(data))
				}
				if time.Now().After(deadline) {
					t.Fatal("tracked helper did not become ready")
				}
				select {
				case err := <-done:
					t.Fatalf("tracked command exited before interrupt: %v", err)
				default:
				}
				time.Sleep(5 * time.Millisecond)
			}
			finished := false
			defer func() {
				if !finished {
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
			}()
			for {
				if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					finished = true
					if mode == "interrupt-clean" {
						if err == nil || !strings.Contains(err.Error(), "engine build interrupted") {
							t.Fatalf("clean interrupt=%v", err)
						}
					} else {
						var exitErr *exec.ExitError
						if !errors.As(err, &exitErr) {
							t.Fatalf("forced termination=%v", err)
						}
					}
					return
				case <-time.After(50 * time.Millisecond):
					if time.Now().After(deadline) {
						t.Fatal("tracked command ignored interrupt")
					}
				}
			}
		})
	}
	terminateTrackedProcessGroup(nil)
}
