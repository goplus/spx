//go:build !packmode

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

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyFilesystemRootsRejectSymlinkEscape(t *testing.T) {
	original := assetPaths
	t.Cleanup(func() {
		assetPaths = original
	})

	projectDir := t.TempDir()
	assetDir := filepath.Join(projectDir, "assets")
	sessionDir := filepath.Join(projectDir, "project")
	if err := os.Mkdir(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "outside.png"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalDir, filepath.Join(assetDir, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	t.Chdir(sessionDir)
	setLegacyFilesystemAssetDir("assets")
	if got := buildFilesystemAssetPath("linked/outside.png"); got != "" {
		t.Fatalf("buildFilesystemAssetPath() followed legacy symlink escape: %q", got)
	}
}
