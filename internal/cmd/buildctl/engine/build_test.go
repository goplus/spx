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

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
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
			} else if fileExists(env["SPX_POST_RECORD"]) {
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
	if fileExists(filepath.Join(root, "web_old.zip")) {
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
		if !fileExists(path) {
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
