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

package launchpack

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/portableconfig"
	"github.com/goplus/spx/v4/internal/release"
)

func TestValidateGraphFlags(t *testing.T) {
	if err := validateGraphFlags([]string{"-trimpath", "-mod=readonly", "-buildvcs=false"}); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"trimpath"}, {"-overlay=/tmp/overlay.json"}, {"-modfile"}} {
		if err := validateGraphFlags(flags); err == nil {
			t.Fatalf("validateGraphFlags(%q) succeeded", flags)
		}
	}
}

func TestValidateBuildFlags(t *testing.T) {
	if err := validateBuildFlags([]string{"-v", "-trimpath=true", "-buildvcs=auto"}); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{{"-tags=custom"}, {"-v=maybe"}, {"-buildvcs"}} {
		if err := validateBuildFlags(flags); err == nil {
			t.Fatalf("validateBuildFlags(%q) succeeded", flags)
		}
	}
}

func TestValidatePackPath(t *testing.T) {
	for _, test := range []struct {
		name      string
		value     string
		directory bool
		wantErr   bool
	}{
		{name: "nested directory", value: "assets/images", directory: true},
		{name: "plain index", value: "index.json"},
		{name: "backslash", value: `assets\images`, directory: true, wantErr: true},
		{name: "absolute", value: "/tmp/assets", directory: true, wantErr: true},
		{name: "windows absolute", value: "C:/assets", directory: true, wantErr: true},
		{name: "parent escape", value: "../assets", directory: true, wantErr: true},
		{name: "dot component", value: "assets/./images", directory: true, wantErr: true},
		{name: "parent component", value: "assets/../images", directory: true, wantErr: true},
		{name: "nested index", value: "indexes/index.json", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validatePackPath("pack path", test.value, test.directory)
			if (err != nil) != test.wantErr {
				t.Fatalf("validatePackPath(%q, directory=%v) error = %v, wantErr=%v", test.value, test.directory, err, test.wantErr)
			}
		})
	}
}

func TestLauncherConfigurationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*testing.T, *Config)
	}{
		{"uninitialized snapshot", "portable config", func(t *testing.T, c *Config) {
			c.PortableConfig = portableconfig.PortableConfigSnapshot{}
			// Acquiring the runtime first would fail on the missing manifest.
			c.RuntimeManifestPath = filepath.Join(c.WorkDir, "missing.json")
		}},
		{"missing project", "project-dir is required", func(t *testing.T, c *Config) { c.ProjectDir = "" }},
		{"missing source", "project-file is required", func(t *testing.T, c *Config) { c.ProjectFile = "" }},
		{"missing output", "output is required", func(t *testing.T, c *Config) { c.Output = "" }},
		{"missing project directory", "project-dir cannot be inspected", func(t *testing.T, c *Config) { c.ProjectDir = filepath.Join(c.WorkDir, "missing") }},
		{"directory source", "project-file has an invalid file type", func(t *testing.T, c *Config) { c.ProjectFile = c.ProjectDir }},
		{"source outside project", "project-file must be within", func(t *testing.T, c *Config) {
			c.ProjectFile = filepath.Join(c.WorkDir, "outside.spx")
			writeProjectTestFile(t, c.ProjectFile, "source")
		}},
		{"missing go", "Go command and work directory", func(t *testing.T, c *Config) { c.GoCommand = "" }},
		{"missing work directory", "work-dir cannot be inspected", func(t *testing.T, c *Config) { c.WorkDir = filepath.Join(c.WorkDir, "missing") }},
		{"missing executable", "go-command cannot be inspected", func(t *testing.T, c *Config) { c.GoCommand = filepath.Join(c.WorkDir, "missing") }},
		{"missing workspace", "go-work cannot be inspected", func(t *testing.T, c *Config) { c.GoWork = filepath.Join(c.WorkDir, "missing.work") }},
		{"overlay", "graph overlay is not supported", func(t *testing.T, c *Config) { c.GraphFlags = []string{"-overlay=bad"} }},
		{"invalid build flag", "invalid build flag", func(t *testing.T, c *Config) { c.BuildFlags = []string{"trimpath"} }},
		{"empty pack directory", "pack directory and index", func(t *testing.T, c *Config) { c.PackDir = "" }},
		{"escaping pack", "invalid path component", func(t *testing.T, c *Config) { c.PackDir = "../assets" }},
		{"nested index", "plain file name", func(t *testing.T, c *Config) { c.PackIndex = "nested/index.json" }},
		{"relative output", "output must be an absolute clean path", func(t *testing.T, c *Config) { c.Output = "launcher" }},
		{"output in pack", "output must not be inside", func(t *testing.T, c *Config) { c.Output = filepath.Join(c.ProjectDir, "assets", "launcher") }},
		{"output is project", "output must not be inside", func(t *testing.T, c *Config) { c.Output = c.ProjectDir }},
		{"missing bridge", "bridge package is required", func(t *testing.T, c *Config) { c.BridgePackage = "" }},
		{"missing source root", "runtime-source-root cannot be inspected", func(t *testing.T, c *Config) { c.RuntimeSourceRoot = filepath.Join(c.WorkDir, "absent") }},
		{"nonexecutable go", "go-command is not executable", func(t *testing.T, c *Config) {
			if runtime.GOOS == "windows" {
				t.Skip("Unix executable bits")
			}
			c.GoCommand = filepath.Join(c.WorkDir, "go")
			writeProjectTestFile(t, c.GoCommand, "go")
		}},
		{"symlink executable", "go-command has an invalid file type", func(t *testing.T, c *Config) {
			link := filepath.Join(c.WorkDir, "go")
			if err := os.Symlink(c.GoCommand, link); err != nil {
				t.Skip(err)
			}
			c.GoCommand = link
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			tt.change(t, &cfg)
			if _, err := BuildLauncher(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildLauncher error = %v, want %q", err, tt.want)
			}
		})
	}
	cfg := validLaunchpackConfig(t)
	cfg.RuntimeSourceRoot = cfg.WorkDir
	cfg.GoWork = filepath.Join(cfg.WorkDir, "go.work")
	writeProjectTestFile(t, cfg.GoWork, "go 1.26\n")
	modfile := filepath.Join(cfg.WorkDir, "go.mod")
	writeProjectTestFile(t, modfile, "module fixture\n")
	cfg.GraphFlags = []string{"-modfile=" + modfile}
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	cfg.GraphFlags = []string{"-modfile=" + modfile + ".missing"}
	if err := cfg.validateGraphInputs(); err == nil || !strings.Contains(err.Error(), "graph modfile cannot be inspected") {
		t.Fatalf("missing modfile: %v", err)
	}
	if err := regularPath("unclean", cfg.ProjectDir+string(filepath.Separator)+".", true); err == nil || !strings.Contains(err.Error(), "absolute clean") {
		t.Fatalf("unclean path: %v", err)
	}
}

func TestLauncherPublicEntryPointsRejectNilContext(t *testing.T) {
	//lint:ignore SA1012 Verify the public API rejects a nil context.
	if _, err := BuildLauncher(nil, Config{}); err == nil || !strings.Contains(err.Error(), "nil context") {
		t.Fatalf("BuildLauncher: %v", err)
	}
	//lint:ignore SA1012 Verify the public API rejects a nil context.
	if _, cleanup, err := BuildSourceBridge(nil, Config{}); err == nil || cleanup != nil {
		t.Fatalf("BuildSourceBridge: cleanup nil=%t, err=%v", cleanup == nil, err)
	}
	if _, cleanup, err := BuildSourceBridge(context.Background(), Config{}); err == nil || cleanup != nil {
		t.Fatalf("BuildSourceBridge invalid config: %v", err)
	}
	//lint:ignore SA1012 Verify the public API rejects a nil context.
	if _, err := AcquireRuntimeAssets(nil, Config{}); err == nil || !strings.Contains(err.Error(), "nil context") {
		t.Fatalf("AcquireRuntimeAssets: %v", err)
	}
}

func TestRuntimeLockSelectsDefaultOrExplicitLock(t *testing.T) {
	explicit := release.DefaultRuntimeLock()
	explicit.RuntimeVersion = "9.9.9"
	explicit.RuntimeABI++
	for _, tt := range []struct {
		name string
		cfg  Config
		want release.RuntimeLock
	}{
		{name: "default", want: release.DefaultRuntimeLock()},
		{name: "explicit", cfg: Config{RuntimeLock: explicit}, want: explicit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runtimeLock(tt.cfg)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("runtimeLock = %#v, %v, want %#v", got, err, tt.want)
			}
		})
	}
}

func TestRuntimeAcquisitionRejectsInvalidLockBeforeMaterialization(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		change     func(*Config)
	}{
		{"version", "invalid runtime version", func(c *Config) { c.RuntimeLock.RuntimeVersion = "invalid version" }},
		{"cache", "runtime cache root", func(c *Config) { c.RuntimeCacheRoot = "relative" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := Config{
				RuntimeLock: release.DefaultRuntimeLock(), RuntimeCacheRoot: filepath.Join(root, "cache"),
				RuntimeManifestPath: filepath.Join(root, "unavailable-manifest.json"),
			}
			tt.change(&cfg)
			if _, err := AcquireRuntimeAssets(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AcquireRuntimeAssets error = %v, want %q", err, tt.want)
			}
			if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
				t.Fatalf("invalid configuration created files %v: %v", entries, err)
			}
		})
	}
}

func TestProjectArchiveIncludesSortedSourcesAndPackDir(t *testing.T) {
	cfg := validLaunchpackConfig(t)
	for _, name := range []string{"Hero.spx", "outside.png", "assets/hero.png"} {
		writeProjectTestFile(t, filepath.Join(cfg.ProjectDir, name), name)
	}
	writeProjectTestFile(t, filepath.Join(cfg.ProjectDir, "assets", "index.json"), `{"backdrops":[{"path":"hero.png"},{"path":"./hero.png"}]}`)
	archiveConfig, err := prepareProjectArchiveConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if archiveConfig.IncludeConfig || archiveConfig.ConfigBytes != nil {
		t.Fatal("archive includes absent .config")
	}
	if archiveConfig.PackDir != "assets" || !slices.Equal(archiveConfig.ProjectFiles, []string{"Hero.spx", "main.spx"}) {
		t.Fatalf("archive inputs = %q and %q", archiveConfig.PackDir, archiveConfig.ProjectFiles)
	}
}

func TestProjectArchiveUsesCapturedPortableConfig(t *testing.T) {
	cfg := validLaunchpackConfig(t)
	path := filepath.Join(cfg.ProjectDir, ".config")
	const original = `{"title":"captured"}`
	writeProjectTestFile(t, path, original)
	var err error
	cfg.PortableConfig, err = portableconfig.SnapshotPortableConfig(cfg.ProjectDir)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := prepareProjectArchiveConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	writeProjectTestFile(t, path, "{}")
	if !archive.IncludeConfig || string(archive.ConfigBytes) != original {
		t.Fatalf("archived snapshot = %q, included = %t", archive.ConfigBytes, archive.IncludeConfig)
	}
	if _, err := prepareProjectArchiveConfig(cfg); err == nil {
		t.Fatal("accepted changed config")
	}
}

func TestProjectArchiveRejectsNestedProjectFile(t *testing.T) {
	projectDir := t.TempDir()
	writeProjectTestFile(t, filepath.Join(projectDir, "main.spx"), "main")
	writeProjectTestFile(t, filepath.Join(projectDir, "nested", "main.spx"), "nested")
	cfg := Config{
		ProjectDir: projectDir, ProjectFile: filepath.Join(projectDir, "nested", "main.spx"),
		ProjectExt: ".spx", PackDir: "assets", PackIndex: "index.json",
	}
	if _, err := collectProjectAllowlist(cfg); err == nil {
		t.Fatal("collectProjectAllowlist accepted a nested project file")
	}
}

func TestProjectAllowlistRejectsUnsafeOrIncompleteProjects(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		change     func(*testing.T, *Config)
	}{
		{"missing directory", "read project directory", func(t *testing.T, c *Config) { c.ProjectDir += ".missing" }},
		{"no matching sources", "no top-level", func(t *testing.T, c *Config) { c.ProjectExt = "gmx" }},
		{"entry not allowlisted", "not in the source allowlist", func(t *testing.T, c *Config) { c.ProjectFile = filepath.Join(c.ProjectDir, "other.spx") }},
		{"directory source", "not a regular non-symlink file", func(t *testing.T, c *Config) {
			if err := os.Mkdir(filepath.Join(c.ProjectDir, "folder.spx"), 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink source", "not a regular non-symlink file", func(t *testing.T, c *Config) {
			if err := os.Symlink(c.ProjectFile, filepath.Join(c.ProjectDir, "alias.spx")); err != nil {
				t.Skip(err)
			}
		}},
		{"invalid resources", "validate typed project resources", func(t *testing.T, c *Config) {
			writeProjectTestFile(t, filepath.Join(c.ProjectDir, c.PackDir, c.PackIndex), "{")
		}},
		{"config appeared", "changed after validation", func(t *testing.T, c *Config) { writeProjectTestFile(t, filepath.Join(c.ProjectDir, ".config"), "{}") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			tt.change(t, &cfg)
			if _, err := prepareProjectArchiveConfig(cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("prepareProjectArchiveConfig error = %v, want %q", err, tt.want)
			}
		})
	}
	cfg := validLaunchpackConfig(t)
	cfg.ProjectExt = "spx"
	if files, err := collectProjectAllowlist(cfg); err != nil || !slices.Equal(files, []string{"main.spx"}) {
		t.Fatalf("extension normalization: %q, %v", files, err)
	}
}

func validLaunchpackConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	writeProjectTestFile(t, filepath.Join(project, "main.spx"), "onStart => {}\n")
	writeProjectTestFile(t, filepath.Join(project, "assets", "index.json"), "{}")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := portableconfig.SnapshotPortableConfig(project)
	if err != nil {
		t.Fatal(err)
	}
	return Config{ProjectDir: project, ProjectFile: filepath.Join(project, "main.spx"), ProjectExt: ".spx",
		PackDir: "assets", PackIndex: "index.json", PortableConfig: snapshot,
		GoCommand: executable, WorkDir: root, GoWork: "off", BridgePackage: "./bridge",
		Output: filepath.Join(root, "launcher"), RuntimeLock: release.DefaultRuntimeLock(),
		IO: IO{Env: []string{}},
	}
}

func writeProjectTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
