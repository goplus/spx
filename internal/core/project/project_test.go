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

package project

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v3/fs"
	_ "github.com/goplus/spx/v3/fs/asset"
)

type fakeGdDir struct {
	path string
}

type localDir struct {
	base string
}

func writeProjectFile(t *testing.T, dir, relPath, content string) {
	t.Helper()

	filePath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (d fakeGdDir) Open(string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (d fakeGdDir) Close() error {
	return nil
}

func (d fakeGdDir) GetPath() string {
	return d.path
}

func (d localDir) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(d.base, name))
}

func (d localDir) Close() error {
	return nil
}

func (d localDir) ReadDir(name string) ([]spxfs.DirEntry, error) {
	entries, err := os.ReadDir(filepath.Join(d.base, filepath.FromSlash(name)))
	if err != nil {
		return nil, err
	}
	result := make([]spxfs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, spxfs.DirEntry{Name: entry.Name(), IsDir: entry.IsDir()})
	}
	return result, nil
}

func TestLoadProjectFontsFromSourceDirectory(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"fontPreferences":["basic chinese", "Scratch", "default"]}`)
	writeProjectFile(t, dir, "fonts/Scratch/index.json", `{"faces":[{"path":"Scratch.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Scratch/Scratch.ttf", "scratch-font")
	writeProjectFile(t, dir, "fonts/Basic Chinese/index.json", `{"faces":[{"path":"basic.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Basic Chinese/basic.ttf", "basic-font")

	loaded, err := LoadBuilderProject(localDir{base: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantFamilies := []ProjectFontFamily{
		{Name: "Basic Chinese", Faces: []ProjectFontFace{{Path: "fonts/Basic Chinese/basic.ttf"}}},
		{Name: "Scratch", Faces: []ProjectFontFace{{Path: "fonts/Scratch/Scratch.ttf"}}},
	}
	if !reflect.DeepEqual(loaded.Fonts.Families, wantFamilies) {
		t.Fatalf("families = %#v, want %#v", loaded.Fonts.Families, wantFamilies)
	}
	wantPreferences := []string{"Basic Chinese", "Scratch", "default"}
	if !reflect.DeepEqual(loaded.Fonts.Preferences, wantPreferences) {
		t.Fatalf("preferences = %#v, want %#v", loaded.Fonts.Preferences, wantPreferences)
	}
}

func TestOpenBuilderResourcesUsesPackedFontsCatalog(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"fontPreferences":["Source"]}`)
	writeProjectFile(t, dir, "index_pack.json", `{
		"fontPreferences":["Packed", "default"],
		"fonts":{"Packed":{"faces":[{"path":"packed.ttf"}]}}
	}`)
	writeProjectFile(t, dir, "fonts/Packed/packed.ttf", "packed-font")
	writeProjectFile(t, dir, "fonts/Source/index.json", `{"faces":[{"path":"source.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Source/source.ttf", "source-font")

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	wantFamilies := []ProjectFontFamily{{Name: "Packed", Faces: []ProjectFontFace{{Path: "fonts/Packed/packed.ttf"}}}}
	if !reflect.DeepEqual(opened.Fonts.Families, wantFamilies) {
		t.Fatalf("families = %#v, want packed catalog %#v", opened.Fonts.Families, wantFamilies)
	}
	if want := []string{"Packed", "default"}; !reflect.DeepEqual(opened.Fonts.Preferences, want) {
		t.Fatalf("preferences = %#v, want %#v", opened.Fonts.Preferences, want)
	}
}

func TestOpenBuilderResourcesScansFontsWhenPackedCatalogMissing(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"fontPreferences":["Source"]}`)
	writeProjectFile(t, dir, "index_pack.json", `{"zorder":[]}`)
	writeProjectFile(t, dir, "fonts/Source/index.json", `{"faces":[{"path":"source.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Source/source.ttf", "source-font")

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	want := []ProjectFontFamily{{Name: "Source", Faces: []ProjectFontFace{{Path: "fonts/Source/source.ttf"}}}}
	if !reflect.DeepEqual(opened.Fonts.Families, want) {
		t.Fatalf("families = %#v, want source scan %#v", opened.Fonts.Families, want)
	}
}

func TestLoadProjectFontsValidation(t *testing.T) {
	t.Run("reserved family", func(t *testing.T) {
		dir := t.TempDir()
		writeProjectFile(t, dir, "fonts/DEFAULT/index.json", `{"faces":[{"path":"font.ttf"}]}`)
		writeProjectFile(t, dir, "fonts/DEFAULT/font.ttf", "font")
		_, err := LoadProjectFonts(localDir{base: dir}, nil)
		if err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("error = %v, want reserved family error", err)
		}
	})

	t.Run("escaping face", func(t *testing.T) {
		dir := t.TempDir()
		writeProjectFile(t, dir, "fonts/Unsafe/index.json", `{"faces":[{"path":"../font.ttf"}]}`)
		_, err := LoadProjectFonts(localDir{base: dir}, nil)
		if err == nil || !strings.Contains(err.Error(), "escapes") {
			t.Fatalf("error = %v, want escaping face error", err)
		}
	})

	t.Run("case folded duplicate", func(t *testing.T) {
		dir := t.TempDir()
		writeProjectFile(t, dir, "index_pack.json", `{
			"fonts": {
				"Latin": {"faces":[{"path":"font.ttf"}]},
				"latin": {"faces":[{"path":"font.ttf"}]}
			}
		}`)
		writeProjectFile(t, dir, "fonts/Latin/font.ttf", "font")
		_, err := OpenBuilderResources(localDir{base: dir}, nil)
		if err == nil || !strings.Contains(err.Error(), "ASCII case folding") {
			t.Fatalf("error = %v, want case-folded duplicate error", err)
		}
	})

	t.Run("exactly one face", func(t *testing.T) {
		dir := t.TempDir()
		writeProjectFile(t, dir, "fonts/Empty/index.json", `{"faces":[]}`)
		_, err := LoadProjectFonts(localDir{base: dir}, nil)
		if err == nil || !strings.Contains(err.Error(), "exactly one face") {
			t.Fatalf("error = %v, want face count error", err)
		}
	})
}

func TestResolveFontPreferences(t *testing.T) {
	value := []string{"Scratch", "basic chinese", "Emoji", "default"}
	families := []ProjectFontFamily{{Name: "Scratch"}, {Name: "Basic Chinese"}, {Name: "Emoji"}}
	got, err := ResolveFontPreferences(value, families)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Scratch", "Basic Chinese", "Emoji", "default"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveFontPreferences() = %#v, want %#v", got, want)
	}
	if got, err := ResolveFontPreferences(nil, families); err != nil || !reflect.DeepEqual(got, []string{"default"}) {
		t.Fatalf("nil preferences = %#v, %v; want default", got, err)
	}
	if got, err := ResolveFontPreferences([]string{}, families); err != nil || len(got) != 0 || got == nil {
		t.Fatalf("empty preferences = %#v, %v; want explicit empty slice", got, err)
	}
	for _, value := range [][]string{{""}, {"unknown"}, {"Scratch", "scratch"}} {
		if _, err := ResolveFontPreferences(value, families); err == nil {
			t.Fatalf("ResolveFontPreferences(%#v) succeeded, want validation error", value)
		}
	}
}

func TestAssetDirFromResource(t *testing.T) {
	if got, ok := AssetDirFromResource("assets"); !ok || got != "assets" {
		t.Fatalf("string asset dir = %q, %v", got, ok)
	}
	if got, ok := AssetDirFromResource(fakeGdDir{path: "res://assets/"}); !ok || got != "res://assets" {
		t.Fatalf("gd asset dir = %q, %v", got, ok)
	}
}

func TestResourceDirAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.json")
	if err := os.WriteFile(indexPath, []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fs, err := ResourceDir(dir)
	if err != nil {
		t.Fatalf("ResourceDir error: %v", err)
	}
	defer fs.Close()
	if fs == nil {
		t.Fatal("ResourceDir returned nil fs")
	}

	loadFS := localDir{base: dir}

	var cfg struct {
		Name string `json:"name"`
	}
	if err := LoadConfig(&cfg, loadFS, nil); err != nil {
		t.Fatalf("LoadConfig(nil) error: %v", err)
	}
	if cfg.Name != "demo" {
		t.Fatalf("cfg.Name = %q, want demo", cfg.Name)
	}

	cfg = struct {
		Name string `json:"name"`
	}{}
	if err := LoadConfig(&cfg, loadFS, strings.NewReader(`{"name":"reader"}`)); err != nil {
		t.Fatalf("LoadConfig(reader) error: %v", err)
	}
	if cfg.Name != "reader" {
		t.Fatalf("cfg.Name = %q, want reader", cfg.Name)
	}

	if _, err := ResourceDir(123); err == nil {
		t.Fatal("expected ResourceDir to reject unsupported resource type")
	}
}

func TestDispatchStageShape(t *testing.T) {
	called := ""
	err := DispatchStageShape(StageShape{"type": "sprites"}, StageShapeHandlers{
		Sprites: func(StageShape) error {
			called = "sprites"
			return nil
		},
	})
	if err != nil {
		t.Fatalf("DispatchStageShape error: %v", err)
	}
	if called != "sprites" {
		t.Fatalf("called = %q, want sprites", called)
	}

	if err := DispatchStageShape(StageShape{"type": "unknown"}, StageShapeHandlers{}); err == nil {
		t.Fatal("expected unknown shape error")
	}
}

func TestAppendStageItems(t *testing.T) {
	items, err := AppendStageItems(
		[]string{"base"},
		StageShape{"type": "sprites"},
		StageItemHandlers[string]{
			Sprites: func(StageShape) ([]string, error) {
				return []string{"a", "b"}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("AppendStageItems(sprites) error: %v", err)
	}
	if want := []string{"base", "a", "b"}; !reflect.DeepEqual(items, want) {
		t.Fatalf("AppendStageItems(sprites) = %v, want %v", items, want)
	}

	items, err = AppendStageItems(
		items,
		StageShape{"type": "sprite"},
		StageItemHandlers[string]{
			Sprite: func(StageShape) (string, error) {
				return "c", nil
			},
		},
	)
	if err != nil {
		t.Fatalf("AppendStageItems(sprite) error: %v", err)
	}
	if want := []string{"base", "a", "b", "c"}; !reflect.DeepEqual(items, want) {
		t.Fatalf("AppendStageItems(sprite) = %v, want %v", items, want)
	}
}

func TestShapeValue(t *testing.T) {
	shape := StageShape{"x": 1.5}
	if got := ShapeValue(shape, "x", 0.0); got != 1.5 {
		t.Fatalf("ShapeValue(existing) = %#v, want 1.5", got)
	}
	if got := ShapeValue(shape, "y", 2.0); got != 2.0 {
		t.Fatalf("ShapeValue(default) = %#v, want 2.0", got)
	}
	if got := ShapeValue(shape, "z"); got != nil {
		t.Fatalf("ShapeValue(missing) = %#v, want nil", got)
	}
}

func TestProjectConfigHelpers(t *testing.T) {
	proj := &ProjectConfig{
		Backdrops:     []*BackdropConfig{{}, {}},
		BackdropIndex: 1,
	}
	if got := proj.GetBackdrops(); len(got) != 2 {
		t.Fatalf("GetBackdrops len = %d, want 2", len(got))
	}
	if got := proj.GetBackdropIndex(); got != 1 {
		t.Fatalf("GetBackdropIndex = %d, want 1", got)
	}

	sprite := &SpriteConfig{CostumeIndex: 3}
	if got := sprite.GetCostumeIndex(); got != 3 {
		t.Fatalf("GetCostumeIndex = %d, want 3", got)
	}

	if got := ToMapMode("repeat"); got != MapModeRepeat {
		t.Fatalf("ToMapMode(repeat) = %d, want %d", got, MapModeRepeat)
	}
	if got := ToMapMode("fillCut"); got != MapModeFillCut {
		t.Fatalf("ToMapMode(fillCut) = %d, want %d", got, MapModeFillCut)
	}
	if got := ToMapMode("actualSize"); got != MapModeActualSize {
		t.Fatalf("ToMapMode(actualSize) = %d, want %d", got, MapModeActualSize)
	}
	if got := ToMapMode("unknown"); got != MapModeFill {
		t.Fatalf("ToMapMode(unknown) = %d, want %d", got, MapModeFill)
	}
}

func TestResolveDisplaySettings(t *testing.T) {
	stretch := false
	settings := ResolveDisplaySettings(&ProjectConfig{
		WindowScale: 2,
		StretchMode: &stretch,
		Debug:       true,
	})
	if settings.WindowScale != 2 || settings.StretchMode || !settings.Debug {
		t.Fatalf("unexpected settings: %+v", settings)
	}
	if settings.Fonts.DefaultPath != "res://engine/fonts/default.ttf" {
		t.Fatalf("default font path = %q", settings.Fonts.DefaultPath)
	}
	if len(settings.Fonts.Faces) != 0 {
		t.Fatalf("built-in font registrations = %+v, want none", settings.Fonts.Faces)
	}

	settings = ResolveDisplaySettings(&ProjectConfig{})
	if settings.WindowScale != 1 || !settings.StretchMode || settings.Debug {
		t.Fatalf("unexpected default settings: %+v", settings)
	}

	nilSettings := ResolveDisplaySettings(nil)
	if nilSettings.WindowScale != 1 || !nilSettings.StretchMode || nilSettings.Debug {
		t.Fatalf("unexpected nil settings defaults: %+v", nilSettings)
	}
	if nilSettings.Fonts.DefaultPath != defaultDisplayFontPath {
		t.Fatalf("nil default font path = %q", nilSettings.Fonts.DefaultPath)
	}
}

func TestApplyDisplayFonts(t *testing.T) {
	settings := ResolveDisplaySettings(&ProjectConfig{})
	settings.Fonts.Faces = []FontFaceRegistration{
		{Path: "asset://fonts/Latin/font.ttf", Family: "Latin"},
		{Path: "asset://fonts/Chinese/font.ttf", Family: "Chinese"},
	}
	var applied ResolvedFontConfig
	if err := ApplyDisplayFonts(settings, func(config ResolvedFontConfig) error {
		applied = config
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(applied, settings.Fonts) {
		t.Fatalf("applied font config = %#v, want %#v", applied, settings.Fonts)
	}

	applied.Faces[0].Path = "mutated"
	applied.Preferences[0] = "mutated"
	if settings.Fonts.Faces[0].Path == "mutated" || settings.Fonts.Preferences[0] == "mutated" {
		t.Fatal("ApplyDisplayFonts passed mutable settings slices to the engine boundary")
	}
	if err := ApplyDisplayFonts(settings, nil); err == nil {
		t.Fatal("ApplyDisplayFonts(nil) succeeded")
	}
	wantErr := errors.New("engine rejected project fonts")
	if err := ApplyDisplayFonts(settings, func(ResolvedFontConfig) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("ApplyDisplayFonts error = %v, want %v", err, wantErr)
	}

	settings.Fonts.Preferences = []string{}
	if err := ApplyDisplayFonts(settings, func(config ResolvedFontConfig) error {
		if config.Preferences == nil {
			t.Fatal("explicit empty preferences became nil at the engine boundary")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestAddProjectFonts(t *testing.T) {
	settings := ResolveDisplaySettings(&ProjectConfig{})
	AddProjectFonts(&settings, ProjectFonts{
		Families: []ProjectFontFamily{
			{Name: "Scratch", Faces: []ProjectFontFace{{Path: "fonts/Scratch/font.ttf"}}},
			{Name: "Chinese", Faces: []ProjectFontFace{{Path: "fonts/Chinese/font.ttf"}}},
		},
		Preferences: []string{"Scratch", "Chinese", "default"},
	}, func(path string) string { return "asset://" + path })

	if got := settings.Fonts.Faces; !reflect.DeepEqual(got, []FontFaceRegistration{
		{Family: "Scratch", Path: "asset://fonts/Scratch/font.ttf"},
		{Family: "Chinese", Path: "asset://fonts/Chinese/font.ttf"},
	}) {
		t.Fatalf("project registrations = %#v", got)
	}
	if want := []string{"Scratch", "Chinese", "default"}; !reflect.DeepEqual(settings.Fonts.Preferences, want) {
		t.Fatalf("preferences = %#v, want %#v", settings.Fonts.Preferences, want)
	}
}

func TestResolveMapConfigAndMetrics(t *testing.T) {
	cfg := ResolveMapConfig(MapConfig{}, true, 480, 360)
	if cfg.Width != 480 || cfg.Height != 360 {
		t.Fatalf("unexpected map config: %+v", cfg)
	}

	metrics := ResolveWorldWindowMetrics(1000, 700, 1200, 600, MapModeRepeat)
	if metrics.MinWorldX != -500 || metrics.MinWorldY != -350 {
		t.Fatalf("unexpected min world: %+v", metrics)
	}
	if metrics.WindowWidth != 1000 || metrics.WindowHeight != 600 {
		t.Fatalf("unexpected clamped window: %+v", metrics)
	}
	if metrics.MapMode != MapModeRepeat {
		t.Fatalf("unexpected map mode: %+v", metrics)
	}
}

func TestResolvePlatformLayout(t *testing.T) {
	layout := ResolvePlatformLayout(PlatformLayoutInput{
		WindowWidth:       400,
		WindowHeight:      200,
		WindowScale:       1,
		Fullscreen:        true,
		CurrentWindowSize: mathf.Vec2{X: 1000, Y: 600},
	})
	if !layout.Fullscreen {
		t.Fatalf("expected fullscreen: %+v", layout)
	}
	if layout.WindowScale != 2.5 || layout.WindowWidth != 1000 || layout.WindowHeight != 500 {
		t.Fatalf("unexpected fullscreen layout: %+v", layout)
	}

	web := ResolvePlatformLayout(PlatformLayoutInput{
		WindowWidth:       400,
		WindowHeight:      200,
		WindowScale:       1,
		IsWeb:             true,
		CurrentWindowSize: mathf.Vec2{X: 900, Y: 500},
	})
	if web.WindowWidth != 900 || web.WindowHeight != 500 {
		t.Fatalf("unexpected web layout: %+v", web)
	}
}

func TestResolveBackdropLayout(t *testing.T) {
	repeat := ResolveBackdropLayout(100, 50, 400, 200, MapModeRepeat)
	if repeat.RepeatScale == nil || repeat.RepeatScale.X != 4 || repeat.RepeatScale.Y != 4 {
		t.Fatalf("unexpected repeat layout: %+v", repeat)
	}
	if repeat.ScaleX != 4 || repeat.ScaleY != 4 {
		t.Fatalf("unexpected repeat scale: %+v", repeat)
	}

	fillCut := ResolveBackdropLayout(100, 50, 100, 100, MapModeFillCut)
	if fillCut.ScaleX != 1 || fillCut.ScaleY != 1 {
		t.Fatalf("unexpected fillCut layout: %+v", fillCut)
	}

	actualSize := ResolveBackdropLayout(100, 50, 400, 200, MapModeActualSize)
	if actualSize.ScaleX != 1 || actualSize.ScaleY != 1 {
		t.Fatalf("unexpected actualSize layout: %+v", actualSize)
	}

	fillRatio := ResolveBackdropLayout(100, 50, 100, 100, MapModeFillRatio)
	if fillRatio.ScaleX != 2 || fillRatio.ScaleY != 2 {
		t.Fatalf("unexpected fillRatio layout: %+v", fillRatio)
	}
}

func TestLoadBuilderProject(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.json")
	if err := os.WriteFile(indexPath, []byte(`{"run":{"title":"from-project"},"fullscreen":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadBuilderProject(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("LoadBuilderProject(nil) error: %v", err)
	}
	if loaded.Config.Title != "from-project" {
		t.Fatalf("loaded.Config.Title = %q, want from-project", loaded.Config.Title)
	}
	if !loaded.Project.FullScreen {
		t.Fatal("expected project fullscreen to be loaded")
	}

	override := &Config{Title: "override"}
	loaded, err = LoadBuilderProject(localDir{base: dir}, override)
	if err != nil {
		t.Fatalf("LoadBuilderProject(override) error: %v", err)
	}
	if loaded.Config.Title != "override" {
		t.Fatalf("loaded.Config.Title = %q, want override", loaded.Config.Title)
	}
}

func TestOpenBuilderResources(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.json")
	if err := os.WriteFile(indexPath, []byte(`{"run":{"title":"demo"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources error: %v", err)
	}
	defer opened.FS.Close()

	if opened.AssetDir != "" {
		t.Fatalf("opened.AssetDir = %q, want empty", opened.AssetDir)
	}
	if opened.Config.Title != "demo" {
		t.Fatalf("opened.Config.Title = %q, want demo", opened.Config.Title)
	}
}

func TestOpenBuilderResourcesFromPackedConfig(t *testing.T) {
	dir := t.TempDir()
	packed := `{
		"run":{"title":"packed-demo"},
		"backdrops":[{"name":"bg","path":"bg.png"}],
		"sprites":{
			"Hero":{
				"costumes":[{"name":"hero","path":"hero.png","x":10,"y":20}],
				"costumeIndex":0,
				"size":80
			}
		},
		"sounds":{
			"Jump":{
				"path":"jump.wav",
				"rate":1
			}
		},
		"zorder":["Hero"]
	}`
	if err := os.WriteFile(filepath.Join(dir, packedIndexJSON), []byte(packed), 0o644); err != nil {
		t.Fatal(err)
	}

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources(packed) error: %v", err)
	}
	defer opened.FS.Close()

	if opened.Config.Title != "packed-demo" {
		t.Fatalf("opened.Config.Title = %q, want packed-demo", opened.Config.Title)
	}
	if len(opened.Project.Zorder) != 1 || opened.Project.Zorder[0] != "Hero" {
		t.Fatalf("opened.Project.Zorder = %#v, want [Hero]", opened.Project.Zorder)
	}
	if got := opened.Project.Backdrops[0].Path; got != "bg.png" {
		t.Fatalf("opened.Project.Backdrops[0].Path = %q, want bg.png", got)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig(packed) error: %v", err)
	}
	if sprite.BaseDir != "sprites/Hero/" {
		t.Fatalf("sprite.BaseDir = %q, want sprites/Hero/", sprite.BaseDir)
	}
	if got := sprite.Config.Costumes[0].Path; got != "sprites/Hero/hero.png" {
		t.Fatalf("sprite.Config.Costumes[0].Path = %q, want sprites/Hero/hero.png", got)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(packed) error: %v", err)
	}
	if sound.BaseDir != "sounds/Jump" {
		t.Fatalf("sound.BaseDir = %q, want sounds/Jump", sound.BaseDir)
	}
	if got := sound.Config.Path; got != "sounds/Jump/jump.wav" {
		t.Fatalf("sound.Config.Path = %q, want sounds/Jump/jump.wav", got)
	}
}

func TestOpenBuilderResourcesFallsBackToSourceRootConfigWhenPackedRootMissing(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{
		"run":{"title":"source-demo"},
		"backdrops":[{"name":"bg","path":"bg.png"}],
		"bgm":"theme.mp3"
	}`)
	writeProjectFile(t, dir, packedIndexJSON, `{
		"zorder":["Hero"]
	}`)

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources(root fallback) error: %v", err)
	}
	defer opened.FS.Close()

	if opened.Config.Title != "source-demo" {
		t.Fatalf("opened.Config.Title = %q, want source-demo", opened.Config.Title)
	}
	if len(opened.Project.Backdrops) != 1 || opened.Project.Backdrops[0].Path != "bg.png" {
		t.Fatalf("opened.Project.Backdrops = %#v, want [bg.png]", opened.Project.Backdrops)
	}
	if opened.Project.Bgm != "theme.mp3" {
		t.Fatalf("opened.Project.Bgm = %q, want theme.mp3", opened.Project.Bgm)
	}
	if len(opened.Project.Zorder) != 1 || opened.Project.Zorder[0] != "Hero" {
		t.Fatalf("opened.Project.Zorder = %#v, want [Hero]", opened.Project.Zorder)
	}
}

func TestOpenBuilderResourcesPrefersPackedConfigOverSource(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"run":{"title":"source-demo"}}`)
	writeProjectFile(t, dir, "sprites/Hero/index.json", `{
		"costumes":[{"name":"hero-source","path":"source.png","x":1,"y":2}],
		"size":60
	}`)
	writeProjectFile(t, dir, "sounds/Jump/index.json", `{"path":"source.wav","rate":1}`)
	writeProjectFile(t, dir, packedIndexJSON, `{
		"run":{"title":"packed-demo"},
		"sprites":{
			"Hero":{
				"costumes":[{"name":"hero-packed","path":"packed.png","x":10,"y":20}],
				"size":80
			}
		},
		"sounds":{
			"Jump":{"path":"packed.wav","rate":2}
		}
	}`)

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources(prefer packed) error: %v", err)
	}
	defer opened.FS.Close()

	if opened.Config.Title != "packed-demo" {
		t.Fatalf("opened.Config.Title = %q, want packed-demo", opened.Config.Title)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig(prefer packed) error: %v", err)
	}
	if got := sprite.Config.Costumes[0].Path; got != "sprites/Hero/packed.png" {
		t.Fatalf("sprite.Config.Costumes[0].Path = %q, want sprites/Hero/packed.png", got)
	}
	if sprite.Config.Size != 80 {
		t.Fatalf("sprite.Config.Size = %v, want 80", sprite.Config.Size)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(prefer packed) error: %v", err)
	}
	if got := sound.Config.Path; got != "sounds/Jump/packed.wav" {
		t.Fatalf("sound.Config.Path = %q, want sounds/Jump/packed.wav", got)
	}
	if sound.Config.Rate != 2 {
		t.Fatalf("sound.Config.Rate = %d, want 2", sound.Config.Rate)
	}
}

func TestOpenBuilderResourcesFallsBackToSourceChildConfigWhenPackedChildMissing(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"run":{"title":"source-demo"}}`)
	writeProjectFile(t, dir, "sprites/Hero/index.json", `{
		"costumes":[{"name":"hero-source","path":"source.png","x":1,"y":2}],
		"size":60
	}`)
	writeProjectFile(t, dir, "sounds/Jump/index.json", `{"path":"source.wav","rate":1}`)
	writeProjectFile(t, dir, packedIndexJSON, `{
		"run":{"title":"packed-demo"},
		"zorder":["Hero"]
	}`)

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources(fallback child) error: %v", err)
	}
	defer opened.FS.Close()

	if opened.Config.Title != "packed-demo" {
		t.Fatalf("opened.Config.Title = %q, want packed-demo", opened.Config.Title)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig(fallback child) error: %v", err)
	}
	if got := sprite.Config.Costumes[0].Path; got != "sprites/Hero/source.png" {
		t.Fatalf("sprite.Config.Costumes[0].Path = %q, want sprites/Hero/source.png", got)
	}
	if sprite.Config.Size != 60 {
		t.Fatalf("sprite.Config.Size = %v, want 60", sprite.Config.Size)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(fallback child) error: %v", err)
	}
	if got := sound.Config.Path; got != "sounds/Jump/source.wav" {
		t.Fatalf("sound.Config.Path = %q, want sounds/Jump/source.wav", got)
	}
	if sound.Config.Rate != 1 {
		t.Fatalf("sound.Config.Rate = %d, want 1", sound.Config.Rate)
	}
}

func TestOpenBuilderResourcesFromStringResourceWithPackedFallback(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"run":{"title":"source-demo"}}`)
	writeProjectFile(t, dir, "sprites/Hero/index.json", `{
		"costumes":[{"name":"hero-source","path":"source.png","x":1,"y":2}],
		"size":60
	}`)
	writeProjectFile(t, dir, packedIndexJSON, `{
		"run":{"title":"packed-demo"}
	}`)

	opened, err := OpenBuilderResources(dir, nil)
	if err != nil {
		t.Fatalf("OpenBuilderResources(string resource) error: %v", err)
	}
	defer opened.FS.Close()

	if opened.AssetDir != dir {
		t.Fatalf("opened.AssetDir = %q, want %q", opened.AssetDir, dir)
	}
	if opened.Config.Title != "packed-demo" {
		t.Fatalf("opened.Config.Title = %q, want packed-demo", opened.Config.Title)
	}
	if _, ok := opened.FS.(spxfs.GdDir); !ok {
		t.Fatalf("opened.FS does not implement spxfs.GdDir: %T", opened.FS)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig(string resource) error: %v", err)
	}
	if got := sprite.Config.Costumes[0].Path; got != "sprites/Hero/source.png" {
		t.Fatalf("sprite.Config.Costumes[0].Path = %q, want sprites/Hero/source.png", got)
	}
}

func TestOpenBuilderResourcesFailsOnInvalidPackedConfig(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"run":{"title":"source-demo"}}`)
	writeProjectFile(t, dir, packedIndexJSON, `{`)

	_, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err == nil {
		t.Fatal("OpenBuilderResources(invalid packed) error = nil, want error")
	}
	if !strings.Contains(err.Error(), packedIndexJSON) {
		t.Fatalf("OpenBuilderResources(invalid packed) error = %q, want mention of %s", err, packedIndexJSON)
	}
}

func TestResolveRuntimeConfig(t *testing.T) {
	conf := &Config{
		Width:            640,
		Height:           480,
		EventQueuePolicy: "block",
	}
	proj := &ProjectConfig{FullScreen: true, Physics: true}
	runtime := ResolveRuntimeConfig(conf, proj, "/tmp/demo", "env-key")
	if runtime.Title != "demo (by XGo Builder)" {
		t.Fatalf("runtime.Title = %q", runtime.Title)
	}
	if !runtime.FullScreen || !runtime.PhysicsEnabled {
		t.Fatalf("unexpected runtime flags: %+v", runtime)
	}
	if runtime.WindowWidth != 640 || runtime.WindowHeight != 480 {
		t.Fatalf("unexpected runtime size: %+v", runtime)
	}
	if runtime.ScreenshotKey != "env-key" {
		t.Fatalf("runtime.ScreenshotKey = %q, want env-key", runtime.ScreenshotKey)
	}
}

func TestLoadSpriteAndSoundConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sprites", "Hero"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sprites", "Hero", "index.json"), []byte(`{"size":80,"costumeSet":{"path":"../../../../res/hero.png"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sounds", "Jump"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sounds", "Jump", "index.json"), []byte(`{"path":"jump.wav","rate":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	loadFS := localDir{base: dir}

	sprite, err := LoadSpriteConfig(loadFS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig error: %v", err)
	}
	if sprite.BaseDir != "sprites/Hero/" {
		t.Fatalf("sprite.BaseDir = %q, want sprites/Hero/", sprite.BaseDir)
	}
	if sprite.Config.Size != 80 {
		t.Fatalf("sprite.Config.Size = %v, want 80", sprite.Config.Size)
	}
	if sprite.Config.CostumeSet == nil || sprite.Config.CostumeSet.Path != "../../res/hero.png" {
		t.Fatalf("sprite.Config.CostumeSet.Path = %q, want ../../res/hero.png", sprite.Config.CostumeSet.Path)
	}

	sound, err := LoadSoundConfig(loadFS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig error: %v", err)
	}
	if sound.BaseDir != "sounds/Jump" {
		t.Fatalf("sound.BaseDir = %q, want sounds/Jump", sound.BaseDir)
	}
	if sound.Config.Path != "sounds/Jump/jump.wav" {
		t.Fatalf("sound.Config.Path = %q, want sounds/Jump/jump.wav", sound.Config.Path)
	}
}

func TestLoadBuilderProjectNormalizesProjectPaths(t *testing.T) {
	dir := t.TempDir()
	indexJSON := `{
		"backdrops":[{"name":"lake","path":"../../res/lake.jpg"}],
		"bgm":"../../res/theme.mp3",
		"tilemapPath":"../maps/level1.json"
	}`
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(indexJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadBuilderProject(localDir{base: dir}, nil)
	if err != nil {
		t.Fatalf("LoadBuilderProject error: %v", err)
	}

	if got := loaded.Project.Backdrops[0].Path; got != "../../res/lake.jpg" {
		t.Fatalf("loaded.Project.Backdrops[0].Path = %q, want ../../res/lake.jpg", got)
	}
	if got := loaded.Project.Bgm; got != "../../res/theme.mp3" {
		t.Fatalf("loaded.Project.Bgm = %q, want ../../res/theme.mp3", got)
	}
	if got := loaded.Project.TilemapPath; got != "../maps/level1.json" {
		t.Fatalf("loaded.Project.TilemapPath = %q, want ../maps/level1.json", got)
	}
}

func TestResolveSystemSettings(t *testing.T) {
	settings := ResolveSystemSettings(&ProjectConfig{})
	if settings.PathCellSizeX != DefaultPathCellSize || settings.PathCellSizeY != DefaultPathCellSize {
		t.Fatalf("unexpected path defaults: %+v", settings)
	}
	if settings.AudioMaxDistance != float64(DefaultAudioMaxDistance) {
		t.Fatalf("unexpected audio defaults: %+v", settings)
	}
	if !settings.CollisionByPixel || !settings.AutoSetCollisionLayer {
		t.Fatalf("unexpected collision defaults: %+v", settings)
	}

	auto := false
	precision := "high"
	settings = ResolveSystemSettings(&ProjectConfig{
		CollisionByShape:        true,
		Physics:                 true,
		AutoSetCollisionLayer:   &auto,
		PixelCollisionPrecision: &precision,
	})
	if settings.CollisionByPixel || settings.AutoSetCollisionLayer {
		t.Fatalf("unexpected collision overrides: %+v", settings)
	}
	if settings.PixelCollisionPrecision == 0 {
		t.Fatalf("expected parsed precision: %+v", settings)
	}
}

func TestWalkZOrder(t *testing.T) {
	var got []string
	err := WalkZOrder(
		[]any{"A", StageShape{"type": "sprite"}, "B"},
		func(layer int, name string) error {
			got = append(got, name)
			return nil
		},
		func(layer int, shape StageShape) error {
			got = append(got, shape["type"].(string))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("WalkZOrder error: %v", err)
	}
	if want := []string{"A", "sprite", "B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkZOrder got %v, want %v", got, want)
	}
}

func TestRunSpriteInitializers(t *testing.T) {
	var got []string
	RunSpriteInitializers(SpriteInitConfig[string]{
		Items: []string{"a", "b"},
		Setup: func(items []string) {
			got = append(got, "setup")
		},
		BeforeMain: func(item string) {
			got = append(got, "before:"+item)
		},
	})
	want := []string{"setup", "before:a", "before:b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RunSpriteInitializers got %v, want %v", got, want)
	}
}

func TestFieldPtrOrAllocAndFinders(t *testing.T) {
	type spriteLike struct{ V int }
	type holder struct {
		Spr *spriteLike
		Any any
		Val int
	}

	h := holder{}
	name, value := FieldPtrOrAlloc(reflect.ValueOf(&h).Elem(), 0, FieldAllocConfig{
		IsPointerSpriteType: func(typ reflect.Type) bool {
			return typ == reflect.TypeOf((*spriteLike)(nil))
		},
	})
	if name != "Spr" {
		t.Fatalf("name = %q, want Spr", name)
	}
	if h.Spr == nil || value.(*spriteLike) != h.Spr {
		t.Fatalf("expected allocated pointer, got %#v / %#v", value, h.Spr)
	}

	_, _ = FieldPtrOrAlloc(reflect.ValueOf(&h).Elem(), 1, FieldAllocConfig{
		ResolveInterfaceSpriteType: func(fieldName string) (reflect.Type, bool) {
			if fieldName == "Any" {
				return reflect.TypeOf(spriteLike{}), true
			}
			return nil, false
		},
	})
	if _, ok := h.Any.(*spriteLike); !ok {
		t.Fatalf("expected interface allocation, got %#v", h.Any)
	}

	h.Val = 7
	if got := FindFieldPtr(reflect.ValueOf(&h), "Val", 0).(*int); *got != 7 {
		t.Fatalf("FindFieldPtr = %d, want 7", *got)
	}
	if got := FindFieldRefCaseInsensitive(reflect.ValueOf(&h), "val", 0).(*int); *got != 7 {
		t.Fatalf("FindFieldRefCaseInsensitive = %d, want 7", *got)
	}
	if got := FindObjectPtr(reflect.ValueOf(&h), "Spr", 0).(*spriteLike); got != h.Spr {
		t.Fatalf("FindObjectPtr = %#v, want %#v", got, h.Spr)
	}
}

func TestParseCommandLineFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	conf := &Config{}
	effects, err := ParseCommandLineFlags(fs, []string{"-v", "-f"}, conf)
	if err != nil {
		t.Fatalf("ParseCommandLineFlags error: %v", err)
	}
	if !effects.Verbose || effects.ShowHelp {
		t.Fatalf("unexpected effects: %+v", effects)
	}
	if !conf.FullScreen {
		t.Fatalf("expected fullscreen config update: %+v", conf)
	}

	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	conf = &Config{}
	effects, err = ParseCommandLineFlags(fs, []string{"-h"}, conf)
	if err != nil {
		t.Fatalf("ParseCommandLineFlags help error: %v", err)
	}
	if !effects.ShowHelp {
		t.Fatalf("expected help effect: %+v", effects)
	}
}

func TestWalkFields(t *testing.T) {
	type holder struct {
		A int
		B string
	}
	v := reflect.ValueOf(holder{A: 1, B: "x"})
	var got []string
	err := WalkFields(v, func(fieldIndex int) (string, any) {
		field := v.Type().Field(fieldIndex)
		return field.Name, v.Field(fieldIndex).Interface()
	}, func(name string, value any) error {
		got = append(got, name)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkFields error: %v", err)
	}
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WalkFields got %v, want %v", got, want)
	}
}

func TestBindStageSprite(t *testing.T) {
	type spriteLike struct {
		Name string
	}
	type holder struct {
		One *spriteLike
	}

	h := holder{One: &spriteLike{}}
	err := BindStageSprite(
		reflect.ValueOf(&h),
		"One",
		FindObjectPtr,
		func(val any) error {
			val.(*spriteLike).Name = "hero"
			return nil
		},
	)
	if err != nil {
		t.Fatalf("BindStageSprite error: %v", err)
	}
	if h.One.Name != "hero" {
		t.Fatalf("h.One.Name = %q, want hero", h.One.Name)
	}
}

func TestBindStageSprites(t *testing.T) {
	type spriteLike struct {
		Name string
	}
	type holder struct {
		Many []*spriteLike
	}

	h := holder{}
	err := BindStageSprites(
		reflect.ValueOf(&h),
		"Many",
		[]any{StageShape{"name": "a"}, StageShape{"name": "b"}},
		FindFieldPtr,
		func(typ reflect.Type) bool {
			return typ == reflect.TypeOf((*spriteLike)(nil))
		},
		func(itemValue reflect.Value, shape StageShape) error {
			itemValue.FieldByName("Name").SetString(shape["name"].(string))
			return nil
		},
	)
	if err != nil {
		t.Fatalf("BindStageSprites error: %v", err)
	}
	if len(h.Many) != 2 || h.Many[0].Name != "a" || h.Many[1].Name != "b" {
		t.Fatalf("unexpected bound slice: %+v", h.Many)
	}
}
