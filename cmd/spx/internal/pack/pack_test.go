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
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/base/stablefile"
)

type zipSnapshot struct {
	counts   map[string]int
	contents map[string]string
}

type packCloseResult struct{ err error }

type packWriteFunc func([]byte) (int, error)

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

func TestLegacyPackValidationRejectsInvalidFilesystemEntries(t *testing.T) {
	for _, tc := range []struct {
		name, path, want string
		directory        bool
	}{
		{"missing project", "missing", "inspect project directory", false},
		{"file project", "file", "must be a real directory", false},
		{"missing assets", ".", "is missing", false},
		{"file assets", "assets", "must be a real directory", false},
		{"directory index", "assets/index.json", "regular non-symlink", true},
		{"directory config", ".config", "regular non-symlink", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.directory {
				if err := os.MkdirAll(filepath.Join(root, tc.path), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if tc.name == "file project" || tc.name == "file assets" {
				writeTestFile(t, filepath.Join(root, tc.path), "file")
			}
			project := root
			if tc.name == "missing project" || tc.name == "file project" {
				project = filepath.Join(root, tc.path)
			}
			if _, err := validateLegacyPackInputs(project); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validation error = %v, want %q", err, tc.want)
			}
		})
	}
	for _, check := range []struct {
		name string
		run  func(string) error
	}{
		{"project config", func(root string) error { _, err := validateLegacyProjectConfig(root); return err }},
		{"external config", func(root string) error { _, err := readExtAssetDir(root); return err }},
		{"external assets", func(root string) error { _, err := collectExternalAssetPaths(root, nil, ""); return err }},
	} {
		t.Run(check.name+" invalid path", func(t *testing.T) {
			if err := check.run("invalid\x00path"); err == nil {
				t.Fatal("accepted invalid path")
			}
		})
	}
}

func TestCollectAssetRefsRejectsMalformedSourceCategories(t *testing.T) {
	for _, category := range []string{"sprites", "sounds", "fonts"} {
		t.Run(category, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, category, "Broken", sourceIndexName), "{")
			if _, err := collectAssetPathRefs(root); err == nil || !strings.Contains(err.Error(), "Broken") {
				t.Fatalf("collect error = %v", err)
			}
		})
	}
	for _, read := range []struct {
		name string
		run  func(string) error
	}{
		{"source", func(root string) error { _, err := readSourceAssetIndexRoot(root); return err }},
		{"packed", func(root string) error { _, _, err := readPackedAssetIndex(root); return err }},
		{"refs", func(root string) error { _, err := collectAssetPathRefs(root); return err }},
	} {
		t.Run(read.name+" invalid path", func(t *testing.T) {
			if err := read.run("invalid\x00path"); err == nil {
				t.Fatal("accepted invalid path")
			}
		})
	}
}

func TestCollectProjectPathsSkipsBuildMetadataAndOutputAliases(t *testing.T) {
	rootDir := t.TempDir()
	for _, name := range []string{".git", "project/generated.go", "assets/project/cache", "assets/.git/config", "assets/keep.png", "assets/keep.png.import", "nested/keep.txt", "archive.zip"} {
		writeTestFile(t, filepath.Join(rootDir, name), name)
	}
	root, err := openPackRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	output := filepath.Join(rootDir, "archive.zip")
	if err := os.Link(output, filepath.Join(rootDir, "alias.zip")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	paths, err := collectProjectPaths(rootDir, output, root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range paths {
		got = append(got, filepath.ToSlash(p.rootPath))
	}
	if want := []string{"assets", "assets/keep.png", "nested", "nested/keep.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collected = %q, want %q", got, want)
	}
	for _, tc := range []struct{ name, base, destination string }{
		{"missing project", filepath.Join(rootDir, "missing"), output},
		{"invalid destination", rootDir, "invalid\x00path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := collectProjectPaths(tc.base, tc.destination, root); err == nil {
				t.Fatal("accepted invalid walk")
			}
		})
	}
}

func TestPackPathHelpersRejectMixedPathForms(t *testing.T) {
	root := t.TempDir()
	opened, err := openPackRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, _, err := inspectExternalAssetPath(opened, root, "relative"); err == nil {
		t.Fatal("accepted mismatched path forms")
	}
	if _, err := relConfigDir(root, "relative"); err == nil {
		t.Fatal("accepted mismatched config path forms")
	}
	if isWithinRoot("relative", root) {
		t.Fatal("relative path treated as inside absolute root")
	}
	if got := rewriteExtAssetZipPath("../..", "shared"); got != "" {
		t.Fatalf("parent-only external path = %q", got)
	}
	if refs, err := collectExternalAssetPaths(root, nil, ""); err != nil || len(refs) != 0 {
		t.Fatalf("missing assets = %v, %v", refs, err)
	}
	writeTestFile(t, filepath.Join(root, "assets"), "not a directory")
	if _, err := collectExternalAssetPaths(root, nil, ""); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("file assets = %v", err)
	}
}

func TestPackZipDetectsWriteFailuresAndConcurrentFileChanges(t *testing.T) {
	for _, kind := range []string{"output failure", "file changed", "file header too long", "directory header too long", "root entry"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			name := filepath.Join(directory, "asset")
			writeTestFile(t, name, strings.Repeat("asset data", 2048))
			if kind == "directory header too long" {
				name = filepath.Join(directory, "folder")
				if err := os.Mkdir(name, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "root entry" {
				name = directory
			}
			root, err := openPackRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			info, err := os.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			relative, err := filepath.Rel(directory, name)
			if err != nil {
				t.Fatal(err)
			}
			entry := dirInfo{path: name, info: info, root: root, rootPath: relative}
			if strings.Contains(kind, "header too long") {
				entry.zipPath = strings.Repeat("a", 1<<16)
			}
			writeErr := errors.New("archive destination failed")
			writes := 0
			writer := zip.NewWriter(packWriteFunc(func(data []byte) (int, error) {
				writes++
				if kind == "output failure" {
					return 0, writeErr
				}
				if kind == "file changed" && writes == 1 {
					if err := os.Truncate(name, 1); err != nil {
						t.Fatal(err)
					}
				}
				return len(data), nil
			}))
			err = packZip(writer, directory, []dirInfo{entry})
			_ = writer.Close()
			switch kind {
			case "root entry":
				if err != nil {
					t.Fatal(err)
				}
			case "output failure":
				if !errors.Is(err, writeErr) {
					t.Fatalf("write failure = %v", err)
				}
			case "file changed":
				if err == nil || !strings.Contains(err.Error(), "changed while packing") {
					t.Fatalf("changed file error = %v", err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), "FileHeader.Name too long") {
					t.Fatalf("header error = %v", err)
				}
			}
		})
	}
	for _, name := range []string{"", "/absolute"} {
		if validZipEntryName(name) {
			t.Fatalf("accepted zip name %q", name)
		}
	}
}

func TestExternalAssetReferencesDeduplicateAndRejectMissingFiles(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Game")
	writeTestFile(t, filepath.Join(project, "assets", "index.json"), `{"backdrops":[null,{"path":"../../shared/bg.png"},{"path":"../../shared/bg.png"}]}`)
	writeTestFile(t, filepath.Join(root, "shared", "bg.png"), "background")
	paths, err := collectExternalAssetPaths(project, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer closePackRoots(paths)
	if len(paths) != 1 || paths[0].zipPath != "shared/bg.png" {
		t.Fatalf("external paths = %+v", paths)
	}
	duplicate, err := collectExternalAssetPaths(project, map[string]struct{}{"shared/bg.png": {}}, "")
	if err != nil || len(duplicate) != 0 {
		t.Fatalf("existing archive paths duplicated = %+v, %v", duplicate, err)
	}
	if err := os.Remove(filepath.Join(root, "shared", "bg.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := collectExternalAssetPaths(project, nil, ""); err == nil || !strings.Contains(err.Error(), "stat external asset") {
		t.Fatalf("missing external asset error = %v", err)
	}
	if got, err := readExtAssetDir(project); err != nil || got != "" {
		t.Fatalf("missing optional config = %q, %v", got, err)
	}
}

func TestPackedSpritesOverrideSourceAndIncludeMultipartCostumes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, packedIndexName), `{"sprites":{"Hero":{"costumes":[null],"costumeMPSet":{"path":"multipart.png"}}}}`)
	writeTestFile(t, filepath.Join(root, "sprites", "Hero", sourceIndexName), "invalid shadowed source")
	refs, err := collectAssetPathRefs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []assetPathRef{{configDir: "sprites/Hero", path: "multipart.png"}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("packed references = %+v, want %+v", refs, want)
	}
}

func TestPackRejectsSocketEntries(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "socket")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("Unix sockets unavailable: %v", err)
	}
	defer listener.Close()
	if err := validatePackDestination(socket); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("socket destination = %v", err)
	}
	opened, err := openPackRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := collectProjectPaths(root, filepath.Join(root, "archive.zip"), opened); err == nil || !strings.Contains(err.Error(), "regular file or directory") {
		t.Fatalf("socket project entry = %v", err)
	}
}

func TestAssetReferenceScanRejectsInvalidPatternsAndSymlinkLoops(t *testing.T) {
	_, err := collectIndexedAssetRefs[struct{}](t.TempDir(), "[", nil, func(refs []assetPathRef, _ string, _ struct{}) []assetPathRef { return refs }, true)
	if err == nil {
		t.Fatal("accepted malformed category pattern")
	}
	root := t.TempDir()
	source := filepath.Join(root, sourceIndexName)
	if err := os.Symlink(source, source); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := collectAssetPathRefs(root); err == nil || !strings.Contains(err.Error(), "stat ") {
		t.Fatalf("source symlink loop = %v", err)
	}
	opened, err := openPackRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, _, err := inspectExternalAssetPath(opened, root, source); err == nil || !strings.Contains(err.Error(), "symlink at") {
		t.Fatalf("external symlink = %v", err)
	}
}

func TestExternalAssetCollectionRejectsSymlinkCompatibilityRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "Game", "assets", sourceIndexName), `{"backdrops":[{"path":"../../shared/bg.png"}]}`)
	writeTestFile(t, filepath.Join(root, "shared", "bg.png"), "image")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := collectExternalAssetPaths(filepath.Join(alias, "Game"), nil, ""); err == nil || !strings.Contains(err.Error(), "must be a real directory") {
		t.Fatalf("symlink compatibility root = %v", err)
	}
	for _, name := range []string{"shared/bg.png", "../other/bg.png"} {
		if got := rewriteExtAssetZipPath(name, "shared"); got != "" {
			t.Fatalf("unconfigured external path %q = %q", name, got)
		}
	}
}

func TestPackProjectPreservesOutputWhenInputIsUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions enforced for an unprivileged user")
	}
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "assets", "index.json"), `{}`)
	asset := filepath.Join(root, "private.txt")
	writeTestFile(t, asset, "private asset")
	if err := os.Chmod(asset, 0); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "game.zip")
	writeTestFile(t, output, "previous archive")
	if err := PackProject(root, output); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("pack unreadable input error = %v, want permission denied", err)
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "previous archive" {
		t.Fatalf("failed pack changed existing output: %q, %v", got, err)
	}
	if matches, err := filepath.Glob(filepath.Join(filepath.Dir(output), ".game.zip.tmp-*")); err != nil || len(matches) != 0 {
		t.Fatalf("failed pack retained temporary outputs: %v, %v", matches, err)
	}
}

func TestReadJSONFileRejectsUnsafeInput(t *testing.T) {
	root := t.TempDir()
	large := filepath.Join(root, "large.json")
	file, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxPackJSONSize + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := readJSONFile(large, new(any)); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("oversized JSON error = %v", err)
	}

	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := readJSONFile(link, new(any)); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("symlink JSON error = %v", err)
	}
}

func TestReadJSONFileMissingAndMalformed(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ name, content string }{{"missing", ""}, {"malformed", "{"}, {"trailing JSON", "{}{}"}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name+".json")
			if tc.content != "" {
				writeTestFile(t, path, tc.content)
			}
			if err := readJSONFile(path, new(any)); err == nil {
				t.Fatal("accepted invalid JSON input")
			}
		})
	}
}

func TestReadJSONFileReportsUnreadableInput(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions enforced for an unprivileged user")
	}
	name := filepath.Join(t.TempDir(), "protected.json")
	writeTestFile(t, name, `{}`)
	if err := os.Chmod(name, 0); err != nil {
		t.Fatal(err)
	}
	if err := readJSONFile(name, new(any)); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unreadable JSON error = %v, want permission denied", err)
	}
}

func TestReadJSONFileMapsStableReadFailures(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "value.json")
	writeTestFile(t, name, `{}`)

	original, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(name, name+".old"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, name, `{}`)
	if err := readJSONFileWithReader(name, new(any), stablefile.Reader{
		MaxBytes: maxPackJSONSize,
		Expected: original,
	}); err == nil || !strings.Contains(err.Error(), "changed while it was opened") {
		t.Fatalf("identity drift error = %v", err)
	}

	writeTestFile(t, name, `{}`)
	calls := 0
	reader := stablefile.Reader{
		MaxBytes: maxPackJSONSize,
		Lstat: func(path string) (os.FileInfo, error) {
			calls++
			if calls == 2 {
				if err := os.Remove(path); err != nil {
					return nil, err
				}
			}
			return os.Lstat(path)
		},
	}
	if err := readJSONFileWithReader(name, new(any), reader); err == nil || !strings.Contains(err.Error(), "changed while it was read") {
		t.Fatalf("read drift error = %v", err)
	}
}

func (c packCloseResult) Close() error { return c.err }

func (f packWriteFunc) Write(data []byte) (int, error) { return f(data) }

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
