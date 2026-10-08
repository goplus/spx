/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

func TestParseEngineExecArgsCommand(t *testing.T) {
	cfg, err := parseEngineExecArgs([]string{"--lock-dir", "godot/.spx_build_lock", "--workdir", "godot", "--", "scons", "target=editor"})
	if err != nil {
		t.Fatalf("parseEngineExecArgs returned error: %v", err)
	}
	if cfg.lockDir != "godot/.spx_build_lock" {
		t.Fatalf("unexpected lock dir: %s", cfg.lockDir)
	}
	if cfg.workdir != "godot" {
		t.Fatalf("unexpected workdir: %s", cfg.workdir)
	}
	if len(cfg.command) != 2 || cfg.command[0] != "scons" || cfg.command[1] != "target=editor" {
		t.Fatalf("unexpected command: %#v", cfg.command)
	}
}

func TestParseEngineExecArgsScript(t *testing.T) {
	cfg, err := parseEngineExecArgs([]string{"--lock-dir", "godot/.spx_build_lock", "--script", "echo ok"})
	if err != nil {
		t.Fatalf("parseEngineExecArgs returned error: %v", err)
	}
	if cfg.script != "echo ok" {
		t.Fatalf("unexpected script: %s", cfg.script)
	}
	if len(cfg.command) != 0 {
		t.Fatalf("unexpected command: %#v", cfg.command)
	}
}

func TestDetectStaleEngineBuildLockInvalidPID(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	mustMkdirAll(t, lockDir)
	mustWriteFile(t, filepath.Join(lockDir, "pid"), []byte("invalid"))

	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if err != nil {
		t.Fatalf("detectStaleEngineBuildLock returned error: %v", err)
	}
	if !stale {
		t.Fatal("expected invalid pid metadata lock to be stale")
	}
	if !strings.Contains(message, "invalid pid metadata") {
		t.Fatalf("unexpected stale lock message: %s", message)
	}
}

func TestDetectStaleEngineBuildLockMissingPIDAfterGracePeriod(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	mustMkdirAll(t, lockDir)
	oldTime := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(lockDir, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes lock dir: %v", err)
	}

	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if err != nil {
		t.Fatalf("detectStaleEngineBuildLock returned error: %v", err)
	}
	if !stale {
		t.Fatal("expected missing pid metadata lock to be stale")
	}
	if !strings.Contains(message, "missing pid metadata") {
		t.Fatalf("unexpected stale lock message: %s", message)
	}
}

func TestDetectStaleEngineBuildLockMissingPIDWithinGracePeriod(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	mustMkdirAll(t, lockDir)
	freshTime := time.Now()
	if err := os.Chtimes(lockDir, freshTime, freshTime); err != nil {
		t.Fatal(err)
	}
	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if time.Since(freshTime) >= 4*time.Second {
		t.Skip("scheduler delay consumed the fresh lock's grace period")
	}
	if err != nil || stale || message != "" {
		t.Fatalf("fresh lock = (%v, %q, %v), want non-stale with no message", stale, message, err)
	}
}

func TestDetectStaleEngineBuildLockProtectsLivePID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("trackedProcessExists opens the process without SYNCHRONIZE, which WaitForSingleObject requires")
	}
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	pidPath := filepath.Join(lockDir, "pid")
	pidData := fmt.Sprintf("  %d\n", os.Getpid())
	mustWriteFile(t, pidPath, []byte(pidData))
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lockDir, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if err != nil || stale || message != "" {
		t.Fatalf("live lock = (%v, %q, %v), want non-stale with no message", stale, message, err)
	}
	if data, err := os.ReadFile(pidPath); err != nil || string(data) != pidData {
		t.Fatalf("live lock metadata = %q (%v), want %q", data, err, pidData)
	}
}

func TestEngineBuildLockRecoversExitedPID(t *testing.T) {
	// Reap a real helper rather than assuming an arbitrary PID is unused.
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run helper: %v\n%s", err, output)
	}
	pid := cmd.Process.Pid
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	mustWriteFile(t, filepath.Join(lockDir, "pid"), []byte(fmt.Sprintf("%d\n", pid)))
	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if err != nil || !stale || !strings.Contains(message, fmt.Sprintf("pid %d is dead", pid)) {
		t.Fatalf("exited PID lock = (%v, %q, %v), want stale dead-PID message", stale, message, err)
	}
	t.Cleanup(func() { releaseEngineBuildLock(lockDir) })
	for i := 0; i < 2; i++ {
		if err := acquireEngineBuildLock(lockDir); err != nil {
			t.Fatalf("acquire lock (attempt %d): %v", i+1, err)
		}
		wantPID := fmt.Sprintf("%d\n", os.Getpid())
		if data, err := os.ReadFile(filepath.Join(lockDir, "pid")); err != nil || string(data) != wantPID {
			t.Fatalf("acquired lock metadata = %q (%v), want %q", data, err, wantPID)
		}
		releaseEngineBuildLock(lockDir)
		if _, err := os.Lstat(lockDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("released lock still exists: %v", err)
		}
	}
}

func TestEngineBuildLockReplacesSymlinkWithoutRemovingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "preserved")
	pidData := fmt.Sprintf("%d\n", os.Getpid())
	mustWriteFile(t, filepath.Join(target, "pid"), []byte(pidData))
	mustWriteFile(t, filepath.Join(target, "artifact"), []byte("preserved artifact"))
	lockDir := filepath.Join(root, ".spx_build_lock")
	if err := os.Symlink(target, lockDir); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
	stale, message, err := detectStaleEngineBuildLock(lockDir)
	if err != nil || !stale || !strings.Contains(message, "invalid build lock symlink") {
		t.Fatalf("symlink lock = (%v, %q, %v), want stale symlink message", stale, message, err)
	}
	if err := acquireEngineBuildLock(lockDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { releaseEngineBuildLock(lockDir) })
	if info, err := os.Lstat(lockDir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("replacement lock is not a directory: %v (%v)", info, err)
	}
	releaseEngineBuildLock(lockDir)
	if _, err := os.Lstat(lockDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("released lock still exists: %v", err)
	}
	for name, want := range map[string]string{"pid": pidData, "artifact": "preserved artifact"} {
		if data, err := os.ReadFile(filepath.Join(target, name)); err != nil || string(data) != want {
			t.Fatalf("symlink target %s = %q (%v), want %q", name, data, err, want)
		}
	}
}

func TestLockedEngineScriptCommand(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	for _, tt := range []struct{ name, workdir, lockDir string }{
		{"engine", "godot", "godot/.spx_build_lock"},
		{"android-post", "godot/platform/android/java", "godot/platform/android/.spx_build_lock"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			workdir := filepath.Join(root, tt.workdir)
			mustMkdirAll(t, workdir)
			lockDir := filepath.Join(root, tt.lockDir)
			env := shared.CurrentEnvMap()
			env["SPX_EXPECTED_DIR"] = workdir
			env["SPX_EXPECTED_LOCK"] = lockDir
			env["SPX_MARKER"] = "space ' quote $literal"
			for _, exitCode := range []int{0, 23} {
				t.Run(fmt.Sprintf("exit-%d", exitCode), func(t *testing.T) {
					script := fmt.Sprintf(`[[ "$PWD" == "$SPX_EXPECTED_DIR" ]] || exit 90
[[ -d "$SPX_EXPECTED_LOCK" && -s "$SPX_EXPECTED_LOCK/pid" ]] || exit 91
printf '%%s' "$SPX_MARKER" > captured
exit %d`, exitCode)
					err := runLockedEngineCommandWithEnv(workdir, env, "bash", "-lc", script)
					if exitCode == 0 {
						if err != nil {
							t.Fatal(err)
						}
					} else {
						var exitErr *exec.ExitError
						if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitCode {
							t.Fatalf("command error = %v, want exit %d", err, exitCode)
						}
					}
					got, err := os.ReadFile(filepath.Join(workdir, "captured"))
					if err != nil || string(got) != env["SPX_MARKER"] {
						t.Fatalf("command environment = %q (%v)", got, err)
					}
					if _, err := os.Stat(lockDir); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("lock was not released: %v", err)
					}
				})
			}
			if err := runLockedEngineCommandWithEnv(workdir, env, filepath.Join(root, "missing-command")); err == nil {
				t.Fatal("missing command succeeded")
			}
			if _, err := os.Stat(lockDir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("start failure retained lock: %v", err)
			}
		})
	}
}
