package main

import (
	"os"
	"path/filepath"
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
