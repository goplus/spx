//go:build !js || !wasm

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
	"context"
	"errors"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/interpruntime"
)

func TestValidateAssetIndex(t *testing.T) {
	for _, test := range []struct {
		name      string
		files     map[string]string
		directory string
	}{
		{name: "regular index", files: map[string]string{"index.json": "{}\n"}},
		{name: "packed only", files: map[string]string{"index_pack.json": `{"zorder":[]}`}},
		{name: "both regular", files: map[string]string{"index.json": "{}", "index_pack.json": "{}"}},
		{name: "missing"},
		{name: "invalid packed index", files: map[string]string{"index.json": "{}"}, directory: "index_pack.json"},
		{name: "invalid unpacked index", files: map[string]string{"index_pack.json": "{}"}, directory: "index.json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assetDir := t.TempDir()
			for name, content := range test.files {
				writeNativeTestFile(t, filepath.Join(assetDir, name), content)
			}
			if test.directory != "" {
				if err := os.Mkdir(filepath.Join(assetDir, test.directory), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			err := validateAssetIndex(assetDir)
			switch {
			case test.directory != "":
				if err == nil || !strings.Contains(err.Error(), strconv.Quote(filepath.Join(assetDir, test.directory))) {
					t.Fatalf("invalid %s error = %v; valid companion must not bypass validation", test.directory, err)
				}
			case len(test.files) == 0:
				if err == nil || !strings.Contains(err.Error(), "neither") {
					t.Fatalf("missing indexes error = %v", err)
				}
			default:
				if err != nil {
					t.Fatalf("valid indexes rejected: %v", err)
				}
			}
		})
	}
}

func TestValidateAssetIndexRejectsSymlink(t *testing.T) {
	assetDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "index.json")
	writeNativeTestFile(t, target, "{}\n")
	if err := os.Symlink(target, filepath.Join(assetDir, "index.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := validateAssetIndex(assetDir); err == nil {
		t.Fatal("validateAssetIndex accepted a symlink index")
	}
}

func TestPinnedProjectRootRejectsEscapingSymlink(t *testing.T) {
	projectDir := t.TempDir()
	externalDir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(externalDir, "index.json"), `{}`)
	if err := os.Mkdir(filepath.Join(projectDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDir, filepath.Join(projectDir, "assets", "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	root, err := openPinnedProjectRoot(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := fs.ReadFile(root.FS(), "assets/linked/index.json"); err == nil {
		t.Fatal("project filesystem followed a symlink outside ProjectDir")
	}
}

func TestBuildPinnedProjectKeepsFilesystemAliveForDeferredResourceLoads(t *testing.T) {
	t.Cleanup(releaseProjectRoot)
	projectDir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n")

	var runtimeFS fs.FS
	if err := buildPinnedProject(projectDir, nil, func(fsys fs.FS) error {
		runtimeFS = fsys
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	if _, err := fs.ReadFile(runtimeFS, "main.spx"); err != nil {
		t.Fatalf("deferred project read failed after build returned: %v", err)
	}
}

func TestBuildPinnedProjectClosesFilesystemOnBuildFailure(t *testing.T) {
	t.Cleanup(releaseProjectRoot)
	projectDir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}\n")

	wantErr := errors.New("build failed")
	var failedFS fs.FS
	err := buildPinnedProject(projectDir, nil, func(fsys fs.FS) error {
		failedFS = fsys
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildPinnedProject() error = %v, want %v", err, wantErr)
	}
	if _, err := fs.ReadFile(failedFS, "main.spx"); err == nil {
		t.Fatal("failed build left the project root open")
	}
}

func TestValidateStableRegularFile(t *testing.T) {
	dir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(dir, "empty"), "")
	writeNativeTestFile(t, filepath.Join(dir, "content"), strings.Repeat("asset data\n", 8192))
	for _, test := range []struct {
		name   string
		path   string
		exists bool
		err    bool
	}{
		{name: "empty regular file", path: filepath.Join(dir, "empty"), exists: true},
		{name: "nonempty regular file", path: filepath.Join(dir, "content"), exists: true},
		{name: "missing", path: filepath.Join(dir, "missing")},
		{name: "directory", path: dir, err: true},
		{name: "invalid path", path: filepath.Join(dir, "invalid\x00name"), err: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			exists, err := validateStableRegularFile(test.path)
			if exists != test.exists || (err != nil) != test.err {
				t.Fatalf("validateStableRegularFile() = (%v, %v), want (%v, error=%v)", exists, err, test.exists, test.err)
			}
		})
	}
	t.Run("dangling symlink", func(t *testing.T) {
		name := filepath.Join(dir, "dangling")
		if err := os.Symlink("missing", name); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if exists, err := validateStableRegularFile(name); exists || err == nil {
			t.Fatalf("dangling symlink treated as missing or valid: (%v, %v)", exists, err)
		}
	})
}

func TestValidateStableRegularFileRejectsUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not control file readability on Windows")
	}
	dir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(dir, "index.json"), "{}")
	name := filepath.Join(dir, "index.json")
	if err := os.Chmod(name, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(name, 0o600) })
	if file, err := os.Open(name); err == nil {
		file.Close()
		t.Skip("current user can read files without read permission")
	} else if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("probe unreadable file: %v", err)
	}
	if exists, err := validateStableRegularFile(name); exists || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("unreadable file validation = (%v, %v), want permission error", exists, err)
	}
	if err := validateAssetIndex(dir); !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), strconv.Quote(name)) {
		t.Fatalf("asset validation lost path or permission error: %v", err)
	}
}

func TestOpenPinnedProjectRootRejectsInvalidRoots(t *testing.T) {
	dir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(dir, "file"), "not a directory")
	for _, test := range []struct {
		name    string
		path    string
		message string
	}{
		{name: "missing", path: filepath.Join(dir, "missing"), message: "inspect project root"},
		{name: "regular file", path: filepath.Join(dir, "file"), message: "must be a real directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, err := openPinnedProjectRoot(test.path)
			if root != nil {
				root.Close()
				t.Fatal("invalid project root was opened")
			}
			if err == nil || !strings.Contains(err.Error(), test.message) || !strings.Contains(err.Error(), strconv.Quote(test.path)) {
				t.Fatalf("openPinnedProjectRoot() error = %v", err)
			}
			if test.name == "missing" && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("missing project root lost underlying error: %v", err)
			}
		})
	}
	t.Run("symlink to directory", func(t *testing.T) {
		name := filepath.Join(t.TempDir(), "linked")
		if err := os.Symlink(dir, name); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		root, err := openPinnedProjectRoot(name)
		if root != nil {
			root.Close()
			t.Fatal("symlink project root was opened")
		}
		if err == nil || !strings.Contains(err.Error(), "must be a real directory") {
			t.Fatalf("symlink project root error = %v", err)
		}
	})
}

func TestOpenPinnedProjectRootRejectsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not control directory readability on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if root, err := os.OpenRoot(dir); err == nil {
		root.Close()
		t.Skip("current platform can open a root without directory read permission")
	} else if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("probe unreadable directory: %v", err)
	}
	root, err := openPinnedProjectRoot(dir)
	if root != nil {
		root.Close()
		t.Fatal("unreadable project root was opened")
	}
	if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), "open project root") {
		t.Fatalf("openPinnedProjectRoot() error = %v, want wrapped permission error", err)
	}
}

func TestBuildPinnedProjectReplacesAndReleasesRoots(t *testing.T) {
	releaseProjectRoot()
	t.Cleanup(releaseProjectRoot)
	var previous fs.FS
	for _, content := range []string{"first project", "second project"} {
		dir := t.TempDir()
		writeNativeTestFile(t, filepath.Join(dir, "main.spx"), content)
		var current fs.FS
		if err := buildPinnedProject(dir, nil, func(fsys fs.FS) error {
			current = fsys
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if previous != nil {
			if _, err := fs.ReadFile(previous, "main.spx"); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("replaced project filesystem read error = %v, want closed", err)
			}
		}
		if got, err := fs.ReadFile(current, "main.spx"); err != nil || string(got) != content {
			t.Fatalf("retained project read = (%q, %v), want %q", got, err, content)
		}
		previous = current
	}
	releaseProjectRoot()
	releaseProjectRoot()
	if _, err := fs.ReadFile(previous, "main.spx"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("released project filesystem read error = %v, want closed", err)
	}
}

func TestBuildPinnedProjectFailurePreservesPreviousRoot(t *testing.T) {
	releaseProjectRoot()
	t.Cleanup(releaseProjectRoot)
	dir := t.TempDir()
	writeNativeTestFile(t, filepath.Join(dir, "main.spx"), "previous project")
	var previous fs.FS
	if err := buildPinnedProject(dir, nil, func(fsys fs.FS) error {
		previous = fsys
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Run("open failure", func(t *testing.T) {
		called := false
		err := buildPinnedProject(filepath.Join(dir, "missing"), nil, func(fs.FS) error {
			called = true
			return nil
		})
		if !errors.Is(err, fs.ErrNotExist) || called {
			t.Fatalf("missing root build error = %v, callback called = %v", err, called)
		}
	})
	t.Run("build failure", func(t *testing.T) {
		failure := errors.New("compiler rejected replacement")
		var rejected fs.FS
		err := buildPinnedProject(dir, nil, func(fsys fs.FS) error {
			rejected = fsys
			return failure
		})
		if !errors.Is(err, failure) {
			t.Fatalf("build error = %v, want %v", err, failure)
		}
		if _, err := fs.ReadFile(rejected, "main.spx"); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("rejected filesystem read error = %v, want closed", err)
		}
	})
	if got, err := fs.ReadFile(previous, "main.spx"); err != nil || string(got) != "previous project" {
		t.Fatalf("failed replacement invalidated retained project: (%q, %v)", got, err)
	}
}

func TestBuildPinnedProjectUsesConfigOverlay(t *testing.T) {
	for _, test := range []struct {
		name    string
		overlay *portableConfigOverlay
		want    string
		absent  bool
	}{
		{name: "legacy", want: "project config"},
		{name: "snapshot", overlay: &portableConfigOverlay{present: true, data: []byte("captured config")}, want: "captured config"},
		{name: "absent snapshot", overlay: &portableConfigOverlay{}, absent: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			releaseProjectRoot()
			t.Cleanup(releaseProjectRoot)
			dir := t.TempDir()
			writeNativeTestFile(t, filepath.Join(dir, ".config"), "project config")
			writeNativeTestFile(t, filepath.Join(dir, "main.spx"), "onStart => {}")
			checkConfig := func(fsys fs.FS) {
				t.Helper()
				got, err := fs.ReadFile(fsys, ".config")
				if test.absent {
					if !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("absent snapshot exposed project config: (%q, %v)", got, err)
					}
				} else if err != nil || string(got) != test.want {
					t.Fatalf("build config = (%q, %v), want %q", got, err, test.want)
				}
				if got, err := fs.ReadFile(fsys, "main.spx"); err != nil || string(got) != "onStart => {}" {
					t.Fatalf("project source read = (%q, %v)", got, err)
				}
			}
			var deferred fs.FS
			if err := buildPinnedProject(dir, test.overlay, func(fsys fs.FS) error {
				checkConfig(fsys)
				deferred = fsys
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			checkConfig(deferred)
			if got, err := os.ReadFile(filepath.Join(dir, ".config")); err != nil || string(got) != "project config" {
				t.Fatalf("building altered project config on disk: (%q, %v)", got, err)
			}
		})
	}
}

func TestRunRejectsInvalidEnvironment(t *testing.T) {
	for _, tt := range []struct {
		name    string
		prepare func(*testing.T, interpruntime.Roots) []string
		want    string
	}{
		{
			name: "missing roots",
			prepare: func(t *testing.T, roots interpruntime.Roots) []string {
				return nil
			},
			want: "required environment variable SPX_PROJECT_DIR is not set",
		},
		{
			name: "duplicate roots",
			prepare: func(t *testing.T, roots interpruntime.Roots) []string {
				return append(nativeRunTestEnv(t, roots), interpruntime.ProjectDirEnv+"="+roots.ProjectDir)
			},
			want: "duplicate environment variable SPX_PROJECT_DIR",
		},
		{
			name: "missing asset index",
			prepare: func(t *testing.T, roots interpruntime.Roots) []string {
				return nativeRunTestEnv(t, roots)
			},
			want: "validate asset index: neither",
		},
		{
			name: "incomplete portable config",
			prepare: func(t *testing.T, roots interpruntime.Roots) []string {
				writeNativeTestFile(t, filepath.Join(roots.AssetDir, "index.json"), "{}")
				return append(nativeRunTestEnv(t, roots), interpruntime.PortableConfigDirEnv+"="+roots.SessionDir)
			},
			want: "portable config environment is incomplete",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			roots := nativeRunTestRoots(t)
			code, err := run(tt.prepare(t, roots))
			if code != 1 || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run() = (%d, %v), want (1, error containing %q)", code, err, tt.want)
			}
		})
	}
}

func TestNativeMainCommand(t *testing.T) {
	if mode := os.Getenv("SPX_NATIVE_TEST_MODE"); mode != "" {
		t.Cleanup(releaseProjectRoot)
		if mode == "run twice" {
			if code, err := run(os.Environ()); code != 0 || err != nil {
				t.Fatalf("first run() = (%d, %v), want success", code, err)
			}
			code, err := run(os.Environ())
			if code != 1 || err == nil || !strings.Contains(err.Error(), "configure filesystem roots: ispx: filesystem roots must be configured before Init") {
				t.Fatalf("second run() = (%d, %v), want filesystem configuration failure", code, err)
			}
			return
		}
		main()
		return
	}

	for _, tt := range []struct {
		name          string
		source        string
		mode          string
		portable      bool
		missingRoots  bool
		wantOutput    string
		wantExit      int
		wantNoRuntime bool
	}{
		{
			name: "legacy project succeeds",
			source: `package main
import "fmt"
func main() { fmt.Println("native project executed") }
`,
			wantOutput: "native project executed",
		},
		{
			name: "portable project succeeds",
			source: `package main
import "fmt"
func main() { fmt.Println("portable project executed") }
`,
			portable:   true,
			wantOutput: "portable project executed",
		},
		{
			name:         "missing roots",
			missingRoots: true,
			wantOutput:   "ispxnative: interpruntime: required environment variable SPX_PROJECT_DIR is not set",
			wantExit:     1,
		},
		{
			name:       "build failure",
			source:     "package main\nfunc main() { missingFunction() }\n",
			wantOutput: "ispxnative: build project:",
			wantExit:   1,
		},
		{
			name:       "interpreter panic",
			source:     "package main\nfunc main() { panic(\"native test panic\") }\n",
			wantOutput: "ispxnative: interpreter exited with code 2: native test panic",
			wantExit:   2,
		},
		{
			name: "project exit status",
			source: `package main
import "os"
func main() { os.Exit(7) }
`,
			wantExit:      7,
			wantNoRuntime: true,
		},
		{
			name:   "interpreter cannot be initialized twice",
			source: "package main\nfunc main() {}\n",
			mode:   "run twice",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Init is process-wide. A fresh process also lets us verify main's
			// real os.Exit behavior without resetting interpreter internals.
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeMainCommand$")
			cmd.Dir = t.TempDir()
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(strings.ToUpper(entry), "SPX_") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			mode := tt.mode
			if mode == "" {
				mode = "main"
			}
			cmd.Env = append(cmd.Env, "SPX_NATIVE_TEST_MODE="+mode)
			if !tt.missingRoots {
				roots := nativeRunTestRoots(t)
				writeNativeTestFile(t, filepath.Join(roots.AssetDir, "index.json"), "{}")
				writeNativeTestFile(t, filepath.Join(roots.ProjectDir, "main.xgo"), tt.source)
				cmd.Env = append(cmd.Env, nativeRunTestEnv(t, roots)...)
				if tt.portable {
					configDir := makePortableConfigTestDir(t, roots.SessionDir)
					writeNativeTestFile(t, filepath.Join(configDir, ".config"), `{"name":"portable test"}`)
					cmd.Env = append(cmd.Env, portableConfigTestEnv(t, configDir)...)
				}
			}
			// Include subprocess execution in the normal go test coverage report.
			if f := flag.Lookup("test.gocoverdir"); f != nil && f.Value.String() != "" {
				dir := f.Value.String()
				cmd.Args = append(cmd.Args, "-test.gocoverdir="+dir)
				cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
			}
			output, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tt.wantExit {
				t.Fatalf("main exit = %v, error %v; want %d\n%s", cmd.ProcessState, err, tt.wantExit, output)
			}
			if !strings.Contains(string(output), tt.wantOutput) {
				t.Errorf("main output = %q, want %q", output, tt.wantOutput)
			}
			if tt.wantNoRuntime && strings.Contains(string(output), "ispxnative:") {
				t.Errorf("clean project exit reported an interpreter error: %s", output)
			}
		})
	}
}

func nativeRunTestRoots(t *testing.T) interpruntime.Roots {
	t.Helper()
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := os.Mkdir(assetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return interpruntime.Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: t.TempDir()}
}

func nativeRunTestEnv(t *testing.T, roots interpruntime.Roots) []string {
	t.Helper()
	env, err := roots.Environment(nil)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func writeNativeTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
