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
	"os"
	"path/filepath"
	"testing"
)

func TestWebLogicAssetsUseInterpreterRuntime(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}

	projectWasm := []byte("project-compiled-wasm")
	projectWasmPath := cmd.getProjectWasmPath()
	if err := os.MkdirAll(filepath.Dir(projectWasmPath), 0o755); err != nil {
		t.Fatalf("mkdir project wasm dir: %v", err)
	}
	if err := os.WriteFile(projectWasmPath, projectWasm, 0o644); err != nil {
		t.Fatalf("write project wasm: %v", err)
	}
	wantWasm := []byte("interpreter-wasm")
	wantWasmBr := []byte("compressed-interpreter-wasm")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), wantWasm, 0o644); err != nil {
		t.Fatalf("write interpreter wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm.br"), wantWasmBr, 0o644); err != nil {
		t.Fatalf("write compressed interpreter wasm: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir exported web dir: %v", err)
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}

	gotWasm, err := os.ReadFile(filepath.Join(webDir, "ispx.wasm"))
	if err != nil {
		t.Fatalf("read exported project wasm: %v", err)
	}
	if !bytes.Equal(gotWasm, wantWasm) {
		t.Fatalf("exported wasm = %q, want %q", gotWasm, wantWasm)
	}
	gotWasmBr, err := os.ReadFile(filepath.Join(webDir, "ispx.wasm.br"))
	if err != nil {
		t.Fatalf("read compressed exported interpreter wasm: %v", err)
	}
	if !bytes.Equal(gotWasmBr, wantWasmBr) {
		t.Fatalf("compressed exported wasm = %q, want %q", gotWasmBr, wantWasmBr)
	}
}

func TestWebLogicAssetsRemoveStaleCompression(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), []byte("interpreter-wasm"), 0o644); err != nil {
		t.Fatalf("write fallback wasm: %v", err)
	}

	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "ispx.wasm.br"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale compressed wasm: %v", err)
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}
	if _, err := os.Stat(filepath.Join(webDir, "ispx.wasm.br")); !os.IsNotExist(err) {
		t.Fatalf("stale compressed wasm still exists: %v", err)
	}
}

func TestWebLogicAssetsCopyCompressedInterpreter(t *testing.T) {
	targetDir := t.TempDir()
	projectDir := filepath.Join(targetDir, "project")
	webDir := filepath.Join(projectDir, ".builds", "web")
	goBinDir := filepath.Join(targetDir, "gobin")
	if err := os.MkdirAll(goBinDir, 0o755); err != nil {
		t.Fatalf("mkdir gobin: %v", err)
	}
	wantWasm := []byte("interpreter-wasm")
	wantWasmBr := []byte("compressed-interpreter-wasm")
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm"), wantWasm, 0o644); err != nil {
		t.Fatalf("write fallback wasm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goBinDir, "ispx.wasm.br"), wantWasmBr, 0o644); err != nil {
		t.Fatalf("write compressed fallback wasm: %v", err)
	}
	if err := os.MkdirAll(webDir, 0o755); err != nil {
		t.Fatalf("mkdir web dir: %v", err)
	}

	cmd := CmdTool{
		TargetDir:  targetDir,
		ProjectDir: projectDir,
		WebDir:     webDir,
		GoBinPath:  goBinDir,
	}
	if err := cmd.writeWebLogicAssets(); err != nil {
		t.Fatalf("writeWebLogicAssets: %v", err)
	}

	for name, want := range map[string][]byte{
		"ispx.wasm":    wantWasm,
		"ispx.wasm.br": wantWasmBr,
	} {
		got, err := os.ReadFile(filepath.Join(webDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}
