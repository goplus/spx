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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSConsScriptIncludesCommonArgs(t *testing.T) {
	script := sconsScript([]string{"platform=android target=template_debug arch=arm32"})
	for _, arg := range []string{
		"scons optimize=size",
		"module_text_server_adv_enabled=true",
		"module_text_server_fb_enabled=false",
		"builtin_harfbuzz=true",
	} {
		if !strings.Contains(script, arg) {
			t.Fatalf("expected %q in script: %s", arg, script)
		}
	}
	if !strings.Contains(script, "platform=android target=template_debug arch=arm32") {
		t.Fatalf("expected command args in script: %s", script)
	}
}

func TestSConsScriptQuotesCommandPath(t *testing.T) {
	script := sconsScriptWithCommand("/tmp/spx tools/scons", []string{"platform=ios target=template_debug"})
	if !strings.HasPrefix(script, "'/tmp/spx tools/scons' optimize=size") {
		t.Fatalf("sconsScriptWithCommand did not quote command path: %q", script)
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
