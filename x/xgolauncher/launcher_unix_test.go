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
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
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
	status, err := run(context.Background(), payload, payloadDigest, manifestDigest,
		[]string{"xgo-driver-v1", "", "a b", "--"}, nil, &stdout, &stderr, cacheRoot)
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
	status, err = run(context.Background(), payload, payloadDigest, manifestDigest, nil, nil, &stdout, &stderr, cacheRoot)
	if err != nil {
		t.Fatalf("cache repair error = %v, stdout = %q, stderr = %q", err, stdout.String(), stderr.String())
	}
	if status.Code != 42 || status.Signal != 0 || !strings.Contains(stdout.String(), "FAKE_ENGINE_OK") {
		t.Fatalf("cache repair status = %+v, stdout = %q, stderr = %q", status, stdout.String(), stderr.String())
	}
}

func TestRunPreservesEngineSignal(t *testing.T) {
	payload, payloadDigest, manifestDigest, _ := launcherTestPayload(t, "#!/bin/sh\nkill -TERM $$\n")
	status, err := run(context.Background(), payload, payloadDigest, manifestDigest, nil, nil, nil, &bytes.Buffer{}, filepath.Join(t.TempDir(), "cache"))
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
	status, err := run(context.Background(), payload, payloadDigest, manifestDigest, []string{"--path=/host"}, nil, nil, &stderr, filepath.Join(t.TempDir(), "cache"))
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
	command := exec.Command(os.Args[0], "-test.run=^TestExitReproducesSignal$")
	command.Env = append(os.Environ(), "SPX_LAUNCHER_EXIT_HELPER=1")
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

func launcherTestPayload(t *testing.T, engineScript string) ([]byte, string, string, runtimepayload.Manifest) {
	t.Helper()
	engineFiles := []runtimepayload.File{
		{Name: "runtime-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-engine/v1"}`)},
		{Name: "engine", Mode: 0o755, Data: []byte(engineScript)},
		{Name: "engine.pck", Mode: 0o644, Data: []byte("pack")},
	}
	bridgeFiles := []runtimepayload.File{
		{Name: "bridge-manifest.json", Mode: 0o644, Data: []byte(`{"schema":"test-bridge/v1"}`)},
		{Name: "bridge.so", Mode: 0o755, Data: []byte("bridge")},
	}
	projectFiles := []runtimepayload.File{
		{Name: "main.spx", Mode: 0o644, Data: []byte("onStart => {}")},
		{Name: "assets/index.json", Mode: 0o644, Data: []byte(`{"zorder":[]}`)},
	}
	engineZIP := launcherTestZIP(t, engineFiles)
	bridgeZIP := launcherTestZIP(t, bridgeFiles)
	projectZIP := launcherTestZIP(t, projectFiles)
	engine := launcherTestBundle(t, engineZIP, runtimebundle.NamespaceEngine)
	bridge := launcherTestBundle(t, bridgeZIP, runtimebundle.NamespaceBridge)
	project := launcherTestBundle(t, projectZIP, runtimebundle.NamespaceProject)
	projectSum := sha256.Sum256(projectZIP)
	interfaceSum := sha256.Sum256([]byte("interface"))
	config := runtimepayload.BuildConfig{
		SPX:    runtimepayload.SourceIdentity{SelectedPath: "github.com/goplus/spx/v3", EffectivePath: "github.com/goplus/spx/v3", SourceMode: true},
		Target: runtimepayload.Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Engine: runtimepayload.Engine{
			RuntimeVersion: "test", RuntimeABI: 1, EngineInterfaceDigest: hex.EncodeToString(interfaceSum[:]),
			Executable: "engine", Pack: "engine.pck", BundleDigest: engine.Digest,
		},
		Bridge: runtimepayload.Bridge{File: "bridge.so", BundleDigest: bridge.Digest},
		Project: runtimepayload.Project{
			PackDirectory: "assets", BundleDigest: project.Digest, ArchiveSHA256: hex.EncodeToString(projectSum[:]),
		},
		Files: []runtimepayload.File{
			{Name: "engine/runtime-manifest.json", Mode: 0o644, Data: engineFiles[0].Data},
			{Name: "engine/engine", Mode: 0o755, Data: engineFiles[1].Data},
			{Name: "engine/engine.pck", Mode: 0o644, Data: engineFiles[2].Data},
			{Name: "bridge/bridge-manifest.json", Mode: 0o644, Data: bridgeFiles[0].Data},
			{Name: "bridge/bridge.so", Mode: 0o755, Data: bridgeFiles[1].Data},
			{Name: runtimepayload.ProjectZipPath, Mode: 0o644, Data: projectZIP},
		},
	}
	payload, payloadDigest, manifestDigest, err := runtimepayload.Build(config)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := runtimepayload.Verify(payload, payloadDigest, manifestDigest, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return payload, payloadDigest, manifestDigest, verified.Manifest
}

func launcherTestZIP(t *testing.T, files []runtimepayload.File) []byte {
	t.Helper()
	data, err := runtimepayload.CanonicalComponentZIP(files)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func launcherTestBundle(t *testing.T, data []byte, namespace runtimebundle.Namespace) runtimebundle.Bundle {
	t.Helper()
	bundle, err := runtimepayload.ComponentBundle(data, namespace)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
