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
	"sync/atomic"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type pullPositionSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	calls     int
	ids       []int64
	positions []float32
}

func (m *pullPositionSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	m.calls++
	m.ids = append(m.ids[:0], ids...)
	copy(out, m.positions)
	return true
}

func setupPullPositionSpriteMgr(t *testing.T) *pullPositionSpriteMgr {
	t.Helper()
	enginewrap.Init(func(call func()) { call() })
	original := pkgengine.SpriteMgr
	mgr := &pullPositionSpriteMgr{}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() { pkgengine.SpriteMgr = original })
	return mgr
}

func newPullPhysicsSprite(id int64, mode PhysicsMode, x, y float64) *SpriteImpl {
	sprite := newPhysicsPositionTestSprite(x, y)
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.runtimeState.SyncSprite.Id = id
	sprite.components.physics = &physicsComponent{sprite: sprite, physicsMode: mode}
	return sprite
}

func setupSyncBufferCapture(t *testing.T) (*Game, *captureFlushSpriteMgr) {
	t.Helper()
	mgr := setupCaptureFlushSpriteMgr(t)
	return &Game{syncBuffer: engine.NewSpriteSyncBuffer(1)}, mgr
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

	defer func() {
		if recover() == nil {
			t.Fatal("invalid sprite ID did not panic during serialization")
		}
		if len(mgr.batches) != 0 {
			t.Fatalf("serialization failure submitted %d batches, want 0", len(mgr.batches))
		}
	}()
	game.flushSyncBuffer()
}

func TestProxyTransformQueryAndBatchMatch(t *testing.T) {
	spy := setupSpyPenMgr(t)
	sprite := newPenTestSprite()
	configurePenRenderOffsetSprite(sprite)
	sprite.spriteState.IsVisible = true
	sprite.transform().direction = 45
	sprite.transform().rotationStyle = Normal

	sprite.ensureProxyQueryStateSynced()
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
		sprite.collectProxyUpdate(buffer)
		if got := buffer.UpdateCount(); got != want {
			t.Fatalf("collected %d transforms, want %d", got, want)
		}
	}

	collect(0)
	sprite.initRuntimeProxy()
	collect(1)
	collect(0)
	oldProxy := sprite.runtimeState.SyncSprite
	sprite.initRuntimeProxy()
	if sprite.runtimeState.SyncSprite == oldProxy {
		t.Fatal("rebuild reused the old proxy")
	}
	collect(1)
	collect(0)

	sprite.Show()
	sprite.destroy()
	operations := len(mgr.recordedOperations())
	sprite.ensureProxyQueryStateSynced()
	collect(0)
	if got := len(mgr.recordedOperations()); got != operations {
		t.Fatalf("destroyed sprite made %d proxy calls", got-operations)
	}
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

func TestPullPhysicsPositionsFiltersAndKeepsIDOrder(t *testing.T) {
	mgr := setupPullPositionSpriteMgr(t)
	mgr.positions = []float32{11, 12, 31, 32}
	first := newPullPhysicsSprite(1, KinematicPhysics, 1, 2)
	noPhysics := newPullPhysicsSprite(2, NoPhysics, 2, 3)
	missing := newPullPhysicsSprite(4, DynamicPhysics, 4, 5)
	missing.runtimeState.SyncSprite = nil
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
		if x != tt.x || y != tt.y || tt.sprite.spriteState.VisualVersion != tt.version {
			t.Errorf("sprite position=(%v,%v) visual version=%d, want (%v,%v) version %d", x, y, tt.sprite.spriteState.VisualVersion, tt.x, tt.y, tt.version)
		}
		if tt.sprite.spriteState.DirtyVersion != 0 {
			t.Errorf("physics pull dirtied proxy: %+v", tt.sprite.spriteState)
		}
	}
	game.pullPhysicsPositions()
	if first.spriteState.VisualVersion != 1 || last.spriteState.VisualVersion != 1 {
		t.Fatal("unchanged position incremented visual version")
	}
	mgr.positions = []float32{13, 14, float32(math.NaN()), 40}
	game.pullPhysicsPositions()
	if x, y := last.transform().getXY(); x != 31 || y != 32 || last.spriteState.VisualVersion != 1 {
		t.Fatalf("missing sentinel changed last sprite to (%v,%v), version %d", x, y, last.spriteState.VisualVersion)
	}
	if x, y := first.transform().getXY(); x != 13 || y != 14 || first.spriteState.VisualVersion != 2 {
		t.Fatalf("valid result not applied to first sprite: (%v,%v), version %d", x, y, first.spriteState.VisualVersion)
	}
	game.shapeMgr.items = []Shape{noPhysics, struct{}{}, missing}
	game.pullPhysicsPositions()
	if mgr.calls != 3 {
		t.Fatalf("empty physics pull made an engine query: %d total calls, want 3", mgr.calls)
	}
}

type reentrantPullPositionMgr struct {
	enginewrap.SpriteMgrImpl
	game   *Game
	nested *SpriteImpl
	calls  int
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

type concurrentPullPositionMgr struct {
	enginewrap.SpriteMgrImpl
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (m *concurrentPullPositionMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	if m.calls.Add(1) == 1 {
		close(m.started)
		<-m.release
	}
	out[0], out[1] = float32(ids[0]*10), float32(ids[0]*10+1)
	return true
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
	enginewrap.Init(func(call func()) { call() })
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

func TestApplyPhysicsPositionInvalidatesVisualsOnlyWhenChanged(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)

	sprite.applyPhysicsPosition(1, 2)
	if sprite.spriteState.VisualVersion != 0 {
		t.Fatalf("unchanged position visual version = %d, want 0", sprite.spriteState.VisualVersion)
	}

	sprite.applyPhysicsPosition(3, 4)
	if sprite.spriteState.VisualVersion != 1 {
		t.Fatalf("changed position visual version = %d, want 1", sprite.spriteState.VisualVersion)
	}
	if sprite.spriteState.DirtyVersion != 0 {
		t.Fatalf("physics writeback dirtied proxy state: DirtyVersion=%d", sprite.spriteState.DirtyVersion)
	}
	if x, y := sprite.transform().getXY(); x != 3 || y != 4 {
		t.Fatalf("physics position = (%v, %v), want (3, 4)", x, y)
	}

	sprite.applyPhysicsPosition(3, 4)
	if sprite.spriteState.VisualVersion != 1 {
		t.Fatalf("repeated position visual version = %d, want 1", sprite.spriteState.VisualVersion)
	}
}

func TestCameraFollowObservesPhysicsPositionChanges(t *testing.T) {
	sprite := newPhysicsPositionTestSprite(1, 2)
	camera := &cameraImpl{
		followTarget:          sprite,
		observedFollowVersion: sprite.spriteState.VisualVersion,
	}

	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("unchanged follow target unexpectedly invalidated the camera")
	}

	sprite.applyPhysicsPosition(3, 4)
	changed, pos := camera.getFollowPos()
	if !changed {
		t.Fatal("physics position change did not invalidate the camera")
	}
	if pos.X != 3 || pos.Y != 4 {
		t.Fatalf("camera follow position = (%v, %v), want (3, 4)", pos.X, pos.Y)
	}

	camera.observedFollowVersion = sprite.spriteState.VisualVersion
	if changed, _ := camera.getFollowPos(); changed {
		t.Fatal("observed follow target still invalidated the camera")
	}
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
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &engine.Sprite{Target: &sprite.SpriteImpl}
	return sprite
}

func TestDispatchPhysicsTriggersFiltersPairs(t *testing.T) {
	game := newPhysicsTriggerTestGame(t)
	source := newPhysicsTriggerTestSprite(game, "source")
	target := newPhysicsTriggerTestSprite(game, "target")
	hidden := newPhysicsTriggerTestSprite(game, "hidden")
	hidden.spriteState.IsVisible = false
	dying := newPhysicsTriggerTestSprite(game, "dying")
	dying.spriteState.IsDying = true
	noHandler := newPhysicsTriggerTestSprite(game, "no-handler")
	var nilSprite *SpriteImpl
	typedNil := &engine.Sprite{Target: nilSprite}
	wrongType := &engine.Sprite{Target: "not a sprite"}
	emptyTarget := &engine.Sprite{}
	src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite

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
		{"hidden source and invalid destination", hidden.runtimeState.SyncSprite, wrongType, false},
		{"hidden source", hidden.runtimeState.SyncSprite, dst, false},
		{"hidden destination", src, hidden.runtimeState.SyncSprite, false},
		{"dying source", dying.runtimeState.SyncSprite, dst, false},
		{"dying destination", src, dying.runtimeState.SyncSprite, false},
		{"hidden source short circuits typed nil destination", hidden.runtimeState.SyncSprite, typedNil, false},
		{"dying source short circuits typed nil destination", dying.runtimeState.SyncSprite, typedNil, false},
		{"no source handler", noHandler.runtimeState.SyncSprite, dst, false},
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
	firstPair := engine.TriggerEvent{Src: first.runtimeState.SyncSprite, Dst: second.runtimeState.SyncSprite}
	backing := make([]engine.TriggerEvent, 4)
	backing[0] = firstPair
	backing[1] = engine.TriggerEvent{Src: second.runtimeState.SyncSprite, Dst: last.runtimeState.SyncSprite}
	backing[2] = engine.TriggerEvent{Src: first.runtimeState.SyncSprite, Dst: last.runtimeState.SyncSprite}
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
					target.spriteState.IsDying = true
				} else {
					target.spriteState.IsVisible = false
				}
			})
			target.addTouchStartHandler(func(Sprite) { t.Error("untouchable source fired a callback") })
			src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite
			game.triggerEvents = []engine.TriggerEvent{
				{Src: src, Dst: dst},
				{Src: dst, Dst: src},
				{Src: src, Dst: dst},
				{Src: src, Dst: last.runtimeState.SyncSprite},
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
			hidden.spriteState.IsVisible = false
			var nilSprite *SpriteImpl
			typedNil := &engine.Sprite{Target: nilSprite}
			calls := 0
			source.addTouchStartHandler(func(Sprite) { calls++ })
			panicValue := new(int)
			panicking.addTouchStartHandler(func(Sprite) { panic(panicValue) })
			src, dst := source.runtimeState.SyncSprite, target.runtimeState.SyncSprite
			pair := engine.TriggerEvent{Src: panicking.runtimeState.SyncSprite, Dst: dst}
			switch name {
			case "typed nil source":
				pair.Src = typedNil
			case "typed nil destination":
				pair.Src, pair.Dst = src, typedNil
			case "typed nil source before hidden destination":
				pair.Src, pair.Dst = typedNil, hidden.runtimeState.SyncSprite
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
