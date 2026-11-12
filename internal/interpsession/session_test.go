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

package interpsession

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/scaffold"
)

func TestPrepareSession(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			cfg := testSessionConfig(t)
			files := map[string]string{
				filepath.Join(bridgeDirectory, filepath.Base(cfg.BridgePath)): "bridge",
				runtimeExtensionFile:                       scaffold.SessionRuntimeGDExtension(),
				projectExtensionFile:                       scaffold.ProjectGDExtension(),
				filepath.Join(".godot", extensionListFile): scaffold.SessionExtensionList(),
			}
			if existing {
				for name := range files {
					path := filepath.Join(cfg.Roots.SessionDir, name)
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			} else if err := os.Remove(cfg.Roots.SessionDir); err != nil {
				t.Fatal(err)
			}
			before, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := PrepareSession(cfg); err != nil {
				t.Fatal(err)
			}
			if after, err := os.Getwd(); err != nil || after != before {
				t.Fatalf("working directory = %q, %v; want %q", after, err, before)
			}
			for name, want := range files {
				path := filepath.Join(cfg.Roots.SessionDir, name)
				if got, err := os.ReadFile(path); err != nil || string(got) != want {
					t.Errorf("%s = %q, %v; want %q", name, got, err, want)
				}
				if runtime.GOOS != "windows" {
					mode := os.FileMode(0o600)
					if filepath.Dir(name) == bridgeDirectory {
						mode = 0o700
					}
					if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
						t.Errorf("%s permissions = %v, %v; want %o", name, info, err, mode)
					}
				}
			}
			if got, err := os.ReadFile(cfg.BridgePath); err != nil || string(got) != "bridge" {
				t.Fatalf("bridge source changed: %q, %v", got, err)
			}
		})
	}
}

func TestPrepareSessionRejectsInvalidInputs(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*SessionConfig)
		want   string
	}{
		{"empty project", func(c *SessionConfig) { c.Roots.ProjectDir = "" }, "ProjectDir is empty"},
		{"relative assets", func(c *SessionConfig) { c.Roots.AssetDir = "assets" }, "not absolute"},
		{"unclean session", func(c *SessionConfig) { c.Roots.SessionDir += string(filepath.Separator) + "." }, "not clean"},
		{"missing project", func(c *SessionConfig) { c.Roots.ProjectDir = filepath.Join(c.Roots.ProjectDir, "missing") }, "stat ProjectDir"},
		{"missing assets", func(c *SessionConfig) { c.Roots.AssetDir = filepath.Join(c.Roots.ProjectDir, "missing") }, "stat AssetDir"},
		{"file project", func(c *SessionConfig) { c.Roots.ProjectDir = c.BridgePath }, "not a directory"},
		{"outside assets", func(c *SessionConfig) { c.Roots.AssetDir = filepath.Dir(c.BridgePath) }, "within ProjectDir"},
		{"file session", func(c *SessionConfig) { c.Roots.SessionDir = c.BridgePath }, "not a directory"},
		{"relative bridge", func(c *SessionConfig) { c.BridgePath = "bridge.so" }, "not absolute"},
		{"missing bridge", func(c *SessionConfig) { c.BridgePath += ".missing" }, "lstat BridgePath"},
		{"directory bridge", func(c *SessionConfig) { c.BridgePath = filepath.Dir(c.BridgePath) }, "not a regular"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testSessionConfig(t)
			original := cfg
			tt.change(&cfg)
			if err := PrepareSession(cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PrepareSession = %v; want %q", err, tt.want)
			}
			if files, err := os.ReadDir(original.Roots.SessionDir); err != nil || len(files) != 0 {
				t.Fatalf("invalid input wrote scaffold: %v, %v", files, err)
			}
			if got, err := os.ReadFile(original.BridgePath); err != nil || string(got) != "bridge" {
				t.Fatalf("bridge source changed: %q, %v", got, err)
			}
		})
	}
}

func TestPrepareSessionRejectsSymlinkPaths(t *testing.T) {
	for _, tt := range []struct {
		name      string
		directory bool
	}{
		{"session", true},
		{"bridge source", false},
		{bridgeDirectory, true},
		{filepath.Join(bridgeDirectory, "bridge.so"), false},
		{runtimeExtensionFile, false},
		{projectExtensionFile, false},
		{".godot", true},
		{filepath.Join(".godot", extensionListFile), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testSessionConfig(t)
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(cfg.Roots.SessionDir, tt.name)
			switch tt.name {
			case "session":
				link = cfg.Roots.SessionDir
			case "bridge source":
				link = cfg.BridgePath
			}
			if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			target := sentinel
			if tt.directory {
				target = outside
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink not supported: %v", err)
			}
			if err := PrepareSession(cfg); err == nil {
				t.Fatal("PrepareSession accepted symlink path")
			}
			if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("symlink was replaced: %v, %v", info, err)
			}
			if got, err := os.ReadFile(sentinel); err != nil || string(got) != "unchanged" {
				t.Fatalf("symlink target was overwritten: %q, %v", got, err)
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
				t.Fatalf("symlink target was modified: %v, %v", entries, err)
			}
		})
	}
}

func TestPrepareSessionRejectsUnsafeBridgeLocation(t *testing.T) {
	for _, name := range []string{runtimeExtensionFile, ".godot", "within session"} {
		t.Run(name, func(t *testing.T) {
			cfg := testSessionConfig(t)
			path := filepath.Join(filepath.Dir(cfg.BridgePath), name)
			want := "unsafe bridge base name"
			if name == "within session" {
				path = filepath.Join(cfg.Roots.SessionDir, "bridge.so")
				want = "BridgePath must be outside SessionDir"
			}
			if err := os.Rename(cfg.BridgePath, path); err != nil {
				t.Fatal(err)
			}
			cfg.BridgePath = path
			if err := PrepareSession(cfg); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("PrepareSession = %v; want %q", err, want)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "bridge" {
				t.Fatalf("bridge source changed: %q, %v", got, err)
			}
			files, err := os.ReadDir(cfg.Roots.SessionDir)
			wantCount := 0
			if name == "within session" {
				wantCount = 1
			}
			if err != nil || len(files) != wantCount {
				t.Fatalf("invalid bridge wrote scaffold: %v, %v", files, err)
			}
		})
	}
}

func TestPrepareSessionRejectsBlockedScaffoldPaths(t *testing.T) {
	for _, blocked := range []string{"lib", "lib/bridge.so", "runtime.gdextension", "gdspx.gdextension", ".godot/extension_list.cfg"} {
		t.Run(blocked, func(t *testing.T) {
			cfg := testSessionConfig(t)
			target := filepath.Join(cfg.Roots.SessionDir, filepath.FromSlash(blocked))
			if blocked == "lib" {
				if err := os.WriteFile(target, []byte("preserved"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := PrepareSession(cfg)
			if err == nil {
				t.Fatal("accepted blocked scaffold destination")
			}
			if blocked == "lib" {
				if data, err := os.ReadFile(target); err != nil || string(data) != "preserved" {
					t.Fatalf("conflicting file changed: %q,%v", data, err)
				}
			} else {
				if info, err := os.Stat(target); err != nil || !info.IsDir() {
					t.Fatalf("conflicting directory replaced: %v,%v", info, err)
				}
			}
			if err := filepath.WalkDir(cfg.Roots.SessionDir, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(entry.Name(), ".tmp-") {
					t.Errorf("temporary scaffold leaked: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrepareSessionPermissionFailuresPreserveExistingFiles(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions and an unprivileged user")
	}
	for _, tt := range []struct {
		name       string
		restricted string
		mode       os.FileMode
		preserved  string
		want       string
	}{
		{name: "unreadable bridge", restricted: "source", mode: 0, want: "open BridgePath"},
		{name: "unreadable session", restricted: ".", mode: 0, preserved: "keep.txt", want: "open SessionDir root"},
		{name: "read-only session", restricted: ".", mode: 0o500, preserved: "keep.txt", want: "create session directory"},
		{name: "unreadable bridge directory", restricted: bridgeDirectory, mode: 0, preserved: "lib/bridge.so", want: "open session directory"},
		{name: "read-only bridge directory", restricted: bridgeDirectory, mode: 0o500, preserved: "lib/bridge.so", want: "create temporary scaffold"},
		{name: "unreadable project data", restricted: ".godot", mode: 0, preserved: ".godot/extension_list.cfg", want: "open session directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testSessionConfig(t)
			preserved := cfg.BridgePath
			wantContent := "bridge"
			if tt.preserved != "" {
				preserved = filepath.Join(cfg.Roots.SessionDir, filepath.FromSlash(tt.preserved))
				wantContent = "existing session file"
				if err := os.MkdirAll(filepath.Dir(preserved), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(preserved, []byte(wantContent), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			restricted := filepath.Join(cfg.Roots.SessionDir, tt.restricted)
			restoreMode := os.FileMode(0o700)
			if tt.restricted == "source" {
				restricted = cfg.BridgePath
				restoreMode = 0o600
			}
			if err := os.Chmod(restricted, tt.mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(restricted, restoreMode) })
			err := PrepareSession(cfg)
			if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PrepareSession = %v; want %q wrapping permission failure", err, tt.want)
			}
			if err := os.Chmod(restricted, restoreMode); err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(preserved); err != nil || string(data) != wantContent {
				t.Fatalf("existing file changed after failure: %q, %v", data, err)
			}
			if err := filepath.WalkDir(cfg.Roots.SessionDir, func(path string, entry os.DirEntry, err error) error {
				if err == nil && strings.Contains(entry.Name(), ".tmp-") {
					t.Errorf("temporary scaffold leaked: %s", path)
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func testSessionConfig(t *testing.T) SessionConfig {
	t.Helper()
	bridge := filepath.Join(t.TempDir(), "bridge.so")
	if err := os.WriteFile(bridge, []byte("bridge"), 0o600); err != nil {
		t.Fatal(err)
	}
	return SessionConfig{Roots: testRoots(t), BridgePath: bridge}
}
