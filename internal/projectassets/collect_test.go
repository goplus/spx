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

package projectassets

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCollectTypedReferences(t *testing.T) {
	projectDir := t.TempDir()
	writeAssetTestFile(t, projectDir, "root/background.png", "image")
	writeAssetTestFile(t, projectDir, "root/music.wav", "sound")
	writeAssetTestFile(t, projectDir, "assets/local.png", "local")
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/hero.png", "hero")
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/jump.wav", "jump")
	writeAssetTestFile(t, projectDir, "assets/fonts/Body/body.ttf", "font")
	writeAssetTestFile(t, projectDir, "assets/index.json", `{
		"description":"../this-is-not-a-resource",
		"backdrops":[{"path":"res://root/background.png"}],
		"bgm":"../root/music.wav",
		"sprites":{"Packed":{"costumeSet":{"path":"res://root/background.png"}}},
		"sounds":{"Packed":{"path":"res://root/music.wav"}}
	}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/index.json", `{"costumes":[{"path":"hero.png"}],"description":"../../../../not-a-resource"}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/index.json", `{"path":"jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/fonts/Body/index.json", `{"faces":[{"path":"body.ttf"}]}`)

	files, err := Collect(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"root/background.png", "root/music.wav"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("Collect() = %#v, want %#v", files, want)
	}
}

func TestResolvePackedOnly(t *testing.T) {
	projectDir := t.TempDir()
	for name := range map[string]string{
		"root/background.png": "image",
		"root/hero.png":       "hero",
		"root/jump.wav":       "sound",
		"root/body.ttf":       "font",
	} {
		writeAssetTestFile(t, projectDir, name, name)
	}
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{
		"backdrops":[{"path":"res://root/background.png"}],
		"sprites":{"Hero":{"costumes":[{"path":"res://root/hero.png"}]}},
		"sounds":{"Jump":{"path":"res://root/jump.wav"}},
		"fonts":{"Body":{"faces":[{"path":"res://root/body.ttf"}]}}
	}`)

	resolved, err := Resolve(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.HasSourceIndex || !resolved.HasPackedIndex {
		t.Fatalf("index presence = source:%v packed:%v, want false/true", resolved.HasSourceIndex, resolved.HasPackedIndex)
	}
	want := []string{"root/background.png", "root/body.ttf", "root/hero.png", "root/jump.wav"}
	if !reflect.DeepEqual(resolved.Files, want) {
		t.Fatalf("Resolve().Files = %#v, want %#v", resolved.Files, want)
	}
}

func TestResolvePackedOverridesStaleSource(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{
		"root/packed-bg.png",
		"root/packed-map.json",
		"root/packed-music.wav",
		"root/packed-hero.png",
		"root/packed-jump.wav",
		"root/packed-font.ttf",
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
		"backdrops":[{"path":"res://root/packed-bg.png"}],
		"bgm":"res://root/packed-music.wav",
		"tilemapPath":"res://root/packed-map.json",
		"sprites":{"Hero":{"costumes":[{"path":"res://root/packed-hero.png"}]}},
		"sounds":{"Jump":{"path":"res://root/packed-jump.wav"}},
		"fonts":{"Packed":{"faces":[{"path":"res://root/packed-font.ttf"}]}}
	}`)

	files, err := Collect(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"root/packed-bg.png",
		"root/packed-font.ttf",
		"root/packed-hero.png",
		"root/packed-jump.wav",
		"root/packed-map.json",
		"root/packed-music.wav",
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("Collect() = %#v, want %#v", files, want)
	}
}

func TestResolveFallsBackToMissingPackedChildren(t *testing.T) {
	projectDir := t.TempDir()
	for _, name := range []string{
		"root/source-bg.png",
		"root/source-hero.png",
		"root/packed-hero.png",
		"root/source-jump.wav",
		"root/packed-jump.wav",
		"root/source-font.ttf",
	} {
		writeAssetTestFile(t, projectDir, name, name)
	}
	writeAssetTestFile(t, projectDir, "assets/index.json", `{
		"backdrops":[{"path":"res://root/source-bg.png"}]
	}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Hero/index.json", `{"costumes":[{"path":"res://root/source-hero.png"}]}`)
	writeAssetTestFile(t, projectDir, "assets/sprites/Packed/index.json", `{"costumes":[{"path":"res://stale/hero.png"}]}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Jump/index.json", `{"path":"res://root/source-jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/sounds/Packed/index.json", `{"path":"res://stale/jump.wav"}`)
	writeAssetTestFile(t, projectDir, "assets/fonts/Source/index.json", `{"faces":[{"path":"res://root/source-font.ttf"}]}`)
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{
		"zorder":["Hero","Packed"],
		"sprites":{"Packed":{"costumes":[{"path":"res://root/packed-hero.png"}]}},
		"sounds":{"Packed":{"path":"res://root/packed-jump.wav"}}
	}`)

	files, err := Collect(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"root/packed-hero.png",
		"root/packed-jump.wav",
		"root/source-bg.png",
		"root/source-font.ttf",
		"root/source-hero.png",
		"root/source-jump.wav",
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("Collect() = %#v, want %#v", files, want)
	}
}

func TestResolveRejectsUnsafePackedEntryName(t *testing.T) {
	projectDir := t.TempDir()
	writeAssetTestFile(t, projectDir, "assets/index_pack.json", `{"sprites":{"../Hero":{}}}`)
	_, err := Resolve(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err == nil || !strings.Contains(err.Error(), `unsafe entry name "../Hero"`) {
		t.Fatalf("Resolve() error = %v, want unsafe packed entry name", err)
	}
}

func TestCollectRejectsUnsafeTypedReferences(t *testing.T) {
	for _, test := range []struct {
		name      string
		reference string
		want      string
	}{
		{name: "escape", reference: "../../outside.png", want: "escapes ProjectDir"},
		{name: "absolute", reference: "/private/host.png", want: "absolute resource path"},
		{name: "host scheme", reference: "file:///private/host.png", want: "unsupported resource path"},
		{name: "missing", reference: "res://missing.png", want: "is missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			projectDir := t.TempDir()
			writeAssetTestFile(t, projectDir, "assets/index.json", `{"backdrops":[{"path":"`+test.reference+`"}]}`)
			_, err := Collect(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Collect() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCollectRejectsSymlinkedReferencedFile(t *testing.T) {
	projectDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeAssetTestFile(t, projectDir, "assets/index.json", `{"backdrops":[{"path":"res://linked.png"}]}`)
	if err := os.Symlink(outside, filepath.Join(projectDir, "linked.png")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := Collect(Config{ProjectDir: projectDir, PackDir: "assets", PackIndex: "index.json"})
	if err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("Collect() error = %v", err)
	}
}

func TestResolveRejectsInvalidConfigurationAndLayout(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		configure  func(*Config)
	}{
		{"empty project", "ProjectDir is empty", func(c *Config) { c.ProjectDir = "" }},
		{"missing project", "inspect ProjectDir", func(c *Config) { c.ProjectDir += ".missing" }},
		{"file project", "not a real directory", func(c *Config) { c.ProjectDir = filepath.Join(c.ProjectDir, "assets", "index.json") }},
		{"empty pack", "PackDir must", func(c *Config) { c.PackDir = "" }},
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
			if _, err := Resolve(cfg); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Resolve = %v, want %q", err, tt.want)
			}
		})
	}
	for _, name := range []string{"index.json", "index_pack.json"} {
		t.Run("directory "+name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "assets", name), 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := Resolve(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "regular non-symlink file") {
				t.Fatalf("directory config = %v", err)
			}
		})
	}
}

func TestResolveReportsMalformedTypedConfiguration(t *testing.T) {
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
		{"sprite costume", "index_pack.json", `{"sprites":{"Hero":{"costumes":[null,{"path":"res://missing"}]}}}`, "is missing"},
		{"sprite set", "index_pack.json", `{"sprites":{"Hero":{"costumeSet":{"path":"res://missing"}}}}`, "is missing"},
		{"sprite MP set", "sprites/Hero/index.json", `{"costumeMPSet":{"path":"res://missing"}}`, "is missing"},
		{"packed sound", "index_pack.json", `{"sounds":{"Bell":{"path":"res://missing"}}}`, "is missing"},
		{"packed font", "index_pack.json", `{"fonts":{"Body":{"faces":[{"path":"res://missing"}]}}}`, "is missing"},
		{"music", "index.json", `{"bgm":"res://missing"}`, "is missing"},
		{"empty resource", "index.json", `{"bgm":"res://"}`, "absolute resource"},
		{"resource scheme", "index.json", `{"bgm":"res://file:private"}`, "unsupported resource"},
		{"backslash", "index.json", `{"bgm":"a\\b"}`, "unsafe resource"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeAssetTestFile(t, root, "assets/index.json", "{}")
			writeAssetTestFile(t, root, "assets/"+tt.file, tt.data)
			if _, err := Resolve(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Resolve = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveSourceSectionsRejectUnsafeLayouts(t *testing.T) {
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
			got, err := Resolve(Config{root, "assets", "index.json"})
			if kind == "ignored files and missing index" {
				if err != nil || len(got.Files) != 0 {
					t.Fatalf("ignored entries: %#v, %v", got, err)
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
	if err := pinRegularFile(root, "assets/config.json"); err != nil {
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
				if err := pinRegularFile(root, "assets/config.json"); !errors.Is(err, os.ErrPermission) {
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
	if err := pinRegularFile(root, "config.json"); err == nil {
		t.Fatal("closed root pin accepted")
	}
}

func TestResolveRejectsOversizedConfig(t *testing.T) {
	root := t.TempDir()
	writeAssetTestFile(t, root, "assets/index.json", strings.Repeat(" ", int(maxConfigBytes)+1))
	if _, err := Resolve(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized config = %v", err)
	}
}

func TestResolveRequiresAtLeastOneIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(Config{root, "assets", "index.json"}); err == nil || !strings.Contains(err.Error(), "contains neither") {
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
