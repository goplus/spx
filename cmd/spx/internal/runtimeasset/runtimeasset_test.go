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

package runtimeasset

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPrepareReturnsFalseWhenAssetMissing(t *testing.T) {
	dir, ok, err := Prepare("9.9.9", "missing-runtime-asset")
	if err != nil || ok || dir != "" {
		t.Fatalf("Prepare() = (%q, %v, %v), want no assets", dir, ok, err)
	}
}

func TestExtractAssetCopiesEmbeddedContent(t *testing.T) {
	cacheDir := t.TempDir()
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	assertEmbeddedContent(t, filepath.Join(cacheDir, "placeholder.txt"))
}

func TestExtractAssetRepairsSameSizeCorruption(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	content, err := embeddedAssets.ReadFile(assetPath("placeholder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachedPath, bytes.Repeat([]byte{0}, len(content)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	assertEmbeddedContent(t, cachedPath)
}

func TestExtractAssetReusesValidCache(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(cachedPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cachedPath, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := extractAsset(cacheDir, "placeholder.txt"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(cachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("valid asset cache was rewritten")
	}
	if runtime.GOOS != "windows" && after.Mode().Perm() != 0o644 {
		t.Fatalf("asset mode = %v, want 0644", after.Mode())
	}
}

func TestExtractAssetReportsCacheRepairFailure(t *testing.T) {
	cacheDir := t.TempDir()
	cachedPath := filepath.Join(cacheDir, "placeholder.txt")
	if err := os.Mkdir(cachedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cachedPath, "occupied"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := extractAsset(cacheDir, "placeholder.txt"); err == nil || !strings.Contains(err.Error(), cachedPath) {
		t.Fatalf("extractAsset() error = %v, want an error identifying %s", err, cachedPath)
	}
	content, err := os.ReadFile(filepath.Join(cachedPath, "occupied"))
	if err != nil || string(content) != "keep" {
		t.Fatalf("cache directory changed: content %q, error %v", content, err)
	}
	leftovers, err := filepath.Glob(cachedPath + ".tmp-*")
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary cache files remain: %v, error: %v", leftovers, err)
	}
}

func assertEmbeddedContent(t *testing.T, path string) {
	t.Helper()
	want, err := embeddedAssets.ReadFile(assetPath("placeholder.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("file %s = %q, want %q", path, got, want)
	}
}
