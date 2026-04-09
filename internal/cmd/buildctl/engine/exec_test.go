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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestAcquireEngineBuildLockTimesOut(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), ".spx_build_lock")
	mustMkdirAll(t, lockDir)
	mustWriteFile(t, filepath.Join(lockDir, "pid"), []byte(strconv.Itoa(os.Getpid())))

	oldPollInterval := EngineBuildLockPollInterval
	oldTimeout := EngineBuildLockAcquireTimeout
	EngineBuildLockPollInterval = 5 * time.Millisecond
	EngineBuildLockAcquireTimeout = 20 * time.Millisecond
	defer func() {
		EngineBuildLockPollInterval = oldPollInterval
		EngineBuildLockAcquireTimeout = oldTimeout
	}()

	err := acquireEngineBuildLock(lockDir)
	if err == nil {
		t.Fatal("expected acquireEngineBuildLock to time out")
	}
	if !strings.Contains(err.Error(), "timed out waiting for build lock") {
		t.Fatalf("unexpected timeout error: %v", err)
	}
}
