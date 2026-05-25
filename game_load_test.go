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
	sawAwakeInMain bool
	onClonedFired  bool
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
	s.sawAwakeInMain = s.spriteState.IsAwakened
	s.OnCloned__0(func() {
		s.onClonedFired = true
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
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
	sprite := &cloneAwakeOrderSprite{
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

	if !cloned.sawAwakeInMain {
		t.Fatal("clone Main ran before awake")
	}
	if !cloned.onClonedFired {
		t.Fatal("clone onCloned did not fire after Main registration")
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
