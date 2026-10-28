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
	"fmt"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type pullPositionSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	calls     int
	ids       []int64
	positions []float32
}

type reentrantPullPositionMgr struct {
	enginewrap.SpriteMgrImpl
	game   *Game
	nested *SpriteImpl
	calls  int
}

type concurrentPullPositionMgr struct {
	enginewrap.SpriteMgrImpl
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

type positionSyncSpriteMgr struct {
	captureFlushSpriteMgr
	fail        bool
	onSubmit    func()
	position    mathf.Vec2
	submissions int
}

type framesChangedSprite struct {
	pkgengine.ISpriter
	changed func()
}

type textureFramesSpriteMgr struct {
	*positionSyncSpriteMgr
	callbacks int
}

type preparedResourceSpriteMgr struct {
	*touchingSyncSpriteMgr
	renderScales map[pkgengine.Object]mathf.Vec2
	visuals      map[pkgengine.Object]proxyVisual
	uvRemaps     map[pkgengine.Object]mathf.Vec4
	failAt       string
}

func TestFlushSyncBufferOnlySubmitsChanges(t *testing.T) {
	game, mgr := setupSyncBufferCapture(t)
	game.flushSyncBuffer()
	if len(mgr.batches) != 0 {
		t.Fatalf("empty buffer submitted %d batches, want 0", len(mgr.batches))
	}

	game.syncBuffer.Add(1, 2, 3, 4, 5, 6, 7, 8, true)
	game.flushSyncBuffer()
	if len(mgr.batches) != 1 {
		t.Fatalf("updated buffer submitted %d batches, want 1", len(mgr.batches))
	}

	game.syncBuffer.Clear()
	game.syncBuffer.AddDelete(1)
	game.flushSyncBuffer()
	if len(mgr.batches) != 2 {
		t.Fatalf("delete buffer submitted %d total batches, want 2", len(mgr.batches))
	}
}

func TestFlushSyncBufferDoesNotSubmitSerializationFailure(t *testing.T) {
	game, mgr := setupSyncBufferCapture(t)
	invalidID := int64(1<<24 + 1)
	game.syncBuffer.Add(invalidID, 0, 0, 0, 0, 0, 0, 0, false)

	requireSyncPanic(t, game.flushSyncBuffer, "invalid sprite ID", "legacy float32 batch encoding")
	if len(mgr.batches) != 0 {
		t.Fatalf("serialization failure submitted %d batches, want 0", len(mgr.batches))
	}
}

func TestProxyTransformQueryAndBatchMatch(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.spriteState.isVisible = true
	sprite.transform().direction = 45
	sprite.transform().rotationStyle = Normal

	sprite.syncProxyForQuery()
	if got, want := spy.spriteMgr.position, mathf.NewVec2(50, 60); got != want {
		t.Fatalf("query position = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.rotation, engine.DegToRad(-45); got != want {
		t.Fatalf("query rotation = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.scale, mathf.NewVec2(1, 1); got != want {
		t.Fatalf("query scale = %v, want %v", got, want)
	}
	if got, want := spy.spriteMgr.renderOffset, mathf.NewVec2(37, -24); got != want {
		t.Fatalf("query offset = %v, want %v", got, want)
	}
	if !spy.spriteMgr.visible {
		t.Fatal("query transform hid a visible sprite")
	}

	buffer := engine.NewSpriteSyncBuffer(1)
	sprite.collectProxyUpdate(buffer)
	if buffer.UpdateCount() != 0 {
		t.Fatal("batch repeated an already synchronized transform")
	}
	sprite.markProxyDirty()
	sprite.collectProxyUpdate(buffer)
	want := []float32{1, 0, 101, 50, 60, float32(spy.spriteMgr.rotation), 1, 1, 37, -24, 1}
	if got := buffer.Serialize(); !slices.Equal(got, want) {
		t.Fatalf("batch transform = %v, want %v", got, want)
	}
}

func TestProxyRebuildAndDestroySubmitOnlyPendingTransforms(t *testing.T) {
	var game Game
	game.initShapeMgr()
	mgr := setupCloneSpriteMgr(t)
	sprite := newCloneAwakeOrderSprite(&game, "Sprite")
	buffer := engine.NewSpriteSyncBuffer(1)
	collect := func(want int) {
		t.Helper()
		buffer.Clear()
		sprite.prepareProxyUpdate()
		update := sprite.collectProxyUpdate(buffer)
		if got := buffer.UpdateCount(); got != want {
			t.Fatalf("collected %d transforms, want %d", got, want)
		}
		engine.SyncBatchUpdateSprites(buffer.Serialize())
		commitProxyChange(update)
	}

	collect(0)
	sprite.initRuntimeProxy()
	collect(1)
	collect(0)
	oldProxy := sprite.runtimeState.syncSprite
	if oldProxy.CallbackAlive == nil || !oldProxy.CallbackAlive() {
		t.Fatal("initialized proxy rejected callbacks for its live sprite")
	}
	sprite.initRuntimeProxy()
	if sprite.runtimeState.syncSprite == oldProxy {
		t.Fatal("rebuild reused the old proxy")
	}
	collect(1)
	collect(0)

	sprite.Show()
	sprite.destroy()
	if oldProxy.CallbackAlive() || sprite.runtimeState.syncSprite.CallbackAlive() {
		t.Fatal("proxy accepted callbacks after logical sprite destruction")
	}
	operations := len(mgr.recordedOperations())
	sprite.syncProxyForQuery()
	collect(0)
	if got := len(mgr.recordedOperations()); got != operations {
		t.Fatalf("destroyed sprite made %d proxy calls", got-operations)
	}
}

func TestPullPhysicsPositionsFiltersAndKeepsIDOrder(t *testing.T) {
	mgr := setupPullPositionSpriteMgr(t)
	mgr.positions = []float32{11, 12, 31, 32}
	first := newPullPhysicsSprite(1, KinematicPhysics, 1, 2)
	noPhysics := newPullPhysicsSprite(2, NoPhysics, 2, 3)
	missing := newPullPhysicsSprite(4, DynamicPhysics, 4, 5)
	missing.runtimeState.syncSprite = nil
	last := newPullPhysicsSprite(3, DynamicPhysics, 3, 4)
	game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(4)}
	game.shapeMgr.items = []Shape{first, noPhysics, struct{}{}, missing, last}

	game.pullPhysicsPositions()
	if mgr.calls != 1 || !slices.Equal(mgr.ids, []int64{1, 3}) {
		t.Fatalf("position query calls=%d ids=%v, want one query for [1 3]", mgr.calls, mgr.ids)
	}
	for _, tt := range []struct {
		sprite  *SpriteImpl
		x, y    float64
		version uint64
	}{
		{first, 11, 12, 1},
		{noPhysics, 2, 3, 0},
		{missing, 4, 5, 0},
		{last, 31, 32, 1},
	} {
		x, y := tt.sprite.transform().getXY()
		if x != tt.x || y != tt.y || tt.sprite.spriteState.visualVersion != tt.version {
			t.Errorf("sprite position=(%v,%v) visual version=%d, want (%v,%v) version %d", x, y, tt.sprite.spriteState.visualVersion, tt.x, tt.y, tt.version)
		}
		if tt.sprite.spriteState.dirtyVersion != 0 {
			t.Errorf("physics pull dirtied proxy: %+v", tt.sprite.spriteState)
		}
	}
	game.pullPhysicsPositions()
	if first.spriteState.visualVersion != 1 || last.spriteState.visualVersion != 1 {
		t.Fatal("unchanged position incremented visual version")
	}
	mgr.positions = []float32{13, 14, float32(math.NaN()), 40}
	game.pullPhysicsPositions()
	if x, y := last.transform().getXY(); x != 31 || y != 32 || last.spriteState.visualVersion != 1 {
		t.Fatalf("missing sentinel changed last sprite to (%v,%v), version %d", x, y, last.spriteState.visualVersion)
	}
	if x, y := first.transform().getXY(); x != 13 || y != 14 || first.spriteState.visualVersion != 2 {
		t.Fatalf("valid result not applied to first sprite: (%v,%v), version %d", x, y, first.spriteState.visualVersion)
	}
	game.shapeMgr.items = []Shape{noPhysics, struct{}{}, missing}
	game.pullPhysicsPositions()
	if mgr.calls != 3 {
		t.Fatalf("empty physics pull made an engine query: %d total calls, want 3", mgr.calls)
	}
}

func TestPullPhysicsPositionsReentrant(t *testing.T) {
	setupPullPositionSpriteMgr(t)
	game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(2)}
	first := newPullPhysicsSprite(1, DynamicPhysics, 0, 0)
	second := newPullPhysicsSprite(2, DynamicPhysics, 0, 0)
	nested := newPullPhysicsSprite(3, DynamicPhysics, 0, 0)
	game.shapeMgr.items = []Shape{first, second}
	mgr := &reentrantPullPositionMgr{game: game, nested: nested}
	pkgengine.SpriteMgr = mgr

	game.pullPhysicsPositions()
	if mgr.calls != 2 {
		t.Fatalf("position queries = %d, want 2", mgr.calls)
	}
	for _, tt := range []struct {
		sprite *SpriteImpl
		x, y   float64
	}{{first, 10, 11}, {second, 20, 21}, {nested, 30, 31}} {
		if x, y := tt.sprite.transform().getXY(); x != tt.x || y != tt.y {
			t.Errorf("position = (%v, %v), want (%v, %v)", x, y, tt.x, tt.y)
		}
	}
	if game.physicsPull.busy.Load() || len(game.physicsPull.sprites) != 0 || game.physicsPull.sprites[:cap(game.physicsPull.sprites)][0] != nil {
		t.Fatal("position pull retained a sprite")
	}
}

func TestPullPhysicsPositionsConcurrent(t *testing.T) {
	setupPullPositionSpriteMgr(t)
	game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(1)}
	sprite := newPullPhysicsSprite(1, DynamicPhysics, 0, 0)
	game.shapeMgr.items = []Shape{sprite}
	mgr := &concurrentPullPositionMgr{started: make(chan struct{}), release: make(chan struct{})}
	pkgengine.SpriteMgr = mgr
	done := make(chan struct{})
	go func() {
		defer close(done)
		game.pullPhysicsPositions()
	}()
	<-mgr.started
	game.pullPhysicsPositions()
	close(mgr.release)
	<-done
	if mgr.calls.Load() != 2 {
		t.Fatalf("position queries = %d, want 2", mgr.calls.Load())
	}
	if x, y := sprite.transform().getXY(); x != 10 || y != 11 {
		t.Fatalf("position = (%v, %v), want (10, 11)", x, y)
	}
}

func BenchmarkPullPhysicsPositions(b *testing.B) {
	setupImmediateEngineCalls(b)
	previous := pkgengine.SpriteMgr
	mgr := &pullPositionSpriteMgr{positions: make([]float32, 600)}
	pkgengine.SpriteMgr = mgr
	b.Cleanup(func() { pkgengine.SpriteMgr = previous })

	for _, scenario := range []struct {
		name   string
		active int
	}{{"none", 0}, {"one", 1}, {"all", 300}} {
		b.Run(scenario.name, func(b *testing.B) {
			game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(300)}
			for i := range 300 {
				mode := NoPhysics
				if i < scenario.active {
					mode = DynamicPhysics
				}
				game.shapeMgr.items = append(game.shapeMgr.items, newPullPhysicsSprite(int64(i+1), mode, 0, 0))
			}
			game.pullPhysicsPositions()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				game.pullPhysicsPositions()
			}
		})
	}
}

func TestApplyPhysicsPositionsStopsAtShortResponse(t *testing.T) {
	sprites := []*SpriteImpl{
		newPhysicsPositionTestSprite(1, 2),
		newPhysicsPositionTestSprite(3, 4),
	}
	applyPhysicsPositions(sprites, []float32{11, 12, 31})
	if x, y := sprites[0].transform().getXY(); x != 11 || y != 12 {
		t.Fatalf("first sprite position=(%v,%v), want (11,12)", x, y)
	}
	if x, y := sprites[1].transform().getXY(); x != 3 || y != 4 {
		t.Fatalf("short response changed second sprite to (%v,%v)", x, y)
	}
}

func TestMergeEnginePositionInvalidatesVisualsOnlyWhenChanged(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)

	sprite.mergeEnginePosition(1, 2)
	if sprite.spriteState.visualVersion != 0 {
		t.Fatalf("unchanged position visual version = %d, want 0", sprite.spriteState.visualVersion)
	}

	sprite.mergeEnginePosition(3, 4)
	if sprite.spriteState.visualVersion != 1 {
		t.Fatalf("changed position visual version = %d, want 1", sprite.spriteState.visualVersion)
	}
	if sprite.spriteState.dirtyVersion != 0 {
		t.Fatalf("physics writeback dirtied proxy state: DirtyVersion=%d", sprite.spriteState.dirtyVersion)
	}
	if x, y := sprite.transform().getXY(); x != 3 || y != 4 {
		t.Fatalf("physics position = (%v, %v), want (3, 4)", x, y)
	}

	sprite.mergeEnginePosition(3, 4)
	if sprite.spriteState.visualVersion != 1 {
		t.Fatalf("repeated position visual version = %d, want 1", sprite.spriteState.visualVersion)
	}
}

func TestDispatchPhysicsTriggersFiltersPairs(t *testing.T) {
	game := newPhysicsTriggerTestGame(t)
	source := newPhysicsTriggerTestSprite(game, "source")
	target := newPhysicsTriggerTestSprite(game, "target")
	hidden := newPhysicsTriggerTestSprite(game, "hidden")
	hidden.spriteState.isVisible = false
	dying := newPhysicsTriggerTestSprite(game, "dying")
	dying.spriteState.isDying = true
	noHandler := newPhysicsTriggerTestSprite(game, "no-handler")
	var nilSprite *SpriteImpl
	typedNil := &engine.Sprite{Target: nilSprite}
	wrongType := &engine.Sprite{Target: "not a sprite"}
	emptyTarget := &engine.Sprite{}
	src, dst := source.runtimeState.syncSprite, target.runtimeState.syncSprite

	var calls []Sprite
	for _, sprite := range []*touchEventSprite{source, target, hidden, dying} {
		sprite.addTouchStartHandler(func(other Sprite) { calls = append(calls, other) })
	}
	for _, tt := range []struct {
		name     string
		src, dst *engine.Sprite
		touch    bool
	}{
		{"valid", src, dst, true},
		{"nil source", nil, dst, false},
		{"nil destination", src, nil, false},
		{"both proxies nil", nil, nil, false},
		{"wrong source type", wrongType, dst, false},
		{"wrong destination type", src, wrongType, false},
		{"both types wrong", wrongType, wrongType, false},
		{"nil source target", emptyTarget, dst, false},
		{"nil destination target", src, emptyTarget, false},
		{"typed nil source and invalid destination", typedNil, wrongType, false},
		{"invalid source and typed nil destination", wrongType, typedNil, false},
		{"hidden source and invalid destination", hidden.runtimeState.syncSprite, wrongType, false},
		{"hidden source", hidden.runtimeState.syncSprite, dst, false},
		{"hidden destination", src, hidden.runtimeState.syncSprite, false},
		{"dying source", dying.runtimeState.syncSprite, dst, false},
		{"dying destination", src, dying.runtimeState.syncSprite, false},
		{"hidden source short circuits typed nil destination", hidden.runtimeState.syncSprite, typedNil, false},
		{"dying source short circuits typed nil destination", dying.runtimeState.syncSprite, typedNil, false},
		{"no source handler", noHandler.runtimeState.syncSprite, dst, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls = nil
			game.triggerEvents = []engine.TriggerEvent{{Src: tt.src, Dst: tt.dst}, {Src: src, Dst: dst}}
			game.dispatchPhysicsTriggers()
			want := []Sprite{target} // Processing must continue after a skipped pair.
			if tt.touch {
				want = append(want, target)
			}
			if !slices.Equal(calls, want) {
				t.Fatalf("touches = %v, want %v", calls, want)
			}
			if len(game.triggerEvents) != 0 || game.triggerEvents[:1][0] != (engine.TriggerEvent{}) {
				t.Fatal("normal dispatch retained a trigger pair")
			}
		})
	}
}

func TestDispatchPhysicsTriggersOrderAndBufferReuse(t *testing.T) {
	game := newPhysicsTriggerTestGame(t)
	first := newPhysicsTriggerTestSprite(game, "first")
	second := newPhysicsTriggerTestSprite(game, "second")
	last := newPhysicsTriggerTestSprite(game, "last")
	var calls []string
	for _, sprite := range []*touchEventSprite{first, second, last} {
		sprite.addTouchStartHandler(func(other Sprite) {
			calls = append(calls, sprite.name+":"+spriteOf(other).name)
		})
	}
	firstPair := engine.TriggerEvent{Src: first.runtimeState.syncSprite, Dst: second.runtimeState.syncSprite}
	backing := make([]engine.TriggerEvent, 4)
	backing[0] = firstPair
	backing[1] = engine.TriggerEvent{Src: second.runtimeState.syncSprite, Dst: last.runtimeState.syncSprite}
	backing[2] = engine.TriggerEvent{Src: first.runtimeState.syncSprite, Dst: last.runtimeState.syncSprite}
	backing[3] = firstPair // Capacity beyond the active batch is not cleared.
	game.triggerEvents = backing[:3]
	game.dispatchPhysicsTriggers()
	if want := []string{"first:second", "second:last", "first:last"}; !slices.Equal(calls, want) {
		t.Fatalf("touch order = %v, want %v", calls, want)
	}
	if len(game.triggerEvents) != 0 || cap(game.triggerEvents) != len(backing) || &game.triggerEvents[:1][0] != &backing[0] {
		t.Fatal("dispatch did not retain the empty reusable buffer")
	}
	for _, pair := range backing[:3] {
		if pair != (engine.TriggerEvent{}) {
			t.Fatal("dispatch retained a processed pair")
		}
	}
	if backing[3] != firstPair {
		t.Fatal("dispatch cleared beyond the active batch")
	}
	game.triggerEvents = append(game.triggerEvents, firstPair)
	game.dispatchPhysicsTriggers()
	if len(calls) != 4 || calls[3] != "first:second" || backing[0] != (engine.TriggerEvent{}) {
		t.Fatal("reused buffer did not dispatch and clear its next batch")
	}
	game.dispatchPhysicsTriggers()
	if len(calls) != 4 {
		t.Fatal("empty dispatch repeated a touch")
	}
}

func TestDispatchPhysicsTriggersRechecksTouchability(t *testing.T) {
	for _, dying := range []bool{false, true} {
		t.Run(fmt.Sprintf("dying=%v", dying), func(t *testing.T) {
			game := newPhysicsTriggerTestGame(t)
			source := newPhysicsTriggerTestSprite(game, "source")
			target := newPhysicsTriggerTestSprite(game, "target")
			last := newPhysicsTriggerTestSprite(game, "last")
			var calls []Sprite
			source.addTouchStartHandler(func(other Sprite) {
				calls = append(calls, other)
				if dying {
					target.spriteState.isDying = true
				} else {
					target.spriteState.isVisible = false
				}
			})
			target.addTouchStartHandler(func(Sprite) { t.Error("untouchable source fired a callback") })
			src, dst := source.runtimeState.syncSprite, target.runtimeState.syncSprite
			game.triggerEvents = []engine.TriggerEvent{
				{Src: src, Dst: dst},
				{Src: dst, Dst: src},
				{Src: src, Dst: dst},
				{Src: src, Dst: last.runtimeState.syncSprite},
			}
			game.dispatchPhysicsTriggers()
			if want := []Sprite{target, last}; !slices.Equal(calls, want) {
				t.Fatalf("touches = %v, want target then last", calls)
			}
		})
	}
}

func TestDispatchPhysicsTriggersPanicRetainsBatch(t *testing.T) {
	for _, name := range []string{"callback", "typed nil source", "typed nil destination", "typed nil source before hidden destination"} {
		t.Run(name, func(t *testing.T) {
			game := newPhysicsTriggerTestGame(t)
			source := newPhysicsTriggerTestSprite(game, "source")
			target := newPhysicsTriggerTestSprite(game, "target")
			panicking := newPhysicsTriggerTestSprite(game, "panicking")
			hidden := newPhysicsTriggerTestSprite(game, "hidden")
			hidden.spriteState.isVisible = false
			var nilSprite *SpriteImpl
			typedNil := &engine.Sprite{Target: nilSprite}
			calls := 0
			source.addTouchStartHandler(func(Sprite) { calls++ })
			panicValue := new(int)
			panicking.addTouchStartHandler(func(Sprite) { panic(panicValue) })
			src, dst := source.runtimeState.syncSprite, target.runtimeState.syncSprite
			pair := engine.TriggerEvent{Src: panicking.runtimeState.syncSprite, Dst: dst}
			switch name {
			case "typed nil source":
				pair.Src = typedNil
			case "typed nil destination":
				pair.Src, pair.Dst = src, typedNil
			case "typed nil source before hidden destination":
				pair.Src, pair.Dst = typedNil, hidden.runtimeState.syncSprite
			}
			game.triggerEvents = []engine.TriggerEvent{{Src: src, Dst: dst}, pair, {Src: src, Dst: dst}}
			before := slices.Clone(game.triggerEvents)
			backing := &game.triggerEvents[0]
			defer func() {
				value := recover()
				if value == nil || name == "callback" && value != panicValue {
					t.Fatalf("panic = %v, want the original panic", value)
				}
				if calls != 1 {
					t.Fatalf("completed touches = %d, want only the first pair", calls)
				}
				if !slices.Equal(game.triggerEvents, before) || &game.triggerEvents[0] != backing {
					t.Fatal("panic cleared or replaced the uncompleted batch")
				}
			}()
			game.dispatchPhysicsTriggers()
		})
	}
}

func TestProxyCallbacksIgnoreMissingAndDestroyedSprites(t *testing.T) {
	destroyed := &SpriteImpl{}
	destroyed.markDestroyed()
	for _, sprite := range []*SpriteImpl{{}, destroyed} {
		sprite.handleAnimationFinished()
		sprite.handleAnimationLooped()
		if sprite.runtimeState.syncSprite != nil {
			t.Fatal("stale callback created a proxy")
		}
	}
	sprite := &SpriteImpl{}
	sprite.runtimeState.syncSprite = &engine.Sprite{}
	sprite.ensureProxyInitialized()

	for _, state := range []string{"missing", "replaced", "destroyed"} {
		t.Run(state, func(t *testing.T) {
			sprite := newPhysicsPositionTestSprite(10, 20)
			proxy := &engine.Sprite{}
			sprite.runtimeState.syncSprite = proxy
			switch state {
			case "missing":
				sprite.runtimeState.syncSprite = nil
			case "replaced":
				sprite.runtimeState.syncSprite = &engine.Sprite{}
			case "destroyed":
				sprite.markDestroyed()
			}
			pending := positionVersions{x: 1, y: 2}
			sprite.transform().pendingPosition = pending
			sprite.mirrorPosition(proxy, mathf.NewVec2(30, 40), engine.PositionXY)
			requirePosition(t, sprite, 10, 20)
			if sprite.transform().pendingPosition != pending {
				t.Fatal("stale position callback acknowledged pending writes")
			}
		})
	}
}

func TestPhysicsPullPreservesExplicitPositionAxes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		write func(*SpriteImpl)
		x, y  float64
	}{
		{"set x", func(s *SpriteImpl) { s.SetXpos(42) }, 42, 200},
		{"set same x", func(s *SpriteImpl) { s.SetXpos(10) }, 10, 200},
		{"set y", func(s *SpriteImpl) { s.SetYpos(42) }, 100, 42},
		{"set same y", func(s *SpriteImpl) { s.SetYpos(20) }, 100, 20},
		{"change x", func(s *SpriteImpl) { s.ChangeXpos(3) }, 13, 200},
		{"change y", func(s *SpriteImpl) { s.ChangeYpos(3) }, 100, 23},
		{"set xy", func(s *SpriteImpl) { s.SetXYpos(42, 43) }, 42, 43},
		{"set same xy", func(s *SpriteImpl) { s.SetXYpos(10, 20) }, 10, 20},
		{"change xy", func(s *SpriteImpl) { s.ChangeXYpos(3, 4) }, 13, 24},
		{"move", func(s *SpriteImpl) { s.transform().direction = 0; s.Move__0(5) }, 10, 25},
		{"hide", func(s *SpriteImpl) { s.Hide() }, 100, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite, _ := setupPositionSyncSprite(t)
			tt.write(sprite)
			sprite.mergeEnginePosition(100, 200)
			requirePosition(t, sprite, tt.x, tt.y)
		})
	}
}

func TestDirectProxyAxisWritesPreserveOtherPendingAxis(t *testing.T) {
	for _, tt := range []struct {
		name  string
		write func(*engine.Sprite)
		axis  engine.PositionAxes
		want  mathf.Vec2
	}{
		{"set x", func(p *engine.Sprite) { p.SetPosX(42) }, engine.PositionX, mathf.NewVec2(42, 99)},
		{"add x", func(p *engine.Sprite) { p.AddPosX(3) }, engine.PositionX, mathf.NewVec2(13, 99)},
		{"set same x", func(p *engine.Sprite) { p.SetPosX(10) }, engine.PositionX, mathf.NewVec2(10, 99)},
		{"add zero x", func(p *engine.Sprite) { p.AddPosX(0) }, engine.PositionX, mathf.NewVec2(10, 99)},
		{"set y", func(p *engine.Sprite) { p.SetPosY(42) }, engine.PositionY, mathf.NewVec2(99, 42)},
		{"add y", func(p *engine.Sprite) { p.AddPosY(3) }, engine.PositionY, mathf.NewVec2(99, 23)},
		{"set same y", func(p *engine.Sprite) { p.SetPosY(20) }, engine.PositionY, mathf.NewVec2(99, 20)},
		{"add zero y", func(p *engine.Sprite) { p.AddPosY(0) }, engine.PositionY, mathf.NewVec2(99, 20)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite, mgr := setupPositionSyncSprite(t)
			if tt.axis == engine.PositionX {
				sprite.SetYpos(99)
			} else {
				sprite.SetXpos(99)
			}
			tt.write(sprite.runtimeState.syncSprite)
			requirePosition(t, sprite, tt.want.X, tt.want.Y)

			sprite.g.flushSpriteProxyChanges(sprite.g.shapeMgr.items)
			if mgr.position != tt.want {
				t.Fatalf("submitted position = %v, want %v", mgr.position, tt.want)
			}
			sprite.mergeEnginePosition(200, 300)
			requirePosition(t, sprite, 200, 300)
		})
	}
}

func TestNonFiniteHeadingDoesNotExitFrame(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			co, game, _ := setupFrameSyncGame(t)
			sprite := addFrameSyncSprite(game, 1, "heading")
			sprite.SetHeading(45)
			previous := pkgengine.ExtMgr
			ext := &inputFrameFailureExtMgr{}
			pkgengine.ExtMgr = ext
			t.Cleanup(func() { pkgengine.ExtMgr = previous })
			sprite.SetHeading(value)
			func() {
				defer engine.CheckPanic()
				runFrameSyncGame(co, game)
			}()
			if sprite.Heading() != 45 || len(ext.exits) != 0 || len(ext.messages) != 0 {
				t.Fatalf("invalid heading changed frame: heading=%v exits=%v errors=%v", sprite.Heading(), ext.exits, ext.messages)
			}
		})
	}
}

func TestProxyQueryAndBatchRejectInvalidTransformsWithoutAcknowledging(t *testing.T) {
	for _, mode := range []string{"query", "batch"} {
		for _, field := range []string{"x", "rotation"} {
			t.Run(mode+"/"+field, func(t *testing.T) {
				sprite, mgr := setupPositionSyncSprite(t)
				sprite.SetXpos(42)
				if field == "x" {
					sprite.SetXpos(math.MaxFloat64)
				} else {
					// Bypass the public setter to exercise the internal-state guard.
					sprite.transform().direction = math.NaN()
				}
				pending := sprite.transform().pendingPosition
				requireSyncPanic(t, func() {
					if mode == "query" {
						sprite.syncProxyForQuery()
					} else {
						sprite.g.flushSpriteProxyChanges(sprite.g.shapeMgr.items)
					}
				}, "sprite 1", field+"=")
				if mgr.submissions != 0 || sprite.transform().pendingPosition != pending ||
					sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
					t.Fatal("invalid transform was submitted or acknowledged")
				}
				sprite.SetHeading(45)
				sprite.SetXpos(80)
				sprite.g.flushSpriteProxyChanges(sprite.g.shapeMgr.items)
				if mgr.submissions != 1 || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
					t.Fatal("corrected transform did not commit")
				}
			})
		}
	}
}

func TestPositionSyncRequiresSuccessfulSubmission(t *testing.T) {
	for _, mode := range []string{"query", "batch", "serialization"} {
		t.Run(mode, func(t *testing.T) {
			sprite, mgr := setupPositionSyncSprite(t)
			sync := sprite.syncProxyForQuery
			if mode != "query" {
				sync = func() { sprite.g.flushSpriteProxyChanges(sprite.g.shapeMgr.items) }
			}
			sprite.SetXpos(42)
			mgr.position = mathf.NewVec2(100, 200)
			sprite.mergeEnginePosition(100, 200)
			wantPanic := "position sync failed"
			if mode == "serialization" {
				sprite.runtimeState.syncSprite.Id = 1<<24 + 1
				wantPanic = "invalid sprite ID"
			} else {
				mgr.fail = true
			}
			requireSyncPanic(t, sync, wantPanic)
			if sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
				t.Fatal("failed sync consumed dirty state")
			}
			mgr.position = mathf.NewVec2(300, 400)
			sprite.mergeEnginePosition(300, 400)
			requirePosition(t, sprite, 42, 400)
			mgr.fail = false
			sprite.runtimeState.syncSprite.Id = 1
			sync()
			if mgr.submissions != 1 || mgr.position != mathf.NewVec2(42, 400) {
				t.Fatalf("submitted %d positions, latest %v", mgr.submissions, mgr.position)
			}
			if sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
				t.Fatal("successful sync left dirty state")
			}
			sprite.mergeEnginePosition(500, 600)
			requirePosition(t, sprite, 500, 600)
		})
	}
}

func TestPositionSyncPreservesLaterWrites(t *testing.T) {
	for _, mode := range []string{"query", "batch"} {
		for _, test := range []struct {
			name  string
			write func(*SpriteImpl)
			x, y  float64
		}{
			{"hide", (*SpriteImpl).Hide, 100, 200},
			{"set same x", func(s *SpriteImpl) { s.SetXpos(42) }, 42, 200},
			{"set x", func(s *SpriteImpl) { s.SetXpos(84) }, 84, 200},
			{"set y", func(s *SpriteImpl) { s.SetYpos(43) }, 100, 43},
		} {
			t.Run(mode+"/"+test.name, func(t *testing.T) {
				sprite, mgr := setupPositionSyncSprite(t)
				sync := sprite.syncProxyForQuery
				if mode == "batch" {
					sync = func() { sprite.g.flushSpriteProxyChanges(sprite.g.shapeMgr.items) }
				}
				sprite.SetXpos(42)
				mgr.onSubmit = func() { test.write(sprite) }
				sync()
				mgr.position = mathf.NewVec2(100, 200)
				sprite.mergeEnginePosition(100, 200)
				requirePosition(t, sprite, test.x, test.y)
				if sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
					t.Fatal("older submission acknowledged a newer write")
				}
				mgr.onSubmit = nil
				sync()
				if mgr.position != mathf.NewVec2(test.x, test.y) {
					t.Fatalf("next submission = %v, want (%v, %v)", mgr.position, test.x, test.y)
				}
				sprite.mergeEnginePosition(300, 400)
				requirePosition(t, sprite, 300, 400)
			})
		}
	}
}

func TestProxyCollectionIncludesCallbackWrites(t *testing.T) {
	for _, test := range []struct {
		name  string
		write func(*SpriteImpl)
		x     float64
	}{
		{"hide", (*SpriteImpl).Hide, 42},
		{"nested query", func(s *SpriteImpl) {
			s.SetXpos(84)
			s.syncProxyForQuery()
		}, 84},
		{"direct position", func(s *SpriteImpl) {
			s.runtimeState.syncSprite.SetPosition(mathf.NewVec2(84, 20))
		}, 84},
	} {
		t.Run(test.name, func(t *testing.T) {
			sprite, mgr := setupPositionSyncSprite(t)
			textures := &textureFramesSpriteMgr{positionSyncSpriteMgr: mgr}
			pkgengine.SpriteMgr = textures
			sprite.SetXpos(42)
			game := sprite.g
			later := &newCloneAwakeOrderSprite(game, "costume").SpriteImpl
			later.currentCostume().path = "sprites/costume.png"
			later.runtimeState.isCostumeDirty = true
			later.runtimeState.isLayerDirty = false
			later.runtimeState.syncSprite = &engine.Sprite{Target: later, MirrorPosition: later.mirrorPosition}
			const id = pkgengine.Object(9001)
			pkgengine.InitSpriteInstance(id, later.runtimeState.syncSprite, nil)
			registry := pkgengine.Sprites()
			previous, existed := registry[id]
			t.Cleanup(func() {
				if existed {
					registry[id] = previous
				} else {
					delete(registry, id)
				}
			})
			registry[id] = &framesChangedSprite{
				ISpriter: later.runtimeState.syncSprite,
				changed:  func() { test.write(sprite) },
			}
			game.shapeMgr.add(later)
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			if textures.callbacks != 1 || mgr.position != mathf.NewVec2(test.x, 20) {
				t.Fatalf("texture callbacks=%d, submitted position=%v, want (%v, 20)", textures.callbacks, mgr.position, test.x)
			}
			requirePosition(t, sprite, test.x, 20)
			if sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
				t.Fatal("callback changes were not included in the final submission")
			}
			mgr.position = mathf.NewVec2(100, 200)
			sprite.mergeEnginePosition(100, 200)
			requirePosition(t, sprite, 100, 200)
			sprite.markProxyDirty()
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			if mgr.position != mathf.NewVec2(100, 200) {
				t.Fatalf("next update reset physics position to %v, want (100, 200)", mgr.position)
			}
		})
	}
}

func TestProxyCommitRetriesPositionAndDeletionAfterFailure(t *testing.T) {
	for _, failure := range []string{"serialization", "bridge"} {
		t.Run(failure, func(t *testing.T) {
			sprite, mgr := setupPositionSyncSprite(t)
			game := sprite.g
			mgr.fail = failure == "bridge"
			sprite.SetXpos(42)
			invalid := &newCloneAwakeOrderSprite(game, "invalid").SpriteImpl
			invalid.runtimeState.isCostumeDirty = false
			invalid.runtimeState.syncSprite = &engine.Sprite{Target: invalid, MirrorPosition: invalid.mirrorPosition}
			invalid.runtimeState.syncSprite.Id = 2
			if failure == "serialization" {
				// A finite Go coordinate can overflow the float32 batch protocol.
				invalid.SetXpos(math.MaxFloat64)
			} else {
				invalid.SetXpos(80)
			}
			game.shapeMgr.add(invalid)
			destroyed := &SpriteImpl{}
			proxy := &engine.Sprite{}
			proxy.Id = 3
			destroyed.runtimeState.syncSprite = proxy
			game.shapeMgr.add(destroyed)
			game.shapeMgr.removeShape(destroyed)
			game.shapeMgr.removeShape(destroyed)
			wantPanic := []string{"position sync failed"}
			if failure == "serialization" {
				wantPanic = []string{"sprite 2 transform x=", "finite after float32 conversion"}
			}
			requireSyncPanic(t, func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }, wantPanic...)
			if mgr.submissions != 0 {
				t.Fatal("failed batch committed positions to the engine")
			}
			if len(mgr.batches) != 0 || destroyed.runtimeState.syncSprite != proxy || len(game.shapeMgr.destroyItems) != 1 {
				t.Fatal("failed batch consumed a native proxy or its pending deletion")
			}
			for _, pending := range []*SpriteImpl{sprite, invalid} {
				if pending.spriteState.proxySyncVersion == pending.spriteState.dirtyVersion {
					t.Fatalf("rejected batch acknowledged sprite %q", pending.name)
				}
			}
			sprite.mergeEnginePosition(100, 200)
			requirePosition(t, sprite, 42, 200)
			invalid.SetXpos(80)
			mgr.fail = false
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			if mgr.submissions != 1 || mgr.position != mathf.NewVec2(42, 200) {
				t.Fatalf("retried batch submitted %d times with position %v", mgr.submissions, mgr.position)
			}
			if len(mgr.batches) != 1 || !slices.Equal(batchDeleteIDs(mgr.batches[0]), []float32{3}) {
				t.Fatalf("retry did not submit exactly one deletion: %v", mgr.batches)
			}
			if destroyed.runtimeState.syncSprite != nil || len(game.shapeMgr.destroyItems) != 0 {
				t.Fatal("successful retry retained the deleted proxy or request")
			}
			for _, synced := range []*SpriteImpl{sprite, invalid} {
				if synced.spriteState.proxySyncVersion != synced.spriteState.dirtyVersion {
					t.Fatalf("successful retry did not acknowledge sprite %q", synced.name)
				}
			}
			sprite.mergeEnginePosition(300, 400)
			requirePosition(t, sprite, 300, 400)
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			if len(mgr.batches) != 1 {
				t.Fatal("completed batch was submitted again")
			}
		})
	}
}

func TestProxyCommitKeepsDeletionsQueuedDuringSubmission(t *testing.T) {
	sprite, mgr := setupPositionSyncSprite(t)
	game := sprite.g
	first, later := &SpriteImpl{}, &SpriteImpl{}
	for i, item := range []*SpriteImpl{first, later} {
		item.runtimeState.syncSprite = &engine.Sprite{}
		item.runtimeState.syncSprite.Id = int64(i + 2)
	}
	game.shapeMgr.add(first)
	game.shapeMgr.removeShape(first)
	mgr.onSubmit = func() {
		game.shapeMgr.add(later)
		game.shapeMgr.removeShape(later)
		mgr.onSubmit = nil
	}
	game.flushSpriteProxyChanges(game.shapeMgr.items)
	if first.runtimeState.syncSprite != nil || later.runtimeState.syncSprite == nil ||
		len(game.shapeMgr.destroyItems) != 1 || game.shapeMgr.destroyItems[0] != later {
		t.Fatal("commit consumed a deletion that was not in the submitted batch")
	}
	game.flushSpriteProxyChanges(game.shapeMgr.items)
	if len(mgr.batches) != 2 || !slices.Equal(batchDeleteIDs(mgr.batches[0]), []float32{2}) ||
		!slices.Equal(batchDeleteIDs(mgr.batches[1]), []float32{3}) {
		t.Fatalf("deletions did not preserve submission boundaries: %v", mgr.batches)
	}
	if later.runtimeState.syncSprite != nil || len(game.shapeMgr.destroyItems) != 0 {
		t.Fatal("later deletion was not committed")
	}
	game.flushSpriteProxyChanges(game.shapeMgr.items)
	if len(mgr.batches) != 2 {
		t.Fatal("committed deletions were submitted again")
	}
}

func TestProxyPreparedResourcesStayPairedAcrossCallbacks(t *testing.T) {
	type expectedFrame struct {
		costume  int
		position mathf.Vec2
		visible  bool
	}
	position := mathf.NewVec2(42, 20)
	for _, test := range []struct {
		name         string
		prepare      func(*testing.T, *SpriteImpl, *preparedResourceSpriteMgr)
		onOwnTexture bool
		onChange     func(*SpriteImpl)
		first        expectedFrame
		nextSize     float64
	}{
		{name: "later costume", first: expectedFrame{1, position, true}, nextSize: 1.5},
		{
			name: "direct position and hide",
			onChange: func(s *SpriteImpl) {
				s.runtimeState.syncSprite.SetPosition(mathf.NewVec2(84, 25))
				s.Hide()
			},
			first: expectedFrame{1, mathf.NewVec2(84, 25), false}, nextSize: 1.5,
		},
		{name: "same sprite query", onOwnTexture: true, onChange: (*SpriteImpl).syncProxyForQuery, first: expectedFrame{1, position, true}, nextSize: 1.5},
		{name: "other sprite query", onChange: (*SpriteImpl).syncProxyForQuery, first: expectedFrame{2, position, true}, nextSize: 1.5},
		{name: "size", onChange: func(s *SpriteImpl) { s.SetSize(2) }, first: expectedFrame{1, position, true}, nextSize: 2},
		{
			name: "clone publication",
			prepare: func(t *testing.T, s *SpriteImpl, mgr *preparedResourceSpriteMgr) {
				s.beginCloneProxyPublication()
				s.markProxyDirty()
				s.syncProxyForQuery()
				if mgr.visible[s.runtimeState.syncSprite.Id] {
					t.Fatal("pending clone was published by query")
				}
				s.finishCloneInitialization()
			},
			first: expectedFrame{1, position, true}, nextSize: 1.5,
		},
		{
			name: "atlas query",
			prepare: func(_ *testing.T, s *SpriteImpl, _ *preparedResourceSpriteMgr) {
				for i, c := range s.costumes {
					c.atlas = true
					c.frame.AtlasUVRect = mathf.NewVec4(float64(i)/4, 0, 0.25, 1)
				}
			},
			onOwnTexture: true, onChange: (*SpriteImpl).syncProxyForQuery,
			first: expectedFrame{1, position, true}, nextSize: 1.5,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sprite, later, mgr := setupPreparedResourceSync(t)
			game := sprite.g
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			requirePreparedResource(t, sprite, mgr, 0, 1.5)
			if test.prepare != nil {
				test.prepare(t, sprite, mgr)
			}
			sprite.SetCostume__0("costume1")
			sprite.SetXpos(42)
			trigger := later.runtimeState.syncSprite
			if test.onOwnTexture {
				trigger = sprite.runtimeState.syncSprite
			}
			calls := 0
			trigger.OnFramesSetChangedEvent.Subscribe(func() {
				calls++
				sprite.SetCostume__0("costume2")
				if test.onChange != nil {
					test.onChange(sprite)
				}
			})
			later.runtimeState.isCostumeDirty = true
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			if calls != 1 {
				t.Fatalf("synchronous resource callbacks = %d, want 1", calls)
			}
			want := test.first
			requirePreparedResource(t, sprite, mgr, want.costume, 1.5)
			if mgr.positions[1] != want.position || mgr.visible[1] != want.visible {
				t.Fatalf("submitted position/visibility = %v/%v, want %v/%v", mgr.positions[1], mgr.visible[1], want.position, want.visible)
			}
			if synced := sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion; synced != (want.costume == 2 || !want.visible) {
				t.Fatalf("captured transform acknowledged = %v", synced)
			}
			if sprite.runtimeState.isCostumeDirty != (want.costume != 2) {
				t.Fatal("resource preparation state does not match the captured costume")
			}
			if sprite.transform().pendingPosition != (positionVersions{}) {
				t.Fatal("successful resource submission retained position intent")
			}
			trigger.OnFramesSetChangedEvent.UnsubscribeAll()
			sprite.Show()
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			requirePreparedResource(t, sprite, mgr, 2, test.nextSize)
			if sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
				t.Fatal("next frame did not acknowledge the current resource generation")
			}
		})
	}
}

func TestProxyPreparedResourcesPreserveCommittedHeading(t *testing.T) {
	for _, query := range []bool{false, true} {
		t.Run(fmt.Sprintf("query=%v", query), func(t *testing.T) {
			sprite, later, mgr := setupPreparedResourceSync(t)
			game := sprite.g
			flush := func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }
			flush()
			sprite.SetHeading(0)
			if query {
				sprite.syncProxyForQuery()
			} else {
				flush()
			}
			want := mgr.visuals[1]
			uploads := mgr.setTextureCalls[1]
			// Clean preparation must not upload resources again.
			flush()
			if mgr.setTextureCalls[1] != uploads {
				t.Fatal("clean preparation uploaded the texture again")
			}

			later.runtimeState.syncSprite.OnFramesSetChangedEvent.Subscribe(func() {
				sprite.SetCostume__0("costume2")
			})
			later.runtimeState.isCostumeDirty = true
			flush()
			got := mgr.visuals[1]
			if math.Abs(got.rotation-want.rotation) > 1e-6 || got.scale != want.scale || got.renderOffset != want.renderOffset {
				t.Fatalf("resource fallback changed committed geometry: got %+v, want %+v", got, want)
			}
			if mgr.setTextureCalls[1] != uploads || mgr.texturePaths[1] != engine.ToAssetPath(sprite.costumes[0].path) {
				t.Fatal("reentrant costume was uploaded before its preparation")
			}
			if !sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
				t.Fatal("unprepared costume was acknowledged")
			}
			flush()
			if mgr.setTextureCalls[1] != uploads+1 || sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
				t.Fatal("next frame did not prepare and acknowledge the new costume exactly once")
			}
		})
	}
}

func TestHiddenProxyDeferredResourcesConverge(t *testing.T) {
	for _, change := range []string{"costume", "size"} {
		for _, query := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/query=%v", change, query), func(t *testing.T) {
				sprite, _, mgr := setupPreparedResourceSync(t)
				game := sprite.g
				flush := func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }
				flush()
				sprite.Hide()
				flush()
				uploads := mgr.setTextureCalls[1]
				costume, size := 0, 1.5
				if change == "costume" {
					sprite.SetCostume__0("costume1")
					costume = 1
				} else {
					sprite.SetSize(2)
					size = 2
				}
				sprite.SetXpos(42)
				mgr.failAt = "batch"
				requireSyncPanic(t, flush, "batch", "failed")
				if sprite.transform().pendingPosition.x == 0 || sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
					t.Fatal("failed hidden submission acknowledged position or transform")
				}
				mgr.failAt = ""
				flush()
				if mgr.positions[1].X != 42 || mgr.visible[1] || sprite.transform().pendingPosition != (positionVersions{}) {
					t.Fatal("hidden submission lost position or visibility")
				}
				for range 3 {
					flush()
					if game.syncBuffer.UpdateCount() != 0 {
						t.Error("unchanged hidden sprite resubmitted its transform")
					}
				}
				if !sprite.runtimeState.isCostumeDirty || mgr.setTextureCalls[1] != uploads {
					t.Fatal("hidden resources were not deferred")
				}
				if query {
					mgr.failAt = "resource"
					requireSyncPanic(t, sprite.syncProxyForQuery, "resource", "failed")
					if !sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
						t.Fatal("failed query acknowledged deferred resources")
					}
					mgr.failAt = ""
					sprite.syncProxyForQuery()
				} else {
					sprite.Show()
					flush()
				}
				requirePreparedResource(t, sprite, mgr, costume, size)
				if mgr.visible[1] == query || sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion {
					t.Fatal("deferred resources did not synchronize with their transform")
				}
				flush()
				if game.syncBuffer.UpdateCount() != 0 {
					t.Fatal("completed resources caused another transform submission")
				}
			})
		}
	}
}

func TestHiddenProxyQueryPreservesReentrantResources(t *testing.T) {
	sprite, _, mgr := setupPreparedResourceSync(t)
	game := sprite.g
	flush := func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }
	flush()
	sprite.Hide()
	sprite.SetCostume__0("costume1")
	flush()
	calls := 0
	proxy := sprite.runtimeState.syncSprite
	proxy.OnFramesSetChangedEvent.Subscribe(func() {
		calls++
		sprite.SetCostume__0("costume2")
		sprite.SetXpos(84)
		sprite.syncProxyForQuery()
	})
	sprite.syncProxyForQuery()
	if calls != 1 || mgr.positions[1].X != 84 || mgr.visible[1] || !sprite.runtimeState.isCostumeDirty {
		t.Fatal("nested hidden query lost position, visibility, or pending resources")
	}
	requirePreparedResource(t, sprite, mgr, 1, 1.5)
	proxy.OnFramesSetChangedEvent.UnsubscribeAll()
	flush()
	if game.syncBuffer.UpdateCount() != 0 {
		t.Fatal("nested query's completed hidden transform was submitted again")
	}
	sprite.syncProxyForQuery()
	requirePreparedResource(t, sprite, mgr, 2, 1.5)
	if sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion || sprite.transform().pendingPosition != (positionVersions{}) {
		t.Fatal("second hidden query did not finish pending resources and position")
	}
}

func TestHiddenProxyHeadingSurvivesCallbackShow(t *testing.T) {
	sprite, later, mgr := setupPreparedResourceSync(t)
	game := sprite.g
	flush := func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }
	flush()
	sprite.Hide()
	sprite.SetHeading(0)
	flush()
	want := mgr.visuals[1]
	later.runtimeState.syncSprite.OnFramesSetChangedEvent.Subscribe(func() {
		sprite.Show()
		sprite.SetCostume__0("costume2")
	})
	later.runtimeState.isCostumeDirty = true
	flush()
	got := mgr.visuals[1]
	if !mgr.visible[1] || math.Abs(got.rotation-want.rotation) > 1e-6 || got.scale != want.scale || got.renderOffset != want.renderOffset {
		t.Fatalf("callback show reverted hidden geometry: got %+v, want %+v", got, want)
	}
	if !sprite.runtimeState.isCostumeDirty || sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
		t.Fatal("callback show acknowledged unprepared resources")
	}
}

func TestProxyPreparedResourcesRetryAfterFailure(t *testing.T) {
	for _, failure := range []string{"layer", "resource", "resource after query", "batch"} {
		t.Run(failure, func(t *testing.T) {
			sprite, later, mgr := setupPreparedResourceSync(t)
			game := sprite.g
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			sprite.SetCostume__0("costume1")
			sprite.SetXpos(42)
			if failure == "layer" {
				sprite.runtimeState.isLayerDirty = true
			}
			later.runtimeState.isCostumeDirty = true
			later.runtimeState.syncSprite.OnFramesSetChangedEvent.Subscribe(func() { sprite.SetCostume__0("costume2") })
			mgr.failAt = failure
			if failure == "resource after query" {
				mgr.failAt = "resource"
				sprite.runtimeState.syncSprite.OnFramesSetChangedEvent.Subscribe(sprite.syncProxyForQuery)
			}
			requireSyncPanic(t, func() { game.flushSpriteProxyChanges(game.shapeMgr.items) }, "resource", "failed")
			if sprite.spriteState.proxySyncVersion == sprite.spriteState.dirtyVersion {
				t.Fatal("failed submission acknowledged resource state")
			}
			if failure == "resource after query" {
				// The nested query submitted position successfully, but the outer
				// resource update still has to complete before confirming visuals.
				if mgr.positions[1] != mathf.NewVec2(42, 20) || sprite.transform().pendingPosition != (positionVersions{}) {
					t.Fatal("successful nested query did not acknowledge its position")
				}
			} else if sprite.transform().pendingPosition.x == 0 {
				t.Fatal("failed submission discarded position intent")
			}
			if mgr.failAt == "resource" && !sprite.runtimeState.isCostumeDirty {
				t.Fatal("failed resource preparation discarded the retry")
			}
			if failure == "layer" && !sprite.runtimeState.isLayerDirty {
				t.Fatal("failed layer preparation discarded the retry")
			}
			mgr.failAt = ""
			sprite.runtimeState.syncSprite.OnFramesSetChangedEvent.UnsubscribeAll()
			later.runtimeState.syncSprite.OnFramesSetChangedEvent.UnsubscribeAll()
			game.flushSpriteProxyChanges(game.shapeMgr.items)
			wantIndex := 2
			if failure != "batch" {
				wantIndex = 1
			}
			requirePreparedResource(t, sprite, mgr, wantIndex, 1.5)
			if sprite.runtimeState.isCostumeDirty || sprite.runtimeState.isLayerDirty || sprite.spriteState.proxySyncVersion != sprite.spriteState.dirtyVersion ||
				sprite.transform().pendingPosition != (positionVersions{}) {
				t.Fatal("successful retry did not finish resource and position synchronization")
			}
		})
	}
}

func (m *pullPositionSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	m.calls++
	m.ids = append(m.ids[:0], ids...)
	copy(out, m.positions)
	return true
}

func (m *reentrantPullPositionMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	m.calls++
	if m.calls == 1 {
		items := m.game.shapeMgr.items
		m.game.shapeMgr.items = []Shape{m.nested}
		m.game.pullPhysicsPositions()
		m.game.shapeMgr.items = items
	}
	for i, id := range ids {
		out[i*2], out[i*2+1] = float32(id*10), float32(id*10+1)
	}
	return true
}

func (m *concurrentPullPositionMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	if m.calls.Add(1) == 1 {
		close(m.started)
		<-m.release
	}
	out[0], out[1] = float32(ids[0]*10), float32(ids[0]*10+1)
	return true
}

func (m *positionSyncSpriteMgr) GetPosition(pkgengine.Object) mathf.Vec2 { return m.position }

func (*positionSyncSpriteMgr) SetZIndex(pkgengine.Object, int64) {}

func (m *positionSyncSpriteMgr) SetPosition(_ pkgengine.Object, pos mathf.Vec2) {
	m.submit(pos)
}

func (m *positionSyncSpriteMgr) SetTransform(_ pkgengine.Object, pos mathf.Vec2, _ float64, _ mathf.Vec2, _ bool, _ mathf.Vec2) {
	m.submit(pos)
}

func (m *positionSyncSpriteMgr) BatchUpdateTransforms(data []float32) {
	var pos mathf.Vec2
	if data[0] != 0 {
		pos = mathf.NewVec2(float64(data[3]), float64(data[4]))
	}
	m.submit(pos)
	m.captureFlushSpriteMgr.BatchUpdateTransforms(data)
}

func (m *positionSyncSpriteMgr) submit(pos mathf.Vec2) {
	if m.fail {
		panic("position sync failed")
	}
	m.position = pos
	m.submissions++
	if m.onSubmit != nil {
		m.onSubmit()
	}
}

func (s *framesChangedSprite) OnFramesSetChanged() { s.changed() }

func (m *textureFramesSpriteMgr) SetTexture(id pkgengine.Object, _ string) {
	// Godot SetTexture synchronously emits sprite_frames_changed. Dispatch it
	// through the public registry in the same order as gdengine's callback.
	m.callbacks++
	sprite := pkgengine.GetSprite(id)
	sprite.V_OnFramesSetChanged()
	sprite.OnFramesSetChanged()
}

func (*textureFramesSpriteMgr) SetRenderScale(pkgengine.Object, mathf.Vec2) {}

func (m *preparedResourceSpriteMgr) SetTexture(id pkgengine.Object, path string) {
	m.touchingSyncSpriteMgr.SetTexture(id, path)
	m.framesChanged(id)
}

func (m *preparedResourceSpriteMgr) SetTextureAtlas(id pkgengine.Object, path string, rect mathf.Rect2) {
	m.touchingSyncSpriteMgr.SetTextureAtlas(id, path, rect)
	m.framesChanged(id)
}

func (m *preparedResourceSpriteMgr) SetRenderScale(id pkgengine.Object, scale mathf.Vec2) {
	if m.failAt == "resource" {
		panic("resource preparation failed")
	}
	m.renderScales[id] = scale
}

func (m *preparedResourceSpriteMgr) SetZIndex(pkgengine.Object, int64) {
	if m.failAt == "layer" {
		panic("resource layer failed")
	}
}

func (m *preparedResourceSpriteMgr) SetMaterialParamsVec(id pkgengine.Object, name string, x, y, z, w float64) {
	if name == "atlas_uv_rect2" {
		m.uvRemaps[id] = mathf.NewVec4(x, y, z, w)
	}
}

func (m *preparedResourceSpriteMgr) SetTransform(id pkgengine.Object, pos mathf.Vec2, rotation float64, scale mathf.Vec2, visible bool, offset mathf.Vec2) {
	m.touchingSyncSpriteMgr.SetTransform(id, pos, rotation, scale, visible, offset)
	m.visuals[id] = proxyVisual{rotation: rotation, scale: scale, renderOffset: offset}
}

func (m *preparedResourceSpriteMgr) BatchUpdateTransforms(data []float32) {
	if m.failAt == "batch" {
		panic("resource batch failed")
	}
	for i := 0; i < int(data[0]); i++ {
		entry := data[2+i*engine.SyncFieldsPerSprite:]
		m.SetTransform(pkgengine.Object(entry[0]), mathf.NewVec2(float64(entry[1]), float64(entry[2])),
			float64(entry[3]), mathf.NewVec2(float64(entry[4]), float64(entry[5])), entry[8] != 0,
			mathf.NewVec2(float64(entry[6]), float64(entry[7])))
	}
}

func (*preparedResourceSpriteMgr) framesChanged(id pkgengine.Object) {
	sprite := pkgengine.GetSprite(id)
	sprite.V_OnFramesSetChanged()
	sprite.OnFramesSetChanged()
}

func setupPullPositionSpriteMgr(t *testing.T) *pullPositionSpriteMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	original := pkgengine.SpriteMgr
	mgr := &pullPositionSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	return mgr
}

func newPullPhysicsSprite(id int64, mode PhysicsMode, x, y float64) *SpriteImpl {
	sprite := newPhysicsPositionTestSprite(x, y)
	sprite.runtimeState.syncSprite = &engine.Sprite{}
	sprite.runtimeState.syncSprite.Id = id
	sprite.components.physics = &physicsComponent{sprite: sprite, physicsMode: mode}
	return sprite
}

func setupSyncBufferCapture(t *testing.T) (*Game, *captureFlushSpriteMgr) {
	t.Helper()
	mgr := setupCaptureFlushSpriteMgr(t)
	return &Game{syncBuffer: engine.NewSpriteSyncBuffer(1)}, mgr
}

func newPhysicsPositionTestSprite(x, y float64) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.components.transform = &transformComponent{
		sprite: sprite,
		x:      x,
		y:      y,
	}
	return sprite
}

func newPhysicsTriggerTestGame(t *testing.T) *Game {
	t.Helper()
	previous := gco
	gco = nil
	t.Cleanup(func() { gco = previous })
	game := new(Game)
	game.bindScriptEvents()
	return game
}

func newPhysicsTriggerTestSprite(game *Game, name string) *touchEventSprite {
	sprite := newTouchEventSprite(game, name)
	sprite.spriteState.isVisible = true
	sprite.runtimeState.syncSprite = &engine.Sprite{Target: &sprite.SpriteImpl, MirrorPosition: sprite.mirrorPosition}
	return sprite
}

func setupPositionSyncSprite(t *testing.T) (*SpriteImpl, *positionSyncSpriteMgr) {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := pkgengine.SpriteMgr
	mgr := &positionSyncSpriteMgr{position: mathf.NewVec2(10, 20)}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = previous })
	game := &Game{syncBuffer: engine.NewSpriteSyncBuffer(1)}
	game.initShapeMgr()
	game.camera = &cameraImpl{g: game}
	sprite := &newCloneAwakeOrderSprite(game, "position").SpriteImpl
	sprite.transform().x, sprite.transform().y = 10, 20
	sprite.runtimeState.isCostumeDirty = false
	sprite.runtimeState.syncSprite = &engine.Sprite{Target: sprite, MirrorPosition: sprite.mirrorPosition}
	sprite.runtimeState.syncSprite.Id = 1
	game.shapeMgr.add(sprite)
	return sprite, mgr
}

func requireSyncPanic(t *testing.T, sync func(), want ...string) {
	t.Helper()
	defer func() {
		value := recover()
		if value == nil {
			t.Fatal("invalid sync unexpectedly succeeded")
		}
		message := fmt.Sprint(value)
		for _, part := range want {
			if !strings.Contains(message, part) {
				t.Fatalf("sync panic = %q, want diagnostic containing %q", message, part)
			}
		}
	}()
	sync()
}

func requirePosition(t *testing.T, sprite *SpriteImpl, x, y float64) {
	t.Helper()
	if gotX, gotY := sprite.getXY(); gotX != x || gotY != y {
		t.Fatalf("position = (%v, %v), want (%v, %v)", gotX, gotY, x, y)
	}
}

func batchDeleteIDs(batch []float32) []float32 {
	return batch[2+int(batch[0])*engine.SyncFieldsPerSprite:]
}

func setupPreparedResourceSync(t *testing.T) (*SpriteImpl, *SpriteImpl, *preparedResourceSpriteMgr) {
	t.Helper()
	sprite, _ := setupPositionSyncSprite(t)
	mgr := &preparedResourceSpriteMgr{
		touchingSyncSpriteMgr: newTouchingSyncSpriteMgr(),
		renderScales:          make(map[pkgengine.Object]mathf.Vec2),
		visuals:               make(map[pkgengine.Object]proxyVisual),
		uvRemaps:              make(map[pkgengine.Object]mathf.Vec4),
	}
	pkgengine.SpriteMgr = mgr
	sprite.costumes = nil
	for i, center := range []mathf.Vec2{mathf.NewVec2(10, 10), mathf.NewVec2(14, 6), mathf.NewVec2(6, 16)} {
		c := newCostumeWithSize(20, 20)
		c.name, c.path = fmt.Sprintf("costume%d", i), fmt.Sprintf("costume%d.png", i)
		c.frame.Center, c.faceRight, c.frame.BitmapResolution = center, float64(i-1)*90, 1<<i
		sprite.costumes = append(sprite.costumes, c)
	}
	sprite.costumeIndex = 0
	sprite.runtimeState.scale = 1.5
	sprite.runtimeState.isCostumeDirty = true
	sprite.transform().direction = 90
	sprite.transform().rotationStyle = Normal
	// A configured pivot must stay paired with the costume anchor and size.
	sprite.transform().pivot = mathf.NewVec2(1, -2)
	sprite.markProxyDirty()
	later := &newCloneAwakeOrderSprite(sprite.g, "later").SpriteImpl
	later.currentCostume().path = "later.png"
	later.runtimeState.isCostumeDirty = false
	later.runtimeState.syncSprite = &engine.Sprite{Target: later, MirrorPosition: later.mirrorPosition}
	sprite.g.shapeMgr.add(later)
	for i, s := range []*SpriteImpl{sprite, later} {
		id := pkgengine.Object(i + 1)
		registry := pkgengine.Sprites()
		previous, existed := registry[id]
		t.Cleanup(func() {
			if existed {
				registry[id] = previous
			} else {
				delete(registry, id)
			}
		})
		pkgengine.InitSpriteInstance(id, s.runtimeState.syncSprite, func(id pkgengine.Object, proxy pkgengine.ISpriter) {
			registry[id] = proxy
		})
	}
	return sprite, later, mgr
}

func requirePreparedResource(t *testing.T, sprite *SpriteImpl, mgr *preparedResourceSpriteMgr, index int, size float64) {
	t.Helper()
	id := sprite.runtimeState.syncSprite.Id
	c := sprite.costumes[index]
	if got, want := mgr.texturePaths[id], engine.ToAssetPath(c.path); got != want {
		t.Fatalf("texture = %q, want %q", got, want)
	}
	if got, want := mgr.renderScales[id], engine.UniformVec2(size/float64(c.frame.BitmapResolution)); got != want {
		t.Fatalf("resource scale = %v, want %v", got, want)
	}
	offsets := []mathf.Vec2{mathf.NewVec2(-1, 2), mathf.NewVec2(-3, 0), mathf.NewVec2(0, 3.5)}
	wantOffset := offsets[index].Mulf(size)
	visual := mgr.visuals[id]
	if math.Abs(visual.rotation-float64(index-1)*math.Pi/2) > 1e-6 ||
		visual.renderOffset != wantOffset || visual.scale != mathf.NewVec2(1, 1) {
		t.Fatalf("geometry = %+v, want costume %d with offset %v", visual, index, wantOffset)
	}
	if c.isAtlas() && mgr.uvRemaps[id] != c.frame.AtlasUVRect {
		t.Fatalf("atlas UV = %v, want %v", mgr.uvRemaps[id], c.frame.AtlasUVRect)
	}
}
