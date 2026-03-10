package project

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	_ "github.com/goplus/spx/v2/fs/asset"
)

type fakeGdDir struct {
	path string
}

type localDir struct {
	base string
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

	settings = ResolveDisplaySettings(&ProjectConfig{})
	if settings.WindowScale != 1 || !settings.StretchMode || settings.Debug {
		t.Fatalf("unexpected default settings: %+v", settings)
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

func TestResolveRuntimeConfig(t *testing.T) {
	conf := &Config{Width: 640, Height: 480, EventQueuePolicy: "block"}
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
		BeforeMain: func(item string) {
			got = append(got, "before:"+item)
		},
		RunMain: func(item string) {
			got = append(got, "main:"+item)
		},
		CameraTarget: "hero",
		FollowCamera: func(target string) {
			got = append(got, "follow:"+target)
		},
		OnLoaded: func() {
			got = append(got, "loaded")
		},
	})
	want := []string{"before:a", "main:a", "before:b", "main:b", "follow:hero", "loaded"}
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

	_, value = FieldPtrOrAlloc(reflect.ValueOf(&h).Elem(), 1, FieldAllocConfig{
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
