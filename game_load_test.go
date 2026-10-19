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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	spxlog "github.com/goplus/spx/v3/internal/log"
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

	inits, err := game.loadAndInitSprites(reflect.Value{}, &coreproject.ProjectConfig{
		Zorder: []any{"Back", "Front"},
	}, game.loadSprite, nil)
	if err != nil {
		t.Fatal(err)
	}
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

	inits, err := game.loadAndInitSprites(reflect.ValueOf(&game).Elem(), &coreproject.ProjectConfig{
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
	if err != nil {
		t.Fatal(err)
	}

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

func TestApplyStoredRuntimeConfigReusesResolvedInput(t *testing.T) {
	t.Setenv("SPX_SCREENSHOT_KEY", "")
	t.Setenv("SPX_PROJECT_DIR", "")

	conf := Config{
		Width:      640,
		Height:     480,
		FullScreen: true,
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

func TestInitialLoadAndReloadShareSpritePreparationErrors(t *testing.T) {
	tests := []struct {
		name       string
		configJSON string
		want       string
	}{
		{
			name:       "null animation",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":null}}`,
			want:       `fAnimations["walk"] is null`,
		},
		{
			name:       "missing frame name",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":{"frameFrom":"missing"}}}`,
			want:       `fAnimations["walk"].frameFrom references missing costume "missing"`,
		},
		{
			name:       "frame index out of range",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"walk":{"frameTo":1}}}`,
			want:       `fAnimations["walk"].frameTo index 1 is outside 1 costumes`,
		},
		{
			name:       "missing costume declaration",
			configJSON: `{}`,
			want:       "configuration must define costumes, costumeSet, or costumeMPSet",
		},
		{
			name:       "multiple invalid animations use stable order",
			configJSON: `{"costumeSet":{"nx":1},"fAnimations":{"z":null,"a":null}}`,
			want:       `fAnimations["a"] is null`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := reloadConfigFS{"sprites/Sprite/index.json": test.configJSON}
			game, _, _ := setupReloadPreflightGame(t, files)
			gamer := reflect.ValueOf(game).Elem()

			initialSprite := &reloadPreflightSprite{reloadPreflightGame: game}
			initialErr := game.loadSprite(initialSprite, "Sprite", gamer)
			if initialErr == nil {
				t.Fatal("initial load accepted invalid sprite config")
			}
			_, reloadErr := prepareReload(&game.Game, gamer, strings.NewReader(`{"zorder":["Sprite"]}`))
			if reloadErr == nil {
				t.Fatal("reload accepted invalid sprite config")
			}

			want := `sprite config "Sprite": ` + test.want
			reloadReason := strings.TrimPrefix(reloadErr.Error(), "reload preflight: ")
			if initialErr.Error() != want || reloadReason != want {
				t.Fatalf("sprite preparation errors: initial=%q reload=%q, want %q", initialErr, reloadReason, want)
			}
		})
	}
}

func TestPrepareSpriteConfigBuildsStableAnimationDefinitions(t *testing.T) {
	sourceAnimation := &coreproject.AniConfig{FrameFrom: "idle", FrameTo: "walk"}
	source := coreproject.SpriteConfig{
		Costumes: []*coreproject.CostumeConfig{{Name: "idle"}, {Name: "walk"}},
		FAnimations: map[string]*coreproject.AniConfig{
			"walk": sourceAnimation,
		},
	}
	sourceSnapshot := *sourceAnimation

	first, err := prepareSpriteConfig(&source)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareSpriteConfig(&source)
	if err != nil {
		t.Fatal(err)
	}

	firstAnimation := first.config.FAnimations["walk"]
	secondAnimation := second.config.FAnimations["walk"]
	if firstAnimation == sourceAnimation || secondAnimation == sourceAnimation || firstAnimation == secondAnimation {
		t.Fatal("preparation reused a mutable input or prior prepared animation definition")
	}
	if !reflect.DeepEqual(*sourceAnimation, sourceSnapshot) {
		t.Fatalf("preparation mutated source animation: got %+v, want %+v", *sourceAnimation, sourceSnapshot)
	}
	if !reflect.DeepEqual(*firstAnimation, *secondAnimation) {
		t.Fatalf("repeated preparation differs: first=%+v second=%+v", *firstAnimation, *secondAnimation)
	}
	if firstAnimation.FrameFps != 25 || firstAnimation.TurnToDuration != 1 || firstAnimation.StepDuration != 0.01 ||
		firstAnimation.IFrameFrom != 0 || firstAnimation.IFrameTo != 1 || firstAnimation.Speed != 1 ||
		firstAnimation.Duration != 2.0/25 {
		t.Fatalf("prepared animation = %+v", *firstAnimation)
	}

	definitionSnapshot := *firstAnimation
	sprite := &SpriteImpl{name: "Sprite"}
	sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	animation := &animationComponent{}
	animation.initialize(sprite, &first.config)
	child := animation.cloneFor(&SpriteImpl{name: "child"})
	grandchild := child.cloneFor(&SpriteImpl{name: "grandchild"})
	if child.shared != animation.shared || grandchild.shared != animation.shared {
		t.Fatal("animation clones did not share the prepared definitions")
	}
	if !reflect.DeepEqual(*firstAnimation, definitionSnapshot) {
		t.Fatalf("initialization or cloning mutated prepared animation: got %+v, want %+v", *firstAnimation, definitionSnapshot)
	}
}

func TestBindSpriteOwnerRejectsInvalidStorage(t *testing.T) {
	game := reflect.ValueOf(&Game{}).Elem()
	for _, test := range []struct {
		name   string
		sprite reflect.Value
		want   string
	}{
		{"missing", reflect.ValueOf(&struct{ SpriteImpl }{}).Elem(), "missing owner field"},
		{"unaddressable", reflect.ValueOf(struct {
			SpriteImpl
			Owner *Game
		}{}), "not addressable"},
		{"wrong type", reflect.ValueOf(&struct {
			SpriteImpl
			Owner *int
		}{}).Elem(), "cannot hold"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := bindSpriteOwner(test.sprite, game)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
}

type gameDisplayPlatformMgr struct {
	reloadCommitPlatformMgr
	width, height int64
	fullscreen    bool
}

func (m *gameDisplayPlatformMgr) SetWindowSize(width, height int64, _ bool) {
	m.width, m.height = width, height
}
func (m *gameDisplayPlatformMgr) SetWindowFullscreen(value bool) { m.fullscreen = value }
func TestGameDisplayConfigurationAndBackgroundMusic(t *testing.T) {
	t.Cleanup(func() { enginewrap.Init(engine.WaitMainThread) })
	game := setupReloadCommitGame(t, reloadConfigFS{})
	manager := &gameDisplayPlatformMgr{}
	pkgengine.PlatformMgr = manager
	previousScale := engine.WindowScale()
	t.Cleanup(func() {
		engine.SetWindowScale(previousScale)
		engine.SetDebugMode(false)
		spxlog.SetLevel(spxlog.LevelInfo)
	})
	project := &coreproject.ProjectConfig{Debug: true, FullScreen: true, WindowScale: 2, Bgm: "theme", Map: coreproject.MapConfig{Width: 480, Height: 360}}
	backend := &fakeAudioBackend{}
	game.soundMgr.Init(backend)
	game.sounds["theme"] = &coreproject.SoundConfig{Path: "sounds/theme.wav"}
	game.setupDisplayConfig(project)
	game.setupWorldAndWindow(project)
	game.setupPlatformAndCamera(project)
	game.setupAudioAndTilemap(project)
	if !game.debugState.Debug || !manager.fullscreen || len(backend.plays) != 1 {
		t.Fatalf("debug=%v fullscreen=%v music plays=%v", game.debugState.Debug, manager.fullscreen, backend.plays)
	}
	game.SetWindowSize(640, 480)
	if manager.width != 640 || manager.height != 480 {
		t.Fatalf("window=%dx%d", manager.width, manager.height)
	}
}
