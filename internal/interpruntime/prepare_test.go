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

package interpruntime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/goplus/spx/v4/internal/scaffold"
)

func testSessionConfig(t *testing.T) SessionConfig {
	t.Helper()
	bridge := filepath.Join(t.TempDir(), "bridge.so")
	if err := os.WriteFile(bridge, []byte("bridge"), 0o600); err != nil {
		t.Fatal(err)
	}
	return SessionConfig{Roots: testRoots(t), BridgePath: bridge}
}

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

func TestPrepareCommandUsesExplicitRootsWithoutGlobalMutation(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	beforeEnv := os.Environ()
	stdin := strings.NewReader("input")
	var stdout, stderr bytes.Buffer
	cmd, err := PrepareCommand(context.Background(), CommandConfig{
		Roots:      roots,
		Executable: executable,
		Args:       []string{"--headless", "--path=/ignored", "--path", "/also-ignored"},
		Env:        []string{"KEEP=value", ProjectDirEnv + "=/ignored"},
		PathPolicy: ReplacePath,
		Stdin:      stdin,
		Stdout:     &stdout,
		Stderr:     &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("working directory changed from %q to %q", before, after)
	}
	if afterEnv := os.Environ(); !reflect.DeepEqual(afterEnv, beforeEnv) {
		t.Fatal("process environment changed")
	}
	if cmd.Stdin != stdin || cmd.Stdout != &stdout || cmd.Stderr != &stderr {
		t.Fatal("command did not retain the configured input and output streams")
	}
	if cmd.Dir != roots.SessionDir {
		t.Fatalf("command Dir = %q, want %q", cmd.Dir, roots.SessionDir)
	}
	wantArgs := []string{executable, "--headless", "--path", roots.SessionDir, "--no-header"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("command Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	wantEnv := []string{
		"KEEP=value",
		ProjectDirEnv + "=" + roots.ProjectDir,
		AssetDirEnv + "=" + roots.AssetDir,
		SessionDirEnv + "=" + roots.SessionDir,
	}
	if !reflect.DeepEqual(cmd.Env, wantEnv) {
		t.Fatalf("command Env = %#v, want %#v", cmd.Env, wantEnv)
	}
}

func TestPrepareCommandRejectsEnginePathByDefault(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--path", "/ignored"}, {"--path=/ignored"}} {
		if _, err := PrepareCommand(context.Background(), CommandConfig{
			Roots:      roots,
			Executable: executable,
			Args:       args,
		}); err == nil {
			t.Fatalf("PrepareCommand(%q) accepted reserved --path", args)
		}
	}
}

func TestPrepareCommandRejectsInvalidInputs(t *testing.T) {
	roots := testRoots(t)
	executable := filepath.Join(t.TempDir(), "engine")
	if err := os.WriteFile(executable, []byte("engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		ctx          context.Context
		executable   string
		policy       PathPolicy
		invalidRoots bool
		want         string
	}{
		{name: "nil context", executable: executable, ctx: nil, want: "nil context"},
		{name: "invalid roots", executable: executable, ctx: context.Background(), invalidRoots: true, want: "ProjectDir is empty"},
		{name: "missing executable", executable: filepath.Join(t.TempDir(), "missing"), ctx: context.Background(), want: "lstat Executable"},
		{name: "directory executable", executable: t.TempDir(), ctx: context.Background(), want: "not a regular"},
		{name: "unknown path policy", executable: executable, ctx: context.Background(), policy: PathPolicy(99), want: "unknown Engine path policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := roots
			if tt.invalidRoots {
				input = Roots{}
			}
			cmd, err := PrepareCommand(tt.ctx, CommandConfig{
				Roots: input, Executable: tt.executable, PathPolicy: tt.policy,
			})
			if cmd != nil || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PrepareCommand() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEngineArgsReplacesTrailingPathOption(t *testing.T) {
	args, err := engineArgs([]string{"--headless", "--path"}, "/session", ReplacePath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--headless", "--path", "/session", "--no-header"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("engineArgs() = %#v, want %#v", args, want)
	}
	if _, err := engineArgs(nil, "/session", PathPolicy(99)); err == nil || !strings.Contains(err.Error(), "unknown Engine path policy") {
		t.Fatalf("unknown path policy error = %v", err)
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

func TestReplaceRootFileFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(dir, "asset"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("source interrupted")
	if err := replaceRootFile(root, "asset", io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(failure)), 0600); !errors.Is(err, failure) {
		t.Fatalf("write failure=%v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "asset")); err != nil || string(data) != "old" {
		t.Fatalf("destination=%q,%v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v,%v", entries, err)
	}
	for _, name := range []string{"", ".", "../escape", "nested/file"} {
		if err := replaceRootFile(root, name, strings.NewReader("bad"), 0600); err == nil {
			t.Errorf("accepted unsafe name%q", name)
		}
	}
	if err := root.Mkdir("directory", 0700); err != nil {
		t.Fatal(err)
	}
	if err := replaceRootFile(root, "directory", strings.NewReader("bad"), 0600); err == nil {
		t.Fatal("replaced directory")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := replaceRootFile(root, "asset", strings.NewReader("bad"), 0600); err == nil {
		t.Fatal("closed root accepted")
	}
}

func TestPinnedDirectoryRejectsInvalidAndChangedRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(dir, "missing")} {
		if root, err := openPinnedRoot("root", path); err == nil {
			root.Close()
			t.Errorf("accepted non-directory %s", path)
		}
	}
	if err := ensureDirectoryNoSymlink("root", file, 0700); err == nil {
		t.Fatal("accepted regular file as directory")
	}
	if err := ensureDirectoryNoSymlink("root", filepath.Join(file, "child"), 0700); err == nil {
		t.Fatal("created directory below file")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, name := range []string{"file", "file/child", "missing/child", "../escape"} {
		if sub, err := ensurePinnedSubdirectory(root, name, 0700); err == nil {
			sub.Close()
			t.Errorf("accepted invalid subdirectory%q", name)
		}
	}
	sub, err := ensurePinnedSubdirectory(root, "valid", 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if err := verifyPinnedSubdirectory(root, "file", sub); err == nil {
		t.Fatal("accepted replaced subdirectory identity")
	}
	if err := verifyPinnedRootPath("root", file, root); err == nil {
		t.Fatal("accepted replaced root identity")
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedSubdirectory(root, "valid", sub); err == nil {
		t.Fatal("accepted closed subdirectory")
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedRootPath("root", dir, root); err == nil {
		t.Fatal("accepted closed root")
	}
	if sub, err := ensurePinnedSubdirectory(root, "valid", 0700); err == nil {
		sub.Close()
		t.Fatal("accepted closed parent")
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

func TestReplaceRootFileTemporaryNameCollisions(t *testing.T) {
	// Keep this serial with scaffold tests: collision names use a shared sequence.
	for _, collisions := range []int{1, 100} {
		t.Run(fmt.Sprintf("collisions=%d", collisions), func(t *testing.T) {
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			const name = "asset"
			if err := os.WriteFile(filepath.Join(dir, name), []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			// Reserve the next names without changing the sequence: an existing
			// temporary file must never be overwritten or removed by a retry.
			sequence := scaffoldTempSequence.Load()
			reserved := make([]string, collisions)
			for i := range reserved {
				reserved[i] = fmt.Sprintf(".%s.tmp-%d-%d", name, os.Getpid(), sequence+uint64(i)+1)
				if err := os.WriteFile(filepath.Join(dir, reserved[i]), []byte("reserved"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err = replaceRootFile(root, name, strings.NewReader("new"), 0o600)
			want := "new"
			if collisions == 100 {
				want = "old"
				if err == nil || !strings.Contains(err.Error(), "could not allocate temporary scaffold") {
					t.Fatalf("exhausted retry error = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != want {
				t.Fatalf("destination = %q, %v; want %q", data, err, want)
			}
			for _, reservedName := range reserved {
				if data, err := os.ReadFile(filepath.Join(dir, reservedName)); err != nil || string(data) != "reserved" {
					t.Errorf("reserved file %q = %q, %v", reservedName, data, err)
				}
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != collisions+1 {
				t.Fatalf("unexpected temporary files: %v, %v", entries, err)
			}
		})
	}
}
