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
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/goplus/spx/v2/cmd/spx/internal/util"
)

func TestResolveXGoModuleInfoFromBuildDataUsesModuleCache(t *testing.T) {
	modCache := t.TempDir()
	xgoRoot := filepath.Join(modCache, "github.com", "goplus", "xgo@v1.7.1")
	writeTestXGoRoot(t, xgoRoot)

	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{Path: "github.com/goplus/xgo", Version: "v1.7.1"},
		},
	}

	got, err := resolveXGoModuleInfoFromBuildData(info, modCache)
	if err != nil {
		t.Fatalf("resolveXGoModuleInfoFromBuildData returned error: %v", err)
	}
	if got.Root != xgoRoot {
		t.Fatalf("xgo root = %s, want %s", got.Root, xgoRoot)
	}
	if got.Version != "v1.7.1" {
		t.Fatalf("xgo version = %s, want v1.7.1", got.Version)
	}
}

func TestResolveXGoModuleInfoFromBuildDataUsesLocalReplace(t *testing.T) {
	xgoRoot := t.TempDir()
	writeTestXGoRoot(t, xgoRoot)

	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{
				Path:    "github.com/goplus/xgo",
				Version: "v1.7.1",
				Replace: &debug.Module{Path: xgoRoot},
			},
		},
	}

	got, err := resolveXGoModuleInfoFromBuildData(info, "")
	if err != nil {
		t.Fatalf("resolveXGoModuleInfoFromBuildData returned error: %v", err)
	}
	if got.Root != xgoRoot {
		t.Fatalf("xgo root = %s, want %s", got.Root, xgoRoot)
	}
	if got.Version != "v1.7.1" {
		t.Fatalf("xgo version = %s, want v1.7.1", got.Version)
	}
}

func TestResolveGoModCacheDirFallsBackToGoEnv(t *testing.T) {
	oldOutputCommand := outputCommand
	oldGoModCache, hadGoModCache := os.LookupEnv("GOMODCACHE")
	t.Cleanup(func() {
		outputCommand = oldOutputCommand
		if hadGoModCache {
			os.Setenv("GOMODCACHE", oldGoModCache)
		} else {
			os.Unsetenv("GOMODCACHE")
		}
	})

	tempCache := filepath.Join(t.TempDir(), "gomodcache")
	if err := os.Unsetenv("GOMODCACHE"); err != nil {
		t.Fatalf("Unsetenv(GOMODCACHE): %v", err)
	}

	outputCommand = func(options util.CommandOptions, name string, args ...string) ([]byte, error) {
		if name != "go" || len(args) != 2 || args[0] != "env" || args[1] != "GOMODCACHE" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return []byte(tempCache + "\n"), nil
	}

	got, err := resolveGoModCacheDir()
	if err != nil {
		t.Fatalf("resolveGoModCacheDir returned error: %v", err)
	}
	if got != tempCache {
		t.Fatalf("go mod cache = %s, want %s", got, tempCache)
	}
}

func writeTestXGoRoot(t *testing.T, root string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, "cmd", "xgo"), 0o755); err != nil {
		t.Fatalf("MkdirAll(cmd/xgo) returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/goplus/xgo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) returned error: %v", err)
	}
}
