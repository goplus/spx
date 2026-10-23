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

package command

import (
	"bytes"
	"errors"
	spxlog "github.com/goplus/spx/v4/internal/log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseIosExportProjectOnly(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "true",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/export_project_only=true
`,
			want: true,
		},
		{
			name: "false",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/export_project_only=false
`,
			want: false,
		},
		{
			name:    "quoted and crlf",
			content: "[preset.0]\r\nname=\"iOS\"\r\n\r\n[preset.0.options]\r\napplication/export_project_only=\"true\"\r\n",
			want:    true,
		},
		{
			name: "spaces around assignment",
			content: `
[preset.0]
name = "iOS"

[preset.0.options]
application/export_project_only = true
`,
			want: true,
		},
		{
			name: "missing key",
			content: `
[preset.0]
name="iOS"

[preset.0.options]
application/icon_interpolation=4
`,
			want: false,
		},
		{
			name: "ignore non-ios preset",
			content: `
[preset.0]
name="Android"

[preset.0.options]
application/export_project_only=true
`,
			want: false,
		},
		{
			name: "use matching ios preset options",
			content: `
[preset.0]
name="Android"

[preset.0.options]
application/export_project_only=true

[preset.1]
name="iOS"

[preset.1.options]
application/export_project_only=false
`,
			want: false,
		},
		{
			name: "ignore matching key outside preset options",
			content: `
[preset.0]
name="iOS"
application/export_project_only=true

[preset.0.options]
application/icon_interpolation=4
`,
			want: false,
		},
		{
			name: "ignore non-preset section",
			content: `
[preset.0]
name="Android"

[other]
application/export_project_only=true

[preset.1]
name="iOS"

[preset.1.options]
application/export_project_only=true
`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseIosExportProjectOnly([]byte(tt.content)); got != tt.want {
				t.Fatalf("parseIosExportProjectOnly() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateIosExportOutput(t *testing.T) {
	for _, projectOnly := range []bool{false, true} {
		mode := "ipa"
		if projectOnly {
			mode = "project"
		}
		for _, state := range []string{"present", "missing", "invalid path"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				dir := t.TempDir()
				output := iosExportOutput{buildDir: dir, ipaPath: filepath.Join(dir, "Game.ipa"), xcodeProjectPath: filepath.Join(dir, "Game.xcodeproj")}
				switch state {
				case "present":
					if projectOnly {
						if err := os.Mkdir(output.xcodeProjectPath, 0o755); err != nil {
							t.Fatal(err)
						}
					} else {
						writeCommandTestFile(t, output.ipaPath, "ipa")
					}
				case "invalid path":
					output.ipaPath, output.xcodeProjectPath = "\x00", "\x00"
				}
				err := validateIosExportOutput(output, projectOnly)
				if state == "present" {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					want := "file not created"
					if state == "invalid path" {
						want = "failed to check"
					}
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("output error = %v, want %q", err, want)
					}
				}
			})
		}
	}
}

func TestIosOutputPathsAndInputs(t *testing.T) {
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	cmd.CmdPath = filepath.Join(cmd.ProjectDir, "godot")
	if err := cmd.validateIosExportInputs(); err == nil || !strings.Contains(err.Error(), "godot binary not found") {
		t.Fatalf("missing binary error = %v", err)
	}
	writeCommandTestFile(t, cmd.CmdPath, "test placeholder")
	if err := cmd.validateIosExportInputs(); err == nil || !strings.Contains(err.Error(), "godot project file not found") {
		t.Fatalf("missing project error = %v", err)
	}
	writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, "project.godot"), "project")
	if err := cmd.validateIosExportInputs(); err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(cmd.ProjectDir, ".builds", "ios")
	for _, projectOnly := range []bool{false, true} {
		got, err := cmd.prepareIosOutput(projectOnly)
		want := iosExportOutput{wantDir, filepath.Join(wantDir, "Game.ipa"), filepath.Join(wantDir, "Game.xcodeproj")}
		if err != nil || got != want {
			t.Fatalf("iOS output = %+v, %v; want %+v", got, err, want)
		}
		if info, err := os.Stat(got.buildDir); err != nil || !info.IsDir() {
			t.Fatalf("iOS output directory missing: %v", err)
		}
	}
	cmd.ProjectDir = t.TempDir()
	writeCommandTestFile(t, filepath.Join(cmd.ProjectDir, ".builds"), "occupied")
	if _, err := cmd.prepareIosOutput(false); err == nil || !strings.Contains(err.Error(), "failed to create build directory") {
		t.Fatalf("blocked output error = %v", err)
	}
}

func TestIosPresetFileAndMalformedSections(t *testing.T) {
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	if got, err := cmd.isIosExportProjectOnly(); err != nil || got {
		t.Fatalf("missing preset = %v, %v", got, err)
	}
	preset := filepath.Join(cmd.ProjectDir, "export_presets.cfg")
	writeCommandTestFile(t, preset, "[preset.0]\nname=\"iOS\"\n[preset.0.options]\napplication/export_project_only=true\n")
	if got, err := cmd.isIosExportProjectOnly(); err != nil || !got {
		t.Fatalf("project-only preset = %v, %v", got, err)
	}
	if err := os.Remove(preset); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(preset, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := cmd.isIosExportProjectOnly(); err == nil || !strings.Contains(err.Error(), "failed to read export presets") {
		t.Fatalf("unreadable preset error = %v", err)
	}
	for _, section := range []string{"preset.0", "[preset.]", "[preset..options]", "[preset.0.extra]", "[other]"} {
		if _, _, ok := parseExportPresetSection(section); ok {
			t.Errorf("malformed preset section accepted: %q", section)
		}
	}
	if _, _, ok := parseExportPresetAssignment("missing assignment"); ok {
		t.Fatal("assignment without '=' was accepted")
	}
}

func TestIosArtifactRenameAndHeaders(t *testing.T) {
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	paths := cmd.iosLibraryPaths()
	if paths.goSrcDir != filepath.Join(cmd.ProjectDir, "go") || paths.xcframeworkPath != filepath.Join(cmd.ProjectDir, "lib", "libgdspx.ios.xcframework") {
		t.Fatalf("iOS library paths = %+v", paths)
	}
	for _, name := range []string{"ios_bridge.m.txt", "ios_keep.m", "other.txt"} {
		writeCommandTestFile(t, filepath.Join(paths.goSrcDir, name), name)
	}
	if err := cmd.renameIosArtifacts(); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(paths.goSrcDir, "ios_bridge.m")); got != "ios_bridge.m.txt" {
		t.Fatalf("restored iOS artifact = %q", got)
	}
	if _, err := os.Stat(filepath.Join(paths.goSrcDir, "ios_bridge.m.txt")); !os.IsNotExist(err) {
		t.Fatalf("renamed artifact retained .txt: %v", err)
	}
	for _, name := range []string{"ios_keep.m", "other.txt"} {
		if got := readTestFile(t, filepath.Join(paths.goSrcDir, name)); got != name {
			t.Fatalf("unrelated artifact changed: %s = %q", name, got)
		}
	}
	writeCommandTestFile(t, filepath.Join(paths.buildDir, "stale"), "old build")
	writeCommandTestFile(t, filepath.Join(paths.xcframeworkPath, "stale"), "old framework")
	if err := prepareIosLibraryDirs(paths); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{filepath.Join(paths.buildDir, "stale"), paths.xcframeworkPath} {
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Fatalf("stale build artifact retained: %s: %v", stale, err)
		}
	}
	for _, dir := range []string{paths.simulatorDir, paths.deviceDir, paths.headersDir, paths.libDir} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("iOS library directory missing: %s: %v", dir, err)
		}
	}
	writeCommandTestFile(t, filepath.Join(paths.goSrcDir, "custom.h"), "custom header")
	if err := cmd.prepareIosHeaders(paths); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(paths.headersDir, "custom.h")); got != "custom header" {
		t.Fatalf("copied header = %q", got)
	}
	if got := readTestFile(t, filepath.Join(paths.headersDir, "libgdspx.h")); !strings.Contains(got, "void GDExtensionInit(") || !strings.Contains(got, "#ifndef LIBGDSPX_H") {
		t.Fatalf("generated extension header = %q", got)
	}
}

func TestIosLibraryPreparationFailures(t *testing.T) {
	cmd := &CmdTool{ProjectDir: t.TempDir()}
	paths := cmd.iosLibraryPaths()
	if err := cmd.prepareIosHeaders(paths); err == nil || !strings.Contains(err.Error(), "failed to create header file") {
		t.Fatalf("missing header directory error = %v", err)
	}
	writeCommandTestFile(t, paths.libDir, "occupied")
	var pathError *os.PathError
	if err := prepareIosLibraryDirs(paths); !errors.As(err, &pathError) || !strings.Contains(err.Error(), paths.libDir) {
		t.Fatalf("blocked library directory error = %v", err)
	}
	cmd.ProjectDir = t.TempDir()
	paths = cmd.iosLibraryPaths()
	if err := prepareIosLibraryDirs(paths); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.goSrcDir, "invalid.h"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmd.prepareIosHeaders(paths); err == nil || !strings.Contains(err.Error(), "failed to copy header file") {
		t.Fatalf("directory header error = %v", err)
	}
	writeCommandTestFile(t, filepath.Join(paths.goSrcDir, "ios_blocked.txt"), "source")
	if err := os.Mkdir(filepath.Join(paths.goSrcDir, "ios_blocked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cmd.renameIosArtifacts(); err == nil || !strings.Contains(err.Error(), "failed to rename") {
		t.Fatalf("blocked artifact rename error = %v", err)
	}
}

func TestIosSDKAndArchiveConfiguration(t *testing.T) {
	for _, path := range []string{"/sdk/with space", "/sdk/with\ttab", "/sdk/with\nnewline"} {
		if err := validateSDKPath("simulator", path); err == nil || !strings.Contains(err.Error(), "simulator SDK path contains whitespace") {
			t.Errorf("invalid SDK path %q: %v", path, err)
		}
	}
	if err := validateSDKPath("device", "/sdk/device"); err != nil {
		t.Fatal(err)
	}
	build := newIosArchiveBuild("device", "device.a", "/sdk/device", "arm64", "-arch arm64", "-mios-version-min=12.0")
	if build.name != "device" || build.outputPath != "device.a" {
		t.Fatalf("archive identity = %+v", build)
	}
	for _, want := range []string{"GOOS=darwin", "GOARCH=arm64", "CGO_ENABLED=1", "CGO_CFLAGS=-isysroot /sdk/device -arch arm64", "CGO_LDFLAGS=-isysroot /sdk/device -mios-version-min=12.0"} {
		if !strings.Contains("\n"+strings.Join(build.env, "\n")+"\n", "\n"+want+"\n") {
			t.Errorf("archive environment missing %q: %v", want, build.env)
		}
	}
	install := false
	cmd := &CmdTool{Args: ExtraArgs{Install: &install}}
	if err := cmd.installIosIPA("unused.ipa", true); err != nil {
		t.Fatal(err)
	}
	install = true
	if err := cmd.installIosIPA("unused.ipa", true); err == nil || !strings.Contains(err.Error(), "cannot install IPA") {
		t.Fatalf("project-only install error = %v", err)
	}
}

func TestIosTemplateDiagnosticsFindOnlyVersionedIOSFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", home)
	var templateDir string
	switch runtime.GOOS {
	case "darwin":
		templateDir = filepath.Join(home, "Library", "Application Support", "Godot", "export_templates")
	case "linux":
		templateDir = filepath.Join(home, ".local", "share", "godot", "export_templates")
	case "windows":
		templateDir = filepath.Join(home, "Godot", "export_templates")
	default:
		t.Skip("no platform export-template location")
	}
	writeCommandTestFile(t, filepath.Join(templateDir, "4.5", "ios.zip"), "template")
	writeCommandTestFile(t, filepath.Join(templateDir, "4.5", "linux.zip"), "other template")
	writeCommandTestFile(t, filepath.Join(templateDir, "README"), "ignored root file")
	var logs bytes.Buffer
	spxlog.SetOutput(&logs)
	t.Cleanup(func() { spxlog.SetStdoutOutput(os.Stdout); spxlog.SetStderrOutput(os.Stderr) })
	(&CmdTool{}).logIosExportTemplates()
	if got := logs.String(); !strings.Contains(got, "Templates for iOS in 4.5: [ios.zip]") || strings.Contains(got, "linux.zip") || strings.Contains(got, "Export template version: README") {
		t.Fatalf("template diagnostics = %q", got)
	}
}

func TestIosSDKLookupRejectsInvalidToolOutput(t *testing.T) {
	for _, sdk := range []string{"iphonesimulator", "iphoneos"} {
		t.Run(sdk, func(t *testing.T) {
			t.Setenv("SPX_TEST_INVALID_SDK", sdk)
			installCommandTestTools(t, map[string]string{"xcrun": `if [ "$2" = "$SPX_TEST_INVALID_SDK" ]; then echo '/invalid sdk'; else echo /valid/sdk; fi`})
			if _, err := lookupIosSDKPaths(); err == nil || !strings.Contains(err.Error(), "whitespace") {
				t.Fatalf("invalid SDK error = %v", err)
			}
		})
	}
}
