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
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	spxlog "github.com/goplus/spx/v4/internal/log"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
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

type layoutInterfaceOwnerSprite struct {
	SpriteImpl
	owner any
}

type gameDisplayPlatformMgr struct {
	gameTestPlatformMgr
	width, height int64
	fullscreen    bool
}

type gameTestFS map[string]string

type spriteLoadTestGame struct {
	Game
}

type spriteLoadTestSprite struct {
	SpriteImpl
	*spriteLoadTestGame
}

type stageLoadTestGame struct {
	Game
	Empty   []*stageLoadTestSprite
	Sprites []*stageLoadTestSprite
}

type stageLoadTestSprite struct {
	SpriteImpl
	*stageLoadTestGame
}

type directSpriteTestGame struct {
	Game
	DirectLoadSprite *DirectLoadSprite
}

type DirectLoadSprite struct {
	SpriteImpl
	*directSpriteTestGame
}

type gameTestSpriteMgr struct {
	*spyCloneSpriteMgr
}

type gameTestExtMgr struct {
	pkgengine.IExtMgr
}

type gameTestPlatformMgr struct {
	pkgengine.IPlatformMgr
}

type gameTestCameraMgr struct {
	pkgengine.ICameraMgr
}

type gameTestInputMgr struct {
	pkgengine.IInputMgr
}

type gameTestPenMgr struct {
	pkgengine.IPenMgr
}

type gameTestPhysicsMgr struct {
	pkgengine.IPhysicsMgr
	collisionModes []bool
}

type gameTestResMgr struct {
	pkgengine.IResMgr
}

type gameTestTilemapparserMgr struct {
	pkgengine.ITilemapparserMgr
	loadedPath   string
	destroyCalls int
}

type projectTestGame struct {
	Game
	projectTestSprite *projectTestSprite
	Dynamic           Sprite
	Value             projectTestSprite
	Last              *projectTestSprite
	Items             []*projectTestSprite
}

type projectTestSprite struct {
	SpriteImpl
	*projectTestGame
	marker string
}

type recordingProjectFS struct {
	files map[string]string
	reads []string
}

const testSpriteConfig = `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}]}`

type prototypeConflictGame struct {
	Game
	projectTestSprite *conflictingSprite
	Items             []*projectTestSprite
}

type conflictingSprite struct {
	SpriteImpl
	*prototypeConflictGame
}

type layoutInvalidOwnerSprite struct {
	SpriteImpl
	owner *int
	value int
}

type recordingProjectFontBridge struct {
	defaultPath string
	paths       []string
	families    []string
	preferences []string
	message     string
}

func TestPrepareStageBootstrapKeepsManualCameraFollowLast(t *testing.T) {
	game := &cameraFollowOverrideGame{}
	game.initShapeMgr()
	game.camera = &cameraImpl{g: &game.Game}
	game.Camera = game.camera

	spriteA := newCameraFollowOverrideSprite(&game.Game, "SpriteA", "")
	spriteB := newCameraFollowOverrideSprite(&game.Game, "SpriteB", "SpriteB")
	game.shapeMgr.add(spriteOf(spriteA))
	game.shapeMgr.add(spriteOf(spriteB))

	generation := game.bootstrapGeneration()
	game.prepareStageBootstrap(
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
	game.prepareStageBootstrap(
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

func TestPrepareStageBootstrapRefreshesCollisionLayersRegisteredInMain(t *testing.T) {
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
	game.prepareStageBootstrap(
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

func TestLoadStageEntriesReservesLayerZeroForPen(t *testing.T) {
	const penCanvasLayer = 0

	var game Game
	game.initShapeMgr()

	back := newCollisionLayerOrderSprite(&game, "Back", nil)
	front := newCollisionLayerOrderSprite(&game, "Front", nil)
	game.sprs = map[string]Sprite{
		"Back":  back,
		"Front": front,
	}

	sprites, err := game.loadStageEntries(reflect.Value{}, []any{"Back", "Front"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(sprites), 2; got != want {
		t.Fatalf("loaded sprite count = %d, want %d", got, want)
	}
	if got, want := back.runtimeState.layer, penCanvasLayer+1; got != want {
		t.Fatalf("back layer = %d, want %d", got, want)
	}
	if got, want := front.runtimeState.layer, penCanvasLayer+2; got != want {
		t.Fatalf("front layer = %d, want %d", got, want)
	}
	if !back.runtimeState.isLayerDirty || !front.runtimeState.isLayerDirty {
		t.Fatal("loaded sprite layers must be marked dirty for engine synchronization")
	}
}

func TestLoadStageEntriesAssignsContiguousLayersToExpandedStageSprites(t *testing.T) {
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

	sprites, err := game.loadStageEntries(reflect.ValueOf(&game).Elem(), []any{
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
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(sprites), 5; got != want {
		t.Fatalf("loaded sprite count = %d, want %d", got, want)
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
		if got, want := spr.runtimeState.layer, firstSpriteLayer+i; got != want {
			t.Fatalf("sprite %d layer = %d, want %d", i, got, want)
		}
		if !spr.runtimeState.isLayerDirty {
			t.Fatalf("sprite %d layer must be marked dirty for engine synchronization", i)
		}
	}
}

func TestInitRuntimeProxyAppliesCostumeBeforeAwake(t *testing.T) {
	var game Game
	setupCloneSpriteMgr(t)

	sprite := newCloneAwakeOrderSprite(&game, "SpriteA")

	sprite.initRuntimeProxy()

	if sprite.runtimeState.syncSprite == nil {
		t.Fatal("SyncSprite = nil, want initialized proxy")
	}
	if sprite.runtimeState.isCostumeDirty {
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
	if dest.runtimeState.syncSprite == nil {
		t.Fatal("SyncSprite = nil, want initialized proxy")
	}
	if dest.runtimeState.isCostumeDirty {
		t.Fatal("IsCostumeDirty = true, want false after instantiateStageSprite")
	}
	if dest.costumeIndex != 1 {
		t.Fatalf("costumeIndex = %d, want 1", dest.costumeIndex)
	}
	if dest.spriteState.isVisible {
		t.Fatal("IsVisible = true, want false from stage shape override")
	}
	mgr.assertNoVisibleWrites(t)
	if dest.runtimeState.scale != 2 {
		t.Fatalf("Scale = %v, want 2", dest.runtimeState.scale)
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
	if dest.spriteState.hasOnCloned || *source.sawAwakeInMain {
		t.Fatal("stage instantiation ran sprite Main")
	}
	if dest.runtimeState.syncSprite == nil {
		t.Fatal("stage instance proxy was not initialized")
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
			got := game.applyRuntimeConfig(&Config{Title: title}, &proj)
			want := title
			if want == "" {
				want = "02-Dragon (by spx)"
			}
			if got != want {
				t.Fatalf("title = %q, want %q", got, want)
			}
		})
	}
}

func TestPrepareSpriteRejectsInvalidConfig(t *testing.T) {
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
			files := gameTestFS{"sprites/Sprite/index.json": test.configJSON}
			gamer := reflect.ValueOf(&spriteLoadTestGame{}).Elem()
			_, err := prepareSprite(files, "Sprite", reflect.TypeFor[spriteLoadTestSprite](), gamer)
			if err == nil {
				t.Fatal("sprite preparation accepted invalid config")
			}

			want := `sprite config "Sprite": ` + test.want
			if err.Error() != want {
				t.Fatalf("sprite preparation error = %q, want %q", err, want)
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

func TestSpriteLayoutRejectsNonStructTypes(t *testing.T) {
	for _, typ := range []reflect.Type{nil, reflect.TypeFor[int](), reflect.TypeFor[*SpriteImpl]()} {
		layout, err := inspectSpriteLayout(typ)
		if layout != nil || err == nil || !strings.Contains(err.Error(), "must be a struct") {
			t.Fatalf("inspectSpriteLayout(%v) = %v, %v; want no layout and a struct type error", typ, layout, err)
		}
	}
}

func TestPrepareSpriteOwner(t *testing.T) {
	files := gameTestFS{"sprites/owner/index.json": testSpriteConfig}
	game := setupStageLoadTestGame(t, files)
	for _, test := range []struct {
		name          string
		sprite        reflect.Value
		unaddressable bool
		want          string
	}{
		{"missing", reflect.ValueOf(&struct{ SpriteImpl }{}).Elem(), false, "missing owner field"},
		{"unaddressable game", reflect.ValueOf(&layoutInterfaceOwnerSprite{}).Elem(), true, "not addressable"},
		{"wrong type", reflect.ValueOf(&struct {
			SpriteImpl
			Owner *int
		}{}).Elem(), false, "cannot hold"},
		{"unexported interface", reflect.ValueOf(&layoutInterfaceOwnerSprite{}).Elem(), false, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			gamer := reflect.ValueOf(&game.Game).Elem()
			if test.unaddressable {
				gamer = reflect.Zero(gamer.Type())
			}
			prepared, err := prepareSprite(files, "owner", test.sprite.Type(), gamer)
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error=%v, want %q", err, test.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sprite := test.sprite.Addr().Interface().(Sprite)
			game.loadPreparedSprite(sprite, "owner", gamer, &prepared)
			owner := coreproject.WritableField(test.sprite.Field(spriteOwnerField))
			if owner.Interface() != gamer.Addr().Interface() {
				t.Fatal("owner does not reference the original game")
			}
		})
	}
}

func TestPrepareProjectRequiresAddressableGame(t *testing.T) {
	game := &Game{}
	fs := &recordingProjectFS{}
	for _, entries := range [][]any{nil, {"Lazy"}} {
		_, err := game.prepareProject(fs, &coreproject.ProjectConfig{Zorder: entries}, reflect.Zero(reflect.TypeFor[Game]()))
		if err == nil || !strings.Contains(err.Error(), "game value is not addressable") {
			t.Fatalf("prepareProject error = %v, want original game addressability error", err)
		}
		if len(fs.reads) != 0 {
			t.Fatalf("unaddressable game read sprite configuration: %v", fs.reads)
		}
	}
}

func TestGameDisplayConfigurationAndBackgroundMusic(t *testing.T) {
	t.Cleanup(func() { enginewrap.Init(engine.WaitMainThread) })
	game := setupStageLoadTestGame(t, gameTestFS{})
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

func TestPrepareSpriteConfigRejectsInvalidLayout(t *testing.T) {
	tests := []struct {
		name      string
		config    coreproject.SpriteConfig
		wantError string
	}{
		{
			name:      "empty costumes",
			config:    coreproject.SpriteConfig{Costumes: []*coreproject.CostumeConfig{}},
			wantError: "costumes must not be empty",
		},
		{
			name:      "null costume",
			config:    coreproject.SpriteConfig{Costumes: []*coreproject.CostumeConfig{nil}},
			wantError: "costumes[0] is null",
		},
		{
			name:      "invalid costume set frame count",
			config:    coreproject.SpriteConfig{CostumeSet: &coreproject.CostumeSet{}},
			wantError: "invalid frame count 0",
		},
		{
			name: "incomplete costume set items",
			config: coreproject.SpriteConfig{CostumeSet: &coreproject.CostumeSet{
				Nx:    2,
				Items: []coreproject.CostumeSetItem{{NamePrefix: "idle", N: 1}},
			}},
			wantError: "incomplete frame loading",
		},
		{
			name:      "empty multipart costume set",
			config:    coreproject.SpriteConfig{CostumeMPSet: &coreproject.CostumeMPSet{}},
			wantError: "costumeMPSet.parts must not be empty",
		},
		{
			name: "animation references missing costume",
			config: coreproject.SpriteConfig{
				CostumeSet: &coreproject.CostumeSet{Nx: 1},
				FAnimations: map[string]*coreproject.AniConfig{
					"walk": {FrameFrom: "missing", FrameTo: "0"},
				},
			},
			wantError: `frameFrom references missing costume "missing"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := prepareSpriteConfig(&test.config)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("prepareSpriteConfig error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestPrepareSpriteConfigAcceptsSupportedLayouts(t *testing.T) {
	tests := []coreproject.SpriteConfig{
		{Costumes: []*coreproject.CostumeConfig{{Name: "idle"}}},
		{CostumeSet: &coreproject.CostumeSet{Nx: 2}},
		{CostumeMPSet: &coreproject.CostumeMPSet{Parts: []coreproject.CostumeSetPart{{Nx: 1}, {Nx: 2}}}},
		{
			CostumeSet: &coreproject.CostumeSet{Nx: 2, Items: []coreproject.CostumeSetItem{{NamePrefix: "walk", N: 2}}},
			FAnimations: map[string]*coreproject.AniConfig{
				"walk": {FrameFrom: "walk0", FrameTo: "walk1"},
			},
		},
	}

	for i := range tests {
		if _, err := prepareSpriteConfig(&tests[i]); err != nil {
			t.Errorf("prepareSpriteConfig(config %d) error = %v", i, err)
		}
	}
}

func TestAllocSpriteFieldsUsesGeneratedStorage(t *testing.T) {
	original := &projectTestSprite{marker: "original"}
	holder := struct {
		pointer *projectTestSprite
		iface   Sprite
		value   projectTestSprite
	}{pointer: original, iface: original, value: projectTestSprite{marker: "value"}}
	game := Game{typs: map[string]reflect.Type{
		"pointer": reflect.TypeFor[projectTestSprite](),
		"iface":   reflect.TypeFor[projectTestSprite](),
		"value":   reflect.TypeFor[projectTestSprite](),
	}}
	fields := reflect.ValueOf(&holder).Elem()
	for i, wantName := range []string{"pointer", "iface", "value"} {
		name, sprite := game.allocSpriteField(fields, i)
		want := []Sprite{holder.pointer, holder.iface, &holder.value}[i]
		if name != wantName || sprite == nil || sprite != want {
			t.Fatalf("field %q = %v, want %q bound to its live field", name, sprite, wantName)
		}
	}
	if holder.pointer == original || holder.iface == original || holder.pointer == holder.iface {
		t.Fatal("pointer and interface fields did not receive separate fresh sprites")
	}
	if holder.value.marker != "value" || original.marker != "original" {
		t.Fatal("allocation changed value fields or previously referenced sprites")
	}
}

func TestPrepareSpriteFieldsReturnsFirstError(t *testing.T) {
	game := &struct {
		Game
		Skipped *spriteLoadTestSprite
		Data    any
		First   *spriteLoadTestSprite
		Later   *spriteLoadTestSprite
	}{}
	game.typs = map[string]reflect.Type{
		"Data":  reflect.TypeFor[int](),
		"First": reflect.TypeFor[spriteLoadTestSprite](),
		"Later": reflect.TypeFor[spriteLoadTestSprite](),
	}
	fs := &recordingProjectFS{}

	_, err := game.prepareProject(fs, &coreproject.ProjectConfig{}, reflect.ValueOf(game).Elem())
	if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), `load sprite "First"`) {
		t.Fatalf("prepareProject error = %v, want named sprite error wrapping os.ErrNotExist", err)
	}
	if game.Skipped != nil || game.First != nil || game.Data != nil || game.Later != nil {
		t.Fatal("failed preparation changed live fields")
	}
	if want := []string{"sprites/First/index.json"}; !reflect.DeepEqual(fs.reads, want) {
		t.Fatalf("resource reads = %v, want %v", fs.reads, want)
	}
}

func TestPrepareProjectFailurePreservesLiveGame(t *testing.T) {
	for _, test := range []struct {
		name    string
		last    string
		project coreproject.ProjectConfig
		want    string
	}{
		{
			name: "late sprite configuration",
			last: `{"costumes":[null]}`,
			want: `load sprite "Last"`,
		},
		{
			name:    "tilemap before stage",
			last:    testSpriteConfig,
			project: coreproject.ProjectConfig{TilemapPath: "broken.json", Zorder: []any{"Missing"}},
			want:    `load tilemap "broken.json"`,
		},
		{
			name:    "late stage entry",
			last:    testSpriteConfig,
			project: coreproject.ProjectConfig{Zorder: []any{"projectTestSprite", coreproject.StageShape{}}},
			want:    `zorder[1]: invalid stage shape type`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := &projectTestSprite{marker: "pointer"}
			dynamic := &projectTestSprite{marker: "interface"}
			last := &projectTestSprite{marker: "last"}
			game := &projectTestGame{
				projectTestSprite: original,
				Dynamic:           dynamic,
				Value:             projectTestSprite{marker: "value"},
				Last:              last,
				Items:             []*projectTestSprite{original},
			}
			game.typs = make(map[string]reflect.Type)
			for _, name := range []string{"projectTestSprite", "Dynamic", "Value", "Last", "Missing"} {
				game.typs[name] = reflect.TypeFor[projectTestSprite]()
			}
			game.sprs = map[string]Sprite{"Existing": original}
			game.shapeMgr.items = []Shape{&original.SpriteImpl}
			beforeValue := &projectTestSprite{marker: "value"}
			fs := &recordingProjectFS{files: map[string]string{
				"sprites/projectTestSprite/index.json": testSpriteConfig,
				"sprites/Dynamic/index.json":           testSpriteConfig,
				"sprites/Value/index.json":             testSpriteConfig,
				"sprites/Last/index.json":              test.last,
				"broken.json":                          `{`,
			}}

			_, err := game.prepareProject(fs, &test.project, reflect.ValueOf(game).Elem())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("prepareProject error = %v, want %q", err, test.want)
			}
			if game.projectTestSprite != original || game.Dynamic != dynamic || game.Last != last || !reflect.DeepEqual(&game.Value, beforeValue) {
				t.Fatal("preparation changed pointer, interface, or value sprite fields")
			}
			if original.marker != "pointer" || dynamic.marker != "interface" || last.marker != "last" || len(game.Items) != 1 || game.Items[0] != original {
				t.Fatal("preparation changed existing sprites or a stage group")
			}
			if len(game.sprs) != 1 || game.sprs["Existing"] != original || len(game.shapeMgr.items) != 1 || game.shapeMgr.items[0] != &original.SpriteImpl {
				t.Fatal("preparation published sprite prototypes or stage shapes")
			}
			wantReads := []string{
				"sprites/projectTestSprite/index.json", "sprites/Dynamic/index.json",
				"sprites/Value/index.json", "sprites/Last/index.json",
			}
			if test.project.TilemapPath != "" {
				wantReads = append(wantReads, test.project.TilemapPath)
			}
			if !reflect.DeepEqual(fs.reads, wantReads) {
				t.Fatalf("resource reads = %v, want %v", fs.reads, wantReads)
			}
		})
	}
}

func TestPrepareProjectReusesSpriteConfigurationAcrossStageEntries(t *testing.T) {
	game := &projectTestGame{}
	game.typs = map[string]reflect.Type{"projectTestSprite": reflect.TypeFor[projectTestSprite]()}
	fs := &recordingProjectFS{files: map[string]string{
		"sprites/projectTestSprite/index.json": testSpriteConfig,
	}}
	group := coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{coreproject.StageShape{"x": 1.0}}}
	project := &coreproject.ProjectConfig{Zorder: []any{"projectTestSprite", group, group, "projectTestSprite"}}

	plan, err := game.prepareProject(fs, project, reflect.ValueOf(game).Elem())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.sprites) != 1 || len(plan.stage) != 4 {
		t.Fatalf("prepared %d prototypes and %d stage entries", len(plan.sprites), len(plan.stage))
	}
	if want := []string{"sprites/projectTestSprite/index.json"}; !reflect.DeepEqual(fs.reads, want) {
		t.Fatalf("resource reads = %v, want %v", fs.reads, want)
	}
	if game.projectTestSprite != nil || game.Items != nil || len(game.sprs) != 0 || len(game.shapeMgr.items) != 0 {
		t.Fatal("successful preparation published fields or stage state")
	}
}

func TestPrepareProjectReadsTilemapOnceAndKeepsDecoratorOptional(t *testing.T) {
	for _, test := range []struct {
		name      string
		path      string
		files     map[string]string
		reads     []string
		decorator bool
		missing   bool
	}{
		{"legacy", "map.json", map[string]string{"map.json": `{}`}, []string{"map.json"}, false, false},
		{"new", "map", map[string]string{"map/tilemap.json": `{}`, "map/decorator.json": `{}`}, []string{"map/tilemap.json", "map/decorator.json"}, false, false},
		{"missing decorator", "map", map[string]string{"map/tilemap.json": `{}`}, []string{"map/tilemap.json", "map/decorator.json"}, true, true},
		{"invalid decorator", "map", map[string]string{"map/tilemap.json": `{}`, "map/decorator.json": `{`}, []string{"map/tilemap.json", "map/decorator.json"}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &projectTestGame{}
			game.sprs = map[string]Sprite{"Existing": &projectTestSprite{}}
			fs := &recordingProjectFS{files: test.files}
			project := &coreproject.ProjectConfig{TilemapPath: test.path, Zorder: []any{"Existing"}}
			plan, err := game.prepareProject(fs, project, reflect.ValueOf(game).Elem())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(fs.reads, test.reads) {
				t.Fatalf("resource reads = %v, want %v", fs.reads, test.reads)
			}
			if (plan.tilemap.DecoratorErr != nil) != test.decorator || test.missing && !errors.Is(plan.tilemap.DecoratorErr, os.ErrNotExist) {
				t.Fatalf("decorator error = %v", plan.tilemap.DecoratorErr)
			}
			if len(plan.stage) != 1 || plan.tilemap.UseNewLoader != (test.path == "map") {
				t.Fatal("tilemap preparation did not retain its format or proceed to stage preparation")
			}
			if test.path == "map.json" && plan.tilemap.Data == nil {
				t.Fatal("legacy tilemap data was not retained")
			}
		})
	}
}

func TestPrepareProjectPreservesStageFailureOrder(t *testing.T) {
	group := func(items ...any) coreproject.StageShape {
		return coreproject.StageShape{"type": "sprites", "target": "Items", "items": items}
	}
	for _, test := range []struct {
		name    string
		entries []any
		want    string
		reads   []string
	}{
		{"lazy prototype before later shape", []any{"Missing", coreproject.StageShape{}}, `zorder[0]: load sprite "Missing"`, []string{"sprites/Missing/index.json"}},
		{"group prototype before later shape", []any{group(coreproject.StageShape{}), coreproject.StageShape{}}, `zorder[0]: load sprite "projectTestSprite"`, []string{"sprites/projectTestSprite/index.json"}},
		{"group items before prototype", []any{group(coreproject.StageShape{}, 42)}, `zorder[0]: stage sprites target "Items" item[1]`, nil},
		{"shape before later prototype", []any{coreproject.StageShape{}, "Missing"}, `zorder[0]: invalid stage shape type`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &projectTestGame{}
			game.typs = map[string]reflect.Type{"Missing": reflect.TypeFor[projectTestSprite]()}
			fs := &recordingProjectFS{}
			_, err := game.prepareProject(fs, &coreproject.ProjectConfig{Zorder: test.entries}, reflect.ValueOf(game).Elem())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("prepareProject error = %v, want %q", err, test.want)
			}
			if !reflect.DeepEqual(fs.reads, test.reads) {
				t.Fatalf("resource reads = %v, want %v", fs.reads, test.reads)
			}
		})
	}
}

func TestPrepareProjectEmptyGroupDoesNotReadPrototype(t *testing.T) {
	original := &projectTestSprite{}
	game := &projectTestGame{Items: []*projectTestSprite{original}}
	fs := &recordingProjectFS{}
	project := &coreproject.ProjectConfig{Zorder: []any{
		coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{}},
	}}
	plan, err := game.prepareProject(fs, project, reflect.ValueOf(game).Elem())
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.reads) != 0 || len(plan.sprites) != 0 {
		t.Fatalf("empty group read resources %v or prepared a prototype", fs.reads)
	}
	if len(plan.stage) != 1 {
		t.Fatalf("prepared %d stage entries, want 1", len(plan.stage))
	}
	entry := plan.stage[0].sprite
	field := reflect.TypeFor[projectTestGame]().Field(entry.field)
	if field.Name != "Items" || entry.properties == nil || len(entry.properties) != 0 {
		t.Fatal("preparation lost the empty group's target or empty items")
	}
	if len(game.Items) != 1 || game.Items[0] != original {
		t.Fatal("preparation replaced the live group")
	}
}

func TestPrepareProjectRejectsNilCachedPrototype(t *testing.T) {
	for _, entry := range []any{
		"projectTestSprite",
		coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{coreproject.StageShape{}}},
	} {
		game := &projectTestGame{}
		game.sprs = map[string]Sprite{"projectTestSprite": (*projectTestSprite)(nil)}
		_, err := game.prepareProject(&recordingProjectFS{}, &coreproject.ProjectConfig{Zorder: []any{entry}}, reflect.ValueOf(game).Elem())
		if err == nil || !strings.Contains(err.Error(), "projectTestSprite") {
			t.Errorf("typed-nil prototype with entry %v: error = %v, want preflight rejection", entry, err)
		}
	}
}

func TestPrepareProjectRejectsPrototypeTypeCollision(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "prepared field", true: "cached prototype"}[cached], func(t *testing.T) {
			game := &prototypeConflictGame{}
			if cached {
				game.sprs = map[string]Sprite{"projectTestSprite": &conflictingSprite{}}
			} else {
				game.typs = map[string]reflect.Type{"projectTestSprite": reflect.TypeFor[conflictingSprite]()}
			}
			fs := &recordingProjectFS{files: map[string]string{
				"sprites/projectTestSprite/index.json": testSpriteConfig,
			}}
			project := &coreproject.ProjectConfig{Zorder: []any{
				coreproject.StageShape{"type": "sprites", "target": "Items", "items": []any{coreproject.StageShape{}}},
			}}
			_, err := game.prepareProject(fs, project, reflect.ValueOf(game).Elem())
			if err == nil || !strings.Contains(err.Error(), "projectTestSprite") {
				t.Fatalf("prototype type collision error = %v, want preflight rejection", err)
			}
		})
	}
}

func TestPrepareSpriteRejectsInvalidLayout(t *testing.T) {
	gamer := reflect.ValueOf(&Game{}).Elem()
	files := gameTestFS{"sprites/invalid/index.json": testSpriteConfig}
	for _, test := range []struct {
		name string
		typ  reflect.Type
		want string
	}{
		{"owner", reflect.TypeFor[layoutInvalidOwnerSprite](), "cannot hold"},
		{"implementation", reflect.TypeFor[spriteWithoutImplementation](), "missing SpriteImpl"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := prepareSprite(files, "invalid", test.typ, gamer)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if !reflect.ValueOf(plan).IsZero() {
				t.Fatal("layout rejection returned a partial sprite plan")
			}
		})
	}
}

func TestApplyRuntimeFontPlan(t *testing.T) {
	bridge := &recordingProjectFontBridge{}
	plan := coreproject.RuntimeFontPlan{
		DefaultPath: "res://engine/fonts/default.ttf",
		Faces: []coreproject.RuntimeFontFace{
			{Path: "asset://scratch.ttf", Family: "Scratch"},
			{Path: "asset://chinese.ttf", Family: "Chinese"},
		},
		Preferences: []string{"Scratch", "Chinese", "default"},
	}
	if err := applyRuntimeFontPlan(bridge, plan); err != nil {
		t.Fatal(err)
	}
	if bridge.defaultPath != plan.DefaultPath ||
		!reflect.DeepEqual(bridge.paths, []string{"asset://scratch.ttf", "asset://chinese.ttf"}) ||
		!reflect.DeepEqual(bridge.families, []string{"Scratch", "Chinese"}) ||
		!reflect.DeepEqual(bridge.preferences, plan.Preferences) {
		t.Fatalf("unexpected bridge payload: %+v", bridge)
	}
	bridge.preferences[0] = "changed by bridge"
	if plan.Preferences[0] != "Scratch" {
		t.Fatal("bridge preferences alias the input plan")
	}

	bridge.message = "engine rejected project fonts"
	if err := applyRuntimeFontPlan(bridge, plan); err == nil || err.Error() != bridge.message {
		t.Fatalf("applyRuntimeFontPlan error = %v, want %q", err, bridge.message)
	}
	if err := applyRuntimeFontPlan(nil, plan); err == nil {
		t.Fatal("applyRuntimeFontPlan(nil) succeeded")
	}

	for _, test := range []struct {
		name        string
		preferences []string
	}{
		{"nil preferences", nil},
		{"empty preferences", []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bridge := &recordingProjectFontBridge{}
			if err := applyRuntimeFontPlan(bridge, coreproject.RuntimeFontPlan{Preferences: test.preferences}); err != nil {
				t.Fatal(err)
			}
			if bridge.paths == nil || len(bridge.paths) != 0 || bridge.families == nil || len(bridge.families) != 0 {
				t.Fatalf("empty faces produced paths %v and families %v, want non-nil empty slices", bridge.paths, bridge.families)
			}
			if !reflect.DeepEqual(bridge.preferences, test.preferences) {
				t.Fatalf("preferences = %#v, want %#v", bridge.preferences, test.preferences)
			}
		})
	}
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

func (*layoutInterfaceOwnerSprite) Main() {}

func (m *gameDisplayPlatformMgr) SetWindowSize(width, height int64, _ bool) {
	m.width, m.height = width, height
}

func (m *gameDisplayPlatformMgr) SetWindowFullscreen(value bool) { m.fullscreen = value }

func (f gameTestFS) Open(name string) (io.ReadCloser, error) {
	content, ok := f[name]
	if !ok {
		return nil, errors.New("file not found: " + name)
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (gameTestFS) Close() error { return nil }

func (*spriteLoadTestSprite) Main() {}

func (*stageLoadTestSprite) Main() {}

func (*DirectLoadSprite) Main() {}

func (m *gameTestSpriteMgr) CreateBackdrop(string) pkgengine.Object {
	return m.CreateBareSprite(mathf.Vec2{})
}

func (*gameTestSpriteMgr) DestroySprite(pkgengine.Object) bool { return true }

func (*gameTestSpriteMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (*gameTestSpriteMgr) SetColliderRect(pkgengine.Object, mathf.Vec2, mathf.Vec2) {
}

func (*gameTestSpriteMgr) SetTriggerRect(pkgengine.Object, mathf.Vec2, mathf.Vec2) {}

func (*gameTestSpriteMgr) SetPixelCollisionSamplingStep(int64) {}

func (*gameTestExtMgr) SetLayerSorterMode(int64) {}

func (*gameTestPlatformMgr) IsMainThread() bool { return true }

func (*gameTestPlatformMgr) GetWindowSize() mathf.Vec2 { return mathf.NewVec2(480, 360) }

func (*gameTestPlatformMgr) SetWindowSize(int64, int64, bool) {}

func (*gameTestPlatformMgr) SetWindowFullscreen(bool) {}

func (*gameTestPlatformMgr) SetMaxFps(int64) {}

func (*gameTestPlatformMgr) SetStretch(bool, int64, int64) {}

func (*gameTestPlatformMgr) SetDebugMode(bool) {}

func (*gameTestCameraMgr) SetCameraZoom(mathf.Vec2) {}

func (*gameTestCameraMgr) SetCameraLimit(int64, int64) {}

func (*gameTestCameraMgr) SetCameraSmoothing(bool) {}

func (*gameTestInputMgr) GetGlobalMousePos() mathf.Vec2 { return mathf.Vec2{} }

func (*gameTestInputMgr) GetMouseState(int64) bool { return false }

func (*gameTestPenMgr) DestroyAllPens() {}

func (*gameTestPenMgr) SetCanvasSize(int64, int64) {}

func (m *gameTestPhysicsMgr) SetCollisionSystemType(byPixel bool) {
	m.collisionModes = append(m.collisionModes, byPixel)
}

func (*gameTestPhysicsMgr) SetGlobalGravity(float64) {}

func (*gameTestPhysicsMgr) SetGlobalFriction(float64) {}

func (*gameTestPhysicsMgr) SetGlobalAirDrag(float64) {}

func (*gameTestResMgr) GetBoundFromAlpha(string) mathf.Rect2 {
	return mathf.Rect2{Size: mathf.NewVec2(10, 10)}
}

func (m *gameTestTilemapparserMgr) LoadTilemap(path string) {
	m.loadedPath = path
}

func (m *gameTestTilemapparserMgr) DestroyAllTilemaps() { m.destroyCalls++ }

func (*projectTestSprite) Main() {}

func (fs *recordingProjectFS) Open(name string) (io.ReadCloser, error) {
	fs.reads = append(fs.reads, name)
	content, ok := fs.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (*recordingProjectFS) Close() error { return nil }

func (*conflictingSprite) Main() {}

func (*layoutInvalidOwnerSprite) Main() {}

func (b *recordingProjectFontBridge) ApplyProjectFonts(defaultPath string, paths, families, preferences engine.Array) string {
	b.defaultPath = defaultPath
	b.paths = paths.([]string)
	b.families = families.([]string)
	b.preferences = preferences.([]string)
	return b.message
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

func setupGameTestRuntime(t *testing.T, files gameTestFS, game Gamer, sprites []Sprite, physics bool) *Game {
	t.Helper()

	cloneMgr := setupCloneSpriteMgr(t)
	originalExtMgr := pkgengine.ExtMgr
	originalPlatformMgr := pkgengine.PlatformMgr
	originalCameraMgr := pkgengine.CameraMgr
	originalInputMgr := pkgengine.InputMgr
	originalPenMgr := pkgengine.PenMgr
	originalPhysicsMgr := pkgengine.PhysicsMgr
	originalResMgr := pkgengine.ResMgr
	pkgengine.SpriteMgr = &gameTestSpriteMgr{spyCloneSpriteMgr: cloneMgr}
	pkgengine.ExtMgr = &gameTestExtMgr{}
	pkgengine.PlatformMgr = &gameTestPlatformMgr{}
	pkgengine.CameraMgr = &gameTestCameraMgr{}
	pkgengine.InputMgr = &gameTestInputMgr{}
	pkgengine.PenMgr = &gameTestPenMgr{}
	pkgengine.PhysicsMgr = &gameTestPhysicsMgr{}
	pkgengine.ResMgr = &gameTestResMgr{}
	t.Cleanup(func() {
		pkgengine.SpriteMgr = cloneMgr
		pkgengine.ExtMgr = originalExtMgr
		pkgengine.PlatformMgr = originalPlatformMgr
		pkgengine.CameraMgr = originalCameraMgr
		pkgengine.InputMgr = originalInputMgr
		pkgengine.PenMgr = originalPenMgr
		pkgengine.PhysicsMgr = originalPhysicsMgr
		pkgengine.ResMgr = originalResMgr
	})

	originalGame := engine.GetGame()
	originalBounds := cachedBounds
	t.Cleanup(func() {
		engine.SetGame(originalGame)
		cachedBounds = originalBounds
	})
	installRuntimeScheduler(t, coroutine.New(nil), engine.ClearAllSprites)
	cachedBounds = make(map[string]mathf.Rect2)

	base := game.baseGame().initGame(sprites)
	engine.SetGame(base)
	base.gamer = game
	base.startLoad(files)
	base.soundMgr.Init(&fakeAudioBackend{})
	base.tilemapMgr.g, base.tilemapMgr.fs = base, files
	base.physicsEnabled = physics
	base.lifecycleState.IsRunned.Store(true)

	return base
}

func setupStageLoadTestGame(t *testing.T, files gameTestFS) *stageLoadTestGame {
	t.Helper()
	game := &stageLoadTestGame{}
	setupGameTestRuntime(t, files, game, []Sprite{&stageLoadTestSprite{}}, false)
	return game
}
