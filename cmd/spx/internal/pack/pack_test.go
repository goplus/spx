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

package pack

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackProjectIncludesResourcesWithinProject(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"res://res/bg.png"}],
  "bgm":"../res/audio/theme.mp3",
  "tilemapPath":"../res/maps/map.json",
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "index.json"), `{
  "costumeSet":{"faceRight":180,"path":"../../../res/hero.png","nx":96}
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sounds", "Bell", "index.json"), `{
  "path":"../../../res/audio/ring.wav"
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "fonts", "Custom", "index.json"), `{
  "faces":[{"path":"../../../res/fonts/custom.ttf"}]
}`)
	writeTestFile(t, filepath.Join(projectDir, "res", "bg.png"), "bg")
	writeTestFile(t, filepath.Join(projectDir, "res", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(projectDir, "res", "audio", "theme.mp3"), "theme")
	writeTestFile(t, filepath.Join(projectDir, "res", "audio", "ring.wav"), "ring")
	writeTestFile(t, filepath.Join(projectDir, "res", "maps", "map.json"), "{}")
	writeTestFile(t, filepath.Join(projectDir, "res", "fonts", "custom.ttf"), "font")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	for name, want := range map[string]string{
		"res/bg.png":           "bg",
		"res/hero.png":         "hero",
		"res/audio/theme.mp3":  "theme",
		"res/audio/ring.wav":   "ring",
		"res/maps/map.json":    "{}",
		"res/fonts/custom.ttf": "font",
	} {
		if got := snapshot.contents[name]; strings.TrimSpace(got) != strings.TrimSpace(want) {
			t.Fatalf("%s content = %q, want %q", name, got, want)
		}
	}
	for name := range snapshot.counts {
		if strings.HasPrefix(name, "../") {
			t.Fatalf("unexpected zip entry %q", name)
		}
	}
}

func TestPackProjectRejectsMissingAssetIndex(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}")
	if err := os.MkdirAll(filepath.Join(projectDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tmpDir, "game.zip")
	err := PackProject(projectDir, zipPath)
	if err == nil || !strings.Contains(err.Error(), "contains neither") {
		t.Fatalf("PackProject() error = %v, want missing-index rejection", err)
	}
	if _, statErr := os.Stat(zipPath); !os.IsNotExist(statErr) {
		t.Fatalf("output created after missing-index rejection: stat error %v", statErr)
	}
}

func TestPackProjectRejectsMalformedAssetIndex(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{not-json")

	zipPath := filepath.Join(tmpDir, "game.zip")
	err := PackProject(projectDir, zipPath)
	if err == nil || !strings.Contains(err.Error(), "validate asset indexes") {
		t.Fatalf("PackProject() error = %v, want malformed-index rejection", err)
	}
	if _, statErr := os.Stat(zipPath); !os.IsNotExist(statErr) {
		t.Fatalf("output created after malformed-index rejection: stat error %v", statErr)
	}
}

func TestPackProjectRejectsMalformedConfigWithoutAssets(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, ".config"), "{not-json")

	zipPath := filepath.Join(tmpDir, "game.zip")
	err := PackProject(projectDir, zipPath)
	if err == nil || !strings.Contains(err.Error(), "parse project config") {
		t.Fatalf("PackProject() error = %v, want malformed-config rejection", err)
	}
	if _, statErr := os.Stat(zipPath); !os.IsNotExist(statErr) {
		t.Fatalf("output created after malformed-config rejection: stat error %v", statErr)
	}
}

func TestPackProjectKeepsExistingOutputWhenValidationFails(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{not-json")
	zipPath := filepath.Join(projectDir, "game.zip")
	old := []byte("previous output")
	if err := os.WriteFile(zipPath, old, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PackProject(projectDir, zipPath); err == nil {
		t.Fatal("PackProject() succeeded for malformed input")
	}
	got, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(old) {
		t.Fatalf("existing output changed to %q", got)
	}
}

func TestPackProjectDoesNotPackItsOutput(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{}`)
	zipPath := filepath.Join(projectDir, "game.zip")
	if err := os.WriteFile(zipPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if _, exists := readZipSnapshot(t, zipPath).contents["game.zip"]; exists {
		t.Fatal("pack output was included in itself")
	}
}

func TestPackProjectIncludesLegacySharedResource(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../../shared/bg.png"}],
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(tmpDir, "shared", "bg.png"), "bg")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if got := readZipSnapshot(t, zipPath).contents["shared/bg.png"]; got != "bg" {
		t.Fatalf("shared/bg.png content = %q, want bg", got)
	}
}

func TestPackProjectRejectsResourceThroughSymlinkOutsideProject(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	externalDir := filepath.Join(tmpDir, "external")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../linked/bg.png"}],
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(externalDir, "bg.png"), "bg")
	if err := os.Symlink(externalDir, filepath.Join(projectDir, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := PackProject(projectDir, filepath.Join(tmpDir, "game.zip"))
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("PackProject() error = %v, want no-follow rejection", err)
	}
}

func TestPackProjectAcceptsPackedOnlyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "main.spx"), "onStart => {}")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index_pack.json"), `{"zorder":[]}`)

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if got := readZipSnapshot(t, zipPath).counts["assets/index_pack.json"]; got != 1 {
		t.Fatalf("assets/index_pack.json count = %d, want 1", got)
	}
}

func TestPackProjectIncludesLegacyExtAssetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"custom_asset"}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../../custom_asset/bg.png"}],
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(tmpDir, "custom_asset", "bg.png"), "external-bg")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	if got := readZipSnapshot(t, zipPath).contents["extasset/bg.png"]; got != "external-bg" {
		t.Fatalf("extasset/bg.png content = %q, want external-bg", got)
	}
}

func TestPackProjectIncludesExternalAssetFromPackedConfig(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")

	writeTestFile(t, filepath.Join(projectDir, "assets", "index_pack.json"), `{
  "backdrops":[{"path":"../../shared/bg.jpg"}],
  "map":{"width":480,"height":360},
  "sprites":{
    "Hero":{
      "costumeSet":{
        "faceRight":180,
        "path":"../../../../shared/hero.png",
        "nx":96
      }
    }
  },
  "sounds":{
    "Bell":{"path":"../../../../shared/audio/ring.wav"}
  },
  "fonts":{"Custom":{"faces":[{"path":"../../../../shared/fonts/custom.ttf"}]}}
}`)
	writeTestFile(t, filepath.Join(tmpDir, "shared", "bg.jpg"), "bg")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "audio", "ring.wav"), "ring")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "fonts", "custom.ttf"), "font")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	snapshot := readZipSnapshot(t, zipPath)
	for name, want := range map[string]string{
		"shared/bg.jpg":           "bg",
		"shared/hero.png":         "hero",
		"shared/audio/ring.wav":   "ring",
		"shared/fonts/custom.ttf": "font",
	} {
		if got := snapshot.contents[name]; got != want {
			t.Fatalf("%s content = %q, want %q", name, got, want)
		}
	}
}

func TestPackProjectIncludesExternalAssetFromSourceRootWhenPackedRootMissing(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")

	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../../shared/bg.jpg"}],
  "bgm":"../../shared/audio/theme.mp3",
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "index_pack.json"), `{
  "sprites":{
    "Hero":{
      "costumeSet":{
        "faceRight":180,
        "path":"../../../../shared/hero.png",
        "nx":96
      }
    }
  },
  "zorder":["Hero"]
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "fonts", "Source", "index.json"), `{
  "faces":[{"path":"../../../../shared/fonts/source.ttf"}]
}`)
	writeTestFile(t, filepath.Join(tmpDir, "shared", "bg.jpg"), "bg")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "audio", "theme.mp3"), "theme")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "fonts", "source.ttf"), "source-font")
	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}
	snapshot := readZipSnapshot(t, zipPath)
	for name, want := range map[string]string{
		"shared/bg.jpg":           "bg",
		"shared/audio/theme.mp3":  "theme",
		"shared/hero.png":         "hero",
		"shared/fonts/source.ttf": "source-font",
	} {
		if got := snapshot.contents[name]; got != want {
			t.Fatalf("%s content = %q, want %q", name, got, want)
		}
	}
}

func TestPackZipRejectsFileReplacedAfterCollection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "asset.txt")
	writeTestFile(t, filePath, "inside")
	info, err := os.Lstat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(t.TempDir(), "external.txt")
	writeTestFile(t, externalPath, "outside")
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalPath, filePath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	var output strings.Builder
	zipWriter := zip.NewWriter(&output)
	err = packZip(zipWriter, tmpDir, []dirInfo{{path: filePath, info: info}})
	_ = zipWriter.Close()
	if err == nil || !strings.Contains(err.Error(), "changed after collection") {
		t.Fatalf("PackZip() error = %v, want replaced-file rejection", err)
	}
}

func TestPackZipRejectsExternalParentReplacedByOutsideSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../../shared/bg.png"}],
  "map":{"width":480,"height":360}
}`)
	sharedDir := filepath.Join(tmpDir, "shared")
	writeTestFile(t, filepath.Join(sharedDir, "bg.png"), "inside")

	extraPaths, err := collectExternalAssetPathsWithConfig(projectDir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closePackRoots(extraPaths)
	if len(extraPaths) != 1 {
		t.Fatalf("collected %d external paths, want 1", len(extraPaths))
	}

	outsideDir := filepath.Join(t.TempDir(), "shared")
	if err := os.Rename(sharedDir, outsideDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, sharedDir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	zipPath := filepath.Join(tmpDir, "game.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zipWriter := zip.NewWriter(zipFile)
	packErr := packZip(zipWriter, projectDir, extraPaths)
	closeZipErr := zipWriter.Close()
	closeFileErr := zipFile.Close()
	if packErr == nil {
		t.Fatalf("PackZip() succeeded after external parent escaped root")
	}
	if closeZipErr != nil {
		t.Fatal(closeZipErr)
	}
	if closeFileErr != nil {
		t.Fatal(closeFileErr)
	}
	if snapshot := readZipSnapshot(t, zipPath); snapshot.counts["shared/bg.png"] != 0 {
		t.Fatalf("escaped external asset was packed: %+v", snapshot.contents)
	}
}

type zipSnapshot struct {
	counts   map[string]int
	contents map[string]string
}

func readZipSnapshot(t *testing.T, zipPath string) zipSnapshot {
	t.Helper()

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	snapshot := zipSnapshot{
		counts:   make(map[string]int),
		contents: make(map[string]string),
	}
	for _, file := range reader.File {
		snapshot.counts[file.Name]++
		if file.FileInfo().IsDir() {
			continue
		}

		content, err := readZipFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snapshot.contents[file.Name] = content
	}
	return snapshot
}

func readZipFile(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}

	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return "", readErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return string(data), nil
}

func writeTestFile(t *testing.T, filePath string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
