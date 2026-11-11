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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMakeExportsSpineSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed")
	}
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	fake := filepath.Join(root, "buildctl")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf 'SPX_SPINE=%s ARGS=%s\\n' \"$SPX_SPINE\" \"$*\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPX_SPINE", "0")
	for _, target := range []string{"dev", "build-editor", "build-desktop", "build-web"} {
		cmd := exec.Command("make", "--no-print-directory", "-f", filepath.Join(repo, "Makefile"), target, "BUILDCTL_BIN="+fake, "BUILDCTL_SOURCES=", "SPX_SPINE=1")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(output), "SPX_SPINE=1 ARGS=build ") {
			t.Fatalf("make %s: %v\n%s", target, err, output)
		}
	}
}

func TestSpineGuardRunsBeforeBuildWorkflow(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", root)
	t.Setenv("SPX_SPINE", "1")
	t.Setenv("SPX_SPINE_GOPATH", "")
	if err := run([]string{"build", "editor"}); err == nil || !strings.Contains(err.Error(), "dedicated") {
		t.Fatalf("missing early guard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "bin")); !os.IsNotExist(err) {
		t.Fatalf("guard wrote bin before rejecting: %v", err)
	}
}

func TestSpineReadOnlyCommandsDoNotPrepareWorkspace(t *testing.T) {
	root := t.TempDir()
	goPath := filepath.Join(root, "spine")
	t.Setenv("GOPATH", goPath)
	t.Setenv("GOBIN", "")
	t.Setenv("SPX_SPINE", "1")
	t.Setenv("SPX_SPINE_GOPATH", goPath)
	t.Setenv("GODOT_SRC", filepath.Join(root, "missing-godot"))
	t.Setenv("SPX_MODULE_SRC", "")
	t.Setenv("PLATFORM", "")
	for _, args := range [][]string{
		{"help"}, {"doctor"}, {"env", "export-shell"},
		{"env", "export-engine-build-shell", "--target", "editor"},
		{"workflow", "list-demos"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if err := run(args); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(goPath); !os.IsNotExist(err) {
				t.Fatalf("read-only command created the workspace: %v", err)
			}
		})
	}
}

func TestSpineHelpAndParseErrorsDoNotRequireWorkspace(t *testing.T) {
	goPath := filepath.Join(t.TempDir(), "spine")
	t.Setenv("GOPATH", goPath)
	t.Setenv("SPX_SPINE", "1")
	t.Setenv("SPX_SPINE_GOPATH", "")
	for _, args := range [][]string{
		{"help"}, {"build", "--help"}, {"setup", "--help"},
		{"doctor", "--help"}, {"env", "export-shell", "--help"},
		{"engine", "download", "--help"}, {"tool", "install", "--help"},
		{"runtime", "export-pack", "--help"}, {"workflow", "list-demos", "--help"},
	} {
		if err := run(args); err != nil {
			t.Fatalf("help %v requires a workspace: %v", args, err)
		}
	}
	// A configured but nonexistent workspace must also remain absent on parse errors.
	t.Setenv("SPX_SPINE_GOPATH", goPath)
	for _, args := range [][]string{
		{"unknown"}, {"build", "unknown"}, {"build", "editor", "--bad"},
		{"setup", "host", "--bad"}, {"doctor", "--bad"},
		{"env", "export-shell", "--bad"}, {"engine", "download", "--bad"},
		{"tool", "install", "--bad"}, {"runtime", "export-pack", "--bad"},
		{"runtime", "export-web", "--mode", "bad"}, {"workflow", "list-demos", "--bad"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("invalid command accepted: %v", args)
		}
	}
	if _, err := os.Stat(goPath); !os.IsNotExist(err) {
		t.Fatalf("help or parse error created the workspace: %v", err)
	}
}
