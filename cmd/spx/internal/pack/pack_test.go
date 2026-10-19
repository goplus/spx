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
	"errors"
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

func TestPackProjectIncludesWebPSequenceFrames(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "Game")
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), `{
  "zorder":["Hero"]
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "index.json"), `{
  "costumes":[
    {"name":"frame-0","path":"frame-0.webp"},
    {"name":"frame-1","path":"frame-1.webp"}
  ]
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "frame-0.webp"), "webp-frame-0")
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "frame-1.webp"), "webp-frame-1")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	for name, want := range map[string]string{
		"assets/sprites/Hero/frame-0.webp": "webp-frame-0",
		"assets/sprites/Hero/frame-1.webp": "webp-frame-1",
	} {
		if got := snapshot.contents[name]; got != want {
			t.Fatalf("%s content = %q, want %q", name, got, want)
		}
	}
}

func TestPackProjectRejectsInvalidInputsWithoutCreatingOutput(t *testing.T) {
	for _, tt := range []struct {
		name, path, content, want string
	}{
		{"missing asset index", "main.spx", "onStart => {}", "contains neither"},
		{"malformed asset index", "assets/index.json", "{not-json", "validate asset indexes"},
		{"malformed config without assets", ".config", "{not-json", "parse project config"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(root, "Game")
			writeTestFile(t, filepath.Join(project, tt.path), tt.content)
			if tt.name == "missing asset index" {
				if err := os.Mkdir(filepath.Join(project, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(root, "game.zip")
			if err := PackProject(project, output); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("PackProject error = %v, want %q", err, tt.want)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("output created after validation failed: %v", err)
			}
		})
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
	root, err := openPackRoot(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	paths, err := collectProjectPaths(tmpDir, filepath.Join(tmpDir, "game.zip"), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0].root != root || paths[0].rootPath != "asset.txt" {
		t.Fatalf("collected paths = %+v, want asset.txt with its project root", paths)
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
	err = packZip(zipWriter, tmpDir, paths)
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

	extAssetDir, err := validateLegacyPackInputs(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	extraPaths, err := collectExternalAssetPaths(projectDir, nil, extAssetDir)
	if err != nil {
		t.Fatal(err)
	}
	defer closePackRoots(extraPaths)
	if len(extraPaths) != 1 {
		t.Fatalf("collected %d external paths, want 1", len(extraPaths))
	}
	if extraPaths[0].root == nil || extraPaths[0].rootPath != filepath.Join("shared", "bg.png") {
		t.Fatalf("external path = %+v, want shared/bg.png with its compatibility root", extraPaths[0])
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

func TestPackZipRejectsInvalidCollectedNames(t *testing.T) {
	for _, tc := range []struct {
		name    string
		zipName string
		wantErr string
	}{
		{name: "parent traversal", zipName: "../asset.txt", wantErr: "unsafe zip name"},
		{name: "nested traversal", zipName: "nested/../../asset.txt", wantErr: "unsafe zip name"},
		{name: "duplicate", zipName: "same.txt", wantErr: "duplicates zip name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeTestFile(t, filepath.Join(projectDir, "first.txt"), "first")
			writeTestFile(t, filepath.Join(projectDir, "second.txt"), "second")
			root, err := openPackRoot(projectDir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			paths, err := collectProjectPaths(projectDir, filepath.Join(projectDir, "game.zip"), root)
			if err != nil {
				t.Fatal(err)
			}
			for i := range paths {
				paths[i].zipPath = tc.zipName
			}
			zipWriter := zip.NewWriter(io.Discard)
			err = packZip(zipWriter, projectDir, paths)
			_ = zipWriter.Close()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("packZip() error = %v, want %q", err, tc.wantErr)
			}
			if _, err := root.Stat("."); err != nil {
				t.Fatalf("packZip() closed the caller's root: %v", err)
			}
		})
	}
}

func TestPackProjectPreservesValidationOrder(t *testing.T) {
	for _, tc := range []struct {
		name             string
		destinationIsDir bool
		wantErr          string
	}{
		{"config before asset indexes", false, "parse project config"},
		{"destination before config", true, "must be a file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectDir := filepath.Join(tmpDir, "Game")
			writeTestFile(t, filepath.Join(projectDir, ".config"), "{not-json")
			writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{not-json")
			zipPath := filepath.Join(tmpDir, "game.zip")
			if tc.destinationIsDir {
				if err := os.Mkdir(zipPath, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			err := PackProject(projectDir, zipPath)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("PackProject() error = %v, want %q", err, tc.wantErr)
			}
		})
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

func TestPackProjectRejectsInvalidRootsAndDestinations(t *testing.T) {
	for _, name := range []string{"missing root", "file root", "symlink root", "directory destination", "symlink destination", "missing destination parent"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			project, output := filepath.Join(root, "project"), filepath.Join(root, "game.zip")
			writeTestFile(t, filepath.Join(project, "assets", "index.json"), `{}`)
			want := ""
			switch name {
			case "missing root":
				project = filepath.Join(root, "missing")
			case "file root":
				project = filepath.Join(project, "assets", "index.json")
				want = "must be a real directory"
			case "symlink root":
				link := filepath.Join(root, "link")
				if err := os.Symlink(project, link); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				project, want = link, "must be a real directory"
			case "directory destination":
				output, want = root, "must be a file"
			case "symlink destination":
				target := filepath.Join(root, "original.zip")
				writeTestFile(t, target, "previous archive")
				if err := os.Symlink(target, output); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				want = "must not be a symlink"
			case "missing destination parent":
				output, want = filepath.Join(root, "missing", "game.zip"), "create temporary pack output"
			}
			if err := PackProject(project, output); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("PackProject error = %v, want %q", err, want)
			}
			if name == "symlink destination" {
				if got, err := os.ReadFile(output); err != nil || string(got) != "previous archive" {
					t.Fatalf("rejected destination changed: %q, %v", got, err)
				}
			}
		})
	}
	if err := validatePackDestination("invalid\x00path"); err == nil || !strings.Contains(err.Error(), "inspect pack destination") {
		t.Fatalf("invalid destination error = %v", err)
	}
}

type packCloseResult struct{ err error }

func (c packCloseResult) Close() error { return c.err }

func TestClosePackOutputPreservesFirstErrorAndClosesFile(t *testing.T) {
	packErr, writerErr := errors.New("packing failed"), errors.New("zip close failed")
	for _, tt := range []struct {
		name      string
		pack, zip error
		closed    bool
		want      error
	}{
		{"success", nil, nil, false, nil},
		{"packing error first", packErr, writerErr, false, packErr},
		{"zip close error", nil, writerErr, false, writerErr},
		{"file sync error", nil, nil, true, os.ErrClosed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "output-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if tt.closed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := closePackOutput(packCloseResult{tt.zip}, file, tt.pack); !errors.Is(err, tt.want) {
				t.Fatalf("closePackOutput = %v, want %v", err, tt.want)
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("output file left open: %v", err)
			}
		})
	}
}

func TestPackOutputPublishesOnlyCompletedTemporaryFile(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "game.zip")
	writeTestFile(t, destination, "previous archive")
	temporary, file, err := createPackOutput(destination)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if filepath.Dir(temporary) != root || !strings.HasPrefix(filepath.Base(temporary), ".game.zip.tmp-") {
		t.Fatalf("temporary output = %q", temporary)
	}
	if _, err := file.WriteString("completed archive"); err != nil {
		t.Fatal(err)
	}
	if err := closePackOutput(packCloseResult{}, file, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "previous archive" {
		t.Fatalf("destination changed before publication: %q, %v", got, err)
	}
	if err := publishPackOutput(temporary, destination); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "completed archive" {
		t.Fatalf("published archive = %q, %v", got, err)
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatalf("temporary output remains after publication: %v", err)
	}
	if err := publishPackOutput(destination, root); err == nil || !strings.Contains(err.Error(), root) {
		t.Fatalf("directory publication error = %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "completed archive" {
		t.Fatalf("failed publication changed archive: %q, %v", got, err)
	}
}

func TestExternalAssetPathsStayWithinCompatibilityRoot(t *testing.T) {
	root := t.TempDir()
	assets := filepath.Join(root, "Game", "assets")
	for _, tt := range []struct {
		name, path, external, source, entry string
	}{
		{"project asset", "sprite.png", "", "", ""},
		{"empty", "", "", "", ""},
		{"absolute", filepath.ToSlash(filepath.Join(root, "absolute.png")), "", "", ""},
		{"remote schema", "https://example.invalid/image.png", "", "", ""},
		{"shared asset", "../../shared/image.png", "", "shared/image.png", "shared/image.png"},
		{"single parent is not shared", "../image.png", "", "", ""},
		{"outside compatibility root", "../../../image.png", "", "", ""},
		{"compatibility root itself", "../..", "extras", "", ""},
		{"configured external folder", "../extras/sound.wav", "extras", "Game/extras/sound.wav", "extasset/sound.wav"},
		{"external folder outside root", "../../../extras/sound.wav", "extras", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source, entry, ok := resolveExternalAssetPath(assets, root, tt.external, tt.path)
			wantSource := ""
			if tt.source != "" {
				wantSource = filepath.ToSlash(filepath.Join(root, tt.source))
			}
			if ok != (tt.entry != "") || source != wantSource || entry != tt.entry {
				t.Fatalf("resolved = (%q, %q, %v), want (%q, %q, %v)", source, entry, ok, wantSource, tt.entry, tt.entry != "")
			}
		})
	}
}

func TestInspectExternalAssetRejectsUnsafeComponents(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "nested", "file.png"), "image")
	root, err := openPackRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, tt := range []struct{ name, path, want string }{
		{"outside", "../outside.png", "outside the compatibility root"},
		{"root itself", ".", "outside the compatibility root"},
		{"missing", "missing.png", ""},
		{"directory", "nested", "must be a regular non-symlink file"},
		{"file parent", "nested/file.png/child", "not a directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			info, relative, err := inspectExternalAssetPath(root, directory, filepath.Join(directory, tt.path))
			if err == nil || !strings.Contains(err.Error(), tt.want) || info != nil || relative != "" {
				t.Fatalf("inspection = (%v, %q, %v), want %q", info, relative, err, tt.want)
			}
		})
	}
	info, relative, err := inspectExternalAssetPath(root, directory, filepath.Join(directory, "nested", "file.png"))
	if err != nil || relative != filepath.Join("nested", "file.png") || info.Size() != int64(len("image")) {
		t.Fatalf("valid external asset = (%v, %q, %v)", info, relative, err)
	}
}

func TestPackZipRejectsDeletedCollectedFileAndLeavesRootOpen(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "asset.txt")
	writeTestFile(t, file, "asset")
	root, err := openPackRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	paths, err := collectProjectPaths(directory, filepath.Join(directory, "game.zip"), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(io.Discard)
	defer writer.Close()
	if err := packZip(writer, directory, paths); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "before packing") {
		t.Fatalf("deleted collected file error = %v", err)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("packing failure closed the caller's root: %v", err)
	}
	closePackRoots([]dirInfo{{}, {root: root}, {root: root}})
	if _, err := root.Stat("."); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("root remained open after cleanup: %v", err)
	}
}

func TestPackedAssetIndexesRejectMalformedCategories(t *testing.T) {
	for _, tt := range []struct{ name, packed, source, want string }{
		{"invalid JSON", `{`, "", "parse"},
		{"invalid source JSON", `{}`, `{`, "index.json"},
		{"invalid project fields", `{"map":[]}`, "", "root"},
		{"invalid sprites", `{"sprites":42}`, "", "sprites"},
		{"invalid sounds", `{"sounds":42}`, "", "sounds"},
		{"invalid fonts", `{"fonts":42}`, "", "fonts"},
		{"invalid sprite entry", `{"sprites":{"Hero":{"costumes":42}}}`, "", "Hero"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, packedIndexName), tt.packed)
			if tt.source != "" {
				writeTestFile(t, filepath.Join(root, sourceIndexName), tt.source)
			}
			if _, _, err := readPackedAssetIndex(root); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("packed index error = %v, want %q", err, tt.want)
			}
		})
	}
}
