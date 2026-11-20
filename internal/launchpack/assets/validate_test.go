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

package assets

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestValidateTypedReferences(t *testing.T) {
	projectDir := t.TempDir()
	writeAssetTestFile(t, projectDir, "assets/root/background.png", "image")
	writeAssetTestFile(t, projectDir, "assets/root/music.wav", "sound")
	writeAssetTestFile(t, projectDir, "assets/maps/forest/tilemap.json", "{}")
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/hero.png", "hero")
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/jump.wav", "jump")
	writeAssetTestFile(t, projectDir, "assets/fonts/Body/body.ttf", "font")
	writeAssetTestFile(t, projectDir, "assets/index.json", `{
		"description":"../this-is-not-a-resource",
		"tilemapPath":"maps/forest",
		"backdrops":[{"path":"root/background.png"}],
		"bgm":"Jump",
		"sprites":{"Packed":{"costumeSet":{"path":"../../root/background.png"}}},
		"sounds":{"Packed":{"path":"../root/music.wav"}}
	}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/index.json", `{"costumes":[{"path":"hero.png"}],"description":"../../../../not-a-resource"}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/index.json", `{"path":"jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/fonts/Body/index.json", `{"faces":[{"path":"body.ttf"}]}`)

	if err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBackgroundSoundIsALogicalName(t *testing.T) {
	for _, index := range []string{"index.json", "index_pack.json"} {
		t.Run(index, func(t *testing.T) {
			project := t.TempDir()
			writeAssetTestFile(t, project, "assets/"+index, `{"bgm":"铃 声"}`)
			cfg := Config{ProjectDir: project, PackDir: "assets", PackIndex: "index.json"}
			if err := Validate(cfg); err != nil {
				t.Fatalf("background sound lookup must be deferred: %v", err)
			}
			writeAssetTestFile(t, project, "assets/sounds/铃 声/index.json", `{"path":"ring.wav"}`)
			writeAssetTestFile(t, project, "assets/sounds/铃 声/ring.wav", "ring")
			if err := Validate(cfg); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidatePackedOnly(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{
		"assets/root/background.png", "assets/root/hero.png",
		"assets/root/jump.wav", "assets/fonts/Body/body.ttf",
	} {
		writeAssetTestFile(t, projectDir, name, name)
	}
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{
		"backdrops":[{"path":"root/background.png"}],
		"sprites":{"Hero":{"costumes":[{"path":"../../root/hero.png"}]}},
		"sounds":{"Jump":{"path":"../../root/jump.wav"}},
		"fonts":{"Body":{"faces":[{"path":"body.ttf"}]}}
	}`)

	if err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackedLogicalNameWithoutPhysicalDirectory(t *testing.T) {
	project := t.TempDir()
	writeAssetTestFile(t, project, "assets/audio/ring.wav", "sound")
	writeAssetTestFile(t, project, "assets/index_pack.json", `{
		"bgm":"铃\\声",
		"sounds":{"铃\\声":{"path":"../../audio/ring.wav"}}
	}`)
	if err := Validate(Config{project, "assets", "index_pack.json"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateExplicitPackedIndexMergesSource(t *testing.T) {
	project := t.TempDir()
	cfg := Config{project, "assets", "index_pack.json"}
	writeAssetTestFile(t, project, "assets/index.json", `{"backdrops":[{"path":"unavailable.png"}]}`)
	writeAssetTestFile(t, project, "assets/index_pack.json", `{}`)
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "unavailable.png") {
		t.Fatalf("inherited backdrop error = %v", err)
	}
	writeAssetTestFile(t, project, "assets/index_pack.json", `{"backdrops":[]}`)
	if err := Validate(cfg); err != nil {
		t.Fatalf("packed backdrop override: %v", err)
	}
}

func TestValidatePackedOverridesStaleSource(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{
		"assets/root/packed-bg.png",
		"assets/root/packed-map.json",
		"assets/root/packed-hero.png",
		"assets/root/packed-jump.wav",
		"assets/fonts/Packed/packed-font.ttf",
	} {
		writeAssetTestFile(t, projectDir, name, name)
	}
	writeAssetTestFile(t, projectDir, "assets/index.json", `{
		"backdrops":[{"path":"res://stale/bg.png"}],
		"bgm":"res://stale/music.wav",
		"tilemapPath":"res://stale/map.json"
	}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/index.json", `{not source json`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/index.json", `{"path":"res://stale/jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/fonts/Stale/index.json", `{not source json`)
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{
		"backdrops":[{"path":"root/packed-bg.png"}],
		"bgm":"Jump",
		"tilemapPath":"root/packed-map.json",
		"sprites":{"Hero":{"costumes":[{"path":"../../root/packed-hero.png"}]}},
		"sounds":{"Jump":{"path":"../../root/packed-jump.wav"}},
		"fonts":{"Packed":{"faces":[{"path":"packed-font.ttf"}]}}
	}`)

	if err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateFallsBackToMissingPackedChildren(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{
		"assets/root/source-bg.png",
		"assets/root/source-hero.png",
		"assets/root/packed-hero.png",
		"assets/root/source-jump.wav",
		"assets/root/packed-jump.wav",
		"assets/fonts/Source/source-font.ttf",
	} {
		writeAssetTestFile(t, projectDir, name, name)
	}
	writeAssetTestFile(t, projectDir, "assets/index.json", `{
		"backdrops":[{"path":"root/source-bg.png"}]
	}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/index.json", `{"costumes":[{"path":"../../root/source-hero.png"}]}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Packed/index.json", `{"costumes":[{"path":"res://stale/hero.png"}]}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/index.json", `{"path":"../../root/source-jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Packed/index.json", `{"path":"res://stale/jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/fonts/Source/index.json", `{"faces":[{"path":"source-font.ttf"}]}`)
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{
		"zorder":["Hero","Packed"],
		"sprites":{"Packed":{"costumes":[{"path":"../../root/packed-hero.png"}]}},
		"sounds":{"Packed":{"path":"../../root/packed-jump.wav"}}
	}`)

	if err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnsafePackedEntryName(t *testing.T) {
	projectDir := t.TempDir()
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{"sprites":{"../Hero":{}}}`)
	err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err == nil || !strings.Contains(err.Error(), `unsafe entry name "../Hero"`) {
		t.Fatalf("Validate() error = %v, want unsafe packed entry name", err)
	}
}

func TestValidateRejectsUnsafeTypedReferences(t *testing.T) {
	for _, test := range []struct {
		name      string
		reference string
		want      string
	}{
		{name: "escape", reference: "../../outside.png", want: "asset directory"},
		{name: "host scheme", reference: "file:///private/host.png", want: "unsupported resource path"},
		{name: "missing", reference: "missing.png", want: "is missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeAssetTestFile(t, projectDir, "assets/index.json", `{"backdrops":[{"path":"`+test.reference+`"}]}`)
			err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestPackersRejectAbsoluteReferencesBeforeJoining(t *testing.T) {
	for _, tt := range []struct {
		name, index, config string
		files               []string
	}{
		{"source backdrop", "index.json", `{"backdrops":[{"path":"/a/b"}]}`, []string{"a/b"}},
		{"packed tilemap", "index_pack.json", `{"tilemapPath":"/a/b"}`, []string{"a/b/tilemap.json"}},
		{"packed font", "index_pack.json", `{"fonts":{"Body":{"faces":[{"path":"/a/b"}]}}}`, []string{"a/b", "fonts/Body/a/b"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			writeAssetTestFile(t, project, "assets/"+tt.index, tt.config)
			for _, file := range tt.files {
				writeAssetTestFile(t, project, "assets/"+file, "{}")
			}
			root, err := os.OpenRoot(project)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if err := ValidateLegacy(root); err == nil || !strings.Contains(err.Error(), "/a/b") {
				t.Fatalf("legacy validation = %v, want rejection of /a/b", err)
			}
			if err := Validate(Config{project, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "/a/b") {
				t.Fatalf("launcher validation = %v, want rejection of /a/b", err)
			}
		})
	}
}

func TestValidateRejectsSymlinkedReferencedFile(t *testing.T) {
	projectDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeAssetTestFile(t, projectDir, "assets/index.json", `{"backdrops":[{"path":"linked.png"}]}`)
	if err := os.Symlink(outside, filepath.Join(projectDir, "assets", "linked.png")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	err := Validate(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidConfigurationAndLayout(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		configure  func(*Config)
	}{
		{"empty project", "ProjectDir is empty", func(c *Config) { c.ProjectDir = "" }},
		{"missing project", "inspect ProjectDir", func(c *Config) { c.ProjectDir += ".missing" }},
		{"file project", "not a real directory", func(c *Config) { c.ProjectDir = filepath.Join(c.ProjectDir, "assets", "index.json") }},
		{"empty pack", "PackDir must", func(c *Config) { c.PackDir = "" }},
		{"drive-absolute pack", "PackDir must", func(c *Config) { c.PackDir = "C:/a/b" }},
		{"escaping pack", "PackDir must", func(c *Config) { c.PackDir = "../assets" }},
		{"nested index", "PackIndex must", func(c *Config) { c.PackIndex = "child/index.json" }},
		{"missing pack", "PackDir \"missing\" is missing", func(c *Config) { c.PackDir = "missing" }},
		{"file pack", "non-directory component", func(c *Config) { c.PackDir = "assets/index.json" }},
		{"missing packed index", "required config", func(c *Config) { c.PackIndex = "index_pack.json" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeAssetTestFile(t, root, "assets/index.json", "{}")
			cfg := Config{ProjectDir: root, PackDir: "assets", PackIndex: "index.json"}
			tt.configure(&cfg)
			if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate = %v, want %q", err, tt.want)
			}
		})
	}
	for _, name := range []string{"index.json", "index_pack.json"} {
		t.Run("directory "+name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "assets", name), 0700); err != nil {
				t.Fatal(err)
			}
			if err := Validate(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "regular non-symlink file") {
				t.Fatalf("directory config = %v", err)
			}
		})
	}
}

func TestValidateReportsMalformedTypedConfiguration(t *testing.T) {
	for _, tt := range []struct{ name, file, data, want string }{
		{"source JSON", "index.json", "{", "decode"},
		{"packed JSON", "index_pack.json", "{", "decode"},
		{"source root type", "index.json", `{"backdrops":1}`, "merged project"},
		{"packed root type", "index_pack.json", `{"backdrops":1}`, "merged project"},
		{"packed sprite section", "index_pack.json", `{"sprites":1}`, `section "sprites"`},
		{"packed sound type", "index_pack.json", `{"sounds":{"Bell":1}}`, `entry "Bell"`},
		{"packed font type", "index_pack.json", `{"fonts":{"Body":1}}`, `entry "Body"`},
		{"source sprite", "sprites/Hero/index.json", "{", "decode"},
		{"source sound", "sounds/Bell/index.json", "{", "decode"},
		{"source font", "fonts/Body/index.json", "{", "decode"},
		{"sprite costume", "index_pack.json", `{"sprites":{"Hero":{"costumes":[null,{"path":"missing"}]}}}`, "is missing"},
		{"sprite set", "index_pack.json", `{"sprites":{"Hero":{"costumeSet":{"path":"missing"}}}}`, "is missing"},
		{"sprite MP set", "sprites/Hero/index.json", `{"costumeMPSet":{"path":"missing"}}`, "is missing"},
		{"packed sound", "index_pack.json", `{"sounds":{"Bell":{"path":"missing"}}}`, "is missing"},
		{"empty font path", "fonts/Body/index.json", `{"faces":[{"path":""}]}`, "must not be empty"},
		{"face outside font family", "fonts/Body/index.json", `{"faces":[{"path":"../Other/regular.ttf"}]}`, "font face"},
		{"packed font", "index_pack.json", `{"fonts":{"Body":{"faces":[{"path":"missing"}]}}}`, "is missing"},
		{"invalid sound name", "index.json", `{"bgm":"sounds/Bell"}`, "segment"},
		{"empty resource", "index.json", `{"tilemapPath":"res://"}`, "unsupported resource"},
		{"resource scheme", "index.json", `{"tilemapPath":"res://file:private"}`, "unsupported resource"},
		{"literal res filename", "index.json", `{"backdrops":[{"path":"res:image.png"}]}`, "unsupported resource"},
		{"backslash", "index.json", `{"tilemapPath":"a\\b"}`, "asset directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeAssetTestFile(t, root, "assets/index.json", "{}")
			writeAssetTestFile(t, root, "assets/"+tt.file, tt.data)
			if err := Validate(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateSourceSectionsRejectUnsafeLayouts(t *testing.T) {
	for _, kind := range []string{"section file", "index directory", "symlink child", "unsafe child name", "ignored files and missing index"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			writeAssetTestFile(t, root, "assets/index.json", "{}")
			switch kind {
			case "section file":
				writeAssetTestFile(t, root, "assets/sprites", "blocked")
			case "index directory":
				if err := os.MkdirAll(filepath.Join(root, "assets/sprites/Hero/index.json"), 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink child":
				writeAssetTestFile(t, root, "assets/sprites/target", "ignored")
				if err := os.Symlink("target", filepath.Join(root, "assets/sprites/Hero")); err != nil {
					t.Skip(err)
				}
			case "unsafe child name":
				if filepath.Separator == '\\' {
					t.Skip("backslash is a path separator on this platform")
				}
				writeAssetTestFile(t, root, `assets/sprites/Hero\Alias/index.json`, "{}")
			case "ignored files and missing index":
				writeAssetTestFile(t, root, "assets/sprites/notes.txt", "ignored")
				if err := os.Mkdir(filepath.Join(root, "assets/sprites/Hero"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := Validate(Config{root, "assets", "index.json"})
			if kind == "ignored files and missing index" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unsafe section accepted")
			} else if kind == "unsafe child name" && !strings.Contains(err.Error(), "unsafe entry name") {
				t.Fatalf("unsafe child name error = %v", err)
			}
		})
	}
}

func TestPinnedProjectRootSurvivesPathReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit renaming directories held by os.Root")
	}
	container := t.TempDir()
	rootDir := filepath.Join(container, "project")
	writeAssetTestFile(t, rootDir, "assets/config.json", "original")
	root, err := openProjectRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if err := os.Rename(rootDir, filepath.Join(container, "original-project")); err != nil {
		t.Fatal(err)
	}
	writeAssetTestFile(t, rootDir, "assets/replacement.json", "replacement")

	data, found, err := readPinnedFile(root, "assets/config.json")
	if err != nil || !found || string(data) != "original" {
		t.Fatalf("pinned file = %q, %v, %v, want original contents", data, found, err)
	}
	entries, found, err := readPinnedDir(root, "assets")
	if err != nil || !found || len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("pinned directory = %v, %v, %v, want original config.json", entries, found, err)
	}
	if err := verifyRegularFile(root, "assets/config.json"); err != nil {
		t.Fatalf("pin original file: %v", err)
	}
	if data, found, err := readPinnedFile(root, "assets/replacement.json"); err != nil || found || data != nil {
		t.Fatalf("replacement leaked into pinned root: %q, %v, %v", data, found, err)
	}
}

func TestPinnedAssetsReportPermissionErrors(t *testing.T) {
	for _, kind := range []string{"config file", "asset directory", "project root"} {
		t.Run(kind, func(t *testing.T) {
			rootDir := t.TempDir()
			writeAssetTestFile(t, rootDir, "assets/config.json", "{}")
			root, err := openProjectRoot(rootDir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			name := rootDir
			switch kind {
			case "config file":
				name = filepath.Join(rootDir, "assets", "config.json")
			case "asset directory":
				name = filepath.Join(rootDir, "assets")
			}
			denyAssetRead(t, name)
			switch kind {
			case "config file":
				data, found, err := readPinnedFile(root, "assets/config.json")
				if !errors.Is(err, os.ErrPermission) || found || data != nil {
					t.Fatalf("unreadable file = %q, %v, %v", data, found, err)
				}
				if err := verifyRegularFile(root, "assets/config.json"); !errors.Is(err, os.ErrPermission) {
					t.Fatalf("pin unreadable file = %v", err)
				}
			case "asset directory":
				entries, found, err := readPinnedDir(root, "assets")
				if !errors.Is(err, os.ErrPermission) || found || entries != nil {
					t.Fatalf("unreadable directory = %v, %v, %v", entries, found, err)
				}
			case "project root":
				opened, err := openProjectRoot(rootDir)
				if opened != nil {
					opened.Close()
				}
				if !errors.Is(err, os.ErrPermission) {
					t.Fatalf("unreadable project root = %v", err)
				}
			}
		})
	}
}

func TestPinnedAssetPathsAndSnapshots(t *testing.T) {
	rootDir := t.TempDir()
	writeAssetTestFile(t, rootDir, "config.json", "{}")
	root, err := openProjectRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	for _, name := range []string{"", ".", "..", "../outside", "/absolute", "a\\b", "a//b", "a\x00b"} {
		if _, err := inspectPath(root, name, false); err == nil {
			t.Errorf("unsafe path %q accepted", name)
		}
	}
	before, err := inspectPath(root, "config.json", false)
	if err != nil {
		t.Fatal(err)
	}
	if samePathSnapshot(before, nil) {
		t.Fatal("different snapshot lengths match")
	}
	writeAssetTestFile(t, rootDir, "other.json", "{}")
	other, err := inspectPath(root, "other.json", false)
	if err != nil {
		t.Fatal(err)
	}
	if samePathSnapshot(before, other) {
		t.Fatal("different files match")
	}
	name := filepath.Join(rootDir, "config.json")
	t.Cleanup(func() {
		if err := os.Chmod(name, 0600); err != nil {
			t.Error(err)
		}
	})
	for _, change := range []struct {
		name   string
		mutate func() error
	}{
		{"size", func() error { return os.WriteFile(name, []byte("changed"), 0600) }},
		{"mode", func() error { return os.Chmod(name, 0400) }},
		{"modification time", func() error { return os.Chtimes(name, time.Unix(42, 0), time.Unix(42, 0)) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			before, err := inspectPath(root, "config.json", false)
			if err != nil {
				t.Fatal(err)
			}
			if err := change.mutate(); err != nil {
				t.Fatal(err)
			}
			after, err := inspectPath(root, "config.json", false)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before[0], after[0]) {
				t.Fatal("fixture replaced the file instead of changing metadata")
			}
			if samePathSnapshot(before, after) {
				t.Fatal("changed file metadata matches")
			}
		})
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readPinnedFile(root, "config.json"); err == nil {
		t.Fatal("closed root read accepted")
	}
	if _, _, err := readPinnedDir(root, "assets"); err == nil {
		t.Fatal("closed root directory read accepted")
	}
	if err := verifyRegularFile(root, "config.json"); err == nil {
		t.Fatal("closed root pin accepted")
	}
}

func TestValidateRejectsOversizedConfig(t *testing.T) {
	root := t.TempDir()
	writeAssetTestFile(t, root, "assets/index.json", strings.Repeat(" ", int(maxConfigBytes)+1))
	if err := Validate(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized config = %v", err)
	}
}

func TestValidateRequiresAtLeastOneIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "contains neither") {
		t.Fatalf("missing indexes = %v", err)
	}
}

func writeAssetTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func denyAssetRead(t *testing.T, name string) {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(name, info.Mode().Perm()); err != nil {
			t.Error(err)
		}
	})
	probe, err := os.Open(name)
	if err == nil {
		probe.Close()
		t.Skip("filesystem or user permits reading mode-000 paths")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("permission probe = %v", err)
	}
}
