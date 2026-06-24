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
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine/platform"
	"github.com/goplus/spx/v2/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
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
	nextID pkgengine.Object
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

func setupCloneSpriteMgr(t *testing.T) {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.SpriteMgr
	pkgengine.SpriteMgr = &spyCloneSpriteMgr{}
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
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
	platform.RunOnMainThread(func() {
		game.runSpriteCallbacks(
			[]Sprite{spriteA, spriteB},
			&coreproject.ProjectConfig{},
			reflect.ValueOf(game).Elem(),
			generation,
		)
	})
	platform.RunOnMainThread(func() {
		game.runBootstrapTasksFor(generation)
		game.refreshCollisionLayers()
	})

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != game.sprCollisionInfos["SpriteA"].Layer {
		t.Fatalf("SpriteB collision mask = %d, want %d", got, game.sprCollisionInfos["SpriteA"].Layer)
	}
}

func TestRunSpriteCallbacksRefreshesCollisionLayersRegisteredInMain(t *testing.T) {
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
	platform.RunOnMainThread(func() {
		game.runSpriteCallbacks(
			[]Sprite{spriteA, spriteB},
			&coreproject.ProjectConfig{},
			reflect.ValueOf(game).Elem(),
			generation,
		)
	})
	platform.RunOnMainThread(func() {
		game.runBootstrapTasksFor(generation)
	})

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

func TestInitRuntimeProxyAppliesCostumeBeforeAwake(t *testing.T) {
	var game Game
	setupCloneSpriteMgr(t)

	sprite := newCloneAwakeOrderSprite(&game, "SpriteA")
	sprite.spriteState.IsVisible = true

	platform.RunOnMainThread(func() {
		sprite.initRuntimeProxy()
	})

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
	platform.RunOnMainThread(func() {
		dest, _ = applySprite(out, source, shape)
	})

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
	platform.RunOnMainThread(func() {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			var ok bool
			cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
			if !ok {
				t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
			}
		})
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

func TestCloneSpriteInsertsImmediatelyAfterSource(t *testing.T) {
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
	platform.RunOnMainThread(func() {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			var ok bool
			cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
			if !ok {
				t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
			}
		})
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
	if shapes[1] != spriteOf(source) {
		t.Fatalf("shape[1] = %v, want source sprite", shapes[1])
	}
	if shapes[2] != spriteOf(cloned) {
		t.Fatalf("shape[2] = %v, want cloned sprite", shapes[2])
	}
	if shapes[3] != spriteOf(front) {
		t.Fatalf("shape[3] = %v, want front sprite", shapes[3])
	}

	if got, want := source.runtimeState.Layer, 2; got != want {
		t.Fatalf("source layer = %d, want %d", got, want)
	}
	if got, want := cloned.runtimeState.Layer, 3; got != want {
		t.Fatalf("clone layer = %d, want %d", got, want)
	}
	if got, want := front.runtimeState.Layer, 4; got != want {
		t.Fatalf("front layer = %d, want %d", got, want)
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
	platform.RunOnMainThread(func() {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			var ok bool
			cloned, ok = sprite.sprite.(*cloneStatePreservingSprite)
			if !ok {
				t.Fatalf("clone sprite type = %T, want *cloneStatePreservingSprite", sprite.sprite)
			}
		})
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
	platform.RunOnMainThread(func() {
		doClone(source, nil, false, func(sprite *SpriteImpl) {
			var ok bool
			cloned, ok = sprite.sprite.(*cloneAllFieldKindsSprite)
			if !ok {
				t.Fatalf("clone sprite type = %T, want *cloneAllFieldKindsSprite", sprite.sprite)
			}
		})
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

	platform.RunOnMainThread(func() {
		game.setupCollisionData([]Sprite{spriteA, spriteB})
		spriteA.physics().collisionTargets["SpriteB"] = true
		cloneA.physics().addCollisionTarget("SpriteB")
	})

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != 0 {
		t.Fatalf("SpriteB collision mask = %d, want 0", got)
	}
}
