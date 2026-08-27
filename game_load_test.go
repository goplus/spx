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
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
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

type bootstrapAwakeOrderSprite struct {
	SpriteImpl
	peer         *bootstrapAwakeOrderSprite
	sawSelfAwake bool
	sawPeerAwake bool
}

type cloneAwakeOrderSprite struct {
	SpriteImpl
	sawAwakeInMain *bool
	onClonedFired  *bool
	clonedDone     chan struct{}
}

type cloneStatePreservingSprite struct {
	SpriteImpl
	cloneValue  float64
	recordValue *float64
	clonedDone  chan struct{}
}

type cloneFieldPayload struct {
	label string
	count int
}

type cloneAllFieldKindsObserved struct {
	boolValue      bool
	intValue       int
	stringValue    string
	floatValue     float64
	valueString    string
	listString     string
	listLen        int
	sliceValue     []int
	mapValue       map[string]int
	structValue    cloneFieldPayload
	arrayValue     [2]string
	pointerValue   *cloneFieldPayload
	interfaceValue any
}

type cloneAllFieldKindsSprite struct {
	SpriteImpl
	boolValue      bool
	intValue       int
	stringValue    string
	floatValue     float64
	valueValue     Value
	listValue      List
	sliceValue     []int
	mapValue       map[string]int
	structValue    cloneFieldPayload
	arrayValue     [2]string
	pointerValue   *cloneFieldPayload
	interfaceValue any
	observed       *cloneAllFieldKindsObserved
	clonedDone     chan struct{}
}

type spyCloneSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	nextID   pkgengine.Object
	zIndexes map[pkgengine.Object]int64
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

func (s *bootstrapAwakeOrderSprite) Main() {
	s.sawSelfAwake = s.spriteState.IsAwakened
	if s.peer != nil {
		s.sawPeerAwake = s.peer.spriteState.IsAwakened
	}
}

func (s *cloneAwakeOrderSprite) Main() {
	if !s.IsCloned() {
		return
	}
	if s.sawAwakeInMain != nil {
		*s.sawAwakeInMain = s.spriteState.IsAwakened
	}
	s.OnCloned__0(func() {
		if s.onClonedFired != nil {
			*s.onClonedFired = true
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneStatePreservingSprite) Main() {
	s.cloneValue = 1
	if !s.IsCloned() {
		return
	}
	s.OnCloned__0(func() {
		if s.recordValue != nil {
			*s.recordValue = s.cloneValue
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneAllFieldKindsSprite) Main() {
	s.XGo_Init()
	if !s.IsCloned() {
		return
	}
	s.OnCloned__0(func() {
		if s.observed != nil {
			s.observed.boolValue = s.boolValue
			s.observed.intValue = s.intValue
			s.observed.stringValue = s.stringValue
			s.observed.floatValue = s.floatValue
			s.observed.valueString = s.valueValue.String()
			s.observed.listString = s.listValue.String()
			s.observed.listLen = s.listValue.Len()
			s.observed.sliceValue = append([]int(nil), s.sliceValue...)
			if s.mapValue != nil {
				s.observed.mapValue = make(map[string]int, len(s.mapValue))
				for key, val := range s.mapValue {
					s.observed.mapValue[key] = val
				}
			}
			s.observed.structValue = s.structValue
			s.observed.arrayValue = s.arrayValue
			s.observed.pointerValue = s.pointerValue
			s.observed.interfaceValue = s.interfaceValue
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneAllFieldKindsSprite) XGo_Init() *cloneAllFieldKindsSprite {
	s.boolValue = false
	s.intValue = 1
	s.stringValue = "init"
	s.floatValue = 2.5
	s.valueValue = NewValue("init-value")
	s.listValue.Init("init")
	s.sliceValue = []int{9}
	s.mapValue = map[string]int{"init": 1}
	s.structValue = cloneFieldPayload{label: "init-struct", count: 1}
	s.arrayValue = [2]string{"init-left", "init-right"}
	s.pointerValue = &cloneFieldPayload{label: "init-pointer", count: 2}
	s.interfaceValue = "init-any"
	return s
}

func (s *spyCloneSpriteMgr) CreateBareSprite(pos mathf.Vec2) pkgengine.Object {
	s.nextID++
	return s.nextID
}

func (s *spyCloneSpriteMgr) SetTypeName(obj pkgengine.Object, typeName string) {}

func (s *spyCloneSpriteMgr) SetVisible(obj pkgengine.Object, visible bool) {}

func (s *spyCloneSpriteMgr) SetZIndex(obj pkgengine.Object, z int64) {
	s.zIndexes[obj] = z
}

func (s *spyCloneSpriteMgr) SetTriggerLayer(obj pkgengine.Object, layer int64) {}

func (s *spyCloneSpriteMgr) SetTriggerMask(obj pkgengine.Object, mask int64) {}

func (s *spyCloneSpriteMgr) SetCollisionLayer(obj pkgengine.Object, layer int64) {}

func (s *spyCloneSpriteMgr) SetCollisionMask(obj pkgengine.Object, mask int64) {}

func (s *spyCloneSpriteMgr) SetTriggerEnabled(obj pkgengine.Object, trigger bool) {}

func (s *spyCloneSpriteMgr) SetCollisionEnabled(obj pkgengine.Object, enabled bool) {}

func (s *spyCloneSpriteMgr) SetGravityScale(obj pkgengine.Object, scale float64) {}

func (s *spyCloneSpriteMgr) SetPhysicsMode(obj pkgengine.Object, mode int64) {}

func newCameraFollowOverrideSprite(g *Game, name string, followTarget SpriteName) *cameraFollowOverrideSprite {
	sprite := &cameraFollowOverrideSprite{followTarget: followTarget}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func newCollisionLayerOrderSprite(g *Game, name string, onMain func()) *collisionLayerOrderSprite {
	sprite := &collisionLayerOrderSprite{onMain: onMain}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func newBootstrapAwakeOrderSprite(g *Game, name string) *bootstrapAwakeOrderSprite {
	sprite := &bootstrapAwakeOrderSprite{}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	return sprite
}

func newCloneAwakeOrderSprite(g *Game, name string) *cloneAwakeOrderSprite {
	sawAwake := false
	onCloned := false
	sprite := &cloneAwakeOrderSprite{
		sawAwakeInMain: &sawAwake,
		onClonedFired:  &onCloned,
		clonedDone:     make(chan struct{}),
	}
	sprite.baseObj.initWithSize(1, 1)
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func newCloneStatePreservingSprite(g *Game, name string, recordValue *float64) *cloneStatePreservingSprite {
	sprite := &cloneStatePreservingSprite{
		recordValue: recordValue,
		clonedDone:  make(chan struct{}),
	}
	sprite.baseObj.initWithSize(1, 1)
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func newCloneAllFieldKindsSprite(g *Game, name string, observed *cloneAllFieldKindsObserved) *cloneAllFieldKindsSprite {
	sprite := &cloneAllFieldKindsSprite{
		observed:   observed,
		clonedDone: make(chan struct{}),
	}
	sprite.baseObj.initWithSize(1, 1)
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func setupCloneSpriteMgr(t *testing.T) *spyCloneSpriteMgr {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.SpriteMgr
	mgr := &spyCloneSpriteMgr{zIndexes: make(map[pkgengine.Object]int64)}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
	return mgr
}

func setupBootstrapScheduler(t *testing.T) {
	t.Helper()

	co := coroutine.New(nil)
	co.OnInited()

	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		co.AbortAllAndWait(time.Second)
		gco = original
		engine.SetCoroutines(original)
	})
}

func runBootstrapTasksWithScheduler(t *testing.T, game *Game, generation uint64) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		game.runBootstrapTasksFor(generation)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		select {
		case <-done:
			return
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("bootstrap tasks did not finish while pumping scheduler")
		}

		gco.Update()
		time.Sleep(time.Millisecond)
	}
}

func TestRunSpriteCallbacksKeepsManualCameraFollowLast(t *testing.T) {
	game := &cameraFollowOverrideGame{}
	game.initShapeMgr()
	game.camera = &cameraImpl{g: &game.Game}
	game.Camera = game.camera

	spriteA := newCameraFollowOverrideSprite(&game.Game, "SpriteA", "")
	spriteB := newCameraFollowOverrideSprite(&game.Game, "SpriteB", "SpriteB")
	game.addShape(spriteOf(spriteA))
	game.addShape(spriteOf(spriteB))

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{Camera: &coreproject.CameraConfig{On: "SpriteA"}},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	game.runBootstrapTasksFor(generation)

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

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(game).Elem(),
		generation,
	)
	game.runBootstrapTasksFor(generation)
	game.refreshCollisionLayers()

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != game.sprCollisionInfos["SpriteA"].Layer {
		t.Fatalf("SpriteB collision mask = %d, want %d", got, game.sprCollisionInfos["SpriteA"].Layer)
	}
}

func TestRunSpriteCallbacksRefreshesCollisionLayersRegisteredInMain(t *testing.T) {
	setupBootstrapScheduler(t)

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
	game.addShape(spriteOf(spriteA))
	game.addShape(spriteOf(spriteB))

	generation := game.currentBootstrapGeneration()
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

func TestRunSpriteCallbacksAwakesAllSpritesBeforeMain(t *testing.T) {
	var game Game

	spriteA := newBootstrapAwakeOrderSprite(&game, "SpriteA")
	spriteB := newBootstrapAwakeOrderSprite(&game, "SpriteB")
	spriteA.peer = spriteB
	spriteB.peer = spriteA

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)
	game.runBootstrapTasksFor(generation)

	if !spriteA.sawSelfAwake || !spriteA.sawPeerAwake {
		t.Fatalf("SpriteA main saw awake state self=%v peer=%v, want both true", spriteA.sawSelfAwake, spriteA.sawPeerAwake)
	}
	if !spriteB.sawSelfAwake || !spriteB.sawPeerAwake {
		t.Fatalf("SpriteB main saw awake state self=%v peer=%v, want both true", spriteB.sawSelfAwake, spriteB.sawPeerAwake)
	}
	if got := game.scriptEvents.manager.SnapshotAwake(); len(got) != 0 {
		t.Fatalf("SnapshotAwake len = %d, want 0 for initial sprites", len(got))
	}
}

func TestRunSpriteCallbacksRunsSpriteMainsInZOrderUntilFirstYield(t *testing.T) {
	setupBootstrapScheduler(t)

	var game Game

	blocked := make(chan struct{})
	defer close(blocked)

	var spriteASeenThreadCount int64
	var spriteBSeenThreadCount int64
	spriteA := newCollisionLayerOrderSprite(&game, "SpriteA", func() {
		spriteASeenThreadCount = gco.LastThreadID()
		var signal struct{}
		engine.WaitForChan(blocked, &signal)
	})
	spriteB := newCollisionLayerOrderSprite(&game, "SpriteB", func() {
		spriteBSeenThreadCount = gco.LastThreadID()
	})

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)

	runBootstrapTasksWithScheduler(t, &game, generation)

	if spriteASeenThreadCount != 1 {
		t.Fatalf("SpriteA saw %d created threads before its first yield, want 1", spriteASeenThreadCount)
	}
	if spriteBSeenThreadCount != 2 {
		t.Fatalf("SpriteB saw %d created threads before its first yield, want 2", spriteBSeenThreadCount)
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
	})
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
	})

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

func TestRunBootstrapMainUntilYieldReleasesFollowingBootstrapTasks(t *testing.T) {
	setupBootstrapScheduler(t)

	var game Game
	blocked := make(chan struct{})
	defer close(blocked)

	stageStarted := make(chan struct{})
	stageResumed := make(chan struct{})
	followingTaskRan := make(chan struct{})
	generation := game.currentBootstrapGeneration()
	game.deferBootstrapFor(generation, func() {
		game.runBootstrapMainUntilYield(&game, func() {
			close(stageStarted)
			var signal struct{}
			engine.WaitForChan(blocked, &signal)
			close(stageResumed)
		})
	})
	game.deferBootstrapFor(generation, func() {
		close(followingTaskRan)
	})

	runBootstrapTasksWithScheduler(t, &game, generation)

	select {
	case <-stageStarted:
	default:
		t.Fatal("stage Main did not start")
	}
	select {
	case <-followingTaskRan:
	default:
		t.Fatal("following bootstrap task did not run after stage Main yielded")
	}
	select {
	case <-stageResumed:
		t.Fatal("stage Main resumed before its wait was released")
	default:
	}
}

func TestRunSpriteCallbacksAllowsOnStartAfterMainFirstYield(t *testing.T) {
	setupBootstrapScheduler(t)

	var game Game
	game.initRuntimeState()
	game.events = make(chan event, eventBufferSize)

	blocked := make(chan struct{})
	defer close(blocked)

	started := make(chan struct{})
	var spriteA *collisionLayerOrderSprite
	spriteA = newCollisionLayerOrderSprite(&game, "SpriteA", func() {
		spriteA.OnStart(func() {
			close(started)
		})
		var signal struct{}
		engine.WaitForChan(blocked, &signal)
	})
	spriteB := newCollisionLayerOrderSprite(&game, "SpriteB", nil)

	generation := game.currentBootstrapGeneration()
	game.runSpriteCallbacks(
		[]Sprite{spriteA, spriteB},
		&coreproject.ProjectConfig{},
		reflect.ValueOf(&game).Elem(),
		generation,
	)

	runBootstrapTasksWithScheduler(t, &game, generation)

	game.markBootstrapDoneFor(generation)
	game.dispatchStartEventIfNeeded()
	gco.Update()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("OnStart did not run after Main yielded once")
	}
}

func TestInitRuntimeProxyAppliesCostumeBeforeAwake(t *testing.T) {
	var game Game
	setupCloneSpriteMgr(t)

	sprite := newCloneAwakeOrderSprite(&game, "SpriteA")
	sprite.spriteState.IsVisible = true

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
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	source.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	source.costumeIndex = 0

	out := reflect.New(reflect.TypeOf(source).Elem()).Elem()
	shape := coreproject.StageShape{
		"visible":      false,
		"size":         2.0,
		"costumeIndex": 1.0,
	}

	var dest *SpriteImpl
	dest, _ = applySprite(out, source, shape)

	if dest == nil {
		t.Fatal("applySprite returned nil sprite")
	}
	if dest.runtimeState.SyncSprite == nil {
		t.Fatal("SyncSprite = nil, want initialized proxy")
	}
	if dest.runtimeState.IsCostumeDirty {
		t.Fatal("IsCostumeDirty = true, want false after applySprite")
	}
	if dest.costumeIndex != 1 {
		t.Fatalf("costumeIndex = %d, want 1", dest.costumeIndex)
	}
	if dest.spriteState.IsVisible {
		t.Fatal("IsVisible = true, want false from stage shape override")
	}
	if dest.runtimeState.Scale != 2 {
		t.Fatalf("Scale = %v, want 2", dest.runtimeState.Scale)
	}
}

func TestCloneSpriteAwakesBeforeMain(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	game.addShape(spriteOf(source))

	var cloned *cloneAwakeOrderSprite
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if cloned.sawAwakeInMain == nil || !*cloned.sawAwakeInMain {
		t.Fatal("clone Main ran before awake")
	}
	if cloned.onClonedFired == nil || !*cloned.onClonedFired {
		t.Fatal("clone onCloned did not fire after Main registration")
	}
}

func TestCloneSpriteInsertsImmediatelyBehindSource(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	back := newCloneAwakeOrderSprite(&game, "Back")
	source := newCloneAwakeOrderSprite(&game, "Source")
	front := newCloneAwakeOrderSprite(&game, "Front")
	game.addShape(spriteOf(back))
	game.addShape(spriteOf(source))
	game.addShape(spriteOf(front))

	var cloned *cloneAwakeOrderSprite
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	shapes := game.getAllShapes()
	if got := len(shapes); got != 4 {
		t.Fatalf("shape count = %d, want 4", got)
	}
	if shapes[0] != spriteOf(back) {
		t.Fatalf("shape[0] = %v, want back sprite", shapes[0])
	}
	if shapes[1] != spriteOf(cloned) {
		t.Fatalf("shape[1] = %v, want cloned sprite", shapes[1])
	}
	if shapes[2] != spriteOf(source) {
		t.Fatalf("shape[2] = %v, want source sprite", shapes[2])
	}
	if shapes[3] != spriteOf(front) {
		t.Fatalf("shape[3] = %v, want front sprite", shapes[3])
	}

	if got, want := source.runtimeState.Layer, 3; got != want {
		t.Fatalf("source layer = %d, want %d", got, want)
	}
	if got, want := cloned.runtimeState.Layer, 2; got != want {
		t.Fatalf("clone layer = %d, want %d", got, want)
	}
	if got, want := front.runtimeState.Layer, 4; got != want {
		t.Fatalf("front layer = %d, want %d", got, want)
	}
}

func TestCloneSpriteSynchronizesCopiedLayerToFreshProxy(t *testing.T) {
	var game Game
	game.initShapeMgr()
	mgr := setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	source.runtimeState.Layer = firstSpriteLayer + 6
	source.runtimeState.IsLayerDirty = false
	game.addShape(spriteOf(source))

	var cloned *SpriteImpl
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		cloned = sprite
	})
	if cloned == nil || cloned.runtimeState.SyncSprite == nil {
		t.Fatal("clone proxy was not initialized")
	}

	cloneID := cloned.runtimeState.SyncSprite.GetId()
	if got, want := mgr.zIndexes[cloneID], int64(firstSpriteLayer+6); got != want {
		t.Fatalf("fresh clone proxy z-index = %d, want copied layer %d", got, want)
	}
}

func TestRepeatedClonesStackNewestBehindSourceAndInFrontOfOlderClone(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	game.addShape(spriteOf(source))

	clones := make([]*SpriteImpl, 0, 2)
	for range 2 {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			clones = append(clones, sprite)
		})
	}

	shapes := game.getAllShapes()
	if got, want := len(shapes), 3; got != want {
		t.Fatalf("shape count = %d, want %d", got, want)
	}
	if shapes[0] != clones[0] {
		t.Fatalf("shape[0] = %v, want oldest clone", shapes[0])
	}
	if shapes[1] != clones[1] {
		t.Fatalf("shape[1] = %v, want newest clone", shapes[1])
	}
	if shapes[2] != spriteOf(source) {
		t.Fatalf("shape[2] = %v, want source sprite", shapes[2])
	}
	if got, want := source.runtimeState.Layer, firstSpriteLayer+2; got != want {
		t.Fatalf("source layer = %d, want %d", got, want)
	}
	if got, want := clones[0].runtimeState.Layer, firstSpriteLayer; got != want {
		t.Fatalf("oldest clone layer = %d, want %d", got, want)
	}
	if got, want := clones[1].runtimeState.Layer, firstSpriteLayer+1; got != want {
		t.Fatalf("newest clone layer = %d, want %d", got, want)
	}
}

func TestCloneOfCloneInsertsImmediatelyBehindItsParent(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	game.addShape(spriteOf(source))

	var first, second *SpriteImpl
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		first = sprite
	})
	doClone(first.sprite, nil, false, func(sprite *SpriteImpl) {
		second = sprite
	})

	shapes := game.getAllShapes()
	if len(shapes) != 3 || shapes[0] != second || shapes[1] != first || shapes[2] != spriteOf(source) {
		t.Fatalf("clone-of-clone order = %v, want second clone, first clone, source", shapes)
	}
}

func TestCloneInsertionUsesSourceCurrentLayer(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	other := newCloneAwakeOrderSprite(&game, "Other")
	game.addShape(spriteOf(source))

	clones := make([]*SpriteImpl, 0, 3)
	for range 2 {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			clones = append(clones, sprite)
		})
	}
	game.addShape(spriteOf(other))
	game.shapeMgr.goBackLayers(spriteOf(other), 1)
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		clones = append(clones, sprite)
	})

	shapes := game.getAllShapes()
	want := []Shape{clones[0], clones[1], spriteOf(other), clones[2], spriteOf(source)}
	if !reflect.DeepEqual(shapes, want) {
		t.Fatalf("clone order after source layer move = %v, want %v", shapes, want)
	}
}

func TestCloneSpritePreservesUserStateAfterMainRegistration(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	var recorded float64
	source := newCloneStatePreservingSprite(&game, "SpriteA", &recorded)
	source.cloneValue = 3
	game.addShape(spriteOf(source))

	var cloned *cloneStatePreservingSprite
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneStatePreservingSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneStatePreservingSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if got := recorded; got != 3 {
		t.Fatalf("clone user state = %v, want 3", got)
	}
}

func TestCloneSpritePreservesAllTopLevelUserFields(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	var observed cloneAllFieldKindsObserved
	source := newCloneAllFieldKindsSprite(&game, "SpriteA", &observed)
	payload := &cloneFieldPayload{label: "orig-pointer", count: 9}
	source.boolValue = true
	source.intValue = 42
	source.stringValue = "orig"
	source.floatValue = 3.14
	source.valueValue = NewValue("orig-value")
	source.listValue.Init("left", 7)
	source.sliceValue = []int{1, 2, 3}
	source.mapValue = map[string]int{"orig": 9}
	source.structValue = cloneFieldPayload{label: "orig-struct", count: 5}
	source.arrayValue = [2]string{"left", "right"}
	source.pointerValue = payload
	source.interfaceValue = payload
	game.addShape(spriteOf(source))

	var cloned *cloneAllFieldKindsSprite
	doClone(source, nil, false, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAllFieldKindsSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAllFieldKindsSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if observed.boolValue != true {
		t.Fatalf("boolValue = %v, want true", observed.boolValue)
	}
	if observed.intValue != 42 {
		t.Fatalf("intValue = %v, want 42", observed.intValue)
	}
	if observed.stringValue != "orig" {
		t.Fatalf("stringValue = %q, want %q", observed.stringValue, "orig")
	}
	if observed.floatValue != 3.14 {
		t.Fatalf("floatValue = %v, want 3.14", observed.floatValue)
	}
	if observed.valueString != "orig-value" {
		t.Fatalf("valueValue = %q, want %q", observed.valueString, "orig-value")
	}
	if observed.listLen != 2 || observed.listString != "left 7" {
		t.Fatalf("listValue = (%d, %q), want (2, %q)", observed.listLen, observed.listString, "left 7")
	}
	if !reflect.DeepEqual(observed.sliceValue, []int{1, 2, 3}) {
		t.Fatalf("sliceValue = %v, want [1 2 3]", observed.sliceValue)
	}
	if !reflect.DeepEqual(observed.mapValue, map[string]int{"orig": 9}) {
		t.Fatalf("mapValue = %v, want map[orig:9]", observed.mapValue)
	}
	if observed.structValue != (cloneFieldPayload{label: "orig-struct", count: 5}) {
		t.Fatalf("structValue = %+v, want %+v", observed.structValue, cloneFieldPayload{label: "orig-struct", count: 5})
	}
	if observed.arrayValue != [2]string{"left", "right"} {
		t.Fatalf("arrayValue = %v, want [left right]", observed.arrayValue)
	}
	if observed.pointerValue != payload {
		t.Fatalf("pointerValue = %p, want %p", observed.pointerValue, payload)
	}
	if observed.interfaceValue != payload {
		t.Fatalf("interfaceValue = %#v, want %#v", observed.interfaceValue, payload)
	}
}

func TestClonedSpriteCollisionTargetRegistrationDoesNotRefreshCollisionLayers(t *testing.T) {
	game := &collisionLayerOrderGame{}
	game.isAutoSetCollisionLayer = true
	game.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {Index: 0, Layer: 1 << 0},
		"SpriteB": {Index: 1, Layer: 1 << 1},
	}

	spriteA := newCollisionLayerOrderSprite(&game.Game, "SpriteA", nil)
	spriteB := newCollisionLayerOrderSprite(&game.Game, "SpriteB", nil)
	cloneA := newCollisionLayerOrderSprite(&game.Game, "SpriteA", nil)
	cloneA.spriteState.Cloned = true

	game.setupCollisionData([]Sprite{spriteA, spriteB})
	spriteA.physics().collisionTargets["SpriteB"] = true
	cloneA.physics().addCollisionTarget("SpriteB")

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != 0 {
		t.Fatalf("SpriteB collision mask = %d, want 0", got)
	}
}
