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

package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesRuntimePackTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"game.go", "package spx\n", true},
		{"runtime_capture_js.go", "package spx\n", false},
		{"runtime_capture_windows.go", "package spx\n", false},
		{"runtime_capture_linux.go", "package spx\n", true},
		{"open_mobile.go", "//go:build android || ios\n\npackage asset\n", false},
		{"path_packmode.go", "//go:build packmode\n\npackage engine\n", false},
		{"facade_pure.go", "//go:build pure_engine\n\npackage facade\n", false},
		{"facade_web.go", "//go:build js\n\npackage facade\n", false},
		{"macro_on.go", "//go:build profiler\n\npackage profiler\n", false},
		{"macro_off.go", "//go:build !profiler\n\npackage profiler\n", true},
		{"ffi.go", "//go:build cgo && !js\n\npackage native\n", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := matchesRuntimePackTarget(test.name, []byte(test.content))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("matchesRuntimePackTarget() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRuntimePackSourceSHA256HonorsBuildConstraints(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=SPX Test", "GIT_AUTHOR_EMAIL=spx@example.com",
			"GIT_COMMITTER_NAME=SPX Test", "GIT_COMMITTER_EMAIL=spx@example.com",
		)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) {
		t.Helper()
		runGit("add", ".")
		runGit("commit", "-q", "-m", message)
	}

	runGit("init", "-q")
	write("game.go", "package spx\n")
	write("runtime_capture_js.go", "package spx\n")
	write(runtimePackExportPresets, `[preset.0]
name="Android"
[preset.0.options]
version/code=1
[preset.1]
name="Linux"
[preset.1.options]
binary_format/architecture="x86_64"
`)
	commit("initial")
	baseline, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	write("runtime_capture_js.go", "package spx\n\nconst changed = true\n")
	commit("change inactive source")
	inactive, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if inactive != baseline {
		t.Fatal("inactive JS source changed Linux runtime pack digest")
	}

	presetsPath := filepath.Join(repo, filepath.FromSlash(runtimePackExportPresets))
	presets, err := os.ReadFile(presetsPath)
	if err != nil {
		t.Fatal(err)
	}
	write(runtimePackExportPresets, strings.Replace(string(presets), "version/code=1", "version/code=2", 1))
	commit("change Android preset")
	android, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if android != baseline {
		t.Fatal("Android preset changed Linux runtime pack digest")
	}

	write("game.go", "package spx\n\nconst changed = true\n")
	commit("change active source")
	active, err := RuntimePackSourceSHA256(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if active == baseline {
		t.Fatal("active Linux source did not change runtime pack digest")
	}
}
