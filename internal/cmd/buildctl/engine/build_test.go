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

package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/goplus/spx/v4/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v4/internal/release"
)

func TestBuildEngineRejectsInvalidProfileBeforePreparingEnvironment(t *testing.T) {
	repoRoot := t.TempDir()
	moduleSource := filepath.Join(repoRoot, "godot_modules", "spx")
	mustWriteFile(t, filepath.Join(moduleSource, "SCsub"), []byte("# fixture\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "config.py"), []byte("# fixture\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "spx_scons_profile.json"), []byte(`{"schema":`))
	t.Setenv("SPX_MODULE_SRC", moduleSource)

	err := BuildEngine(BuildConfig{Target: "template", Platform: "linux"}, repoRoot)
	if err == nil || !strings.Contains(err.Error(), "parse SCons profile") {
		t.Fatalf("BuildEngine error = %v, want profile parse error", err)
	}
}

func TestSConsBuildScriptIncludesProfileAndCustomModule(t *testing.T) {
	module, moduleSource := loadEngineTestSPXModule(t, `{
  "schema": 1,
  "common": ["optimize=size", "module_text_server_adv_enabled=true"],
  "editor_release": [],
  "template_release": ["debug_symbols=false"]
}`)
	script := templateSConsBuildScript(
		"/tmp/spx tools/scons",
		module,
		[]string{"platform=android target=template_debug arch=arm32"},
	)
	for _, arg := range []string{
		"'/tmp/spx tools/scons'",
		"'optimize=size'",
		"'module_text_server_adv_enabled=true'",
		"'debug_symbols=false'",
		"'custom_modules=" + moduleSource + "'",
	} {
		if !strings.Contains(script, arg) {
			t.Fatalf("expected %q in script: %s", arg, script)
		}
	}
	if !strings.Contains(script, "'platform=android' 'target=template_debug' 'arch=arm32'") {
		t.Fatalf("expected command args in script: %s", script)
	}
}

func TestSConsScriptQuotesCommandPath(t *testing.T) {
	module, moduleSource := loadEngineTestSPXModule(t, `{
  "schema": 1,
  "common": [],
  "editor_release": [],
  "template_release": []
}`)
	script := templateSConsBuildScript("/tmp/spx tools/scons", module, []string{"platform=ios target=template_debug"})
	want := "'/tmp/spx tools/scons' 'platform=ios' 'target=template_debug' 'custom_modules=" + moduleSource + "'"
	if script != want {
		t.Fatalf("templateSConsBuildScript did not quote command path: %q", script)
	}
}

func loadEngineTestSPXModule(t *testing.T, profile string) (shared.SPXModule, string) {
	t.Helper()
	moduleSource := filepath.Join(t.TempDir(), "SPX Modules", "spx")
	mustWriteFile(t, filepath.Join(moduleSource, "SCsub"), []byte("# fixture\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "config.py"), []byte("# fixture\n"))
	mustWriteFile(t, filepath.Join(moduleSource, shared.SConsProfileFilename), []byte(profile))
	module, err := shared.LoadSPXModule(moduleSource)
	if err != nil {
		t.Fatalf("LoadSPXModule returned error: %v", err)
	}
	return module, moduleSource
}

func writeEngineTestTool(t *testing.T, path, script string) {
	t.Helper()
	mustWriteFile(t, path, []byte("#!/bin/sh\n"+script))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertEngineTestCommandRecord(t *testing.T, path string, want []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); err != nil || !slices.Equal(got, want) {
		t.Fatalf("command record = %q (%v), want %q", data, err, want)
	}
}

func TestBuildEngineDesktopCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executables require a POSIX shell")
	}
	module, moduleSource := loadEngineTestSPXModule(t, `{
  "schema": 1, "common": ["optimize=size"],
  "editor_release": ["debug_symbols=true"], "template_release": ["debug_symbols=false"]
}`)
	for _, target := range []string{"editor", "template"} {
		for _, platform := range []string{"linux", "macos", "windows"} {
			for _, failure := range []string{"success", "build"} {
				t.Run(target+"/"+platform+"/"+failure, func(t *testing.T) {
					root, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					buildEnv := buildEnvironment{
						EngineDir: filepath.Join(root, "godot"), GoPath: filepath.Join(root, "go path"),
						Platform: platform, Arch: "arm64", Version: "test",
					}
					plan, err := resolveEngineBuildShellPlan(buildEnv, BuildConfig{Target: target, Platform: platform})
					if err != nil {
						t.Fatal(err)
					}
					source, destination := plan.EditorSource, plan.EditorDestination
					wantArgs := []string{"optimize=size", "debug_symbols=true", "target=editor", "dev_build=yes"}
					if plan.EditorUseVSProj {
						wantArgs = append(wantArgs, "vsproj=yes")
					}
					if target == "template" {
						source, destination = plan.TemplateSource, plan.TemplateDestination
						wantArgs = []string{"optimize=size", "debug_symbols=false", "platform=" + plan.TemplateSConsPlatform, "target=template_release"}
					}
					source = filepath.Join(buildEnv.EngineDir, source)
					wantArgs = append(wantArgs, "custom_modules="+moduleSource)
					mustWriteFile(t, source, []byte("stale"))
					mustWriteFile(t, destination, []byte("old installation"))
					scons := filepath.Join(root, "build tools", "scons")
					writeEngineTestTool(t, scons, `set -eu
test -s .spx_build_lock/pid
printf '%s\n' "$PWD" "$SPX_MARKER" "$@" > "$SPX_BUILD_RECORD"
if [ "$SPX_FAILURE" = build ]; then exit 23; fi
printf built > "$SPX_ARTIFACT"
`)
					env := shared.CurrentEnvMap()
					env["SPX_MARKER"] = "space ' quote $literal"
					env["SPX_FAILURE"] = failure
					env["SPX_ARTIFACT"] = source
					env["SPX_BUILD_RECORD"] = filepath.Join(root, "build record")
					if target == "editor" {
						err = buildEngineEditor(buildEnv, env, scons, module, plan)
					} else {
						err = buildEngineTemplate(buildEnv, env, scons, module, plan)
					}
					wantOutput := "built"
					if failure == "success" {
						if err != nil {
							t.Fatal(err)
						}
					} else {
						var exitErr *exec.ExitError
						if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
							t.Fatalf("build error = %v, want exit 23", err)
						}
						wantOutput = "old installation"
					}
					wantRecord := append([]string{buildEnv.EngineDir, env["SPX_MARKER"]}, wantArgs...)
					assertEngineTestCommandRecord(t, env["SPX_BUILD_RECORD"], wantRecord)
					data, err := os.ReadFile(destination)
					if err != nil || string(data) != wantOutput {
						t.Fatalf("installed artifact = %q (%v), want %q", data, err, wantOutput)
					}
					if _, err := os.Stat(filepath.Join(buildEnv.EngineDir, ".spx_build_lock")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("build lock was not released: %v", err)
					}
				})
			}
		}
	}
}

func TestBuildEngineMobileTemplateCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture executables require a POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	module, moduleSource := loadEngineTestSPXModule(t, `{
  "schema": 1, "common": ["optimize=size"],
  "editor_release": [], "template_release": ["debug_symbols=false"]
}`)
	for _, tt := range []struct{ platform, failure string }{
		{"ios", "success"}, {"ios", "build"},
		{"android", "success"}, {"android", "build"}, {"android", "post"},
	} {
		t.Run(tt.platform+"/"+tt.failure, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			buildEnv := buildEnvironment{
				EngineDir: filepath.Join(root, "godot"), TemplateDir: filepath.Join(root, "templates"),
			}
			plan := BuildShellPlan{
				Platform:              tt.platform,
				TemplateSConsCommands: []string{"platform=" + tt.platform + " target=template_release arch=arm64"},
				TemplatePostDir:       filepath.Join("platform", "android", "java"),
				TemplatePostCommands:  []string{"./gradlew generateGodotTemplates"},
			}
			postDir := filepath.Join(buildEnv.EngineDir, plan.TemplatePostDir)
			mustMkdirAll(t, buildEnv.TemplateDir)
			for _, name := range []string{"godot_ios.zip", "android_debug.apk", "android_release.apk", "android_source.zip"} {
				// Existing artifacts must not be copied when either command fails.
				mustWriteFile(t, filepath.Join(buildEnv.EngineDir, "bin", name), []byte("stale"))
			}
			javaHome := filepath.Join(root, "test jdk")
			writeEngineTestTool(t, filepath.Join(javaHome, "bin", "java"),
				"printf '%s\\n' 'openjdk version \""+release.DefaultRuntimeLock().Toolchain.JDK+".0.0\"'\n")
			t.Setenv("JAVA_HOME", javaHome)
			t.Setenv("PATH", shared.PrependToPath(os.Getenv("PATH"), filepath.Join(javaHome, "bin")))
			scons := filepath.Join(root, "build tools", "scons")
			writeEngineTestTool(t, scons, `set -eu
test -s .spx_build_lock/pid
printf '%s\n' "$PWD" "$SPX_MARKER" "$JAVA_HOME" "$@" > "$SPX_BUILD_RECORD"
if [ "$SPX_FAILURE" = build ]; then exit 23; fi
for name in godot_ios.zip android_debug.apk android_release.apk android_source.zip; do
  printf built > "bin/$name"
done
`)
			writeEngineTestTool(t, filepath.Join(postDir, "gradlew"), `set -eu
test -s ../.spx_build_lock/pid
test ! -e "$SPX_ENGINE_DIR/.spx_build_lock"
test -s "$SPX_BUILD_RECORD"
printf '%s\n' "$PWD" "$SPX_MARKER" "$JAVA_HOME" "$@" > "$SPX_POST_RECORD"
if [ "$SPX_FAILURE" = post ]; then exit 23; fi
printf post > "$SPX_ENGINE_DIR/bin/android_source.zip"
`)
			env := shared.CurrentEnvMap()
			env["JAVA_HOME"] = "caller java home"
			env["SPX_MARKER"] = "space ' quote $literal"
			env["SPX_FAILURE"] = tt.failure
			env["SPX_ENGINE_DIR"] = buildEnv.EngineDir
			env["SPX_BUILD_RECORD"] = filepath.Join(root, "build-record")
			env["SPX_POST_RECORD"] = filepath.Join(root, "post-record")
			err = buildEngineTemplate(buildEnv, env, scons, module, plan)
			if tt.failure == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
					t.Fatalf("build error = %v, want exit 23", err)
				}
			}
			wantJavaHome := env["JAVA_HOME"]
			if tt.platform == "android" {
				wantJavaHome = javaHome
			}
			assertEngineTestCommandRecord(t, env["SPX_BUILD_RECORD"], []string{
				buildEnv.EngineDir, env["SPX_MARKER"], wantJavaHome, "optimize=size", "debug_symbols=false",
				"platform=" + tt.platform, "target=template_release", "arch=arm64", "custom_modules=" + moduleSource,
			})
			if tt.platform == "android" && tt.failure != "build" {
				assertEngineTestCommandRecord(t, env["SPX_POST_RECORD"], []string{postDir, env["SPX_MARKER"], javaHome, "generateGodotTemplates"})
			} else if shared.FileExists(env["SPX_POST_RECORD"]) {
				t.Fatal("unexpected post-build command")
			}
			if env["JAVA_HOME"] != "caller java home" {
				t.Fatal("JDK exports mutated the caller environment")
			}
			for _, lock := range []string{
				filepath.Join(buildEnv.EngineDir, ".spx_build_lock"),
				filepath.Join(filepath.Dir(postDir), ".spx_build_lock"),
			} {
				if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("build lock %s was not released: %v", lock, err)
				}
			}
			outputs := map[string]string{}
			if tt.failure == "success" {
				outputs["ios.zip"] = "built"
				if tt.platform == "android" {
					outputs = map[string]string{"android_debug.apk": "built", "android_release.apk": "built", "android_source.zip": "post"}
				}
			}
			entries, err := os.ReadDir(buildEnv.TemplateDir)
			if err != nil || len(entries) != len(outputs) {
				t.Fatalf("template outputs = %v (%v), want %v", entries, err, outputs)
			}
			for name, want := range outputs {
				data, err := os.ReadFile(filepath.Join(buildEnv.TemplateDir, name))
				if err != nil || string(data) != want {
					t.Fatalf("template %s = %q (%v), want %q", name, data, err, want)
				}
			}
		})
	}
}

func TestShellJoinRoundTrip(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	want := []string{
		"platform=windows",
		"custom_modules=C:/SPX Modules/module's $(not-executed)",
	}
	script := "set -- " + shellJoin(want) + `; printf '%s\n' "$@"`
	output, err := exec.Command(bash, "-c", script).Output()
	if err != nil {
		t.Fatalf("shell round trip returned error: %v", err)
	}
	got := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shell round trip = %#v, want %#v", got, want)
	}
}

func TestMergeStringMapsOverridesExisting(t *testing.T) {
	merged := mergeStringMaps(
		map[string]string{"PATH": "/usr/bin", "A": "1"},
		map[string]string{"PATH": "/custom/bin:/usr/bin", "B": "2"},
	)
	if merged["PATH"] != "/custom/bin:/usr/bin" || merged["A"] != "1" || merged["B"] != "2" {
		t.Fatalf("unexpected merged map: %#v", merged)
	}
}

func TestPopulateWebTemplateCopies(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "godot.web.template_release.wasm32.nothreads.zip")
	mustWriteFile(t, src, []byte("zip"))
	mustWriteFile(t, filepath.Join(root, "web_old.zip"), []byte("old"))

	if err := populateWebTemplateCopies(src, root); err != nil {
		t.Fatalf("populateWebTemplateCopies returned error: %v", err)
	}
	if shared.FileExists(filepath.Join(root, "web_old.zip")) {
		t.Fatal("expected old web zip to be removed")
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"web_dlink_nothreads_debug.zip",
		"web_dlink_nothreads_release.zip",
		"web_nothreads_debug.zip",
		"web_nothreads_release.zip",
		"web_dlink_debug.zip",
		"web_dlink_release.zip",
		"web_debug.zip",
		"web_release.zip",
	} {
		path := filepath.Join(root, name)
		if !shared.FileExists(path) {
			t.Fatalf("expected %s to exist", name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) returned error: %v", path, err)
		}
		if string(content) != "zip" {
			t.Fatalf("%s content = %q, want zip", name, string(content))
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(srcInfo, info) {
			t.Fatalf("%s is a hard link, want an independent copy", name)
		}
	}
}

func TestBuildEngineEntryPoint(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("isolated Linux toolchain fixture")
	}
	for _, scenario := range []string{"editor", "template", "invalid platform", "invalid target", "template directory", "missing python", "missing source"} {
		t.Run(scenario, func(t *testing.T) {
			root := isolateEngineEnvironment(t)
			t.Chdir(root)
			_, moduleSource := loadEngineTestSPXModule(t, `{"schema":1,"common":[],"editor_release":[],"template_release":[]}`)
			t.Setenv("SPX_MODULE_SRC", moduleSource)
			cfg := BuildConfig{Target: "template", Platform: "linux"}
			if scenario == "editor" {
				cfg.Target = "editor"
			}
			env, err := shared.ResolveBuildEnvironment(root, cfg.Platform)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := resolveEngineBuildShellPlan(env, cfg)
			if err != nil {
				t.Fatal(err)
			}
			lock := release.DefaultRuntimeLock()
			bin := filepath.Join(root, "tools")
			for _, name := range []string{"go", "xgo", "python3"} {
				writeEngineTestTool(t, filepath.Join(bin, name), "exit 0\n")
			}
			// Any source acquisition is intercepted; no real git or installer is on PATH.
			writeEngineTestTool(t, filepath.Join(bin, "git"), "printf '%s\\n' '"+lock.Godot.Commit+"'\n")
			venvBin := filepath.Join(root, ".bin", "scons-"+lock.Toolchain.SCons, "bin")
			writeEngineTestTool(t, filepath.Join(venvBin, "python"), "printf '%s\\n' '"+lock.Toolchain.SCons+"'\n")
			record := filepath.Join(root, "scons-record")
			t.Setenv("SPX_BUILD_RECORD", record)
			writeEngineTestTool(t, filepath.Join(venvBin, "scons"), "[ \"$1\" = --version ] && exit 0\nprintf '%s\\n' \"$@\" > \"$SPX_BUILD_RECORD\"\n")
			t.Setenv("PATH", bin)
			source, destination := plan.TemplateSource, plan.TemplateDestination
			if scenario == "editor" {
				source, destination = plan.EditorSource, plan.EditorDestination
			}
			mustWriteFile(t, filepath.Join(env.EngineDir, source), []byte("built artifact"))
			wantError := ""
			switch scenario {
			case "invalid platform":
				cfg.Platform, wantError = "invalid", "unsupported platform"
			case "invalid target":
				cfg.Target, wantError = "invalid", "unsupported build target"
			case "template directory":
				mustWriteFile(t, filepath.Join(root, ".local"), []byte("blocked"))
				wantError = "not a directory"
			case "missing python":
				if err := os.Remove(filepath.Join(bin, "python3")); err != nil {
					t.Fatal(err)
				}
				wantError = "neither python3 nor python"
			case "missing source":
				writeEngineTestTool(t, filepath.Join(bin, "git"), "exit 73\n")
				if err := os.RemoveAll(env.EngineDir); err != nil {
					t.Fatal(err)
				}
				wantError = "exit status 73"
			}
			err = BuildEngine(cfg, root)
			if wantError != "" {
				if err == nil || !strings.Contains(err.Error(), wantError) {
					t.Fatalf("BuildEngine error = %v, want %q", err, wantError)
				}
				if shared.FileExists(record) || shared.FileExists(destination) {
					t.Fatal("preparation failure ran the build or installed an artifact")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(destination); err != nil || string(data) != "built artifact" {
				t.Fatalf("installed artifact = %q, %v", data, err)
			}
			if data, err := os.ReadFile(record); err != nil || !strings.Contains(string(data), "target="+map[string]string{"editor": "editor", "template": "template_release"}[scenario]) {
				t.Fatalf("build command = %q, %v", data, err)
			}
		})
	}
}

func TestBuildEngineWebTemplate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("isolated Linux EMSDK fixture")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	module, moduleSource := loadEngineTestSPXModule(t, `{"schema":1,"common":[],"editor_release":[],"template_release":["optimize=size"]}`)
	for _, scenario := range []string{"normal", "worker", "sdk failure", "build failure", "cache failure"} {
		t.Run(scenario, func(t *testing.T) {
			root := isolateEngineEnvironment(t)
			sdk := filepath.Join(root, ".local", "share", "emsdk", "emsdk")
			compilerBin := filepath.Join(sdk, "upstream", "emscripten")
			t.Setenv("EM_CACHE", filepath.Join(root, "em-cache"))
			t.Setenv("EM_CONFIG", filepath.Join(root, "em-config"))
			version := release.DefaultRuntimeLock().Toolchain.EMSDK
			for _, compiler := range []string{"emcc", "em++"} {
				writeEngineTestTool(t, filepath.Join(compilerBin, compiler), "printf '%s\\n' 'emcc fixture "+version+"'\n")
			}
			mustWriteFile(t, filepath.Join(sdk, "emsdk_env.sh"), []byte("export EMSDK="+shared.ShellQuote(sdk)+"\nexport PATH="+shared.ShellQuote(compilerBin)+":\"$PATH\"\n"))
			writeEngineTestTool(t, filepath.Join(sdk, "emsdk"), "[ \"$1\" = activate ] || exit 90\n[ \"$SPX_WEB_CASE\" != 'sdk failure' ] || exit 23\n")
			t.Setenv("SPX_WEB_CASE", scenario)
			env := buildEnvironment{EngineDir: filepath.Join(root, "godot"), TemplateDir: filepath.Join(root, "templates"), GoPath: filepath.Join(root, "go"), Version: "test"}
			mode := "normal"
			if scenario == "worker" {
				mode = "worker"
			}
			plan, err := resolveEngineBuildShellPlan(env, BuildConfig{Target: "template", Platform: "web", Mode: mode})
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(env.EngineDir, "bin", "godot.web.template_release.wasm32"+plan.WebThreadSuffix+".zip")
			mustWriteFile(t, source, []byte("web template"))
			mustWriteFile(t, filepath.Join(env.TemplateDir, "web_old.zip"), []byte("stale"))
			commandEnv := shared.CurrentEnvMap()
			commandEnv["EMSDK"] = "old SDK"
			commandEnv["SPX_BUILD_RECORD"] = filepath.Join(root, "build-record")
			scons := filepath.Join(root, "scons")
			writeEngineTestTool(t, scons, "[ \"$SPX_WEB_CASE\" != 'build failure' ] || exit 23\nprintf '%s\\n' \"$EMSDK\" \"$@\" > \"$SPX_BUILD_RECORD\"\n")
			if scenario == "cache failure" {
				mustWriteFile(t, filepath.Dir(plan.WebCachedTemplateZip), []byte("blocked"))
			}
			err = buildEngineTemplate(env, commandEnv, scons, module, plan)
			if commandEnv["EMSDK"] != "old SDK" {
				t.Fatal("SDK exports mutated the caller environment")
			}
			if strings.HasSuffix(scenario, "failure") {
				if scenario == "cache failure" {
					if err == nil || !strings.Contains(err.Error(), "not a directory") {
						t.Fatalf("cache error = %v, want blocked destination", err)
					}
				} else {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
						t.Fatalf("build error = %v, want exit 23", err)
					}
				}
				if !shared.FileExists(filepath.Join(env.TemplateDir, "web_old.zip")) {
					t.Fatal("failed build replaced existing templates")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantArgs := []string{sdk, "optimize=size", "platform=web", "target=template_release", "threads=" + plan.WebThreads}
			if mode == "worker" {
				wantArgs = append(wantArgs, "proxy_to_pthread=true")
			}
			wantArgs = append(wantArgs, "custom_modules="+moduleSource)
			assertEngineTestCommandRecord(t, commandEnv["SPX_BUILD_RECORD"], wantArgs)
			for _, destination := range append([]string{plan.WebCachedTemplateZip}, webTemplateNames[:]...) {
				if !filepath.IsAbs(destination) {
					destination = filepath.Join(env.TemplateDir, destination)
				}
				if data, err := os.ReadFile(destination); err != nil || string(data) != "web template" {
					t.Fatalf("web template %s = %q, %v", destination, data, err)
				}
			}
			if shared.FileExists(filepath.Join(env.TemplateDir, "web_old.zip")) {
				t.Fatal("old template was retained")
			}
		})
	}
}

func TestBuildArtifactHelpers(t *testing.T) {
	root := t.TempDir()
	if err := copyGlobMatches("[", root); err == nil {
		t.Fatal("invalid glob accepted")
	}
	if err := copyGlobMatches(filepath.Join(root, "*.apk"), root); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "artifact.apk")
	mustWriteFile(t, source, []byte("artifact"))
	blocked := filepath.Join(root, "blocked")
	mustWriteFile(t, blocked, nil)
	if err := copyGlobMatches(filepath.Join(root, "*.apk"), blocked); err == nil {
		t.Fatal("blocked copy destination accepted")
	}
	if err := populateWebTemplateCopies(filepath.Join(root, "missing"), filepath.Join(root, "templates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing web artifact=%v", err)
	}
	if err := buildEngineTemplate(buildEnvironment{}, nil, "", shared.SPXModule{}, BuildShellPlan{Platform: "unsupported"}); err == nil || !strings.Contains(err.Error(), "unknown platform") {
		t.Fatalf("unsupported template=%v", err)
	}
	if script := templateSConsBuildScript("scons", shared.SPXModule{}, []string{"", " \t"}); script != "" {
		t.Fatalf("blank script=%q", script)
	}
}

func TestWaitForNonEmptyFile(t *testing.T) {
	root := t.TempDir()
	empty := filepath.Join(root, "empty")
	mustWriteFile(t, empty, nil)
	for _, tt := range []struct {
		path    string
		message string
	}{{filepath.Join(root, "missing"), "waiting for"}, {empty, "file is still empty"}, {root, "file is still empty"}} {
		if err := waitForNonEmptyFile(tt.path, 0); err == nil || !strings.Contains(err.Error(), tt.message) {
			t.Fatalf("wait %s=%v", tt.path, err)
		}
	}
	synctest.Test(t, func(t *testing.T) {
		ready := filepath.Join(root, "eventual")
		done := make(chan error, 1)
		go func() { done <- waitForNonEmptyFile(ready, time.Second) }()
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatalf("wait completed before the file was ready: %v", err)
		default:
		}
		mustWriteFile(t, ready, []byte("ready"))
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}
