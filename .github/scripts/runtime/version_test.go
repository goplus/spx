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
	"strconv"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestDescribeRelease(t *testing.T) {
	description, err := describeRelease()
	if err != nil {
		t.Fatal(err)
	}
	meta := release.DefaultReleaseMeta()
	lock := release.DefaultRuntimeLock()
	if description.ReleaseTag != meta.SPXVersion || description.Version != strings.TrimPrefix(meta.SPXVersion, "v") {
		t.Fatalf("unexpected SPX release: %#v", description)
	}
	if description.RuntimeVersion != lock.RuntimeVersion || description.RuntimeABI != lock.RuntimeABI || description.RuntimeTag != lock.RuntimeReleaseTag() || description.RuntimeManifest != lock.Manifest {
		t.Fatalf("unexpected runtime release: %#v", description)
	}
	if description.ReleaseRepository != lock.ReleaseRepository || description.GodotRepository != "goplus/godot" {
		t.Fatalf("unexpected repositories: %#v", description)
	}
	if description.ModulePath != lock.Module.Path {
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
		"release_tag=" + description.ReleaseTag,
		"version=" + description.Version,
		"runtime_version=" + description.RuntimeVersion,
		"runtime_abi=" + strconv.Itoa(description.RuntimeABI),
		"runtime_tag=" + description.RuntimeTag,
		"runtime_manifest=" + description.RuntimeManifest,
		"release_repository=" + description.ReleaseRepository,
		"godot_repository=" + description.GodotRepository,
		"godot_commit=" + description.GodotCommit,
		"module_path=" + description.ModulePath,
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
