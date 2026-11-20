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
  "backdrops":[{"path":"res/bg.png"}],
  "bgm":"Bell",
  "tilemapPath":"res/maps/map.json",
  "map":{"width":480,"height":360}
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sprites", "Hero", "index.json"), `{
  "costumeSet":{"faceRight":180,"path":"../../res/hero.png","nx":96}
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "sounds", "Bell", "index.json"), `{
  "path":"../../res/audio/ring.wav"
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "fonts", "Custom", "index.json"), `{
  "faces":[{"path":"custom.ttf"}]
}`)
	writeTestFile(t, filepath.Join(projectDir, "assets", "res", "bg.png"), "bg")
	writeTestFile(t, filepath.Join(projectDir, "assets", "res", "hero.png"), "hero")
	writeTestFile(t, filepath.Join(projectDir, "assets", "res", "audio", "ring.wav"), "ring")
	writeTestFile(t, filepath.Join(projectDir, "assets", "res", "maps", "map.json"), "{}")
	writeTestFile(t, filepath.Join(projectDir, "assets", "fonts", "Custom", "custom.ttf"), "font")

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := PackProject(projectDir, zipPath); err != nil {
		t.Fatal(err)
	}

	snapshot := readZipSnapshot(t, zipPath)
	for name, want := range map[string]string{
		"assets/res/bg.png":              "bg",
		"assets/res/hero.png":            "hero",
		"assets/res/audio/ring.wav":      "ring",
		"assets/res/maps/map.json":       "{}",
		"assets/fonts/Custom/custom.ttf": "font",
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

func TestPackProjectPreservesLiteralResFilenames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames cannot contain a colon")
	}
	project := t.TempDir()
	files := map[string]string{
		"assets/sprites/Cat/index.json":    `{"costumes":[{"path":"res:image.png"}]}`,
		"assets/sprites/Cat/res:image.png": "costume",
		"assets/index_pack.json":           `{"fonts":{"Body":{"faces":[{"path":"res:body.ttf"}]}}}`,
		"assets/fonts/Body/res:body.ttf":   "font",
	}
	for name, content := range files {
		writeTestFile(t, filepath.Join(project, name), content)
	}
	archive := filepath.Join(t.TempDir(), "game.zip")
	if err := PackProject(project, archive); err != nil {
		t.Fatal(err)
	}
	contents := readZipSnapshot(t, archive).contents
	for name, want := range files {
		if got := contents[name]; got != want {
			t.Errorf("%s content = %q, want %q", name, got, want)
		}
	}
}

func TestPackProjectRejectsInvalidInputsWithoutCreatingOutput(t *testing.T) {
	for _, tt := range []struct {
		name, path, content, want string
	}{
		{"missing asset index", "main.spx", "onStart => {}", "contains neither"},
		{"malformed asset index", "assets/index.json", "{not-json", "validate asset indexes"},
		{"engine URI", "assets/index.json", `{"backdrops":[{"path":"res://image.png"}]}`, "relative"},
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

func TestPackProjectRejectsReferencesOutsideAssets(t *testing.T) {
	for _, tt := range []struct {
		name, source, packed string
	}{
		{"source", `{"backdrops":[{"path":"../../shared/bg.jpg"}]}`, ""},
		{"packed sprite", "", `{"sprites":{"Hero":{"costumeSet":{"path":"../../../shared/hero.png"}}}}`},
		{"unshadowed source field", `{"tilemapPath":"../map.json"}`, `{"zorder":["Hero"]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			if tt.source != "" {
				writeTestFile(t, filepath.Join(project, "assets", "index.json"), tt.source)
			}
			if tt.packed != "" {
				writeTestFile(t, filepath.Join(project, "assets", "index_pack.json"), tt.packed)
			}
			if err := PackProject(project, filepath.Join(t.TempDir(), "game.zip")); err == nil || !strings.Contains(err.Error(), "asset directory") {
				t.Fatalf("PackProject error = %v, want asset directory rejection", err)
			}
		})
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

func TestPackProjectArchivesRegularFileNamedLikeIndexSection(t *testing.T) {
	projectDir := t.TempDir()
	writeTestFile(t, filepath.Join(projectDir, "assets", "index.json"), "{}")
	writeTestFile(t, filepath.Join(projectDir, "assets", "sounds"), "ordinary asset")
	archive := filepath.Join(t.TempDir(), "game.zip")
	if err := PackProject(projectDir, archive); err != nil {
		t.Fatal(err)
	}
	if got := readZipSnapshot(t, archive).contents["assets/sounds"]; got != "ordinary asset" {
		t.Fatalf("archived assets/sounds = %q", got)
	}
}

func TestWriteZipEntriesRejectsFileReplacedAfterCollection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "asset.txt")
	writeTestFile(t, filePath, "inside")
	root, err := openPackRoot(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	entries, err := collectProjectEntries(tmpDir, filepath.Join(tmpDir, "game.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].relPath != "asset.txt" {
		t.Fatalf("collected entries = %+v, want asset.txt relative to its project root", entries)
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
	err = writeZipEntries(zipWriter, root, entries)
	_ = zipWriter.Close()
	if err == nil || !strings.Contains(err.Error(), "changed after collection") {
		t.Fatalf("writeZipEntries() error = %v, want replaced-file rejection", err)
	}
}

func TestWriteZipEntriesRejectsInvalidCollectedNames(t *testing.T) {
	for _, tc := range []struct {
		name    string
		zipName string
		wantErr string
	}{
		{name: "drive absolute", zipName: "C:/asset.txt", wantErr: "unsafe zip name"},
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
			entries, err := collectProjectEntries(projectDir, filepath.Join(projectDir, "game.zip"))
			if err != nil {
				t.Fatal(err)
			}
			for i := range entries {
				entries[i].zipName = tc.zipName
			}
			zipWriter := zip.NewWriter(io.Discard)
			err = writeZipEntries(zipWriter, root, entries)
			_ = zipWriter.Close()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("writeZipEntries() error = %v, want %q", err, tc.wantErr)
			}
			if _, err := root.Stat("."); err != nil {
				t.Fatalf("writeZipEntries() closed the caller's root: %v", err)
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

func TestWriteZipEntriesRechecksCollectedFile(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "asset.txt")
	writeTestFile(t, file, "asset")
	root, err := openPackRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	entries, err := collectProjectEntries(directory, filepath.Join(directory, "game.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file, file+".original"); err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(io.Discard)
	defer writer.Close()
	if err := writeZipEntries(writer, root, entries); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "before packing") {
		t.Fatalf("deleted collected file error = %v", err)
	}
	if err := os.Mkdir(file, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeZipEntries(writer, root, entries); err == nil || !strings.Contains(err.Error(), "changed type after collection") {
		t.Fatalf("collected file replaced by directory: %v", err)
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("packing failure closed the caller's root: %v", err)
	}
}

func TestPackedAssetIndexesRejectMalformedCategories(t *testing.T) {
	for _, tt := range []struct{ name, packed, source, want string }{
		{"invalid JSON", `{`, "", "decode"},
		{"invalid source JSON", `{}`, `{`, "index.json"},
		{"invalid project fields", `{"map":[]}`, "", "root"},
		{"invalid sprites", `{"sprites":42}`, "", "sprites"},
		{"invalid sounds", `{"sounds":42}`, "", "sounds"},
		{"invalid fonts", `{"fonts":42}`, "", "fonts"},
		{"face outside font family", `{"fonts":{"Body":{"faces":[{"path":"../Other/regular.ttf"}]}}}`, "", "font face"},
		{"invalid packed name with no resources", `{"sprites":{"..":{}}}`, "", "unsafe entry name"},
		{"invalid sprite entry", `{"sprites":{"Hero":{"costumes":42}}}`, "", "Hero"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "assets", "index_pack.json"), tt.packed)
			if tt.source != "" {
				writeTestFile(t, filepath.Join(root, "assets", "index.json"), tt.source)
			}
			if err := PackProject(root, filepath.Join(t.TempDir(), "game.zip")); err == nil || !strings.Contains(err.Error(), tt.want) {
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
		{"missing project", "missing", "missing", false},
		{"file project", "file", "must be a real directory", false},
		{"missing assets", ".", "is missing", false},
		{"file assets", "assets", "non-directory", false},
		{"directory index", "assets/index.json", "regular non-symlink", true},
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
			if err := PackProject(project, filepath.Join(t.TempDir(), "game.zip")); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validation error = %v, want %q", err, tc.want)
			}
		})
	}

}

func TestPackProjectChecksSourceChildrenInBracketedFolder(t *testing.T) {
	for _, category := range []string{"sprites", "sounds", "fonts"} {
		t.Run(category, func(t *testing.T) {
			project := filepath.Join(t.TempDir(), "Game [1]")
			writeTestFile(t, filepath.Join(project, "assets", "index.json"), "{}")
			writeTestFile(t, filepath.Join(project, "assets", category, "Broken", "index.json"), "{")
			if err := PackProject(project, filepath.Join(t.TempDir(), "game.zip")); err == nil || !strings.Contains(err.Error(), "Broken") {
				t.Fatalf("PackProject error = %v, want malformed child index", err)
			}
		})
	}
}

func TestCollectProjectEntriesSkipsBuildMetadataAndOutputAliases(t *testing.T) {
	rootDir := t.TempDir()
	for _, name := range []string{".git", "project/generated.go", "assets/project/cache", "assets/.git/config", "assets/keep.png", "assets/keep.png.import", "nested/keep.txt", "archive.zip"} {
		writeTestFile(t, filepath.Join(rootDir, name), name)
	}
	output := filepath.Join(rootDir, "archive.zip")
	if err := os.Link(output, filepath.Join(rootDir, "alias.zip")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	entries, err := collectProjectEntries(rootDir, output)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		got = append(got, filepath.ToSlash(entry.relPath))
	}
	if want := []string{"assets", "assets/keep.png", "nested", "nested/keep.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("collected = %q, want %q", got, want)
	}
	for _, tc := range []struct{ name, base, destination string }{
		{"missing project", filepath.Join(rootDir, "missing"), output},
		{"invalid destination", rootDir, "invalid\x00path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := collectProjectEntries(tc.base, tc.destination); err == nil {
				t.Fatal("accepted invalid walk")
			}
		})
	}
}

func TestWriteZipEntriesDetectsWriteFailuresAndConcurrentFileChanges(t *testing.T) {
	for _, kind := range []string{"output failure", "file changed", "file replaced before opening", "file header too long", "directory header too long"} {
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
			entry := archiveEntry{zipName: filepath.ToSlash(relative), info: info, relPath: relative}
			if strings.Contains(kind, "header too long") {
				entry.zipName = strings.Repeat("a", 1<<16)
			}
			if kind == "file replaced before opening" {
				// Flush the ZIP header before StableReader checks the collected file.
				entry.zipName = strings.Repeat("a", 8192)
			}
			writeErr := errors.New("archive destination failed")
			writes := 0
			writer := zip.NewWriter(packWriteFunc(func(data []byte) (int, error) {
				writes++
				if kind == "output failure" {
					return 0, writeErr
				}
				if kind == "file replaced before opening" && writes == 1 {
					if err := os.Rename(name, name+".original"); err != nil {
						t.Fatal(err)
					}
					writeTestFile(t, name, "replacement")
				}
				if kind == "file changed" && writes == 1 {
					if err := os.Truncate(name, 1); err != nil {
						t.Fatal(err)
					}
				}
				return len(data), nil
			}))
			err = writeZipEntries(writer, root, []archiveEntry{entry})
			_ = writer.Close()
			switch kind {
			case "output failure":
				if !errors.Is(err, writeErr) {
					t.Fatalf("write failure = %v", err)
				}
			case "file replaced before opening":
				if err == nil || !strings.Contains(err.Error(), "changed while opening") {
					t.Fatalf("replaced file error = %v", err)
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
	for _, name := range []string{"", "/absolute", "C:/a/b", `\\host\share`} {
		if validZipEntryName(name) {
			t.Fatalf("accepted zip name %q", name)
		}
	}
}

func TestPackProjectAllowsMissingBinaryForPackedSprite(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, filepath.Join(project, "assets", "index_pack.json"), `{"sprites":{"Hero":{"costumes":[null],"costumeMPSet":{"path":"multipart.png"}}}}`)
	writeTestFile(t, filepath.Join(project, "assets", "sprites", "Hero", "index.json"), "invalid shadowed source")
	if err := PackProject(project, filepath.Join(t.TempDir(), "game.zip")); err != nil {
		t.Fatal(err)
	}
}

func TestPackProjectRejectsUnrepresentablePhysicalName(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("backslash separates Windows paths")
	}
	project := t.TempDir()
	writeTestFile(t, filepath.Join(project, "assets", "index.json"), "{}")
	writeTestFile(t, filepath.Join(project, "assets", `sprite\alias.png`), "image")
	if err := PackProject(project, filepath.Join(t.TempDir(), "game.zip")); err == nil || !strings.Contains(err.Error(), "unsafe zip name") {
		t.Fatalf("PackProject error = %v, want unsafe zip name", err)
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
	if _, err := collectProjectEntries(root, filepath.Join(root, "archive.zip")); err == nil || !strings.Contains(err.Error(), "regular file or directory") {
		t.Fatalf("socket project entry = %v", err)
	}
}

func TestPackProjectPreservesOutputWhenInputIsUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions enforced for an unprivileged user")
	}
	for _, blocked := range []string{".", "private.txt"} {
		t.Run(blocked, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "assets", "index.json"), "{}")
			writeTestFile(t, filepath.Join(root, "private.txt"), "private asset")
			unreadable := filepath.Join(root, blocked)
			if err := os.Chmod(unreadable, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(unreadable, 0700); err != nil {
					t.Error(err)
				}
			})
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
		})
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
