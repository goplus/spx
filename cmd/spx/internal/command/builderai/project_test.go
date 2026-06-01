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

package builderai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDefaultGoModPropagatesStatErrors(t *testing.T) {
	parentPath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parentPath, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("write parent blocker: %v", err)
	}

	err := createDefaultGoMod(parentPath, "module example.com/demo\n", false)
	if err == nil {
		t.Fatal("createDefaultGoMod returned nil, want stat error")
	}
}

func TestCreateDefaultGoModSkipsExistingFileWithoutForce(t *testing.T) {
	projectRoot := t.TempDir()
	goModPath := filepath.Join(projectRoot, "go.mod")
	original := "module example.com/original\n"
	if err := os.WriteFile(goModPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := createDefaultGoMod(projectRoot, "module example.com/updated\n", false); err != nil {
		t.Fatalf("createDefaultGoMod failed: %v", err)
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if string(data) != original {
		t.Fatalf("go.mod = %q, want %q", string(data), original)
	}
}
