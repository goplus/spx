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

package shared

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSpineEnabled(t *testing.T) {
	for _, value := range []string{"", "0", "false", "FALSE", "no", "off", "1", "true", "TRUE", "yes", "on"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SPX_SPINE", value)
			got, err := SpineEnabled()
			want := slices.Contains([]string{"1", "true", "TRUE", "yes", "on"}, value)
			if err != nil || got != want {
				t.Fatalf("SpineEnabled() = %v, %v; want %v", got, err, want)
			}
		})
	}
	t.Setenv("SPX_SPINE", "typo")
	if _, err := SpineEnabled(); err == nil {
		t.Fatal("invalid feature switch accepted")
	}
}

func TestSpineBuildArgsAcrossTargets(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		module := SPXModule{source: "/host/spx", spine: enabled}
		want := "spx_spine=no"
		if enabled {
			want = "spx_spine=yes"
		}
		for name, args := range map[string][]string{
			"editor":    module.EditorBuildArgs("target=editor"),
			"desktop":   module.TemplateBuildArgs("platform=linux"),
			"web":       module.TemplateBuildArgs("platform=web"),
			"container": module.TemplateBuildArgsAt("/mounted/spx", "platform=linux"),
		} {
			if !slices.Contains(args, want) {
				t.Errorf("%s args %v missing %s", name, args, want)
			}
		}
	}
}

func TestLoadModuleCapturesSpineSelection(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{
		"SCsub": "", "config.py": "",
		SConsProfileFilename: `{"schema":1,"common":[],"editor_release":[],"template_release":[]}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SPX_SPINE", "1")
	module, err := LoadSPXModule(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPX_SPINE", "0")
	if !slices.Contains(module.TemplateBuildArgs(), "spx_spine=yes") {
		t.Fatal("module did not retain selected feature state")
	}
	t.Setenv("SPX_SPINE", "typo")
	if _, err := LoadSPXModule(root); err == nil {
		t.Fatal("module accepted invalid feature state")
	}
}

func TestProfileCannotEnableSpine(t *testing.T) {
	_, err := parseSConsProfile([]byte(`{"schema":1,"common":["spx_spine=yes"],"editor_release":[],"template_release":[]}`))
	if err == nil || !strings.Contains(err.Error(), "owned by build orchestration") {
		t.Fatalf("profile must not control Spine: %v", err)
	}
}

func TestSpineWorkspaceIsolation(t *testing.T) {
	root := t.TempDir()
	if err := prepareSpineWorkspace(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{spineWorkspaceMarker, "_sc_"} {
		if _, err := os.Stat(filepath.Join(root, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "gdspx"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareSpineWorkspace(root); err != nil {
		t.Fatalf("reuse dedicated workspace: %v", err)
	}
	t.Setenv("GOPATH", root)
	t.Setenv("SPX_SPINE", "0")
	if err := PrepareSpineEnvironment(); err == nil {
		t.Fatal("default build accepted Spine workspace")
	}
	t.Setenv("SPX_SPINE", "1")
	t.Setenv("SPX_SPINE_GOPATH", "")
	if err := PrepareSpineEnvironment(); err == nil {
		t.Fatal("missing dedicated path accepted")
	}
	t.Setenv("SPX_SPINE_GOPATH", root)
	t.Setenv("GOBIN", "")
	if err := PrepareSpineEnvironment(); err != nil {
		t.Fatalf("valid dedicated workspace: %v", err)
	}
	t.Setenv("GOBIN", filepath.Join(root, "elsewhere"))
	if err := PrepareSpineEnvironment(); err == nil {
		t.Fatal("external GOBIN accepted")
	}
	t.Setenv("GOBIN", "")
	t.Setenv("SPX_SPINE_GOPATH", filepath.Join(root, "other"))
	if err := PrepareSpineEnvironment(); err == nil {
		t.Fatal("mismatched path accepted")
	}
	existing := t.TempDir()
	if err := os.MkdirAll(filepath.Join(existing, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(existing, "bin", "gdspx"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareSpineWorkspace(existing); err == nil {
		t.Fatal("nonempty unmarked bin accepted")
	}
}

func TestSpineTemplateDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", root)
	t.Setenv("SPX_SPINE", "1")
	got, err := detectGodotTemplateDir("4.5.test")
	want := filepath.Join(root, "bin", "editor_data", "export_templates", "4.5.test")
	if err != nil || got != want {
		t.Fatalf("template path = %q, %v; want %q", got, err, want)
	}
}
