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
	"strings"
	"testing"
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

	runner := commandRunner{repoRoot: root}
	if err := runner.runCommand(".", "fakecmd"); err != nil {
		t.Fatalf("runCommand returned error: %v", err)
	}

	if !fileExists(outPath) {
		t.Fatalf("expected fake command output at %s", outPath)
	}
}

func TestCommandRunnerRunCommandReturnsEnvironmentError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")

	runner := commandRunner{repoRoot: root}
	err := runner.runCommand(".", "fakecmd")
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

	runner := commandRunner{repoRoot: root}
	err := runner.runCommand(".", "missingcmd")
	if err == nil {
		t.Fatal("expected runCommand to fail when the command cannot be found")
	}
	if !strings.Contains(err.Error(), "resolve command path for missingcmd") {
		t.Fatalf("runCommand error = %v, want command path resolution context", err)
	}
}
