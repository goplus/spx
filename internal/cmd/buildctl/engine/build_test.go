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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
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
	}
}

func TestLockedEngineScriptCommand(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	for _, subdir := range []string{"godot", filepath.Join("godot", "platform")} {
		for _, exitCode := range []int{0, 23} {
			t.Run(fmt.Sprintf("%s/exit-%d", subdir, exitCode), func(t *testing.T) {
				root := t.TempDir()
				workdir := filepath.Join(root, subdir)
				mustMkdirAll(t, workdir)
				workdir, err := filepath.EvalSymlinks(workdir)
				if err != nil {
					t.Fatal(err)
				}
				lockDir := filepath.Join(workdir, ".spx_build_lock")
				if filepath.Base(workdir) != "godot" {
					lockDir = filepath.Join(filepath.Dir(workdir), ".spx_build_lock")
				}
				env := map[string]string{}
				for _, item := range os.Environ() {
					if key, value, ok := strings.Cut(item, "="); ok {
						env[key] = value
					}
				}
				env["SPX_EXPECTED_DIR"] = workdir
				env["SPX_EXPECTED_LOCK"] = lockDir
				env["SPX_MARKER"] = "space ' quote $literal"
				script := fmt.Sprintf(`[[ "$PWD" == "$SPX_EXPECTED_DIR" ]] || exit 90
[[ -d "$SPX_EXPECTED_LOCK" && -s "$SPX_EXPECTED_LOCK/pid" ]] || exit 91
printf '%%s' "$SPX_MARKER" > captured
exit %d`, exitCode)
				err = runLockedEngineCommandWithEnv(workdir, env, "bash", "-lc", script)
				if exitCode == 0 {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) || exitErr.ExitCode() != exitCode {
						t.Fatalf("command error = %v, want exit %d", err, exitCode)
					}
				}
				got, err := os.ReadFile(filepath.Join(workdir, "captured"))
				if err != nil || string(got) != env["SPX_MARKER"] {
					t.Fatalf("command environment = %q (%v)", got, err)
				}
				if _, err := os.Stat(lockDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("lock was not released: %v", err)
				}
				if err := runLockedEngineCommandWithEnv(workdir, env, filepath.Join(root, "missing-command")); err == nil {
					t.Fatal("missing command succeeded")
				}
				if _, err := os.Stat(lockDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("start failure retained lock: %v", err)
				}
			})
		}
	}
}
