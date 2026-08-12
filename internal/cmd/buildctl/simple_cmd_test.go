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
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/workflow"
	"github.com/goplus/spx/v3/internal/release"
)

func TestParseSetupCommandArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want setupConfig
	}{
		{name: "host", args: []string{"host"}, want: setupConfig{target: "host"}},
		{name: "web default", args: []string{"web"}, want: setupConfig{target: "web", mode: "normal"}},
		{name: "full mode", args: []string{"full", "--mode", "worker"}, want: setupConfig{target: "full", mode: "worker"}},
		{name: "local assets", args: []string{"web", "--asset-dir", "artifacts/runtime"}, want: setupConfig{target: "web", mode: "normal", assetDir: "artifacts/runtime"}},
		{name: "published host", args: []string{"host", "--published-runtime"}, want: setupConfig{target: "host", publishedRuntime: true}},
		{name: "published full", args: []string{"full", "--published-runtime"}, want: setupConfig{target: "full", mode: "normal", publishedRuntime: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseSetupCommandArgs(test.args)
			if err != nil {
				t.Fatalf("parseSetupCommandArgs returned error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("config = %#v, want %#v", got, test.want)
			}
		})
	}

	invalid := [][]string{
		{},
		{"unknown"},
		{"host", "--mode", "worker"},
		{"web", "--mode", "unknown"},
		{"web", "--published-runtime"},
		{"host", "--asset-dir", "artifacts", "--published-runtime"},
		{"full", "extra"},
	}
	for _, args := range invalid {
		if _, err := parseSetupCommandArgs(args); err == nil {
			t.Fatalf("parseSetupCommandArgs(%q) succeeded, want error", args)
		}
	}
}

func TestParseBuildCommandArgs(t *testing.T) {
	tests := []struct {
		args []string
		want workflow.BuildConfig
	}{
		{args: []string{"dev"}, want: workflow.BuildConfig{Target: "dev", Mode: "normal"}},
		{args: []string{"dev", "--mode", "miniprogram"}, want: workflow.BuildConfig{Target: "dev", Mode: "miniprogram"}},
		{args: []string{"web"}, want: workflow.BuildConfig{Target: "web", Mode: "normal"}},
		{args: []string{"web", "--mode", "worker"}, want: workflow.BuildConfig{Target: "web", Mode: "worker"}},
		{args: []string{"editor"}, want: workflow.BuildConfig{Target: "editor"}},
		{args: []string{"desktop"}, want: workflow.BuildConfig{Target: "desktop"}},
		{args: []string{"android"}, want: workflow.BuildConfig{Target: "android"}},
		{args: []string{"ios"}, want: workflow.BuildConfig{Target: "ios"}},
	}
	for _, test := range tests {
		got, err := parseBuildCommandArgs(test.args)
		if err != nil {
			t.Fatalf("parseBuildCommandArgs(%q) returned error: %v", test.args, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("parseBuildCommandArgs(%q) = %#v, want %#v", test.args, got, test.want)
		}
	}

	invalid := [][]string{
		{},
		{"unknown"},
		{"editor", "--mode", "worker"},
		{"desktop", "--mode", "normal"},
		{"android", "--mode", "normal"},
		{"ios", "--mode", "normal"},
		{"web", "--mode", "unknown"},
		{"web", "extra"},
	}
	for _, args := range invalid {
		if _, err := parseBuildCommandArgs(args); err == nil {
			t.Fatalf("parseBuildCommandArgs(%q) succeeded, want error", args)
		}
	}
}

func TestRunDispatchesSimpleCommands(t *testing.T) {
	originalSetup := rootRunSetup
	originalBuild := rootRunBuild
	originalDoctor := rootRunDoctor
	t.Cleanup(func() {
		rootRunSetup = originalSetup
		rootRunBuild = originalBuild
		rootRunDoctor = originalDoctor
	})

	var called string
	var calledArgs []string
	recorder := func(name string) func([]string) error {
		return func(args []string) error {
			called = name
			calledArgs = append([]string(nil), args...)
			return nil
		}
	}
	rootRunSetup = recorder("setup")
	rootRunBuild = recorder("build")
	rootRunDoctor = recorder("doctor")

	for _, test := range []struct {
		args     []string
		wantName string
		wantArgs []string
	}{
		{args: []string{"setup", "web", "--mode", "worker"}, wantName: "setup", wantArgs: []string{"web", "--mode", "worker"}},
		{args: []string{"build", "desktop"}, wantName: "build", wantArgs: []string{"desktop"}},
		{args: []string{"doctor"}, wantName: "doctor", wantArgs: nil},
	} {
		called = ""
		calledArgs = nil
		if err := run(test.args); err != nil {
			t.Fatalf("run(%q) returned error: %v", test.args, err)
		}
		if called != test.wantName || !reflect.DeepEqual(calledArgs, test.wantArgs) {
			t.Fatalf("run(%q) called %q %#v, want %q %#v", test.args, called, calledArgs, test.wantName, test.wantArgs)
		}
	}
}

func TestInspectBuildConfiguration(t *testing.T) {
	repoRoot := t.TempDir()
	lockData, err := release.DefaultRuntimeLock().JSON()
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"), lockData)
	moduleSource := filepath.Join(repoRoot, "godot_modules", "spx")
	mustWriteFile(t, filepath.Join(moduleSource, "SCsub"), []byte("# test\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "config.py"), []byte("# test\n"))
	mustWriteFile(t, filepath.Join(moduleSource, "spx_scons_profile.json"), []byte(`{
  "schema": 1,
  "common": [],
  "editor_release": [],
  "template_release": []
}`))

	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("HOME", repoRoot)
	t.Setenv("APPDATA", filepath.Join(repoRoot, "AppData"))
	t.Setenv("PLATFORM", "")
	t.Setenv("GODOT_SRC", filepath.Join(repoRoot, "missing-godot"))
	t.Setenv("SPX_MODULE_SRC", moduleSource)

	var output bytes.Buffer
	if err := inspectBuildConfiguration(repoRoot, &output); err != nil {
		t.Fatalf("inspectBuildConfiguration returned error: %v", err)
	}
	for _, text := range []string{
		"Repository: " + repoRoot,
		"Runtime lock: " + filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"),
		"Godot source: " + filepath.Join(repoRoot, "missing-godot") + " (missing;",
		"SCons profile: " + filepath.Join(moduleSource, "spx_scons_profile.json") + " (valid)",
		"Toolchain:",
		"scons=",
		"Status: OK",
	} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("doctor output missing %q:\n%s", text, output.String())
		}
	}
}

func TestInspectBuildConfigurationRejectsStaleEmbeddedLock(t *testing.T) {
	repoRoot := t.TempDir()
	lock := release.DefaultRuntimeLock()
	lock.RuntimeABI++
	lockData, err := lock.JSON()
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(repoRoot, "internal", "release", "runtime.lock.json"), lockData)

	var output bytes.Buffer
	err = inspectBuildConfiguration(repoRoot, &output)
	if err == nil || !strings.Contains(err.Error(), "rebuild buildctl") {
		t.Fatalf("inspectBuildConfiguration error = %v, want stale buildctl error", err)
	}
}
