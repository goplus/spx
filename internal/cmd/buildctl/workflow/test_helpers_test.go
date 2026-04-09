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

package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

type recordedCall struct {
	script string
	args   []string
}

type recordingRunner struct {
	calls       []recordedCall
	commands    []recordedCommand
	repoRoot    string
	commandHook func(workdir string, name string, args ...string) error
}

func (r *recordingRunner) runScript(relativePath string, args ...string) error {
	r.calls = append(r.calls, recordedCall{
		script: relativePath,
		args:   append([]string(nil), args...),
	})
	return nil
}

func (r *recordingRunner) runCommand(workdir string, name string, args ...string) error {
	dir := workdir
	if r.repoRoot != "" && !filepath.IsAbs(dir) {
		dir = filepath.Join(r.repoRoot, dir)
	}
	r.commands = append(r.commands, recordedCommand{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})
	if r.commandHook != nil {
		return r.commandHook(dir, name, args...)
	}
	return nil
}

func (r *recordingRunner) repoRootDir() string {
	if r.repoRoot == "" {
		return "."
	}
	return r.repoRoot
}

func newRuntimeFixtureRunner(t *testing.T) *recordingRunner {
	t.Helper()

	root := t.TempDir()
	gopath := filepath.Join(root, "gopath")
	t.Setenv("GOPATH", gopath)

	mustMkdirAll(t, filepath.Join(root, "cmd", "spx", "template", "project"))
	mustWriteFile(t, filepath.Join(root, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), []byte("runtime extension"))

	return &recordingRunner{
		repoRoot:    root,
		commandHook: simulateRuntimeCommandOutputs,
	}
}

func simulateRuntimeCommandOutputs(workdir string, name string, args ...string) error {
	if name != "spx" || len(args) == 0 {
		return nil
	}

	switch args[0] {
	case "export":
		dst := filepath.Join(workdir, "project", ".builds", "pc", "gdexport.pck")
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte("runtime-pack"), 0o644)
	case "exporttemplateweb":
		dstDir := filepath.Join(workdir, "project", ".builds", "webi")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dstDir, "engine.pck"), []byte("engine-pack"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "engine.js"), []byte("console.log('engine');\n"), 0o644)
	case "exportweb", "exportwebworker", "exportminigame", "exportminiprogram":
		dstDir := filepath.Join(workdir, "project", ".builds", "web", "subdir")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(workdir, "project", ".builds", "web", "index.html"), []byte("<html></html>"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dstDir, "game.js"), []byte("console.log('game');\n"), 0o644)
	default:
		return nil
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
