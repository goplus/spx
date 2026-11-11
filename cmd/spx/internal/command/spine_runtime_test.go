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

package command

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/goplus/spx/v4/internal/runtimevariant"
)

func TestSpineRuntimeDisallowsFallback(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, runtimevariant.SpineMarker), nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOPATH", root)
	t.Setenv("SPX_SPINE", "1")
	cmd := &CmdTool{GoBinPath: bin, Version: "4.0.0"}
	if got := cmd.runtimeSearchDirs(); !reflect.DeepEqual(got, []string{bin}) {
		t.Fatalf("fallback directories = %v", got)
	}
	if _, err := cmd.resolveInterpretedRuntimeAssets(); err == nil {
		t.Fatal("missing local engine should not select embedded runtime")
	}
	if err := cmd.installRepoWebRuntime(webNormalMode); err == nil {
		t.Fatal("missing Spine Web assets should not install published runtime")
	}
	other := t.TempDir()
	name := envName + cmd.Version + executableSuffix(runtime.GOOS)
	if err := os.WriteFile(filepath.Join(other, name), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", other)
	got, err := resolveAppPath(bin, envName, cmd.Version)
	if err != nil || got != filepath.Join(bin, name) {
		t.Fatalf("editor selected PATH fallback: %q, %v", got, err)
	}
	t.Setenv("SPX_SPINE", "0")
	if _, err := cmd.resolveInterpretedRuntimeAssets(); err == nil {
		t.Fatal("default selection accepted marked Spine bin")
	}
}
