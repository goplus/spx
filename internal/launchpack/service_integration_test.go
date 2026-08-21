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

package launchpack

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/internal/release"
)

func TestBuildLauncherEndToEnd(t *testing.T) {
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	if output, err := exec.Command(goCommand, "env", "CGO_ENABLED").Output(); err != nil || strings.TrimSpace(string(output)) != "1" {
		t.Skip("cgo toolchain unavailable")
	}
	goCommand, err = filepath.EvalSymlinks(goCommand)
	if err != nil {
		t.Fatal(err)
	}

	repoRoot := launchpackRepoRoot(t)
	moduleRoot := t.TempDir()
	writeProjectTestFile(t, filepath.Join(moduleRoot, "go.mod"), "module example.com/launcher\n\ngo 1.25.0\n\nrequire github.com/goplus/spx/v3 v3.0.0\nreplace github.com/goplus/spx/v3 => "+filepath.ToSlash(repoRoot)+"\n")
	writeProjectTestFile(t, filepath.Join(moduleRoot, "bridge", "main.go"), "package main\n\n// void bridge(void) {}\nimport \"C\"\n\nfunc main() {}\n")

	projectDir := filepath.Join(moduleRoot, "game")
	projectFile := filepath.Join(projectDir, "main.spx")
	writeProjectTestFile(t, projectFile, "onStart => {}\n")
	writeProjectTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}\n")
	snapshot, err := projectpolicy.SnapshotPortableConfig(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	lock := release.DefaultRuntimeLock()
	spec, err := release.HostRuntimeSpecFor(lock, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	runtimeRoot := filepath.Join(moduleRoot, "runtime")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := publishLocalRuntimeTest(t, runtimeRoot, filepath.Join(runtimeRoot, "manifest.json"), spec, "engine", "pack")
	output := filepath.Join(moduleRoot, "launcher"+executableSuffix(runtime.GOOS))

	var buildLog bytes.Buffer
	graphChecks := 0
	result, err := BuildLauncher(context.Background(), Config{
		ProjectDir: projectDir, ProjectFile: projectFile, ProjectExt: ".spx",
		PackDir: "assets", PackIndex: "index.json", PortableConfig: snapshot,
		RuntimeManifestPath: manifest, RuntimeCacheRoot: filepath.Join(moduleRoot, "cache"),
		RuntimeIdentity: RuntimeIdentity{Version: lock.RuntimeVersion, ABI: lock.RuntimeABI, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		RuntimeLock:     lock,
		Source:          SourceIdentity{SelectedPath: "github.com/goplus/spx/v3", SelectedVersion: "v3.0.0", EffectivePath: repoRoot, SourceMode: true},
		GoCommand:       goCommand, WorkDir: moduleRoot, GoWork: "off", GraphFlags: []string{"-mod=mod"},
		BuildFlags: []string{"-trimpath=true", "-buildvcs=false"},
		Output:     output, BridgePackage: "./bridge", VerifyGraph: func(context.Context) error {
			graphChecks++
			return nil
		},
		IO: IO{Stdout: &buildLog, Stderr: &buildLog, Env: os.Environ()},
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, buildLog.String())
	}
	if result.Output != output || len(result.PayloadSHA256) != 64 || len(result.ManifestSHA256) != 64 || graphChecks != 4 {
		t.Fatalf("result = %#v", result)
	}
}

func launchpackRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(dir, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func executableSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}
