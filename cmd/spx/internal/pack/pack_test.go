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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackProjectIncludesSharedExternalAssets(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "All")
	if err := os.MkdirAll(filepath.Join(projectDir, "assets", "sprites", "SpMotion"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "res"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(projectDir, "assets", "index.json"), []byte(`{"map":{"width":480,"height":360}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "assets", "sprites", "SpMotion", "index.json"), []byte(`{
  "costumeSet": {
    "faceRight": 180,
    "path": "../../../../res/monkey.png",
    "nx": 96
  }
}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "res", "monkey.png"), []byte("png"), 0644); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	if snapshot.counts["res/monkey.png"] != 1 {
		t.Fatalf("res/monkey.png count = %d, want 1", snapshot.counts["res/monkey.png"])
	}
	if snapshot.contents["res/monkey.png"] != "png" {
		t.Fatalf("res/monkey.png content = %q, want %q", snapshot.contents["res/monkey.png"], "png")
	}
	for name := range snapshot.counts {
		if strings.HasPrefix(name, "../") {
			t.Fatalf("unexpected zip entry %q", name)
		}
	}
}

func TestPackProjectCoversExternalAssetVariants(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	escapeDir := filepath.Join(filepath.Dir(tmpDir), filepath.Base(tmpDir)+"-escape")
	t.Cleanup(func() {
		_ = os.RemoveAll(escapeDir)
	})

	writeTestFile(t, filepath.Join(projectDir, ".config"), `{"extasset":"custom_asset"}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), fmt.Sprintf(`{
  "backdrops": [
    {"path":"../../shared/bg.png"},
    {"path":"../../../%s/ignored.png"}
  ],
  "bgm":"../../shared/audio/theme.mp3",
  "tilemapPath":"../../shared/maps/map.json",
  "map":{"width":480,"height":360}
}`, filepath.Base(escapeDir)))
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "index.json"), `{
  "costumes":[
    {"path":"../../../../custom_asset/shared.png"}
  ],
  "costumeSet":{
    "faceRight":180,
    "path":"../../../../custom_asset/hero.png",
    "nx":96
  }
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sounds", "Bell", "index.json"), `{
  "path":"../../../../shared/audio/ring.wav"
}`)
	writeTestFile(t, filepath.Join(projectDir, "extasset", "shared.png"), "local")

	writeTestFile(t, filepath.Join(tmpDir, "shared", "bg.png"), "bg")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "audio", "theme.mp3"), "theme")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "audio", "ring.wav"), "ring")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "maps", "map.json"), "{}")
	writeTestFile(t, filepath.Join(tmpDir, "custom_asset", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(tmpDir, "custom_asset", "shared.png"), "external-duplicate")
	writeTestFile(t, filepath.Join(escapeDir, "ignored.png"), "ignored")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	assertZipEntryContent(t, snapshot, "shared/bg.png", "bg")
	assertZipEntryContent(t, snapshot, "shared/audio/theme.mp3", "theme")
	assertZipEntryContent(t, snapshot, "shared/audio/ring.wav", "ring")
	assertZipEntryContent(t, snapshot, "shared/maps/map.json", "{}")
	assertZipEntryContent(t, snapshot, "extasset/hero.png", "hero")
	assertZipEntryContent(t, snapshot, "extasset/shared.png", "local")

	if snapshot.counts["extasset/shared.png"] != 1 {
		t.Fatalf("extasset/shared.png count = %d, want 1", snapshot.counts["extasset/shared.png"])
	}
	if _, exists := snapshot.counts[filepath.Base(escapeDir)+"/ignored.png"]; exists {
		t.Fatalf("unexpected escaped asset %q in zip", filepath.Base(escapeDir)+"/ignored.png")
	}
	for name := range snapshot.counts {
		if strings.HasPrefix(name, "../") {
			t.Fatalf("unexpected zip entry %q", name)
		}
	}
}

func TestPackProjectFailsOnMissingExternalAsset(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")

	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "backdrops":[{"path":"../../shared/missing.png"}],
  "map":{"width":480,"height":360}
}`)

	zipPath := filepath.Join(tmpDir, "game.zip")
	err := PackProject(projectDir, zipPath)
	if err == nil {
		t.Fatal("PackProject() error = nil, want missing external asset error")
	}
	if !strings.Contains(err.Error(), "missing.png") {
		t.Fatalf("PackProject() error = %q, want mention of missing.png", err)
	}
}

func TestPackProjectIncludesExternalAssetsFromPackedConfigFallback(t *testing.T) {
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
  }
}`)
	writeTestFile(t, filepath.Join(tmpDir, "shared", "bg.jpg"), "bg")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(tmpDir, "shared", "audio", "ring.wav"), "ring")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	assertZipEntryContent(t, snapshot, "shared/bg.jpg", "bg")
	assertZipEntryContent(t, snapshot, "shared/hero.png", "hero")
	assertZipEntryContent(t, snapshot, "shared/audio/ring.wav", "ring")
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

func assertZipEntryContent(t *testing.T, snapshot zipSnapshot, name, want string) {
	t.Helper()

	if snapshot.counts[name] != 1 {
		t.Fatalf("%s count = %d, want 1", name, snapshot.counts[name])
	}
	if got := snapshot.contents[name]; got != want {
		t.Fatalf("%s content = %q, want %q", name, got, want)
	}
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
