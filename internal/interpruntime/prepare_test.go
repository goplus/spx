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
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goplus/spx/v3/internal/scaffold"
)

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func TestPrepareSession(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	sessionDir := filepath.Join(t.TempDir(), "session")
	bridgePath := filepath.Join(t.TempDir(), "gdspx-test.bridge")
	if err := os.WriteFile(bridgePath, []byte("bridge"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir}

	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareSession(SessionConfig{Roots: roots, BridgePath: bridgePath}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("working directory changed from %q to %q", before, after)
	}

	files := map[string]string{
		filepath.Join(sessionDir, bridgeDirectory, filepath.Base(bridgePath)): "bridge",
		filepath.Join(sessionDir, runtimeExtensionFile):                       scaffold.SessionRuntimeGDExtension(),
		filepath.Join(sessionDir, projectExtensionFile):                       scaffold.ProjectGDExtension(),
		filepath.Join(sessionDir, ".godot", extensionListFile):                scaffold.SessionExtensionList(),
	}
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
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
	cmd, err := PrepareCommand(context.Background(), CommandConfig{
		Roots:      roots,
		Executable: executable,
		Args:       []string{"--headless", "--path=/ignored", "--path", "/also-ignored"},
		Env:        []string{"KEEP=value", ProjectDirEnv + "=/ignored"},
		PathPolicy: ReplacePath,
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

func TestPrepareSessionRejectsSymlinkScaffoldTarget(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	bridgePath := filepath.Join(t.TempDir(), "gdspx-test.bridge")
	if err := os.WriteFile(bridgePath, []byte("bridge"), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(sessionDir, runtimeExtensionFile)); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	err := PrepareSession(SessionConfig{
		Roots: Roots{
			ProjectDir: projectDir,
			AssetDir:   assetDir,
			SessionDir: sessionDir,
		},
		BridgePath: bridgePath,
	})
	if err == nil {
		t.Fatal("PrepareSession accepted symlink scaffold target")
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "unchanged" {
		t.Fatalf("symlink target was overwritten: %q", got)
	}
}

func TestPrepareSessionReplacesExistingRegularScaffold(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	bridgePath := filepath.Join(t.TempDir(), "gdspx-test.bridge")
	if err := os.WriteFile(bridgePath, []byte("bridge-v2"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{runtimeExtensionFile, projectExtensionFile} {
		if err := os.WriteFile(filepath.Join(sessionDir, name), []byte("stale"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(sessionDir, bridgeDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, bridgeDirectory, filepath.Base(bridgePath)), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(sessionDir, ".godot"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, ".godot", extensionListFile), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := PrepareSession(SessionConfig{
		Roots:      Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir},
		BridgePath: bridgePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(sessionDir, bridgeDirectory, filepath.Base(bridgePath))); err != nil || string(got) != "bridge-v2" {
		t.Fatalf("replaced bridge = %q, err=%v", got, err)
	}
}

func TestPrepareSessionRejectsSymlinkProjectDataDirectory(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(sessionDir, ".godot")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	bridgePath := filepath.Join(t.TempDir(), "gdspx-test.bridge")
	if err := os.WriteFile(bridgePath, []byte("bridge"), 0o700); err != nil {
		t.Fatal(err)
	}

	err := PrepareSession(SessionConfig{
		Roots:      Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir},
		BridgePath: bridgePath,
	})
	if err == nil {
		t.Fatal("PrepareSession accepted a symlinked .godot directory")
	}
	if entries, readErr := os.ReadDir(outside); readErr != nil {
		t.Fatal(readErr)
	} else if len(entries) != 0 {
		t.Fatalf("symlink target was modified: %#v", entries)
	}
}

func TestPrepareSessionRejectsSymlinkBridgeSource(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	realBridge := filepath.Join(t.TempDir(), "bridge-real")
	if err := os.WriteFile(realBridge, []byte("bridge"), 0o700); err != nil {
		t.Fatal(err)
	}
	bridgeLink := filepath.Join(t.TempDir(), "bridge-link")
	if err := os.Symlink(realBridge, bridgeLink); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	err := PrepareSession(SessionConfig{
		Roots:      Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: filepath.Join(t.TempDir(), "session")},
		BridgePath: bridgeLink,
	})
	if err == nil {
		t.Fatal("PrepareSession accepted a symlink bridge source")
	}
}

func TestPrepareSessionRejectsBridgeSourceAsDestination(t *testing.T) {
	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	if err := mkdirAll(assetDir); err != nil {
		t.Fatal(err)
	}
	sessionDir := t.TempDir()
	bridgePath := filepath.Join(sessionDir, "bridge")
	if err := os.WriteFile(bridgePath, []byte("unchanged"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := PrepareSession(SessionConfig{
		Roots:      Roots{ProjectDir: projectDir, AssetDir: assetDir, SessionDir: sessionDir},
		BridgePath: bridgePath,
	})
	if err == nil {
		t.Fatal("PrepareSession accepted BridgePath as its own destination")
	}
	if got, readErr := os.ReadFile(bridgePath); readErr != nil || string(got) != "unchanged" {
		t.Fatalf("bridge source changed: %q, err=%v", got, readErr)
	}
}
