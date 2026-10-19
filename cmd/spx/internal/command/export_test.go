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

package command

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestExportPackUsesPackOnlyExport(t *testing.T) {
	for _, tt := range []struct{ goos, platform string }{{goosDarwin, "Mac"}, {goosLinux, "Linux"}, {goosWindows, "Win"}} {
		t.Run(tt.goos, func(t *testing.T) {
			projectDir := t.TempDir()
			cmd := &CmdTool{ProjectDir: projectDir, CmdPath: "godot"}
			calls := 0
			targetPath := filepath.Join(projectDir, ".builds", "pc", "gdexport.pck")
			err := cmd.exportPack(tt.goos, func(dir, name string, args ...string) error {
				calls++
				want := []string{"--headless", "--quit", "--path", projectDir, "--export-pack", tt.platform, targetPath}
				if dir != projectDir || name != "godot" || !reflect.DeepEqual(args, want) {
					t.Fatalf("export command = (%q, %q, %v), want (%q, godot, %v)", dir, name, args, projectDir, want)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("exportPack = %v, calls = %d; want nil, 1", err, calls)
			}
			if info, err := os.Stat(filepath.Dir(targetPath)); err != nil || !info.IsDir() {
				t.Fatalf("pack output directory was not created: %v", err)
			}
		})
	}
}

func TestExportPackRejectsUnsupportedPlatformBeforeRunning(t *testing.T) {
	called := false
	cmd := &CmdTool{ProjectDir: t.TempDir(), CmdPath: "godot"}
	err := cmd.exportPack("plan9", func(string, string, ...string) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("unsupported export = %v, command called = %v", err, called)
	}
}

func TestExportPackPreservesFailures(t *testing.T) {
	for _, name := range []string{"runner error", "blocked directory"} {
		blocked := name == "blocked directory"
		t.Run(name, func(t *testing.T) {
			cmd := &CmdTool{ProjectDir: t.TempDir()}
			if blocked {
				writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, ".builds"), "occupied")
			}
			failure := errors.New("export failed")
			calls := 0
			err := cmd.exportPack(goosDarwin, func(string, string, ...string) error { calls++; return failure })
			if blocked {
				if err == nil || !strings.Contains(err.Error(), "failed to create export directory") || calls != 0 {
					t.Fatalf("blocked output error = %v, calls = %d", err, calls)
				}
			} else if !errors.Is(err, failure) || calls != 1 {
				t.Fatalf("runner error = %v, calls = %d", err, calls)
			}
		})
	}
}

func TestPrepareExportStagesAssetsFromLogicalProject(t *testing.T) {
	source := t.TempDir()
	cmd := &CmdTool{TargetAbsDir: source, ProjectDir: filepath.Join(source, "project")}
	for _, name := range []string{"index.json", "images/sprite.svg"} {
		writeCommandTestFile(t, filepath.Join(source, "assets", name), "asset "+name)
	}
	if err := cmd.prepareExport(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.json", "images/sprite.svg"} {
		if got := readTestFile(t, filepath.Join(cmd.ProjectDir, "assets", name)); got != "asset "+name {
			t.Fatalf("staged %s = %q", name, got)
		}
	}
	// A second staging pass updates existing resources.
	writeCommandTestFile(t, filepath.Join(source, "assets", "index.json"), "updated")
	if err := cmd.prepareExport(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(cmd.ProjectDir, "assets", "index.json")); got != "updated" {
		t.Fatalf("updated index = %q", got)
	}
}

func TestPrepareExportRejectsInvalidAssets(t *testing.T) {
	for _, name := range []string{"missing project", "missing assets", "assets file", "asset symlink", "source symlink", "destination symlink", "same directory", "nested destination", "blocked destination"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			cmd := &CmdTool{TargetAbsDir: filepath.Join(root, "game"), ProjectDir: filepath.Join(root, "generated")}
			source := filepath.Join(cmd.TargetAbsDir, "assets")
			writeCommandTestFile(t, filepath.Join(source, "index.json"), "source")
			switch name {
			case "missing project":
				cmd.TargetAbsDir = ""
			case "missing assets":
				cmd.TargetAbsDir = filepath.Join(root, "missing")
			case "assets file":
				cmd.TargetAbsDir = filepath.Join(root, "not-a-directory")
				writeCommandTestFile(t, filepath.Join(cmd.TargetAbsDir, "assets"), "occupied")
			case "asset symlink", "source symlink", "destination symlink":
				link := filepath.Join(source, "linked")
				if name == "source symlink" {
					cmd.TargetAbsDir = filepath.Join(root, "linked-game")
					link = filepath.Join(cmd.TargetAbsDir, "assets")
				} else if name == "destination symlink" {
					link = filepath.Join(cmd.ProjectDir, "assets")
				}
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				target := source
				if name == "asset symlink" {
					target = filepath.Join(root, "outside.txt")
					writeCommandTestFile(t, target, "outside")
				}
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			case "same directory":
				cmd.ProjectDir = cmd.TargetAbsDir
			case "nested destination":
				cmd.ProjectDir = filepath.Join(source, "generated")
			case "blocked destination":
				writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, "assets"), "occupied")
			}
			if err := cmd.prepareExport(); err == nil {
				t.Fatal("invalid export staging succeeded")
			}
			if readTestFile(t, filepath.Join(source, "index.json")) != "source" {
				t.Fatal("failed staging changed the source asset")
			}
		})
	}
}

func TestDesktopExportPaths(t *testing.T) {
	for _, tt := range []struct{ goos, executable, library, target, platform string }{
		{goosWindows, ".exe", ".dll", ".exe", "Win"},
		{goosDarwin, "", ".dylib", ".app", "Mac"},
		{goosLinux, "", ".so", "", "Linux"},
	} {
		t.Run(tt.goos, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "game")
			target, platform, err := resolveDesktopExportTarget(tt.goos, base)
			if err != nil || target != base+tt.target || platform != tt.platform {
				t.Fatalf("desktop target = %q, %q, %v", target, platform, err)
			}
			if got := executableSuffix(tt.goos); got != tt.executable {
				t.Fatalf("executable suffix = %q, want %q", got, tt.executable)
			}
			if got, want := libraryFileName("gdspx", tt.goos, "arm64"), "gdspx-"+tt.goos+"-arm64"+tt.library; got != want {
				t.Fatalf("library filename = %q, want %q", got, want)
			}
		})
	}
	if target, platform, err := resolveDesktopExportTarget("plan9", "game"); err == nil || target != "" || platform != "" {
		t.Fatalf("unsupported desktop target = %q, %q, %v", target, platform, err)
	}
}

func TestAndroidBuildConfiguration(t *testing.T) {
	want := []androidBuildConfig{
		{"arm64-v8a", "arm64", "libgdspx-android-arm64.so", "aarch64-linux-android"},
		{"armeabi-v7a", "arm", "libgdspx-android-arm32.so", "armv7a-linux-androideabi"},
	}
	if got := androidBuildConfigs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Android builds = %+v, want %+v", got, want)
	}
	host := map[string]string{"darwin": "darwin-x86_64", "windows": "windows-x86_64"}[runtime.GOOS]
	if runtime.GOOS == "linux" {
		host = map[string]string{"amd64": "linux-x86_64", "arm64": "linux-aarch64"}[runtime.GOARCH]
	}
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	ndk := filepath.Join(t.TempDir(), "ndk")
	got, err := cmd.resolveAndroidBuildContext(ndk)
	if host == "" {
		if err == nil {
			t.Fatal("unsupported NDK host was accepted")
		}
		return
	}
	wantPaths := androidBuildContext{filepath.Join(cmd.ProjectDir, "lib"), filepath.Join(cmd.ProjectDir, "go"), filepath.Join(ndk, "toolchains", "llvm", "prebuilt", host, "bin"), "21"}
	if err != nil || got != wantPaths {
		t.Fatalf("Android build context = %+v, %v; want %+v", got, err, wantPaths)
	}
	for range 2 {
		if err := prepareAndroidLibraryDir(got.libDir); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(got.libDir); err != nil || !info.IsDir() {
		t.Fatalf("Android library directory missing: %v", err)
	}
}

func TestAndroidBuildRejectsMissingNDKAndBlockedLibraryDirectory(t *testing.T) {
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	t.Setenv("ANDROID_NDK_ROOT", "")
	if err := cmd.buildAndroidLibraries(); err == nil || !strings.Contains(err.Error(), "missing ANDROID_NDK_ROOT") {
		t.Fatalf("missing NDK error = %v", err)
	}
	t.Setenv("ANDROID_NDK_ROOT", t.TempDir())
	if _, err := resolveAndroidHostTag(); err != nil {
		t.Skipf("NDK host is unsupported: %v", err)
	}
	lib := filepath.Join(cmd.ProjectDir, "lib")
	writeCommandTestFile(t, lib, "occupied")
	if err := cmd.buildAndroidLibraries(); err == nil || !strings.Contains(err.Error(), "failed to create lib directory") {
		t.Fatalf("blocked library directory error = %v", err)
	}
	if readTestFile(t, lib) != "occupied" {
		t.Fatal("failed build changed the blocking file")
	}
}
