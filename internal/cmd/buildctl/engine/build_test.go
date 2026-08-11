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

	prepareCalled := false
	prepareErr := errors.New("expensive build preparation must not run")
	err := buildEngineWithEnvironmentPreparer(
		BuildConfig{Target: "template", Platform: "linux"},
		repoRoot,
		func(buildEnvironment) (map[string]string, string, error) {
			prepareCalled = true
			return nil, "", prepareErr
		},
	)
	if err == nil || !strings.Contains(err.Error(), "parse SCons profile") {
		t.Fatalf("buildEngineWithEnvironmentPreparer error = %v, want profile parse error", err)
	}
	if errors.Is(err, prepareErr) || prepareCalled {
		t.Fatal("build preparation ran before the SCons profile was validated")
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
