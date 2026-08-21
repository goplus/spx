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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
