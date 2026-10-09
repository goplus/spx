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

package spx

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/enginewrap"
	"github.com/goplus/spx/v3/internal/ui"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type cameraFollowOverrideGame struct {
	Game
}

type cameraFollowOverrideSprite struct {
	SpriteImpl
	followTarget SpriteName
}

type collisionLayerOrderGame struct {
	Game
}

type stageLayerOrderGame struct {
	Game
	Middle *collisionLayerOrderSprite
	Group  []*collisionLayerOrderSprite
}

type collisionLayerOrderSprite struct {
	SpriteImpl
	onMain func()
}

func (s *cameraFollowOverrideSprite) Main() {
	if s.followTarget != "" {
		s.g.Camera.Follow__1(s.followTarget)
	}
}

func (s *collisionLayerOrderSprite) Main() {
	if s.onMain != nil {
		s.onMain()
	}
}

func newCameraFollowOverrideSprite(g *Game, name string, followTarget SpriteName) *cameraFollowOverrideSprite {
	sprite := &cameraFollowOverrideSprite{followTarget: followTarget}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func newCollisionLayerOrderSprite(g *Game, name string, onMain func()) *collisionLayerOrderSprite {
	sprite := &collisionLayerOrderSprite{onMain: onMain}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func TestRunSpriteCallbacksKeepsManualCameraFollowLast(t *testing.T) {
	game := &cameraFollowOverrideGame{}
	game.initShapeMgr()
	game.camera = &cameraImpl{g: &game.Game}
	game.Camera = game.camera

	spriteA := newCameraFollowOverrideSprite(&game.Game, "SpriteA", "")
	spriteB := newCameraFollowOverrideSprite(&game.Game, "SpriteB", "SpriteB")
	game.shapeMgr.add(spriteOf(spriteA))
	game.shapeMgr.add(spriteOf(spriteB))

	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{Camera: &coreproject.CameraConfig{On: "SpriteA"}},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	game.runBootstrapTasks(generation)

	followTarget, ok := game.camera.followTarget.(*SpriteImpl)
	if !ok {
		t.Fatalf("camera follow target type = %T, want *SpriteImpl", game.camera.followTarget)
	}
	if followTarget != spriteOf(spriteB) {
		t.Fatalf("camera follow target = %q, want %q", followTarget.name, spriteOf(spriteB).name)
	}
}

func TestRefreshCollisionLayersUsesCurrentTargetsFromSetupCollisionData(t *testing.T) {
	game := &collisionLayerOrderGame{}
	game.isAutoSetCollisionLayer = true
	game.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {Index: 0, Layer: 1 << 0},
		"SpriteB": {Index: 1, Layer: 1 << 1},
	}

	var spriteA *collisionLayerOrderSprite
	spriteA = newCollisionLayerOrderSprite(&game.Game, "SpriteA", func() {
		delete(spriteA.physics().collisionTargets, "SpriteB")
	})
	spriteB := newCollisionLayerOrderSprite(&game.Game, "SpriteB", nil)

	spriteA.physics().addCollisionTarget("SpriteB")
	spriteB.physics().addCollisionTarget("SpriteA")

	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	game.runBootstrapTasks(generation)
	game.refreshCollisionLayers()

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != game.sprCollisionInfos["SpriteA"].Layer {
		t.Fatalf("SpriteB collision mask = %d, want %d", got, game.sprCollisionInfos["SpriteA"].Layer)
	}
}

func TestRunSpriteCallbacksRefreshesCollisionLayersRegisteredInMain(t *testing.T) {
	setupRuntimeScheduler(t)

	game := &collisionLayerOrderGame{}
	game.initShapeMgr()
	game.isAutoSetCollisionLayer = true
	game.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {Index: 0, Layer: 1 << 0},
		"SpriteB": {Index: 1, Layer: 1 << 1},
	}

	var spriteA *collisionLayerOrderSprite
	spriteA = newCollisionLayerOrderSprite(&game.Game, "SpriteA", func() {
		spriteA.OnTouchStart__0("SpriteB", func() {})
	})
	spriteB := newCollisionLayerOrderSprite(&game.Game, "SpriteB", nil)
	game.shapeMgr.add(spriteOf(spriteA))
	game.shapeMgr.add(spriteOf(spriteB))

	generation := game.bootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	runBootstrapTasksWithScheduler(t, &game.Game, generation)

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != game.sprCollisionInfos["SpriteB"].Layer {
		t.Fatalf("SpriteA collision mask = %d, want %d", got, game.sprCollisionInfos["SpriteB"].Layer)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != 0 {
		t.Fatalf("SpriteB collision mask = %d, want 0", got)
	}
}

func TestLoadAndInitSpritesReservesLayerZeroForPen(t *testing.T) {
	const penCanvasLayer = 0

	var game Game
	game.initShapeMgr()

	back := newCollisionLayerOrderSprite(&game, "Back", nil)
	front := newCollisionLayerOrderSprite(&game, "Front", nil)
	game.sprs = map[string]Sprite{
		"Back":  back,
		"Front": front,
	}

	inits := game.loadAndInitSprites(reflect.Value{}, &coreproject.ProjectConfig{
		Zorder: []any{"Back", "Front"},
	}, game.loadSprite, nil)
	if got, want := len(inits), 2; got != want {
		t.Fatalf("initialized sprite count = %d, want %d", got, want)
	}
	if got, want := back.runtimeState.Layer, penCanvasLayer+1; got != want {
		t.Fatalf("back layer = %d, want %d", got, want)
	}
	if got, want := front.runtimeState.Layer, penCanvasLayer+2; got != want {
		t.Fatalf("front layer = %d, want %d", got, want)
	}
	if !back.runtimeState.IsLayerDirty || !front.runtimeState.IsLayerDirty {
		t.Fatal("loaded sprite layers must be marked dirty for engine synchronization")
	}
}

func TestLoadAndInitSpritesAssignsContiguousLayersToExpandedStageSprites(t *testing.T) {
	setupCloneSpriteMgr(t)

	var game stageLayerOrderGame
	game.initShapeMgr()

	back := newCollisionLayerOrderSprite(&game.Game, "Back", nil)
	front := newCollisionLayerOrderSprite(&game.Game, "Front", nil)
	game.Middle = newCollisionLayerOrderSprite(&game.Game, "Middle", nil)
	groupProto := newCollisionLayerOrderSprite(&game.Game, "collisionLayerOrderSprite", nil)
	groupProto.baseObj.initWithSize(1, 1)
	groupProto.physics().collisionInfo.Type = physicsColliderNone
	groupProto.physics().triggerInfo.Type = physicsColliderNone
	game.sprs = map[string]Sprite{
		"Back":                      back,
		"Front":                     front,
		"collisionLayerOrderSprite": groupProto,
	}

	inits := game.loadAndInitSprites(reflect.ValueOf(&game).Elem(), &coreproject.ProjectConfig{
		Zorder: []any{
			"Back",
			coreproject.StageShape{
				"type":   "sprites",
				"target": "Group",
				"items": []any{
					coreproject.StageShape{"x": float64(-10)},
					coreproject.StageShape{"x": float64(10)},
				},
			},
			coreproject.StageShape{"type": "sprite", "target": "Middle"},
			"Front",
		},
	}, game.loadSprite, nil)

	if got, want := len(inits), 5; got != want {
		t.Fatalf("initialized sprite count = %d, want %d", got, want)
	}
	if got, want := len(game.Group), 2; got != want {
		t.Fatalf("expanded group size = %d, want %d", got, want)
	}

	wantOrder := []Shape{
		spriteOf(back),
		spriteOf(game.Group[0]),
		spriteOf(game.Group[1]),
		spriteOf(game.Middle),
		spriteOf(front),
	}
	if got := game.getAllShapes(); !reflect.DeepEqual(got, wantOrder) {
		t.Fatalf("loaded shape order = %v, want %v", got, wantOrder)
	}
	for i, shape := range wantOrder {
		spr := shape.(*SpriteImpl)
		if got, want := spr.runtimeState.Layer, firstSpriteLayer+i; got != want {
			t.Fatalf("sprite %d layer = %d, want %d", i, got, want)
		}
		if !spr.runtimeState.IsLayerDirty {
			t.Fatalf("sprite %d layer must be marked dirty for engine synchronization", i)
		}
	}
}

func TestInitRuntimeProxyAppliesCostumeBeforeAwake(t *testing.T) {
	var game Game
	setupCloneSpriteMgr(t)

	sprite := newCloneAwakeOrderSprite(&game, "SpriteA")

	sprite.initRuntimeProxy()

	if sprite.runtimeState.SyncSprite == nil {
		t.Fatal("SyncSprite = nil, want initialized proxy")
	}
	if sprite.runtimeState.IsCostumeDirty {
		t.Fatal("IsCostumeDirty = true, want false after initRuntimeProxy")
	}
}

func TestApplySpritePropsBeforeInitRuntimeProxy(t *testing.T) {
	var game Game
	mgr := setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	source.costumeIndex = 0

	out := reflect.New(reflect.TypeOf(source).Elem()).Elem()
	shape := coreproject.StageShape{
		"visible":      false,
		"size":         2.0,
		"costumeIndex": 1.0,
	}
	properties, err := parseSpriteProperties(shape)
	if err != nil {
		t.Fatal(err)
	}

	var dest *SpriteImpl
	dest, _ = instantiateStageSprite(out, source, properties)

	if dest == nil {
		t.Fatal("instantiateStageSprite returned nil sprite")
	}
	if dest.runtimeState.SyncSprite == nil {
		t.Fatal("SyncSprite = nil, want initialized proxy")
	}
	if dest.runtimeState.IsCostumeDirty {
		t.Fatal("IsCostumeDirty = true, want false after instantiateStageSprite")
	}
	if dest.costumeIndex != 1 {
		t.Fatalf("costumeIndex = %d, want 1", dest.costumeIndex)
	}
	if dest.spriteState.IsVisible {
		t.Fatal("IsVisible = true, want false from stage shape override")
	}
	mgr.assertNoVisibleWrites(t)
	if dest.runtimeState.Scale != 2 {
		t.Fatalf("Scale = %v, want 2", dest.runtimeState.Scale)
	}
}

func TestInstantiateStageSpriteSkipsRuntimeCloneLifecycle(t *testing.T) {
	var game Game
	setupCloneSpriteMgr(t)
	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	out := reflect.New(reflect.TypeOf(source).Elem()).Elem()
	properties, err := parseSpriteProperties(coreproject.StageShape{"visible": true})
	if err != nil {
		t.Fatal(err)
	}
	dest, _ := instantiateStageSprite(out, source, properties)

	if dest.IsCloned() {
		t.Fatal("stage instance is marked cloned")
	}
	if dest.isCloneProxyPublicationBlocked() {
		t.Fatal("stage instance has clone publication state")
	}
	if got := dest.CostumeIndex(); got != 1 {
		t.Fatalf("stage instance costume = %d, want 1 before awake", got)
	}
	if dest.spriteState.HasOnCloned || *source.sawAwakeInMain {
		t.Fatal("stage instantiation ran sprite Main")
	}
	if dest.runtimeState.SyncSprite == nil {
		t.Fatal("stage instance proxy was not initialized")
	}
}

func TestLoadSoundCachesIndependentConfigs(t *testing.T) {
	fs := reloadConfigFS{
		"sounds/Jump/index.json": `{"path":"jump.wav","rate":1,"sampleCount":10}`,
		"sounds/Land/index.json": `{"path":"land.wav","rate":2,"sampleCount":20}`,
	}
	game := &Game{fs: fs, sounds: make(map[string]sound)}
	jump, err := game.loadSound("Jump")
	if err != nil {
		t.Fatal(err)
	}
	land, err := game.loadSound("Land")
	if err != nil {
		t.Fatal(err)
	}
	if jump == land {
		t.Fatal("different sounds share a config pointer")
	}
	if jump.Path != "sounds/Jump/jump.wav" || jump.Rate != 1 || jump.SampleCount != 10 {
		t.Fatalf("jump config changed after loading land: %+v", jump)
	}
	if land.Path != "sounds/Land/land.wav" || land.Rate != 2 || land.SampleCount != 20 {
		t.Fatalf("land config = %+v", land)
	}
	jump.Rate = 3
	if land.Rate != 2 {
		t.Fatal("changing jump affected land")
	}
	delete(fs, "sounds/Jump/index.json")
	cached, err := game.loadSound("Jump")
	if err != nil {
		t.Fatal(err)
	}
	if cached != jump || cached.Rate != 3 {
		t.Fatalf("cached config = %+v, want original pointer %+v", cached, jump)
	}
}

func TestLoadSoundDoesNotCacheErrors(t *testing.T) {
	for _, tt := range []struct{ name, content string }{
		{"missing", ""},
		{"malformed", `{`},
		{"trailing value", `{"path":"jump.wav"} {}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := reloadConfigFS{}
			if tt.content != "" {
				fs["sounds/Jump/index.json"] = tt.content
			}
			game := &Game{fs: fs, sounds: make(map[string]sound)}
			if media, err := game.loadSound("Jump"); err == nil || media != nil {
				t.Fatalf("failed load = %+v, %v; want nil config and an error", media, err)
			}
			if _, ok := game.sounds["Jump"]; ok {
				t.Fatal("failed load was cached")
			}
			fs["sounds/Jump/index.json"] = `{"path":"jump.wav"}`
			media, err := game.loadSound("Jump")
			if err != nil {
				t.Fatal(err)
			}
			if media.Path != "sounds/Jump/jump.wav" || game.sounds["Jump"] != media {
				t.Fatalf("retry did not cache the normalized config: %+v", media)
			}
		})
	}
}

func TestApplyStoredRuntimeConfigReusesResolvedInput(t *testing.T) {
	t.Setenv("SPX_SCREENSHOT_KEY", "")
	t.Setenv("SPX_PROJECT_DIR", "")

	conf := Config{
		Width:            640,
		Height:           480,
		FullScreen:       true,
		EventQueuePolicy: "block",
	}

	var game Game

	proj := coreproject.ProjectConfig{}
	game.applyRuntimeConfig(&conf, &proj)

	cwd, _ := os.Getwd()
	wantTitle := filepath.Base(cwd) + " (by spx)"
	if game.runtimeConfigInput.Title != wantTitle {
		t.Fatalf("runtimeConfigInput.Title = %q, want %q", game.runtimeConfigInput.Title, wantTitle)
	}
	if !proj.FullScreen {
		t.Fatal("applyRuntimeConfig did not propagate fullscreen override")
	}
	if got := game.eventQueueState.EventQueuePolicy; got != coreevent.QueueBlock {
		t.Fatalf("EventQueuePolicy = %v, want block", got)
	}
	if game.displayState.WindowWidth != 640 || game.displayState.WindowHeight != 480 {
		t.Fatalf("window size = %dx%d, want 640x480", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}

	proj = coreproject.ProjectConfig{}
	game.applyStoredRuntimeConfig(&proj)
	if !proj.FullScreen {
		t.Fatal("applyStoredRuntimeConfig did not reuse stored fullscreen override")
	}
	if game.displayState.WindowWidth != 640 || game.displayState.WindowHeight != 480 {
		t.Fatalf("reapplied window size = %dx%d, want 640x480", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}
}

func TestRuntimeTitleUsesProjectDirectory(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "02-Dragon")
	sessionDir := filepath.Join(projectDir, ".temp")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sessionDir)
	t.Setenv("SPX_PROJECT_DIR", projectDir)
	t.Setenv("SPX_SCREENSHOT_KEY", "")

	for _, title := range []string{"", "My Dragon Game"} {
		t.Run(title, func(t *testing.T) {
			var game Game
			proj := coreproject.ProjectConfig{}
			game.applyRuntimeConfig(&Config{Title: title}, &proj)
			want := title
			if want == "" {
				want = "02-Dragon (by spx)"
			}
			if got := game.runtimeConfigInput.Title; got != want {
				t.Fatalf("title = %q, want %q", got, want)
			}
			// Reload must retain the resolved title even if the environment changes.
			t.Setenv("SPX_PROJECT_DIR", filepath.Join(projectDir, "other"))
			game.applyStoredRuntimeConfig(&proj)
			if got := game.runtimeConfigInput.Title; got != want {
				t.Fatalf("reloaded title = %q, want %q", got, want)
			}
		})
	}
}

type stagePlanGame struct {
	Game
	Value    collisionLayerOrderSprite
	Pointer  *collisionLayerOrderSprite
	Dynamic  Sprite
	values   []collisionLayerOrderSprite
	pointers []*collisionLayerOrderSprite
}

func prepareStageEntries(gamer reflect.Value, zorder []any) ([]preparedStageEntry, error) {
	entries := make([]preparedStageEntry, len(zorder))
	for i, raw := range zorder {
		entry, err := prepareStageEntry(gamer, raw)
		if err != nil {
			return nil, err
		}
		entries[i] = entry
	}
	return entries, nil
}

func TestPreparedStageUsesLiveFieldsAndOwnsParsedValues(t *testing.T) {
	for _, shadow := range []bool{false, true} {
		name := "cold"
		if shadow {
			name = "reload shadow"
		}
		t.Run(name, func(t *testing.T) {
			setupCloneSpriteMgr(t)
			game := &stagePlanGame{}
			game.initShapeMgr()
			game.Value = *newCollisionLayerOrderSprite(&game.Game, "Value", nil)
			game.Pointer = newCollisionLayerOrderSprite(&game.Game, "Pointer", nil)
			game.Dynamic = newCollisionLayerOrderSprite(&game.Game, "Dynamic", nil)
			back := newCollisionLayerOrderSprite(&game.Game, "Back", nil)
			front := newCollisionLayerOrderSprite(&game.Game, "Front", nil)
			prototype := newCollisionLayerOrderSprite(&game.Game, "collisionLayerOrderSprite", nil)
			prototype.baseObj.initWithSize(1, 1)
			prototype.physics().collisionInfo.Type = physicsColliderNone
			prototype.physics().triggerInfo.Type = physicsColliderNone
			game.sprs = map[string]Sprite{"Back": back, "Front": front, "collisionLayerOrderSprite": prototype}
			raw := []any{
				"Back",
				coreproject.StageShape{"type": "sprite", "target": "Value", "x": 1.0},
				coreproject.StageShape{"type": "sprite", "target": "Pointer", "x": 2.0},
				coreproject.StageShape{"type": "sprite", "target": "Dynamic", "x": 3.0},
				coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{coreproject.StageShape{"x": 4.0}, coreproject.StageShape{"x": 5.0}}},
				coreproject.StageShape{"type": "sprites", "target": "pointers", "items": []any{coreproject.StageShape{"x": 6.0}}},
				coreproject.StageShape{"type": "monitor", "target": "", "val": "missing", "name": "missing", "label": "Missing", "mode": 1.0, "x": 0.0, "y": 0.0, "visible": true},
				"Front",
			}
			source := game
			if shadow {
				source = &stagePlanGame{Dynamic: &collisionLayerOrderSprite{}}
			}
			entries, err := prepareStageEntries(reflect.ValueOf(source).Elem(), raw)
			if err != nil {
				t.Fatal(err)
			}
			for i, value := range raw {
				if shape, ok := value.(coreproject.StageShape); ok {
					clear(shape)
				}
				raw[i] = nil
			}
			inits := game.loadAndInitSprites(reflect.ValueOf(game).Elem(), nil, nil, entries)
			if len(inits) != 8 {
				t.Fatalf("initialized %d sprites, want 8", len(inits))
			}
			if len(game.values) != 2 || len(game.pointers) != 1 {
				t.Fatalf("group sizes = %d, %d", len(game.values), len(game.pointers))
			}
			want := []Sprite{back, &game.Value, game.Pointer, game.Dynamic, &game.values[0], &game.values[1], game.pointers[0], front}
			shapes := game.getAllShapes()
			for i, sprite := range want {
				if inits[i] != sprite || shapes[i] != spriteOf(sprite) {
					t.Fatalf("sprite %d bound to wrong live object", i)
				}
				if spriteOf(sprite).runtimeState.Layer != firstSpriteLayer+i {
					t.Fatalf("sprite %d layer = %d", i, spriteOf(sprite).runtimeState.Layer)
				}
				if i > 0 && i < 7 && spriteOf(sprite).Xpos() != float64(i) {
					t.Fatalf("sprite %d x = %v", i, spriteOf(sprite).Xpos())
				}
			}
		})
	}
}

func TestPreparedDisplayShapesOwnAllConfiguration(t *testing.T) {
	for _, kind := range []string{"monitor", "stageMonitor"} {
		t.Run(kind, func(t *testing.T) {
			shape := coreproject.StageShape{
				"type": kind, "target": "", "val": "score", "name": "score", "label": "Score", "mode": "slider",
				"x": 2.0, "y": 3.0, "visible": true, "color": "#123456", "width": 120.0, "height": 90.0,
				"sliderMin": 4.0, "sliderMax": 8.0, "isDiscrete": false,
			}
			entry, err := prepareStageEntry(reflect.Value{}, shape)
			if err != nil {
				t.Fatal(err)
			}
			clear(shape)
			if entry.kind != stageMonitor {
				t.Fatalf("kind = %v, want stageMonitor", entry.kind)
			}
			got := entry.monitor
			if got.config.Name != "score" || got.config.X != 2 || got.config.Y != 3 || !got.config.Visible {
				t.Fatalf("prepared monitor config = %+v", got.config)
			}
			color, _ := mathf.NewColorAny("#123456")
			if got.style.Appearance != ui.MonitorAppearanceSlider || got.style.Color != color || got.style.Dimensions != mathf.NewVec2(120, 90) || got.style.Slider != (ui.MonitorSlider{Min: 4, Max: 8, Step: 0.01}) {
				t.Fatalf("prepared monitor style = %+v", got.style)
			}
		})
	}
	t.Run("measure", func(t *testing.T) {
		shape := coreproject.StageShape{"type": "measure", "size": 10.0, "x": 20.0, "y": 30.0, "scale": 2.0, "heading": 45.0}
		entry, err := prepareStageEntry(reflect.Value{}, shape)
		if err != nil {
			t.Fatal(err)
		}
		clear(shape)
		if entry.kind != stageMeasure {
			t.Fatalf("kind = %v, want stageMeasure", entry.kind)
		}
		got := entry.measure
		if got.Size != 10 || got.X != 20 || got.Y != 30 || got.Scale != 2 || got.Heading != 45 {
			t.Fatalf("prepared measure = %+v", got)
		}
	})
}

func TestPreparedStagePreservesDirectFieldLookup(t *testing.T) {
	type embedded struct{ Target *collisionLayerOrderSprite }
	type holder struct{ embedded }
	game := holder{embedded{Target: &collisionLayerOrderSprite{}}}
	_, err := prepareStageEntries(reflect.ValueOf(&game).Elem(), []any{coreproject.StageShape{"type": "sprite", "target": "Target"}})
	if err == nil {
		t.Fatal("promoted field unexpectedly accepted as direct stage target")
	}
}

func TestColdLoadKeepsUnregisteredSpriteFieldsUnloaded(t *testing.T) {
	game := &reloadDirectCommitGame{}
	files := reloadConfigFS{}
	setupReloadCommitRuntime(t, files, game, nil, false)
	loadGameSprites(&game.Game, reflect.ValueOf(game).Elem(), files, &coreproject.ProjectConfig{})
	if game.DirectCommitSprite == nil {
		t.Fatal("cold load did not allocate the sprite field")
	}
	if len(game.sprs) != 0 {
		t.Fatalf("cold load initialized unregistered sprites: %v", game.sprs)
	}
}

func TestReloadStageDoesNotReadProjectZOrderDuringCommit(t *testing.T) {
	game := setupReloadCommitGame(t, reloadConfigFS{"sprites/reloadCommitSprite/index.json": preparedPropertiesSpriteConfig})
	gamer := reflect.ValueOf(game).Elem()
	plan, err := prepareReload(&game.Game, gamer, strings.NewReader(`{"zorder":[{"type":"sprites","target":"Sprites","items":[{"x":7},{"x":9}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	clear(plan.project.Zorder[0].(coreproject.StageShape))
	plan.project.Zorder = []any{false}
	inits := game.loadAndInitSprites(gamer, &plan.project, plan.spriteLoader(&game.Game), plan.stage)
	if len(inits) != 2 || len(game.Sprites) != 2 || game.Sprites[0].Xpos() != 7 || game.Sprites[1].Xpos() != 9 {
		t.Fatalf("reload did not commit prepared stage entries: %v", game.Sprites)
	}
}

func TestColdStageKeepsLazyLoadFailureBeforeLaterParseFailure(t *testing.T) {
	var game Game
	game.initShapeMgr()
	game.sprs = make(map[string]Sprite)
	game.typs = map[string]reflect.Type{"Sprite": reflect.TypeFor[collisionLayerOrderSprite]()}
	want := errors.New("first sprite config failed")
	defer func() {
		if got := recover(); got != want {
			t.Fatalf("cold load failure = %v, want first sprite config error", got)
		}
	}()
	game.loadAndInitSprites(reflect.Value{}, &coreproject.ProjectConfig{Zorder: []any{
		"Sprite", coreproject.StageShape{"type": "measure", "size": "invalid"},
	}}, func(Sprite, string, reflect.Value) error { return want }, nil)
	t.Fatal("cold stage load unexpectedly succeeded")
}

func TestPrepareStageEntryEmptyValues(t *testing.T) {
	game := &stagePlanGame{}
	gamer := reflect.ValueOf(game).Elem()
	for _, test := range []struct {
		name   string
		raw    any
		kind   stageEntryKind
		target string
	}{
		{"empty sprite name", "", stageNamedSprite, ""},
		{"empty value sprite group", coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{}}, stageSprites, "values"},
		{"empty pointer sprite group", coreproject.StageShape{"type": "sprites", "target": "pointers", "items": []any{}}, stageSprites, "pointers"},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry, err := prepareStageEntry(gamer, test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if entry.kind != test.kind || entry.name != test.target || len(entry.properties) != 0 {
				t.Fatalf("entry = %+v, want kind %v, target %q and no properties", entry, test.kind, test.target)
			}
			if entry.kind == stageSprites {
				if gamer.Type().Field(entry.fieldIndex).Name != test.target || entry.spriteType != reflect.TypeFor[collisionLayerOrderSprite]() {
					t.Fatalf("empty group target = %q, field index = %d, sprite type = %v", entry.name, entry.fieldIndex, entry.spriteType)
				}
			}
		})
	}
}

func TestPrepareStageEntryErrors(t *testing.T) {
	game := &struct {
		Sprite   *collisionLayerOrderSprite
		Number   int
		BadItems []int
		Sprites  []*collisionLayerOrderSprite
	}{}
	tests := []struct {
		name string
		raw  any
		want string
	}{
		{"nil", nil, "invalid zorder entry type <nil>"},
		{"number", 42, "invalid zorder entry type int"},
		{"boolean", false, "invalid zorder entry type bool"},
		{"array", []any{}, "invalid zorder entry type []interface {}"},
		{"nil shape", coreproject.StageShape(nil), "invalid stage shape type"},
		{"missing type", coreproject.StageShape{}, "invalid stage shape type"},
		{"null type", coreproject.StageShape{"type": nil}, "invalid stage shape type"},
		{"non-string type", coreproject.StageShape{"type": 1.0}, "invalid stage shape type"},
		{"unknown shape", coreproject.StageShape{"type": "unknown"}, "unknown shape - unknown"},
		{"case-sensitive alias", coreproject.StageShape{"type": "StageMonitor"}, "unknown shape - StageMonitor"},
		{"empty type", coreproject.StageShape{"type": ""}, "unknown shape - "},
		{"monitor validation", coreproject.StageShape{"type": "monitor"}, `stage shape field "target" is required`},
		{"alias validation", coreproject.StageShape{"type": "stageMonitor"}, `stage shape field "target" is required`},
		{"measure validation", coreproject.StageShape{"type": "measure", "size": "large"}, `stage shape field "size" has type string, want float64`},
		{"measure color", coreproject.StageShape{"type": "measure", "size": 10.0, "x": 0.0, "y": 0.0, "color": []any{1.0}}, `stage shape field "color": unsupported color format`},
		{"monitor mode", coreproject.StageShape{"type": "monitor", "target": "", "val": "score", "name": "score", "label": "Score", "mode": nil, "x": 0.0, "y": 0.0, "visible": true}, `stage shape field "mode" has type <nil>, want float64`},
		{"missing target", coreproject.StageShape{"type": "sprite"}, "stage shape target must be a non-empty string"},
		{"empty target", coreproject.StageShape{"type": "sprite", "target": ""}, "stage shape target must be a non-empty string"},
		{"non-string target", coreproject.StageShape{"type": "sprites", "target": 1.0}, "stage shape target must be a non-empty string"},
		{"unknown sprite", coreproject.StageShape{"type": "sprite", "target": "Missing"}, `stage sprite target "Missing" is not defined`},
		{"non-sprite field", coreproject.StageShape{"type": "sprite", "target": "Number"}, `stage sprite target "Number" is not a sprite field`},
		{"sprite property", coreproject.StageShape{"type": "sprite", "target": "Sprite", "x": "left"}, `stage shape field "x" has type string, want float64`},
		{"missing items", coreproject.StageShape{"type": "sprites", "target": "Sprites"}, "stage shape items must be an array"},
		{"null items", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": nil}, "stage shape items must be an array"},
		{"non-array items", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": false}, "stage shape items must be an array"},
		{"unknown sprites", coreproject.StageShape{"type": "sprites", "target": "Missing", "items": []any{}}, `stage sprites target "Missing" is not defined`},
		{"non-slice field", coreproject.StageShape{"type": "sprites", "target": "Sprite", "items": []any{}}, `stage sprites target "Sprite" is not a slice`},
		{"non-sprite items", coreproject.StageShape{"type": "sprites", "target": "BadItems", "items": []any{}}, `stage sprites target "BadItems" has invalid item type int`},
		{"invalid item", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": []any{coreproject.StageShape{}, false}}, `stage sprites target "Sprites" item[1] has invalid type bool`},
		{"item property", coreproject.StageShape{"type": "sprites", "target": "Sprites", "items": []any{coreproject.StageShape{}, coreproject.StageShape{"x": "left"}}}, `stage sprites target "Sprites" item[1]: stage shape field "x" has type string, want float64`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := prepareStageEntry(reflect.ValueOf(game).Elem(), test.raw)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

type stageEntryPanicExtMgr struct{ pkgengine.IExtMgr }

func (*stageEntryPanicExtMgr) OnRuntimePanic(message string) { panic(message) }

func TestColdStageWrapsEntryFailureWithLayer(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	originalExtMgr := pkgengine.ExtMgr
	pkgengine.ExtMgr = &stageEntryPanicExtMgr{}
	t.Cleanup(func() { pkgengine.ExtMgr = originalExtMgr })
	game := &stagePlanGame{}
	game.initShapeMgr()
	defer func() {
		got := recover()
		if got != "zorder[1]: invalid stage shape type" {
			t.Fatalf("cold load failure = %v, want indexed stage shape error", got)
		}
	}()
	game.loadAndInitSprites(reflect.ValueOf(game).Elem(), &coreproject.ProjectConfig{Zorder: []any{
		coreproject.StageShape{"type": "sprites", "target": "values", "items": []any{}}, coreproject.StageShape{},
	}}, nil, nil)
	t.Fatal("cold stage load unexpectedly succeeded")
}
