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
	"fmt"
	"net"
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
	cmd := &CmdTool{TargetDir: source, ProjectDir: filepath.Join(source, "project")}
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
	for _, name := range []string{"missing project", "missing assets", "assets file", "asset symlink", "source symlink", "destination symlink", "same directory", "nested destination", "blocked destination", "destination parent loop"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			cmd := &CmdTool{TargetDir: filepath.Join(root, "game"), ProjectDir: filepath.Join(root, "generated")}
			source := filepath.Join(cmd.TargetDir, "assets")
			writeCommandTestFile(t, filepath.Join(source, "index.json"), "source")
			switch name {
			case "missing project":
				cmd.TargetDir = ""
			case "missing assets":
				cmd.TargetDir = filepath.Join(root, "missing")
			case "assets file":
				cmd.TargetDir = filepath.Join(root, "not-a-directory")
				writeCommandTestFile(t, filepath.Join(cmd.TargetDir, "assets"), "occupied")
			case "asset symlink", "source symlink", "destination symlink":
				link := filepath.Join(source, "linked")
				if name == "source symlink" {
					cmd.TargetDir = filepath.Join(root, "linked-game")
					link = filepath.Join(cmd.TargetDir, "assets")
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
				cmd.ProjectDir = cmd.TargetDir
			case "nested destination":
				cmd.ProjectDir = filepath.Join(source, "generated")
			case "destination parent loop":
				loop := filepath.Join(root, "loop")
				if err := os.Symlink("loop", loop); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				cmd.ProjectDir = filepath.Join(loop, "generated")
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

func TestMobileExportToolWorkflows(t *testing.T) {
	for _, platform := range []struct {
		name, installStep string
		export            func(*CmdTool) error
		outputs, steps    []string
	}{
		{"ios", "ios-deploy", (*CmdTool).ExportIos,
			[]string{"libgdspx-x86_64.a", "libgdspx-arm64-sim.a", "libgdspx-arm64.a"},
			[]string{"sdk:iphonesimulator", "sdk:iphoneos", "go:darwin:amd64", "go:darwin:arm64", "lipo", "xcrun"}},
		{"android", "adb:install", (*CmdTool).ExportApk,
			[]string{"libgdspx-android-arm64.so", "libgdspx-android-arm32.so"},
			[]string{"go:android:arm64", "go:android:arm"}},
	} {
		for _, scenario := range []struct {
			name, failure, output, wantError string
			install                          bool
		}{
			{name: "export"},
			{name: "install", install: true},
			{name: "import warning", failure: "import"},
			{name: "export failure", failure: "export", wantError: "failed to export"},
			{name: "missing output", output: "missing", wantError: "file not created"},
			{name: "install failure", failure: platform.installStep, install: true, wantError: "failed to install"},
		} {
			t.Run(platform.name+"/"+scenario.name, func(t *testing.T) {
				cmd, logPath := newMobileExportFixture(t)
				*cmd.Args.Install = scenario.install
				t.Setenv("SPX_MOBILE_FAIL", scenario.failure)
				if scenario.output != "" {
					t.Setenv("SPX_MOBILE_OUTPUT", scenario.output)
				}
				err := platform.export(cmd)
				wantError := scenario.wantError
				if wantError == "" && err != nil || wantError != "" && (err == nil || !strings.Contains(err.Error(), wantError)) {
					t.Fatalf("export error = %v, want context %q", err, wantError)
				}
				calls := readTestFile(t, logPath)
				if !strings.Contains(calls, "import|") || !strings.Contains(calls, "export|") {
					t.Fatalf("missing import/export invocation: %s", calls)
				}
				if strings.Index(calls, "import|") > strings.Index(calls, "export|") {
					t.Fatalf("export preceded import: %s", calls)
				}
				if got := readTestFile(t, filepath.Join(cmd.ProjectDir, "assets", "index.json")); got != "{}" {
					t.Fatalf("staged asset = %q", got)
				}
				var mobileBuilds []string
				for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
					fields := strings.Split(line, "|")
					if strings.HasPrefix(fields[0], "go:android:") || strings.HasPrefix(fields[0], "go:darwin:") {
						mobileBuilds = append(mobileBuilds, fields[2])
						if fields[1] != cmd.GoDir || fields[3] != "1" || !strings.Contains(fields[2], "-tags="+platform.name+",packmode") {
							t.Fatalf("mobile build invocation = %q", line)
						}
						if platform.name == "android" && !strings.Contains(fields[4], "21-clang") {
							t.Fatalf("missing NDK compiler: %q", line)
						}
						if platform.name == "ios" && (!strings.Contains(fields[5], "-isysroot /test-sdk/") || !strings.Contains(fields[6], "version-min=12.0")) {
							t.Fatalf("missing iOS SDK flags: %q", line)
						}
					}
				}
				wantOutputs := platform.outputs
				if len(mobileBuilds) != len(wantOutputs) {
					t.Fatalf("mobile build count = %d, want %d: %v", len(mobileBuilds), len(wantOutputs), mobileBuilds)
				}
				for i, output := range wantOutputs {
					if !strings.Contains(mobileBuilds[i], string(filepath.Separator)+output+" .") {
						t.Errorf("mobile build %d = %q, want output %s", i, mobileBuilds[i], output)
					}
				}
				for _, step := range []string{"ios-deploy", "adb:install"} {
					installed := strings.Contains(calls, step+"|")
					wantInstalled := scenario.install && step == platform.installStep
					if installed != wantInstalled {
						t.Fatalf("%s invoked = %v, want %v: %s", step, installed, wantInstalled, calls)
					}
				}
				for _, step := range platform.steps {
					if !strings.Contains(calls, step+"|") {
						t.Errorf("missing %s stage %q: %s", platform.name, step, calls)
					}
				}
				if platform.name == "ios" {
					if _, err := os.Stat(cmd.iosLibraryPaths().buildDir); !os.IsNotExist(err) {
						t.Fatalf("temporary iOS build directory retained: %v", err)
					}
				}
			})
		}
	}
}

func TestMobileLibraryBuildStopsAtFailedTool(t *testing.T) {
	for _, tt := range []struct{ step, context string }{
		{"sdk:iphonesimulator", "failed to get simulator SDK path"},
		{"sdk:iphoneos", "failed to get device SDK path"},
		{"go:darwin:amd64", "failed to build for iOS Simulator (x86_64)"},
		{"go:darwin:arm64", "failed to build for iOS Simulator (arm64)"},
		{"lipo", "failed to create fat binary"},
		{"xcrun", "failed to create XCFramework"},
		{"go:android:arm64", "failed to build for arm64-v8a"},
		{"go:android:arm", "failed to build for armeabi-v7a"},
	} {
		t.Run(tt.step, func(t *testing.T) {
			cmd, logPath := newMobileExportFixture(t)
			t.Setenv("SPX_MOBILE_FAIL", tt.step)
			var err error
			if strings.Contains(tt.step, "android") {
				err = cmd.ExportApk()
			} else {
				err = cmd.ExportIos()
			}
			if err == nil || !strings.Contains(err.Error(), tt.context) {
				t.Fatalf("build error = %v, want %q", err, tt.context)
			}
			calls := strings.Split(strings.TrimSpace(readTestFile(t, logPath)), "\n")
			if !strings.HasPrefix(calls[len(calls)-1], tt.step+"|") {
				t.Fatalf("commands continued after failed tool: %v", calls)
			}
		})
	}
}

func TestAndroidDeviceChecksBeforeInstallation(t *testing.T) {
	for _, scenario := range []string{"missing adb", "device lookup failure", "no connected device"} {
		t.Run(scenario, func(t *testing.T) {
			cmd, logPath := newMobileExportFixture(t)
			*cmd.Args.Install = true
			var want string
			switch scenario {
			case "missing adb":
				if err := os.Remove(filepath.Join(filepath.Dir(cmd.CmdPath), "adb")); err != nil {
					t.Fatal(err)
				}
				want = "adb command not found"
			case "device lookup failure":
				t.Setenv("SPX_MOBILE_FAIL", "adb:devices")
				want = "failed to check connected devices"
			case "no connected device":
				t.Setenv("SPX_MOBILE_DEVICES", "List of devices attached\ntest-device\tunauthorized")
				want = "no Android device connected"
			}
			if err := cmd.ExportApk(); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("install error = %v, want %q", err, want)
			}
			if calls := readTestFile(t, logPath); strings.Contains(calls, "adb:install|") {
				t.Fatalf("install attempted before device validation: %s", calls)
			}
		})
	}
}

func TestExportBuildInvokesRequestedPlatform(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "export.log")
			t.Setenv("SPX_EXPORT_LOG", logPath)
			script := `printf '%s\n' "$@" > "$SPX_EXPORT_LOG"`
			if fail {
				script += "\nexit 23"
			}
			binDir := installCommandTestTools(t, map[string]string{"godot": script})
			cmd := &CmdTool{ProjectDir: t.TempDir(), CmdPath: filepath.Join(binDir, "godot")}
			err := cmd.ExportBuild("Custom Platform")
			if (err != nil) != fail {
				t.Fatalf("platform export error = %v, want failure %v", err, fail)
			}
			if got := readTestFile(t, logPath); got != "--headless\n--quit\n--path\n"+cmd.ProjectDir+"\n--export-debug\nCustom Platform\n" {
				t.Fatalf("platform export args = %q", got)
			}
			if info, err := os.Stat(filepath.Join(cmd.ProjectDir, ".builds", "custom platform")); err != nil || !info.IsDir() {
				t.Fatalf("platform output directory missing: %v", err)
			}
		})
	}
}

func TestPrepareExportRejectsSocketAssets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket asset fixture")
	}
	// Keep the pathname below the Unix socket limit, independent of the test name.
	root, err := os.MkdirTemp("", "spx-assets-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	cmd := &CmdTool{TargetDir: root, ProjectDir: filepath.Join(root, "project")}
	asset := filepath.Join(root, "assets", "index.json")
	writeCommandTestFile(t, asset, "source")
	listener, err := net.Listen("unix", filepath.Join(root, "assets", "socket"))
	if err != nil {
		t.Skipf("Unix socket creation unavailable: %v", err)
	}
	defer listener.Close()
	if err := cmd.prepareExport(); err == nil || !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("socket asset error = %v", err)
	}
	if readTestFile(t, asset) != "source" {
		t.Fatal("failed staging changed source")
	}
	if _, err := os.Stat(filepath.Join(cmd.ProjectDir, "assets")); !os.IsNotExist(err) {
		t.Fatalf("invalid assets were staged: %v", err)
	}
}

func newMobileExportFixture(t *testing.T) (*CmdTool, string) {
	t.Helper()
	const script = `
tool=${0##*/}
step=$tool
case "$tool" in
  go) step="go:${GOOS-}:${GOARCH-}" ;;
  xcrun) if [ "$1" = --sdk ]; then step="sdk:$2"; fi ;;
  godot) case "$*" in *--editor*) step=import ;; *) step=export ;; esac ;;
  adb) step="adb:$1" ;;
esac
printf '%s|%s|%s|%s|%s|%s|%s\n' "$step" "$PWD" "$*" "${CGO_ENABLED-}" "${CC-}" "${CGO_CFLAGS-}" "${CGO_LDFLAGS-}" >> "$SPX_MOBILE_LOG"
if [ "$step" = "$SPX_MOBILE_FAIL" ]; then exit 29; fi
case "$tool" in
  xgo) printf 'package main\n' > xgo_autogen.go ;;
  xcrun)
    if [ "$1" = --sdk ]; then
      printf '  /test-sdk/%s\n' "$2"
    else
      for arg do output=$arg; done
      /bin/mkdir -p "$output"
    fi ;;
  godot)
    if [ "$step" = export ] && [ "$SPX_MOBILE_OUTPUT" != missing ]; then
      for arg do output=$arg; done
      printf 'exported artifact' > "$output"
    fi ;;
  adb)
    if [ "$1" = devices ]; then printf '%s\n' "$SPX_MOBILE_DEVICES"; fi
    if [ "$1" = install ]; then test -f "$3"; fi ;;
  ios-deploy) test -f "$2" ;;
esac
`
	tools := map[string]string{}
	for _, name := range []string{"go", "xgo", "godot", "xcrun", "lipo", "adb", "ios-deploy"} {
		tools[name] = script
	}
	binDir := installCommandTestTools(t, tools)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "commands.log")
	t.Setenv("SPX_MOBILE_LOG", logPath)
	t.Setenv("SPX_MOBILE_FAIL", "")
	t.Setenv("SPX_MOBILE_OUTPUT", "artifact")
	t.Setenv("SPX_MOBILE_DEVICES", "List of devices attached\ntest-device\tdevice")
	t.Setenv("ANDROID_NDK_ROOT", filepath.Join(root, "ndk"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	for _, name := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS", "CC"} {
		t.Setenv(name, "")
	}
	arch, tags, install := runtime.GOARCH, "", false
	cmd := &CmdTool{
		TargetDir: root, ProjectDir: filepath.Join(root, "project"),
		GoDir: filepath.Join(root, "project", "go"), LibPath: filepath.Join(root, "spx-test-"+arch+".so"),
		CmdPath: filepath.Join(binDir, "godot"), Args: ExtraArgs{Arch: &arch, Tags: &tags, Install: &install},
	}
	writeCommandTestFile(t, filepath.Join(root, "assets", "index.json"), "{}")
	writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, "project.godot"), "[application]\n")
	writeCommandTestFile(t, filepath.Join(cmd.GoDir, "ios_bridge.m"), "bridge")
	return cmd, logPath
}
