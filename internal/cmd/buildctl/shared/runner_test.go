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

package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestCommandRunnerRunCommandUsesGoPathBin(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	binDir := filepath.Join(gopath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmdPath := filepath.Join(binDir, "fakecmd")
	outPath := filepath.Join(root, "ran.txt")
	script := "#!/bin/sh\nprintf 'ok' > \"$OUT_PATH\"\n"
	if err := os.WriteFile(cmdPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("OUT_PATH", outPath)

	runner := CommandRunner{RepoRoot: root}
	if err := runner.RunCommand(".", "fakecmd"); err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	if !fileExists(outPath) {
		t.Fatalf("expected fake command output at %s", outPath)
	}
}

func TestCommandRunnerRunScriptUsesLockedGoToolchain(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))

	outputPath := filepath.Join(root, "toolchain.txt")
	scriptPath := filepath.Join(root, "capture-toolchain.sh")
	script := "#!/bin/bash\nset -euo pipefail\nprintf '%s' \"$GOTOOLCHAIN\" > \"$OUTPUT_PATH\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("OUTPUT_PATH", outputPath)

	runner := CommandRunner{RepoRoot: root}
	if err := runner.RunScript(filepath.Base(scriptPath)); err != nil {
		t.Fatalf("runScript returned error: %v", err)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	want := "go" + release.DefaultRuntimeLock().Toolchain.Go
	if string(got) != want {
		t.Fatalf("script GOTOOLCHAIN = %q, want %q", got, want)
	}
}

func TestBuildctlCommandEnvPrependsGoBinDirs(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	env, err := buildctlCommandEnv()
	if err != nil {
		t.Fatalf("buildctlCommandEnv returned error: %v", err)
	}

	pathDirs := filepath.SplitList(pathEnvValue(env))
	wantDirs := []string{filepath.Join(gopath, "bin")}
	if goRoot := runtime.GOROOT(); goRoot != "" {
		wantDirs = append(wantDirs, filepath.Join(goRoot, "bin"))
	}
	if len(pathDirs) < len(wantDirs) {
		t.Fatalf("PATH dirs = %v, want prefix %v", pathDirs, wantDirs)
	}
	for i, want := range wantDirs {
		if pathDirs[i] != want {
			t.Fatalf("PATH dirs = %v, want prefix %v", pathDirs, wantDirs)
		}
	}
	if got, want := env["GOTOOLCHAIN"], "go"+release.DefaultRuntimeLock().Toolchain.Go; got != want {
		t.Fatalf("GOTOOLCHAIN = %q, want %q", got, want)
	}
}

func TestResolveCommandPathUsesCaseInsensitivePathFallback(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmdPath := filepath.Join(binDir, "fakecmd")
	if err := os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := resolveCommandPath("fakecmd", map[string]string{"Path": binDir})
	if err != nil {
		t.Fatalf("resolveCommandPath returned error: %v", err)
	}
	if got != cmdPath {
		t.Fatalf("resolveCommandPath = %q, want %q", got, cmdPath)
	}
}

func TestIsCommandFileForOSAllowsWindowsFilesWithoutUnixExecBit(t *testing.T) {
	root := t.TempDir()
	cmdPath := filepath.Join(root, "fakecmd.exe")
	if err := os.WriteFile(cmdPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	info, err := os.Stat(cmdPath)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}

	if !isCommandFileForOS(info, "windows") {
		t.Fatal("expected regular Windows command file to be accepted without Unix exec bit")
	}
	if isCommandFileForOS(info, "linux") {
		t.Fatal("expected non-executable Unix command file to be rejected")
	}
}

func TestCommandRunnerRunCommandReturnsEnvironmentError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")

	runner := CommandRunner{RepoRoot: root}
	err := runner.RunCommand(".", "fakecmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when command environment cannot be resolved")
	}
	if !strings.Contains(err.Error(), "resolve command environment") {
		t.Fatalf("runCommand error = %v, want environment resolution context", err)
	}
}

func TestCommandRunnerRunCommandReturnsResolvePathError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))

	runner := CommandRunner{RepoRoot: root}
	err := runner.RunCommand(".", "missingcmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when the command cannot be found")
	}
	if !strings.Contains(err.Error(), "resolve command path for missingcmd") {
		t.Fatalf("runCommand error = %v, want command path resolution context", err)
	}
}
