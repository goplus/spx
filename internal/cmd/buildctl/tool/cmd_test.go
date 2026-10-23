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

package tool

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

type toolRecordedCall struct {
	path string
	args []string
}

type toolRecordingRunner struct {
	calls []toolRecordedCall
	err   error
}

func (r *toolRecordingRunner) RunScript(path string, args ...string) error {
	r.calls = append(r.calls, toolRecordedCall{path: path, args: append([]string(nil), args...)})
	return r.err
}

func (r *toolRecordingRunner) RunCommand(string, string, ...string) error { return nil }

func (r *toolRecordingRunner) RepoRootDir() string { return "." }

func TestToolDispatchAndInstallArguments(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		contains string
	}{
		{name: "missing command", wantErr: errUsage},
		{name: "help", args: []string{"help"}},
		{name: "clean help", args: []string{"clean-assets", "--help"}},
		{name: "install help", args: []string{"install", "--help"}},
		{name: "unknown", args: []string{"unknown"}, contains: `unknown tool command "unknown"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Run(test.args)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Run(%q) error = %v, want %v", test.args, err, test.wantErr)
				}
				return
			}
			if test.contains == "" {
				if err != nil {
					t.Fatalf("Run(%q) returned error: %v", test.args, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Run(%q) error = %v, want substring %q", test.args, err, test.contains)
			}
		})
	}

	for _, test := range []struct {
		name string
		cfg  InstallConfig
		args []string
	}{
		{name: "default", args: nil},
		{name: "web without embed", cfg: InstallConfig{Web: true, NoEmbedRuntime: true}, args: []string{"--web", "--no-embed-runtime"}},
	} {
		t.Run("install/"+test.name, func(t *testing.T) {
			runner := &toolRecordingRunner{}
			if err := InstallTools(test.cfg, runner); err != nil {
				t.Fatalf("InstallTools returned error: %v", err)
			}
			want := []toolRecordedCall{{path: "cmd/spx/install.sh", args: test.args}}
			if !reflect.DeepEqual(runner.calls, want) {
				t.Fatalf("recorded calls = %#v, want %#v", runner.calls, want)
			}
		})
	}

	sentinel := errors.New("runner failed")
	runner := &toolRecordingRunner{err: sentinel}
	if err := InstallTools(InstallConfig{}, runner); !errors.Is(err, sentinel) {
		t.Fatalf("InstallTools error = %v, want runner error", err)
	}
}

func TestParseToolInstallArgsDefault(t *testing.T) {
	cfg, err := parseToolInstallArgs(nil)
	if err != nil {
		t.Fatalf("parseToolInstallArgs returned error: %v", err)
	}
	if cfg.Web || cfg.NoEmbedRuntime {
		t.Fatalf("unexpected tool install flags: %#v", cfg)
	}
}

func TestParseToolInstallArgsNoEmbedRuntime(t *testing.T) {
	cfg, err := parseToolInstallArgs([]string{"--no-embed-runtime"})
	if err != nil {
		t.Fatalf("parseToolInstallArgs returned error: %v", err)
	}
	if !cfg.NoEmbedRuntime || cfg.Web {
		t.Fatalf("unexpected tool install flags: %#v", cfg)
	}
}

func TestToolCommandArgumentValidation(t *testing.T) {
	for _, command := range []string{"clean-assets", "install", "setup-emsdk", "setup-jdk", "setup-ndk", "setup-scons"} {
		t.Run(command, func(t *testing.T) {
			if err := Run([]string{command, "--help"}); err != nil {
				t.Fatalf("help: %v", err)
			}
			if err := Run([]string{command, "unexpected"}); !errors.Is(err, errUsage) {
				t.Fatalf("positional argument error = %v, want usage error", err)
			}
			if err := Run([]string{command, "--unknown-flag"}); err == nil {
				t.Fatal("unknown flag was accepted")
			}
		})
	}

}

func writeToolFixture(t *testing.T, path, content string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestToolInstallFindsRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installation script uses bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("installation fixture requires bash")
	}
	root := t.TempDir()
	// Keep Darwin environment normalization independent of installed SDKs.
	compiler := filepath.Join(root, "compiler")
	writeToolFixture(t, compiler, "#!/bin/sh\nexit 0\n", 0o755)
	t.Setenv("SDKROOT", root)
	t.Setenv("CC", compiler)
	t.Setenv("CXX", compiler)
	for _, key := range []string{"CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS"} {
		t.Setenv(key, "")
	}
	t.Chdir(root)
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))
	if err := Run([]string{"install"}); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("missing repository error = %v", err)
	}
	writeToolFixture(t, filepath.Join(root, "go.mod"), "module fixture\n", 0o644)
	writeToolFixture(t, filepath.Join(root, "internal", "tools", "fixture"), "", 0o644)
	log := filepath.Join(root, "install-args")
	t.Setenv("TOOL_LOG", log)
	writeToolFixture(t, filepath.Join(root, "cmd", "spx", "install.sh"), "printf '%s\\n' \"$@\" > \"$TOOL_LOG\"\n", 0o644)
	if err := Run([]string{"install", "--web", "--no-embed-runtime"}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(log); err != nil || string(data) != "--web\n--no-embed-runtime\n" {
		t.Fatalf("install arguments = %q, %v", data, err)
	}
}

func TestCleanInstalledAssetsRemovesKnownArtifactsOnly(t *testing.T) {
	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)
	version := release.DefaultRuntimeLock().RuntimeVersion

	binDir := filepath.Join(gopath, "bin")
	toCreate := []string{
		"spx",
		"ispx",
		"ispx.wasm",
		"runtime.gdextension",
		"gdspx" + version,
		"gdspx" + version + "_webpack.zip",
		"gdspxrt" + version,
		"gdspxrt" + version + ".pck",
		"gdspxrt" + version + "_webnormal/engine.js",
	}
	for _, rel := range toCreate {
		writeToolFixture(t, filepath.Join(binDir, rel), "data", 0o644)
	}

	keep := filepath.Join(binDir, "xgotest")
	writeToolFixture(t, keep, "keep", 0o644)

	if err := cleanInstalledAssets(); err != nil {
		t.Fatalf("cleanInstalledAssets returned error: %v", err)
	}

	for _, rel := range []string{
		"spx",
		"ispx",
		"ispx.wasm",
		"runtime.gdextension",
		"gdspx" + version,
		"gdspx" + version + "_webpack.zip",
		"gdspxrt" + version,
		"gdspxrt" + version + ".pck",
		"gdspxrt" + version + "_webnormal",
	} {
		if shared.FileExists(filepath.Join(binDir, rel)) {
			t.Fatalf("expected %s to be removed", rel)
		}
	}

	if !shared.FileExists(keep) {
		t.Fatalf("expected unrelated file to remain: %s", keep)
	}
}

func TestCleanInstalledAssetsWithNothingInstalled(t *testing.T) {
	for _, binExists := range []bool{false, true} {
		t.Run(fmt.Sprint(binExists), func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("GOPATH", root)
			if binExists {
				if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := Run([]string{"clean-assets"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCleanInstalledAssetsRejectsLookupAndGlobFailures(t *testing.T) {
	t.Run("GOPATH unavailable", func(t *testing.T) {
		t.Setenv("GOPATH", "")
		t.Setenv("PATH", t.TempDir())
		if err := cleanInstalledAssets(); err == nil {
			t.Fatal("missing Go environment ignored")
		}
	})
	t.Run("malformed glob", func(t *testing.T) {
		gopath := filepath.Join(t.TempDir(), "[")
		if err := os.MkdirAll(filepath.Join(gopath, "bin"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOPATH", gopath)
		if err := cleanInstalledAssets(); err != filepath.ErrBadPattern {
			t.Fatalf("malformed asset glob = %v", err)
		}
	})
}
