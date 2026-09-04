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

package xgodriver

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/goplus/spx/v3/internal/driverbundle"
	"github.com/goplus/spx/v3/internal/projectpolicy"
)

func TestLaunchpackConfigMapsRuntimeBridgeAndEnvironment(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "spx")
	cfg := Config{
		ProjectDir: filepath.Join(root, "project"), ProjectFile: filepath.Join(root, "project", "main.spx"),
		Project: ProjectMetadata{Extension: ".spx", FullExtension: "main.spx", PackDirectory: "assets", PackIndexFile: "index.json"},
		DriverOrigin: ModuleOrigin{
			Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.0"},
			Replace:  &ModuleRef{Path: selected, Dir: selected},
		},
		GoCommand: "/bin/go", GraphWorkDir: root, GoWork: "off", BuildFlags: []string{"-v=true"},
	}
	env := []string{"PATH=/bin", "SPX_RUNTIME_CACHE=/tmp/cache"}
	got := cfg.launchpackConfig(projectpolicy.PortableConfigSnapshot{}, IO{Env: env})
	if got.RuntimeSourceRoot != selected || got.BridgePackage != driverbundle.SPXModulePath+"/cmd/ispxnative" {
		t.Fatalf("source inputs = %q, %q", got.RuntimeSourceRoot, got.BridgePackage)
	}
	if got.VerifyBridge == nil {
		t.Fatal("source bridge verifier is nil")
	}
	if got.Source.EffectivePath != selected || got.Source.EffectiveVersion != "" || !got.Source.SourceMode {
		t.Fatalf("Source = %#v", got.Source)
	}
	if len(got.IO.Env) != 2 || got.IO.Env[1] != env[1] {
		t.Fatalf("IO.Env = %#v, want %#v", got.IO.Env, env)
	}
	if got.RuntimeIdentity.GOOS != runtime.GOOS || got.RuntimeIdentity.GOARCH != runtime.GOARCH {
		t.Fatalf("RuntimeIdentity = %#v", got.RuntimeIdentity)
	}
	if got.ProjectFile != cfg.ProjectFile || got.ProjectExt != cfg.Project.Extension {
		t.Fatalf("project shape = %q/%q", got.ProjectFile, got.ProjectExt)
	}
}

func TestLaunchpackConfigUsesPublishedBundleWithoutSourceInputs(t *testing.T) {
	root := t.TempDir()
	cfg := Config{
		ProjectDir: filepath.Join(root, "project"),
		Project:    ProjectMetadata{Extension: ".spx", FullExtension: "main.spx", PackDirectory: "assets", PackIndexFile: "index.json"},
		DriverOrigin: ModuleOrigin{Selected: ModuleRef{
			Path: driverbundle.SPXModulePath, Version: "v3.2.4", Dir: filepath.Join(root, "module-cache"),
		}},
	}
	got := cfg.launchpackConfig(projectpolicy.PortableConfigSnapshot{}, IO{})
	if got.Source.SourceMode || got.Source.SelectedVersion != "v3.2.4" {
		t.Fatalf("Source = %#v", got.Source)
	}
	if got.RuntimeSourceRoot != "" || got.BridgePackage != "" || got.VerifyBridge != nil {
		t.Fatalf("published source inputs = %q, %q, verifier %v", got.RuntimeSourceRoot, got.BridgePackage, got.VerifyBridge != nil)
	}
}

func TestValidateSPXRequest(t *testing.T) {
	valid := Config{
		ProjectFile: filepath.Join(t.TempDir(), "main.spx"),
		Project: ProjectMetadata{
			Extension: ".spx", FullExtension: "main.spx",
			PackDirectory: "assets", PackIndexFile: "index.json",
		},
	}
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"extension", func(cfg *Config) { cfg.Project.Extension = ".foo" }},
		{"full extension", func(cfg *Config) { cfg.Project.FullExtension = "game.spx" }},
		{"project file", func(cfg *Config) { cfg.ProjectFile = filepath.Join(filepath.Dir(cfg.ProjectFile), "game.spx") }},
		{"pack directory", func(cfg *Config) { cfg.Project.PackDirectory = "" }},
		{"pack index", func(cfg *Config) { cfg.Project.PackIndexFile = "" }},
		{"pack root", func(cfg *Config) { cfg.Project.PackDirectory = "." }},
	}
	if err := validateSPXRequest(valid); err != nil {
		t.Fatalf("valid SPX request was rejected: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.mutate(&cfg)
			if err := validateSPXRequest(cfg); err == nil {
				t.Fatal("invalid SPX request was accepted")
			}
		})
	}
}

func TestValidateDriverOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin ModuleOrigin
		ok     bool
	}{
		{"main module", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath}, Main: true}, true},
		{"local replace", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.4"}, Replace: &ModuleRef{Path: "/tmp/spx", Dir: "/tmp/spx"}}, true},
		{"released dependency", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.4"}}, true},
		{"released prerelease", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.3.0-rc.1"}}, true},
		{"pseudo version", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.5-0.20260821120000-0123456789ab"}}, false},
		{"versioned replace", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.4"}, Replace: &ModuleRef{Path: driverbundle.SPXModulePath, Version: "v3.2.3"}}, false},
		{"foreign module", ModuleOrigin{Selected: ModuleRef{Path: "example.com/spx", Version: "v3.2.4"}}, false},
		{"foreign main module", ModuleOrigin{Selected: ModuleRef{Path: "example.com/spx"}, Main: true}, false},
		{"noncanonical version", ModuleOrigin{Selected: ModuleRef{Path: driverbundle.SPXModulePath, Version: "3.2.4"}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDriverOrigin(test.origin)
			if (err == nil) != test.ok {
				t.Fatalf("validateDriverOrigin() error = %v, want success %t", err, test.ok)
			}
		})
	}
}
