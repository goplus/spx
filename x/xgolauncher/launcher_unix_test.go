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
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func TestRunEmbeddedPayloadAndRepairCache(t *testing.T) {
	script := `#!/bin/sh
printf 'ARGV='
for arg in "$@"; do printf '<%s>' "$arg"; done
printf '\n'
actual_cwd=$(pwd -P)
printf 'ROOTS=<%s><%s><%s><%s>\n' "$SPX_PROJECT_DIR" "$SPX_ASSET_DIR" "$SPX_SESSION_DIR" "$actual_cwd"
[ -f "$SPX_PROJECT_DIR/main.spx" ] || { printf 'missing project\n' >&2; exit 91; }
[ -f "$SPX_ASSET_DIR/index.json" ] || { printf 'missing asset index\n' >&2; exit 92; }
[ "$actual_cwd" -ef "$SPX_SESSION_DIR" ] || { printf 'wrong cwd\n' >&2; exit 93; }
printf 'FAKE_ENGINE_OK\n'
exit 42
`
	payload, payloadDigest, manifestDigest, manifest := launcherTestPayload(t, script)
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	var stdout, stderr bytes.Buffer
	cfg := Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
		Args: []string{"xgo-driver-v1", "", "a b", "--"}, Stdout: &stdout, Stderr: &stderr,
	}
	status, err := run(context.Background(), cfg, cacheRoot)
	if err != nil {
		t.Fatalf("run error = %v, stdout = %q, stderr = %q", err, stdout.String(), stderr.String())
	}
	if status != (ProcessStatus{Code: 42}) {
		t.Fatalf("status = %+v, stdout = %q, stderr = %q", status, stdout.String(), stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "ARGV=<xgo-driver-v1><><a b><-->") || !strings.Contains(got, "FAKE_ENGINE_OK") {
		t.Fatalf("stdout = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}

	enginePath := filepath.Join(cacheRoot, string(runtimebundle.NamespaceEngine), manifest.Engine.BundleDigest, manifest.Engine.Executable)
	if err := os.WriteFile(enginePath, []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	cfg.Args = nil
	status, err = run(context.Background(), cfg, cacheRoot)
	if err != nil {
		t.Fatalf("cache repair error = %v, stdout = %q, stderr = %q", err, stdout.String(), stderr.String())
	}
	if status.Code != 42 || status.Signal != 0 || !strings.Contains(stdout.String(), "FAKE_ENGINE_OK") {
		t.Fatalf("cache repair status = %+v, stdout = %q, stderr = %q", status, stdout.String(), stderr.String())
	}
}

func TestRunPreservesEngineSignal(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "#!/bin/sh\nkill -TERM $$\n")
	status, err := run(context.Background(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest, Stderr: &bytes.Buffer{},
	}, filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatalf("run error = %v", err)
	}
	if status.Code != 0 || status.Signal != int(syscall.SIGTERM) {
		t.Fatalf("status = %+v, want signal %d", status, syscall.SIGTERM)
	}
}

func TestRunRejectsReservedEnginePath(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "#!/bin/sh\nexit 0\n")
	var stderr bytes.Buffer
	status, err := run(context.Background(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
		Args: []string{"--path=/host"}, Stderr: &stderr,
	}, filepath.Join(t.TempDir(), "cache"))
	if err == nil || !strings.Contains(err.Error(), "--path is reserved") {
		t.Fatalf("error = %v, want reserved path error", err)
	}
	if status != (ProcessStatus{}) || stderr.Len() != 0 {
		t.Fatalf("status = %+v, stderr = %q", status, stderr.String())
	}
}

func TestExitReproducesSignal(t *testing.T) {
	if os.Getenv("SPX_LAUNCHER_EXIT_HELPER") == "1" {
		Exit(ProcessStatus{Signal: int(syscall.SIGTERM)})
		return
	}
	command := launcherTestCommand(t, "SPX_LAUNCHER_EXIT_HELPER=1")
	err := command.Run()
	exitError, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("helper error = %v, want ExitError", err)
	}
	wait, ok := exitError.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !wait.Signaled() || wait.Signal() != syscall.SIGTERM {
		t.Fatalf("helper wait status = %v, want SIGTERM", exitError.ProcessState.Sys())
	}
}

func TestRunUsesDefaultCache(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME controls the default cache on Linux")
	}
	payload, payloadDigest, manifestDigest, manifest := launcherTestPayload(t, `#!/bin/sh
IFS= read -r line
printf 'input:<%s>\n' "$line"
printf 'engine stderr\n' >&2
`)
	cacheHome := t.TempDir()
	tempRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	launcherTestTempDir(t, tempRoot)
	var stdout, stderr bytes.Buffer
	status, err := Run(Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
		Stdin: strings.NewReader("hello launcher\n"), Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil || !status.Success() {
		t.Fatalf("Run() = (%+v, %v), want success", status, err)
	}
	if stdout.String() != "input:<hello launcher>\n" || stderr.String() != "engine stderr\n" {
		t.Fatalf("stdout = %q, stderr = %q, want forwarded I/O", stdout.String(), stderr.String())
	}
	engine := filepath.Join(cacheHome, "spx", "runtimebundle", string(runtimebundle.NamespaceEngine), manifest.Engine.BundleDigest, manifest.Engine.Executable)
	if _, err := os.Stat(engine); err != nil {
		t.Fatalf("engine in default cache: %v", err)
	}
	if entries, err := os.ReadDir(tempRoot); err != nil || len(entries) != 0 {
		t.Fatalf("temporary files = %v, error = %v, want work and session cleanup", entries, err)
	}
}

func TestRunEngineStartError(t *testing.T) {
	missingInterpreter := filepath.Join(t.TempDir(), "missing-interpreter")
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "#!"+missingInterpreter+"\n")
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	tempRoot := t.TempDir()
	launcherTestTempDir(t, tempRoot)
	status, err := run(t.Context(), Config{
		Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
	}, cacheRoot)
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "run Engine:") || status != (ProcessStatus{}) {
		t.Fatalf("run() = (%+v, %v), want engine start error", status, err)
	}
	if entries, err := os.ReadDir(tempRoot); err != nil || len(entries) != 0 {
		t.Fatalf("temporary files = %v, error = %v, want work and session cleanup", entries, err)
	}
}

func TestRunSessionDirectoryErrorReleasesLeases(t *testing.T) {
	payload, payloadDigest, manifestDigest, manifest := launcherTestPayload(t, "#!/bin/sh\nexit 0\n")
	// The temporary root is a literal path, even when its name contains glob metacharacters.
	tempRoot := filepath.Join(t.TempDir(), "[launcher]")
	if err := os.Mkdir(tempRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	locks := runtimebundle.CrossProcessLockProvider{}
	held, err := locks.AcquireExclusive(ctx, filepath.Join(cacheRoot, string(runtimebundle.NamespaceProject), manifest.Project.BundleDigest))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		_ = held.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("launcher goroutine did not stop")
		}
	})
	var status ProcessStatus
	var runErr error
	go func() {
		status, runErr = run(ctx, Config{
			Payload: payload, PayloadSHA256: payloadDigest, ManifestSHA256: manifestDigest,
		}, cacheRoot)
		close(done)
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		matches, err := fs.Glob(os.DirFS(tempRoot), "spx-launcher-*/project.zip")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 1 {
			break
		}
		select {
		case <-done:
			t.Fatalf("run returned before project materialization: (%+v, %v)", status, runErr)
		case <-ctx.Done():
			t.Fatal("launcher did not reach project materialization")
		case <-ticker.C:
		}
	}
	// The project lease prevents session creation until TMPDIR is unavailable.
	t.Setenv("TMPDIR", filepath.Join(tempRoot, "missing"))
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("launcher did not return after the project lease was released")
	}
	if status != (ProcessStatus{}) || !errors.Is(runErr, os.ErrNotExist) || !strings.Contains(runErr.Error(), "xgolauncher: create Engine session:") {
		t.Fatalf("run = (%+v, %v), want session directory creation error", status, runErr)
	}
	if entries, err := os.ReadDir(tempRoot); err != nil || len(entries) != 0 {
		t.Fatalf("temporary directories after run = %v, %v; want none", entries, err)
	}
	for namespace, digest := range map[runtimebundle.Namespace]string{
		runtimebundle.NamespaceEngine:  manifest.Engine.BundleDigest,
		runtimebundle.NamespaceBridge:  manifest.Bridge.BundleDigest,
		runtimebundle.NamespaceProject: manifest.Project.BundleDigest,
	} {
		lease, err := locks.AcquireExclusive(ctx, filepath.Join(cacheRoot, string(namespace), digest))
		if err != nil {
			t.Fatalf("acquire exclusive %s lease after run: %v", namespace, err)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
