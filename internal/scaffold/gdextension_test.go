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

package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGDExtensionTemplatesMatchProjectFiles(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	assertTemplateFile(t, filepath.Join(repoRoot, "cmd", "spx", "template", "project", "runtime.gdextension.txt"), RuntimeGDExtension())
	assertTemplateFile(t, filepath.Join(repoRoot, "cmd", "spx", "template", "project", "gdspx.gdextension"), ProjectGDExtension())
}

func TestRuntimeExtensionListUsesStandardProjectEntry(t *testing.T) {
	if got, want := RuntimeExtensionList(), "res://runtime.gdextension\n"; got != want {
		t.Fatalf("RuntimeExtensionList() = %q, want %q", got, want)
	}
}

func assertTemplateFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of sync; run go generate ./cmd/spx", path)
	}
}
