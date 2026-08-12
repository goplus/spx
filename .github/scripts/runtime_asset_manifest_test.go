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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goplus/spx/v3/internal/release"
)

func TestRunReleaseManifestGenerateAndVerify(t *testing.T) {
	t.Parallel()

	lock := release.DefaultRuntimeLock()
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, lock.Manifest)
	checksumsPath := filepath.Join(dir, release.SHA256SumsFileName)
	args := []string{
		"--output", manifestPath,
		"--checksums", checksumsPath,
		"--spx-commit", strings.Repeat("1", 40),
		"--module-tree", strings.Repeat("2", 40),
		"--runtime-pack-source-sha256", strings.Repeat("3", 64),
		"--build-recipe-sha256", strings.Repeat("4", 64),
	}
	for _, name := range lock.RequiredAssets {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("asset:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--asset", name+"="+path)
	}
	if err := runReleaseManifest(args); err != nil {
		t.Fatalf("generate manifest: %v", err)
	}
	if err := runReleaseManifest([]string{
		"--verify-manifest", manifestPath,
		"--asset-directory", dir,
	}); err != nil {
		t.Fatalf("verify generated manifest: %v", err)
	}
	if err := runReleaseManifest([]string{"--verify-manifest", manifestPath}); err != nil {
		t.Fatalf("validate generated manifest metadata: %v", err)
	}

	corruptPath := filepath.Join(dir, lock.RequiredAssets[0])
	if err := os.WriteFile(corruptPath, []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runReleaseManifest([]string{
		"--verify-manifest", manifestPath,
		"--asset-directory", dir,
	}); err == nil {
		t.Fatal("verify corrupted asset: got nil error")
	}
}

func TestRunReleaseManifestRejectsMixedModes(t *testing.T) {
	t.Parallel()

	err := runReleaseManifest([]string{
		"--verify-manifest", "runtime-manifest.json",
		"--asset-directory", ".",
		"--asset", "runtime.zip=runtime.zip",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("mixed modes error = %v", err)
	}
}
