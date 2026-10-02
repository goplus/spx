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
	"reflect"
	"strings"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
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

func TestMalformedDisplayShapeReturnsLoadError(t *testing.T) {
	for _, test := range []struct {
		name  string
		shape coreproject.StageShape
		want  string
	}{
		{"measure", coreproject.StageShape{"type": "measure", "size": 10.0, "x": 0.0, "y": 0.0, "color": []any{1.0}}, `stage shape field "color"`},
		{"monitor", coreproject.StageShape{"type": "monitor", "target": "", "val": "score", "name": "score", "label": "Score", "mode": nil, "x": 0.0, "y": 0.0, "visible": true}, `stage shape field "mode"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := prepareStageEntries(reflect.Value{}, []any{test.shape})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestMissingMonitorBindingStillSkipsShape(t *testing.T) {
	game := &Game{}
	shape := coreproject.StageShape{
		"type": "monitor", "target": "", "val": "missing", "name": "missing", "label": "Missing",
		"mode": 1.0, "x": 0.0, "y": 0.0, "visible": true,
	}
	gamer := reflect.ValueOf(&monitorEvalFixture{}).Elem()
	entries, err := prepareStageEntries(gamer, []any{shape})
	if err == nil {
		game.loadAndInitSprites(gamer, nil, nil, entries)
	}
	if err != nil {
		t.Fatalf("missing monitor binding changed load behavior: %v", err)
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
