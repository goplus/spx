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

package shared

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestEnsureEngineSourceChecksOutLockedCommit(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("GODOT_SRC", filepath.Join(repoRoot, "godot-source"))
	t.Setenv("SPX_MODULE_SRC", filepath.Join(repoRoot, "godot_modules", "spx"))
	if err := os.MkdirAll(filepath.Join(repoRoot, "godot_modules", "spx"), 0o755); err != nil {
		t.Fatal(err)
	}

	type invocation struct {
		name string
		args []string
	}
	var got []invocation
	err := ensureEngineSource(repoRoot, func(name string, args ...string) error {
		got = append(got, invocation{name: name, args: append([]string(nil), args...)})
		return nil
	})
	if err != nil {
		t.Fatalf("ensureEngineSource returned error: %v", err)
	}

	lock := release.DefaultRuntimeLock()
	if len(got) != 5 || len(got[0].args) != 3 {
		t.Fatalf("git invocations = %#v", got)
	}
	stagingDir := got[0].args[1]
	if filepath.Dir(stagingDir) != repoRoot || filepath.Base(stagingDir) == "godot-source" {
		t.Fatalf("staging directory = %q", stagingDir)
	}
	want := []invocation{
		{name: "git", args: []string{"-C", stagingDir, "init"}},
		{name: "git", args: []string{"-C", stagingDir, "remote", "add", "origin", lock.Godot.Repository}},
		{name: "git", args: []string{"-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", lock.Godot.Ref}},
		{name: "git", args: []string{"-C", stagingDir, "fetch", "--filter=blob:none", "--depth", "1", "origin", lock.Godot.Commit}},
		{name: "git", args: []string{"-C", stagingDir, "checkout", "--detach", lock.Godot.Commit}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("git invocations = %#v, want %#v", got, want)
	}
}

func TestEnsureEngineSourceDeepensLockedRefWhenCommitFetchFails(t *testing.T) {
	repoRoot := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("GODOT_SRC", filepath.Join(repoRoot, "godot-source"))
	t.Setenv("SPX_MODULE_SRC", filepath.Join(repoRoot, "godot_modules", "spx"))
	if err := os.MkdirAll(filepath.Join(repoRoot, "godot_modules", "spx"), 0o755); err != nil {
		t.Fatal(err)
	}

	lock := release.DefaultRuntimeLock()
	var got [][]string
	err := ensureEngineSource(repoRoot, func(_ string, args ...string) error {
		got = append(got, append([]string(nil), args...))
		if len(args) > 0 && args[len(args)-1] == lock.Godot.Commit && slices.Contains(args, "fetch") {
			return errors.New("unadvertised object")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ensureEngineSource returned error: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("git invocation count = %d, want 6: %#v", len(got), got)
	}
	wantFallback := []string{"-C", got[0][1], "fetch", "--filter=blob:none", "--unshallow", "origin", lock.Godot.Ref}
	if !reflect.DeepEqual(got[4], wantFallback) {
		t.Fatalf("fallback invocation = %#v, want %#v", got[4], wantFallback)
	}
}

func TestEnsureEngineSourceCleansFailedClone(t *testing.T) {
	repoRoot := t.TempDir()
	engineDir := filepath.Join(repoRoot, "godot-source")
	t.Setenv("GOPATH", filepath.Join(repoRoot, "gopath"))
	t.Setenv("GODOT_SRC", engineDir)
	t.Setenv("SPX_MODULE_SRC", filepath.Join(repoRoot, "godot_modules", "spx"))
	if err := os.MkdirAll(filepath.Join(repoRoot, "godot_modules", "spx"), 0o755); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("fetch failed")
	err := ensureEngineSource(repoRoot, func(_ string, args ...string) error {
		for _, arg := range args {
			if arg == "fetch" {
				return wantErr
			}
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ensureEngineSource error = %v", err)
	}
	if _, err := os.Stat(engineDir); !os.IsNotExist(err) {
		t.Fatalf("failed clone left target directory: %v", err)
	}
	if matches, err := filepath.Glob(filepath.Join(repoRoot, ".godot-source.clone-*")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("failed clone left staging directories: %v", matches)
	}
}
