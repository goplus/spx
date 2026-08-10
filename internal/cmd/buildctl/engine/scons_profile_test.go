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
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

func TestParseSConsProfilePreservesOrderedArguments(t *testing.T) {
	profile, err := parseSConsProfile([]byte(`{
  "schema": 1,
  "common": ["optimize=size", "vulkan=false"],
  "editor_release": ["debug_symbols=true"],
  "template_release": ["debug_symbols=false", "disable_3d=true"]
}`))
	if err != nil {
		t.Fatalf("parseSConsProfile returned error: %v", err)
	}

	if got, want := profile.EditorReleaseArgs(), []string{"optimize=size", "vulkan=false", "debug_symbols=true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("editor release args = %#v, want %#v", got, want)
	}
	if got, want := profile.TemplateReleaseArgs(), []string{"optimize=size", "vulkan=false", "debug_symbols=false", "disable_3d=true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("template release args = %#v, want %#v", got, want)
	}
	editorArgs := profile.EditorReleaseArgs()
	editorArgs[0] = "changed=yes"
	if got := profile.Common[0]; got != "optimize=size" {
		t.Fatalf("merged editor args mutated the profile: %q", got)
	}
}

func TestParseSConsProfileRejectsInvalidInput(t *testing.T) {
	valid := `{"schema":1,"common":["optimize=size"],"editor_release":["debug_symbols=true"],"template_release":["debug_symbols=false"]}`
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "malformed JSON", input: `{`, wantErr: "invalid SCons profile JSON"},
		{name: "non-object", input: `[]`, wantErr: "top-level value must be an object"},
		{name: "unknown field", input: strings.Replace(valid, `"schema":1`, `"schema":1,"extra":true`, 1), wantErr: `unknown SCons profile field "extra"`},
		{name: "duplicate field", input: strings.Replace(valid, `"schema":1`, `"schema":1,"schema":1`, 1), wantErr: `duplicate SCons profile field "schema"`},
		{name: "missing field", input: `{"schema":1,"common":[],"editor_release":[]}`, wantErr: `missing SCons profile field "template_release"`},
		{name: "wrong schema", input: strings.Replace(valid, `"schema":1`, `"schema":2`, 1), wantErr: "unsupported SCons profile schema 2"},
		{name: "non-integer schema", input: strings.Replace(valid, `"schema":1`, `"schema":"1"`, 1), wantErr: "invalid SCons profile schema"},
		{name: "null group", input: strings.Replace(valid, `"common":["optimize=size"]`, `"common":null`, 1), wantErr: `field "common" must be an array`},
		{name: "non-array group", input: strings.Replace(valid, `"common":["optimize=size"]`, `"common":"optimize=size"`, 1), wantErr: `invalid SCons profile field "common"`},
		{name: "empty entry", input: strings.Replace(valid, `"optimize=size"`, `""`, 1), wantErr: "entry 0 is empty"},
		{name: "whitespace", input: strings.Replace(valid, `"optimize=size"`, `"optimize =size"`, 1), wantErr: "contains whitespace"},
		{name: "null control", input: strings.Replace(valid, `"optimize=size"`, `"optimize=size\u0000"`, 1), wantErr: "contains a control character"},
		{name: "missing equals", input: strings.Replace(valid, `"optimize=size"`, `"optimize"`, 1), wantErr: "key=value"},
		{name: "multiple equals", input: strings.Replace(valid, `"optimize=size"`, `"optimize=size=extra"`, 1), wantErr: "key=value"},
		{name: "empty value", input: strings.Replace(valid, `"optimize=size"`, `"optimize="`, 1), wantErr: "key=value"},
		{name: "invalid key", input: strings.Replace(valid, `"optimize=size"`, `"1optimize=size"`, 1), wantErr: "key=value"},
		{name: "orchestration key", input: strings.Replace(valid, `"optimize=size"`, `"platform=web"`, 1), wantErr: "owned by build orchestration"},
		{name: "duplicate key in group", input: strings.Replace(valid, `"optimize=size"`, `"optimize=size","optimize=none"`, 1), wantErr: `duplicate SCons key "optimize"`},
		{name: "duplicate common editor key", input: strings.Replace(valid, `"debug_symbols=true"`, `"optimize=none"`, 1), wantErr: `duplicate SCons key "optimize" across "common" and "editor_release"`},
		{name: "trailing value", input: valid + `{}`, wantErr: "trailing value"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseSConsProfile([]byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("parseSConsProfile error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestRepositorySConsProfile(t *testing.T) {
	repoRoot, err := shared.FindRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot returned error: %v", err)
	}
	profile, err := loadSConsProfile(filepath.Join(repoRoot, "godot_modules", "spx"))
	if err != nil {
		t.Fatalf("loadSConsProfile returned error: %v", err)
	}

	common := strings.Join(profile.Common, "\n")
	for _, required := range []string{
		"module_text_server_adv_enabled=true",
		"module_text_server_fb_enabled=false",
		"module_godot_physics_3d_enabled=false",
		"builtin_harfbuzz=true",
		"custom_modules_recursive=false",
		"module_spx_enabled=true",
	} {
		if !strings.Contains(common, required) {
			t.Fatalf("repository SCons profile is missing %q", required)
		}
	}
	if strings.Contains(common, "disable_navigation=") {
		t.Fatal("repository SCons profile must keep navigation enabled for the SPX module")
	}
	if strings.Contains(common, "disable_3d_physics=") {
		t.Fatal("repository SCons profile must not contain the unsupported disable_3d_physics option")
	}
}
