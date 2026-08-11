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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDescribeRelease(t *testing.T) {
	description, err := describeRelease()
	if err != nil {
		t.Fatal(err)
	}
	if description.ReleaseTag != "v3.2.0" || description.Version != "3.2.0" {
		t.Fatalf("unexpected SPX release: %#v", description)
	}
	if description.RuntimeVersion != "2.4.0" || description.RuntimeABI != 2 || description.RuntimeTag != "runtime-v2.4.0" || description.RuntimeManifest != "runtime-manifest.json" {
		t.Fatalf("unexpected runtime release: %#v", description)
	}
	if description.ReleaseRepository != "goplus/spx" || description.GodotRepository != "goplus/godot" {
		t.Fatalf("unexpected repositories: %#v", description)
	}
	if description.ModulePath != "godot_modules/spx" {
		t.Fatalf("unexpected module path: %#v", description)
	}
}

func TestGitHubRepositoryName(t *testing.T) {
	if got, err := githubRepositoryName("https://github.com/goplus/godot.git"); err != nil || got != "goplus/godot" {
		t.Fatalf("githubRepositoryName = %q, %v", got, err)
	}
	for _, invalid := range []string{"goplus/godot", "https://example.com/goplus/godot.git", "https://github.com/goplus/godot"} {
		if _, err := githubRepositoryName(invalid); err == nil {
			t.Errorf("githubRepositoryName(%q) succeeded", invalid)
		}
	}
}

func TestWriteGitHubOutput(t *testing.T) {
	description, err := describeRelease()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "output")
	if err := writeGitHubOutput(path, description.githubOutputs()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wantLines := []string{
		"release_tag=v3.2.0",
		"version=3.2.0",
		"runtime_version=2.4.0",
		"runtime_abi=2",
		"runtime_tag=runtime-v2.4.0",
		"runtime_manifest=runtime-manifest.json",
		"release_repository=goplus/spx",
		"godot_repository=goplus/godot",
		"godot_commit=" + description.GodotCommit,
		"module_path=godot_modules/spx",
	}
	if got, want := string(data), strings.Join(wantLines, "\n")+"\n"; got != want {
		t.Fatalf("GitHub outputs:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteGitHubOutputRejectsEmptyAndMultilineValues(t *testing.T) {
	for _, value := range []string{"", "godot_modules/spx\nmodule_path=other"} {
		t.Run(value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "output")
			err := writeGitHubOutput(path, []githubOutput{{name: "module_path", value: value}})
			if err == nil {
				t.Fatalf("writeGitHubOutput accepted %q", value)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("output was created after validation failure: %v", statErr)
			}
		})
	}
}
