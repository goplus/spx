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
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/goplus/spbase/mathf"
	spxfs "github.com/goplus/spx/v4/fs"
	_ "github.com/goplus/spx/v4/fs/asset"
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

func TestLoadConfigRejectsTrailingContent(t *testing.T) {
	for _, input := range []string{`{} {}`, `{} trailing`} {
		var config ProjectConfig
		if err := LoadConfig(&config, nil, strings.NewReader(input)); err == nil {
			t.Fatalf("LoadConfig(%q) error = nil", input)
		}
	}
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

func TestOpenBuilderResourcesUsesPackedFonts(t *testing.T) {
	dir := t.TempDir()
	writeProjectFile(t, dir, "index.json", `{"fontPreferences":["Source"]}`)
	writeProjectFile(t, dir, "index_pack.json", `{
		"fontPreferences":["Packed", "default"],
		"fonts":{
			"Packed":{"faces":[{"path":"packed.ttf"}]},
			"alpha":{"faces":[{"path":"alpha.ttf"}]}
		}
	}`)
	writeProjectFile(t, dir, "fonts/Packed/packed.ttf", "packed-font")
	writeProjectFile(t, dir, "fonts/alpha/alpha.ttf", "alpha-font")
	writeProjectFile(t, dir, "fonts/Source/index.json", `{"faces":[{"path":"source.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Source/source.ttf", "source-font")
	writeProjectFile(t, dir, "sprites/Hero/index.json", `{}`)

	opened, err := OpenBuilderResources(localDir{base: dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.FS.Close()
	wantFamilies := []ProjectFontFamily{
		{Name: "alpha", Faces: []ProjectFontFace{{Path: "fonts/alpha/alpha.ttf"}}},
		{Name: "Packed", Faces: []ProjectFontFace{{Path: "fonts/Packed/packed.ttf"}}},
	}
	if !reflect.DeepEqual(opened.Fonts.Families, wantFamilies) {
		t.Fatalf("families = %#v, want sorted packed fonts %#v", opened.Fonts.Families, wantFamilies)
	}
	if want := []string{"Packed", "default"}; !reflect.DeepEqual(opened.Fonts.Preferences, want) {
		t.Fatalf("preferences = %#v, want %#v", opened.Fonts.Preferences, want)
	}
	if names, err := spxfs.ListDirectories(opened.FS, "sprites"); err != nil || !reflect.DeepEqual(names, []string{"Hero"}) {
		t.Fatalf("non-font directories = %v, %v; want source directory Hero", names, err)
	}
}

func TestOpenBuilderResourcesScansFontsOnlyWhenPackedFontsAbsent(t *testing.T) {
	for _, tt := range []struct {
		name, packed string
		wantSource   bool
	}{
		{name: "absent", packed: `{"zorder":[]}`, wantSource: true},
		{name: "null", packed: `{"fonts":null,"fontPreferences":[]}`},
		{name: "empty", packed: `{"fonts":{},"fontPreferences":[]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, "index.json", `{"fontPreferences":["Source"]}`)
			writeProjectFile(t, dir, "index_pack.json", tt.packed)
			writeProjectFile(t, dir, "fonts/Source/index.json", `{"faces":[{"path":"source.ttf"}]}`)
			writeProjectFile(t, dir, "fonts/Source/source.ttf", "source-font")

			opened, err := OpenBuilderResources(localDir{base: dir}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.FS.Close()
			wantFamilies, wantPreferences := []ProjectFontFamily{}, []string{}
			if tt.wantSource {
				wantFamilies = []ProjectFontFamily{{Name: "Source", Faces: []ProjectFontFace{{Path: "fonts/Source/source.ttf"}}}}
				wantPreferences = []string{"Source"}
			}
			if !reflect.DeepEqual(opened.Fonts.Families, wantFamilies) || !reflect.DeepEqual(opened.Fonts.Preferences, wantPreferences) {
				t.Fatalf("fonts = %#v, want families %#v and preferences %#v", opened.Fonts, wantFamilies, wantPreferences)
			}
		})
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

	for _, tt := range []struct{ name, first, want string }{
		{"case folded duplicate", `{"faces":[{"path":"font.ttf"}]}`, `font family "latin" conflicts with "Latin" after ASCII case folding`},
		{"family error before duplicate", `{"faces":[]}`, `font family "Latin" must contain exactly one face, got 0`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, "index_pack.json", `{
				"fonts": {
					"Latin": `+tt.first+`,
					"latin": {"faces":[]}
				}
			}`)
			writeProjectFile(t, dir, "fonts/Latin/font.ttf", "font")
			_, err := OpenBuilderResources(localDir{base: dir}, nil)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %s", err, tt.want)
			}
		})
	}

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
	if got := AssetDirFromResource("assets"); got != "assets" {
		t.Fatalf("string asset dir = %q", got)
	}
	if got := AssetDirFromResource(fakeGdDir{path: "res://assets/"}); got != "res://assets" {
		t.Fatalf("gd asset dir = %q", got)
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
	if err := LoadConfig(&cfg, loadFS, "index.json"); err != nil {
		t.Fatalf("LoadConfig(path) error: %v", err)
	}
	if err := LoadConfig(&cfg, loadFS, 123); err != syscall.EINVAL {
		t.Fatalf("LoadConfig(unsupported index) error = %v, want %v", err, syscall.EINVAL)
	}

	if _, err := ResourceDir(123); err == nil {
		t.Fatal("expected ResourceDir to reject unsupported resource type")
	}
	if opened, err := OpenBuilderResources(123, nil); err == nil || opened.FS != nil {
		t.Fatalf("OpenBuilderResources(unsupported resource) = %+v, %v", opened, err)
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

	for _, tt := range []struct {
		name string
		want int
	}{
		{"repeat", MapModeRepeat},
		{"fillCut", MapModeFillCut},
		{"fillRatio", MapModeFillRatio},
		{"actualSize", MapModeActualSize},
		{"unknown", MapModeFill},
	} {
		if got := ToMapMode(tt.name); got != tt.want {
			t.Errorf("ToMapMode(%q) = %d, want %d", tt.name, got, tt.want)
		}
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

	settings = ResolveDisplaySettings(&ProjectConfig{})
	if settings.WindowScale != 1 || !settings.StretchMode || settings.Debug {
		t.Fatalf("unexpected default settings: %+v", settings)
	}

	nilSettings := ResolveDisplaySettings(nil)
	if nilSettings.WindowScale != 1 || !nilSettings.StretchMode || nilSettings.Debug {
		t.Fatalf("unexpected nil settings defaults: %+v", nilSettings)
	}
}

func TestResolveRuntimeFontPlan(t *testing.T) {
	fonts := ProjectFonts{
		Families: []ProjectFontFamily{
			{Name: "Scratch", Faces: []ProjectFontFace{{Path: "fonts/Scratch/font.ttf"}}},
			{Name: "Chinese", Faces: []ProjectFontFace{{Path: "fonts/Chinese/font.ttf"}}},
		},
		Preferences: []string{"Scratch", "Chinese", "default"},
	}
	plan := ResolveRuntimeFontPlan(fonts, func(path string) string { return "asset://" + path })

	if plan.DefaultPath != "res://engine/fonts/default.ttf" {
		t.Fatalf("default font path = %q", plan.DefaultPath)
	}
	if got := plan.Faces; !reflect.DeepEqual(got, []RuntimeFontFace{
		{Family: "Scratch", Path: "asset://fonts/Scratch/font.ttf"},
		{Family: "Chinese", Path: "asset://fonts/Chinese/font.ttf"},
	}) {
		t.Fatalf("project registrations = %#v", got)
	}
	if !reflect.DeepEqual(plan.Preferences, fonts.Preferences) {
		t.Fatalf("preferences = %#v, want %#v", plan.Preferences, fonts.Preferences)
	}

	clone := plan.Clone()
	clone.Faces[0].Path = "mutated"
	clone.Preferences[0] = "mutated"
	if plan.Faces[0].Path == "mutated" || plan.Preferences[0] == "mutated" {
		t.Fatal("RuntimeFontPlan.Clone shared mutable slices")
	}

	empty := ResolveRuntimeFontPlan(ProjectFonts{Preferences: []string{}}, nil)
	if empty.Preferences == nil || empty.Clone().Preferences == nil {
		t.Fatal("explicit empty preferences became nil")
	}
	missing := ResolveRuntimeFontPlan(ProjectFonts{}, nil)
	if missing.Preferences != nil || missing.Clone().Preferences != nil {
		t.Fatal("nil preferences became explicitly empty")
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
	if !IsWindowWorldSizeEqual(1000, 700, 1000, 700) {
		t.Fatal("IsWindowWorldSizeEqual(equal) = false")
	}
	if IsWindowWorldSizeEqual(1000, 700, 1000, 600) {
		t.Fatal("IsWindowWorldSizeEqual(different height) = true")
	}
}

func TestResolvePlatformLayout(t *testing.T) {
	tests := []struct {
		name                    string
		mobile, fullscreen, web bool
		want                    PlatformLayout
	}{
		{"windowed", false, false, false, PlatformLayout{1, 400, 200, 400, 200, false}},
		{"fullscreen", false, true, false, PlatformLayout{2.5, 1000, 500, 1000, 500, true}},
		{"mobile", true, false, false, PlatformLayout{2.5, 1000, 500, 1000, 500, true}},
		{"mobile fullscreen", true, true, false, PlatformLayout{2.5, 1000, 500, 1000, 500, true}},
		{"web", false, false, true, PlatformLayout{2.5, 1000, 600, 1000, 500, false}},
		{"web fullscreen", false, true, true, PlatformLayout{2.5, 1000, 600, 1000, 500, true}},
		{"web mobile", true, false, true, PlatformLayout{2.5, 1000, 600, 1000, 500, true}},
		{"web mobile fullscreen", true, true, true, PlatformLayout{2.5, 1000, 600, 1000, 500, true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePlatformLayout(PlatformLayoutInput{
				WindowWidth:       400,
				WindowHeight:      200,
				WindowScale:       1,
				Fullscreen:        tt.fullscreen,
				IsMobile:          tt.mobile,
				IsWeb:             tt.web,
				CurrentWindowSize: mathf.Vec2{X: 1000, Y: 600},
			})
			if got != tt.want {
				t.Fatalf("ResolvePlatformLayout() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestResolveBackdropLayout(t *testing.T) {
	for _, tt := range []struct {
		name       string
		imgW, imgH float64
		mode       int
		want       BackdropLayout
	}{
		{"repeat", 100, 50, MapModeRepeat, BackdropLayout{ScaleX: 1, ScaleY: 2, RepeatScale: &mathf.Vec4{X: 1, Y: 2}}},
		{"actual size", 100, 50, MapModeActualSize, BackdropLayout{ScaleX: 1, ScaleY: 1}},
		{"fill wide", 100, 50, MapModeFill, BackdropLayout{ScaleX: 1, ScaleY: 2}},
		{"fill cut wide", 100, 50, MapModeFillCut, BackdropLayout{ScaleX: 1, ScaleY: 1}},
		{"fill cut tall", 50, 100, MapModeFillCut, BackdropLayout{ScaleX: 1, ScaleY: 1}},
		{"fill ratio wide", 100, 50, MapModeFillRatio, BackdropLayout{ScaleX: 2, ScaleY: 2}},
		{"fill ratio tall", 50, 100, MapModeFillRatio, BackdropLayout{ScaleX: 2, ScaleY: 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBackdropLayout(tt.imgW, tt.imgH, 100, 100, tt.mode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveBackdropLayout() = %+v, want %+v", got, tt.want)
			}
		})
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
	for _, tt := range []struct {
		name    string
		packed  bool
		listing bool
	}{
		{name: "source with listing", listing: true},
		{name: "source without listing"},
		{name: "packed with listing", packed: true, listing: true},
		{name: "packed without listing", packed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, "index.json", `{"run":{"title":"demo"}}`)
			writeProjectFile(t, dir, "sprites/Hero/index.json", `{}`)
			if tt.packed {
				writeProjectFile(t, dir, packedIndexJSON, `{}`)
			}
			var resource spxfs.Dir = localDir{base: dir}
			if !tt.listing {
				resource = struct{ spxfs.Dir }{resource}
			}
			opened, err := OpenBuilderResources(resource, nil)
			if err != nil {
				t.Fatalf("OpenBuilderResources() error: %v", err)
			}
			defer opened.FS.Close()
			if opened.AssetDir != "" || opened.Config.Title != "demo" || len(opened.Fonts.Families) != 0 {
				t.Fatalf("unexpected loaded project: %+v", opened)
			}
			if tt.packed {
				if got := opened.FS.(spxfs.GdDir).GetPath(); got != "" {
					t.Fatalf("packed directory without path capability returned %q", got)
				}
				reader := opened.FS.(spxfs.ReadDirer)
				entries, err := reader.ReadDir("sprites")
				if err != nil {
					t.Fatal(err)
				}
				if tt.listing {
					if want := []spxfs.DirEntry{{Name: "Hero", IsDir: true}}; !reflect.DeepEqual(entries, want) {
						t.Fatalf("packed directory listing = %#v, want %#v", entries, want)
					}
					if entries, err := reader.ReadDir("missing"); entries != nil || !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("missing directory listing = %#v, %v", entries, err)
					}
				} else if entries != nil {
					t.Fatalf("directory without listing capability returned %#v", entries)
				}
			}
		})
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

	original, err := opened.FS.Open(packedIndexJSON)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	data, err := io.ReadAll(original)
	if err != nil || string(data) != packed {
		t.Fatalf("original packed config = %s, %v; want %s", data, err, packed)
	}

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
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/hero.png" {
		t.Fatalf("sprite.Costumes[0].Path = %q, want sprites/Hero/hero.png", got)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(packed) error: %v", err)
	}
	if got := sound.Path; got != "sounds/Jump/jump.wav" {
		t.Fatalf("sound.Path = %q, want sounds/Jump/jump.wav", got)
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
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/packed.png" {
		t.Fatalf("sprite.Costumes[0].Path = %q, want sprites/Hero/packed.png", got)
	}
	if sprite.Size != 80 {
		t.Fatalf("sprite.Size = %v, want 80", sprite.Size)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(prefer packed) error: %v", err)
	}
	if got := sound.Path; got != "sounds/Jump/packed.wav" {
		t.Fatalf("sound.Path = %q, want sounds/Jump/packed.wav", got)
	}
	if sound.Rate != 2 {
		t.Fatalf("sound.Rate = %d, want 2", sound.Rate)
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

	writeProjectFile(t, dir, "custom/Settings/index.json", `{"name":"custom"}`)
	writeProjectFile(t, dir, "custom/settings.json", `{"name":"custom"}`)
	writeProjectFile(t, dir, "custom/Settings/details.json", `{"name":"custom"}`)
	if err := os.MkdirAll(filepath.Join(dir, "custom", "Broken", "index.json"), 0o755); err != nil {
		t.Fatal(err)
	}

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
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/source.png" {
		t.Fatalf("sprite.Costumes[0].Path = %q, want sprites/Hero/source.png", got)
	}
	if sprite.Size != 60 {
		t.Fatalf("sprite.Size = %v, want 60", sprite.Size)
	}

	sound, err := LoadSoundConfig(opened.FS, "Jump")
	if err != nil {
		t.Fatalf("LoadSoundConfig(fallback child) error: %v", err)
	}
	if got := sound.Path; got != "sounds/Jump/source.wav" {
		t.Fatalf("sound.Path = %q, want sounds/Jump/source.wav", got)
	}
	if sound.Rate != 1 {
		t.Fatalf("sound.Rate = %d, want 1", sound.Rate)
	}
	for _, tt := range []struct {
		path    string
		wantErr string
	}{
		{path: "custom/Settings/index.json"},
		{path: "custom/settings.json"},
		{path: "custom/Settings/details.json"},
		{path: "custom/Missing/index.json", wantErr: "custom/Missing/index.json"},
		{path: "custom/Broken/index.json", wantErr: "custom/Broken/index.json"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			var config struct{ Name string }
			err := LoadJSON(&config, opened.FS, tt.path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadJSON() error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil || config.Name != "custom" {
				t.Fatalf("LoadJSON() = %+v, %v; want custom config", config, err)
			}
		})
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
	if gdDir, ok := opened.FS.(spxfs.GdDir); !ok {
		t.Fatalf("opened.FS does not implement spxfs.GdDir: %T", opened.FS)
	} else if got, want := gdDir.GetPath(), dir+"/"; got != want {
		t.Fatalf("packed directory path = %q, want %q", got, want)
	}

	sprite, err := LoadSpriteConfig(opened.FS, "Hero")
	if err != nil {
		t.Fatalf("LoadSpriteConfig(string resource) error: %v", err)
	}
	if got := sprite.Costumes[0].Path; got != "sprites/Hero/source.png" {
		t.Fatalf("sprite.Costumes[0].Path = %q, want sprites/Hero/source.png", got)
	}
}

func TestOpenBuilderResourcesFailsOnInvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name, source, packed, directory, want string
	}{
		{name: "packed root", source: `{}`, packed: `{`, want: packedIndexJSON},
		{name: "source root", source: `{`, packed: `{}`, want: "parse index.json"},
		{name: "packed is directory", source: `{}`, directory: packedIndexJSON, want: packedIndexJSON},
		{name: "source is directory", packed: `{}`, directory: "index.json", want: "index.json"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.source != "" {
				writeProjectFile(t, dir, "index.json", tt.source)
			}
			if tt.packed != "" {
				writeProjectFile(t, dir, packedIndexJSON, tt.packed)
			}
			if tt.directory != "" {
				if err := os.Mkdir(filepath.Join(dir, tt.directory), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := OpenBuilderResources(localDir{base: dir}, nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("OpenBuilderResources() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveRuntimeConfig(t *testing.T) {
	conf := &Config{
		Width:  640,
		Height: 480,
	}
	proj := &ProjectConfig{FullScreen: true, Physics: true}
	runtime := ResolveRuntimeConfig(conf, proj, "/tmp/demo", "env-key")
	if runtime.Title != "demo (by spx)" {
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
	conf.Title = "My Game"
	runtime = ResolveRuntimeConfig(conf, proj, "/tmp/demo", "env-key")
	if runtime.Title != conf.Title {
		t.Fatalf("runtime.Title = %q, want %q", runtime.Title, conf.Title)
	}
}

func TestLoadSpriteAndSoundConfigNormalizesPaths(t *testing.T) {
	for _, tt := range []struct {
		name, input, spritePath, soundPath string
	}{
		{"relative", "./images/../asset.png", "sprites/Hero/asset.png", "sounds/Jump/asset.png"},
		{"parent", "../shared/asset.png", "sprites/shared/asset.png", "sounds/shared/asset.png"},
		{"outside project", "../../../../res/hero.png", "../../res/hero.png", "../../res/hero.png"},
		{"absolute", "/shared/asset.png", "/shared/asset.png", "/shared/asset.png"},
		{"engine", "res://shared/asset.png", "res://shared/asset.png", "res://shared/asset.png"},
		{"empty", "", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			quoted, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			writeProjectFile(t, dir, "sprites/Hero/index.json", `{
                "size":80,
                "costumes":[null,{"path":`+string(quoted)+`}],
                "costumeSet":{"path":`+string(quoted)+`},
                "costumeMPSet":{"path":`+string(quoted)+`}
            }`)
			writeProjectFile(t, dir, "sounds/Jump/index.json", `{"path":`+string(quoted)+`,"rate":1}`)
			fs := localDir{base: dir}
			sprite, err := LoadSpriteConfig(fs, "Hero")
			if err != nil {
				t.Fatal(err)
			}
			if sprite.Size != 80 {
				t.Fatalf("sprite.Size = %v, want 80", sprite.Size)
			}
			if sprite.Costumes[0] != nil {
				t.Fatal("null costume was changed")
			}
			for _, got := range []string{sprite.Costumes[1].Path, sprite.CostumeSet.Path, sprite.CostumeMPSet.Path} {
				if got != tt.spritePath {
					t.Errorf("sprite asset path = %q, want %q", got, tt.spritePath)
				}
			}
			sound, err := LoadSoundConfig(fs, "Jump")
			if err != nil {
				t.Fatal(err)
			}
			if sound.Rate != 1 {
				t.Fatalf("sound.Rate = %v, want 1", sound.Rate)
			}
			if sound.Path != tt.soundPath {
				t.Errorf("sound path = %q, want %q", sound.Path, tt.soundPath)
			}
		})
	}
}

func TestLoadSpriteAndSoundConfigErrorsReturnZeroValues(t *testing.T) {
	for _, tt := range []struct {
		name, content string
	}{
		{"missing", ""},
		{"malformed", `{`},
		{"wrong type", `[1,2,3]`},
		{"trailing value", `{"size":80,"path":"jump.wav"} {}`},
		{"trailing garbage", `{"size":80,"path":"jump.wav"} trailing`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.name != "missing" {
				writeProjectFile(t, dir, "sprites/Hero/index.json", tt.content)
				writeProjectFile(t, dir, "sounds/Jump/index.json", tt.content)
			}
			fs := localDir{base: dir}
			sprite, spriteErr := LoadSpriteConfig(fs, "Hero")
			sound, soundErr := LoadSoundConfig(fs, "Jump")
			if spriteErr == nil || soundErr == nil {
				t.Fatalf("loader errors = %v, %v; want both to fail", spriteErr, soundErr)
			}
			if !reflect.DeepEqual(sprite, SpriteConfig{}) || sound != (SoundConfig{}) {
				t.Fatalf("failed loads returned nonzero configs: sprite=%+v sound=%+v", sprite, sound)
			}
			if tt.name == "missing" && (!errors.Is(spriteErr, os.ErrNotExist) || !errors.Is(soundErr, os.ErrNotExist)) {
				t.Fatalf("missing config errors = %v, %v; want os.ErrNotExist", spriteErr, soundErr)
			}
		})
	}
}

func TestLoadBuilderProjectNormalizesProjectPaths(t *testing.T) {
	dir := t.TempDir()
	indexJSON := `{
		"backdrops":[{"name":"lake","path":"../../res/lake.jpg"},null],
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
	if len(loaded.Project.Backdrops) != 2 || loaded.Project.Backdrops[1] != nil {
		t.Fatalf("null backdrop was not preserved: %#v", loaded.Project.Backdrops)
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

func TestFindFieldPtr(t *testing.T) {
	h := struct{ Val int }{Val: 7}
	if got := FindFieldPtr(reflect.ValueOf(&h), "Val", 0).(*int); *got != 7 {
		t.Fatalf("FindFieldPtr = %d, want 7", *got)
	}
	if got := FindFieldPtr(reflect.ValueOf(&h), "missing", 0); got != nil {
		t.Fatalf("missing field = %v, want nil", got)
	}
}

func TestParseCommandLineFlags(t *testing.T) {
	for _, tt := range []struct {
		name       string
		args       []string
		conf       Config
		want       CommandLineEffects
		fullscreen bool
		wantErr    bool
	}{
		{name: "verbose fullscreen", args: []string{"-v", "-f"}, want: CommandLineEffects{Verbose: true}, fullscreen: true},
		{name: "fullscreen alias", args: []string{"-fullscreen"}, fullscreen: true},
		{name: "help", args: []string{"-h"}, want: CommandLineEffects{ShowHelp: true}},
		{name: "preserve fullscreen", conf: Config{FullScreen: true}, fullscreen: true},
		{name: "invalid flag", args: []string{"-f", "-unknown"}, wantErr: true},
		{name: "disabled parsing", args: []string{"-unknown"}, conf: Config{DontParseFlags: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			conf := tt.conf
			effects, err := ParseCommandLineFlags(fs, tt.args, &conf)
			if (err != nil) != tt.wantErr || effects != tt.want || conf.FullScreen != tt.fullscreen {
				t.Fatalf("ParseCommandLineFlags() = (%+v, %v), fullscreen %v; want (%+v, error %v), fullscreen %v", effects, err, conf.FullScreen, tt.want, tt.wantErr, tt.fullscreen)
			}
			if tt.conf.DontParseFlags && fs.Lookup("v") != nil {
				t.Fatal("disabled parsing registered flags")
			}
		})
	}
}

func TestLoadProjectFontsRejectsUnreadableAndMalformedFamilies(t *testing.T) {
	for _, tc := range []struct{ name, config, want string }{
		{"invalid JSON", "{", "load font family"},
		{"missing face", `{"faces":[{"path":"missing.ttf"}]}`, "open font face"},
		{"empty path", `{"faces":[{"path":""}]}`, "must not be empty"},
		{"absolute path", `{"faces":[{"path":"/font.ttf"}]}`, "relative POSIX path"},
		{"backslash path", `{"faces":[{"path":"dir\\font.ttf"}]}`, "relative POSIX path"},
		{"dot path", `{"faces":[{"path":"."}]}`, "escapes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeProjectFile(t, dir, "fonts/Family/index.json", tc.config)
			if _, err := LoadProjectFonts(localDir{base: dir}, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v,want %q", err, tc.want)
			}
		})
	}
	t.Run("font root is file", func(t *testing.T) {
		dir := t.TempDir()
		writeProjectFile(t, dir, "fonts", "not a directory")
		if _, err := LoadProjectFonts(localDir{base: dir}, nil); err == nil || !strings.Contains(err.Error(), "scan fonts") {
			t.Fatalf("LoadProjectFonts() error = %v, want scan fonts error", err)
		}
	})

	dir := t.TempDir()
	writeProjectFile(t, dir, "fonts/Family/index.json", `{"faces":[{"path":"font.ttf"}]}`)
	writeProjectFile(t, dir, "fonts/Family/font.ttf", "font")
	if _, err := LoadProjectFonts(localDir{base: dir}, []string{"unknown"}); err == nil {
		t.Fatal("unknown preference accepted")
	}
	if _, err := validateFontFamilyName(""); err == nil {
		t.Fatal("empty family accepted")
	}
}

func TestLoadBuilderProjectRejectsInvalidIndexWithoutPartialResult(t *testing.T) {
	for _, test := range []struct {
		name, contents string
		wantErr        error
	}{
		{name: "missing", wantErr: os.ErrNotExist},
		{name: "malformed", contents: `{"fullscreen":true,"run":`, wantErr: io.ErrUnexpectedEOF},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if test.contents != "" {
				writeProjectFile(t, dir, "custom.json", test.contents)
			}
			config := &Config{Index: "custom.json", Title: "caller title"}
			before := *config
			loaded, err := LoadBuilderProject(localDir{base: dir}, config)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("invalid index error = %v, want %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(loaded, LoadedBuilderProject{}) {
				t.Fatalf("failed load published partial project: %+v", loaded)
			}
			if !reflect.DeepEqual(*config, before) {
				t.Fatal("failed load mutated caller config")
			}
		})
	}
}
