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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type cloneLifecycleScenario struct {
	mu                       sync.Mutex
	clone                    *cloneLifecycleTestSprite
	mainAction               func(*cloneLifecycleTestSprite)
	onClonedAction           func(*cloneLifecycleTestSprite)
	registerRollbackHandlers bool
	abortAllWhileMatching    bool
}

func (s *cloneLifecycleScenario) capture(clone *cloneLifecycleTestSprite) {
	s.mu.Lock()
	s.clone = clone
	s.mu.Unlock()
}

func (s *cloneLifecycleScenario) captured() *cloneLifecycleTestSprite {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clone
}

type cloneLifecycleTestSprite struct {
	SpriteImpl
	scenario *cloneLifecycleScenario
}

type panicVisibleCloneSpriteMgr struct {
	*spyCloneSpriteMgr
	panicNextVisible atomic.Bool
	panicValue       any
}

func (s *panicVisibleCloneSpriteMgr) SetTransform(
	obj pkgengine.Object,
	pos mathf.Vec2,
	rot float64,
	scale mathf.Vec2,
	visible bool,
	pivot mathf.Vec2,
) {
	s.spyCloneSpriteMgr.SetTransform(obj, pos, rot, scale, visible, pivot)
	if visible && s.panicNextVisible.CompareAndSwap(true, false) {
		panic(s.panicValue)
	}
}

func (s *cloneLifecycleTestSprite) Main() {
	if !s.IsCloned() {
		return
	}
	s.scenario.capture(s)
	if s.scenario.registerRollbackHandlers {
		s.OnMsg__1("rollback-probe", func() {})
		s.OnCloned__0(func() {})
	}
	if action := s.scenario.onClonedAction; action != nil {
		if s.scenario.abortAllWhileMatching {
			s.spriteState.HasOnCloned = true
			s.scriptEventRegistry.manager.AddCloned(coreevent.NewSink(
				&s.SpriteImpl,
				func(any) { action(s) },
				func(data any) bool {
					gco.AbortAll()
					return data == &s.SpriteImpl
				},
			))
		} else {
			s.OnCloned__0(func() {
				action(s)
			})
		}
	}
	if action := s.scenario.mainAction; action != nil {
		action(s)
	}
}

func newCloneLifecycleTestSprite(
	g *Game, scenario *cloneLifecycleScenario, visible bool,
) *cloneLifecycleTestSprite {
	sprite := &cloneLifecycleTestSprite{scenario: scenario}
	placeholder := newCostumeWithSize(1, 1)
	placeholder.name, placeholder.path = "placeholder", "placeholder.png"
	final := newCostumeWithSize(1, 1)
	final.name, final.path = "final", "final.png"
	sprite.costumes = []*costume{placeholder, final}
	sprite.costumeIndex = 0
	sprite.runtimeState.Scale = 1
	sprite.spriteState.IsVisible = visible
	sprite.g = g
	sprite.name = "CloneLifecycle"
	sprite.sprite = sprite
	sprite.scriptEventBindings.init(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func setupCloneLifecycleTest(
	t *testing.T,
) (*coroutine.Coroutines, *Game, *spyCloneSpriteMgr) {
	t.Helper()
	// Register the engine-manager cleanup before the scheduler cleanup. Tests
	// deliberately abort clone handlers, so the scheduler must drain those
	// goroutines before the process-wide engine managers are restored.
	mgr := setupCloneSpriteMgr(t)
	game := new(Game)
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	game.scriptEventBindings.init(&game.scriptEvents, game)
	originalGame := engine.GetGame()
	engine.SetGame(game)
	t.Cleanup(func() { engine.SetGame(originalGame) })
	co := setupRuntimeEventScheduler(t)
	return co, game, mgr
}

func waitForCloneLifecycle(
	t *testing.T, co *coroutine.Coroutines, condition func() bool,
) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for clone lifecycle")
		}
		co.Update()
		time.Sleep(time.Millisecond)
	}
}

func waitForCloneSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal(failure)
	}
}

func firstCloneOperation(
	operations []cloneProxyOperation,
	object pkgengine.Object,
	match func(cloneProxyOperation) bool,
) int {
	for i, operation := range operations {
		if operation.object == object && match(operation) {
			return i
		}
	}
	return -1
}

func lastCloneOperation(
	operations []cloneProxyOperation,
	object pkgengine.Object,
	match func(cloneProxyOperation) bool,
) int {
	for i := len(operations) - 1; i >= 0; i-- {
		if operations[i].object == object && match(operations[i]) {
			return i
		}
	}
	return -1
}

func configureCloneTestPhysics(sprite *cloneLifecycleTestSprite) {
	physics := sprite.physics()
	physics.collisionInfo.Type = physicsColliderRect
	physics.collisionInfo.Params = []float64{2, 2}
	physics.triggerInfo.Type = physicsColliderRect
	physics.triggerInfo.Params = []float64{2, 2}
	physics.collisionEnabled = true
	physics.triggerEnabled = true
	physics.physicsMode = DynamicPhysics
}

func TestColliderDesiredEnabledStateIsIndependentOfShape(t *testing.T) {
	_, game, _ := setupCloneLifecycleTest(t)
	sprite := newCloneLifecycleTestSprite(game, new(cloneLifecycleScenario), true)
	for _, isTrigger := range []bool{false, true} {
		if err := sprite.SetColliderShape(
			isTrigger, ColliderShapeType(physicsColliderNone), nil,
		); err != nil {
			t.Fatalf("SetColliderShape(trigger=%v): %v", isTrigger, err)
		}
		if isTrigger {
			sprite.SetTriggerEnabled(true)
			if !sprite.TriggerEnabled() {
				t.Fatal("trigger desired state was lost for a disabled shape")
			}
		} else {
			sprite.SetCollisionEnabled(true)
			if !sprite.CollisionEnabled() {
				t.Fatal("collision desired state was lost for a disabled shape")
			}
		}
	}
}

func assertCloneProxyHiddenBeforeTexture(
	t *testing.T, mgr *spyCloneSpriteMgr, object pkgengine.Object,
) {
	t.Helper()
	operations := mgr.recordedOperations()
	hidden := firstCloneOperation(operations, object, func(operation cloneProxyOperation) bool {
		return operation.kind == "visible" && !operation.visible
	})
	texture := firstCloneOperation(operations, object, func(operation cloneProxyOperation) bool {
		return operation.kind == "texture"
	})
	if hidden < 0 || texture < 0 || hidden >= texture {
		t.Fatalf("clone was not explicitly hidden before its first texture: operations = %#v", operations)
	}
	for _, operation := range operations[:texture] {
		if operation.object == object &&
			(operation.kind == "visible" || operation.kind == "transform") && operation.visible {
			t.Fatalf("clone became visible before its first texture: operations = %#v", operations)
		}
	}
}

func flushCloneProxyDestroys(game *Game) {
	game.syncBuffer.Clear()
	game.shapeMgr.flushDestroy(game.syncBuffer)
	game.flushSyncBuffer()
}

func TestCloneBareProxyIsHiddenBeforeFirstTexture(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	scenario := new(cloneLifecycleScenario)
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.runtimeState.SyncSprite == nil {
		t.Fatal("clone proxy was not created")
	}
	assertCloneProxyHiddenBeforeTexture(t, mgr, clone.runtimeState.SyncSprite.GetId())
}

func TestCloneWithoutOnClonedClearsPublicationGate(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	scenario := new(cloneLifecycleScenario)
	source := newCloneLifecycleTestSprite(game, scenario, false)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil {
		t.Fatal("clone Main did not run")
	}
	if clone.spriteState.IsProxyPublicationPending {
		t.Fatal("clone without onCloned remained publication-pending")
	}
	clone.Show()
	clone.ensureProxyQueryStateSynced()
	operations := mgr.recordedOperations()
	cloneID := clone.runtimeState.SyncSprite.GetId()
	if firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	}) < 0 {
		t.Fatalf("published hidden clone could not subsequently be shown: operations = %#v", operations)
	}
}

func TestClonePublishesAfterFirstOnClonedSlice(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	release := make(chan struct{})
	defer close(release)
	handlerReturned := make(chan struct{})
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			clone.SetCostume__0("final")
			var signal struct{}
			engine.WaitForChan(release, &signal)
			close(handlerReturned)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.spriteState.IsProxyPublicationPending {
		t.Fatal("clone was not published after the handler's first slice")
	}
	select {
	case <-handlerReturned:
		t.Fatal("clone publication waited for the entire handler")
	default:
	}
	operations := mgr.recordedOperations()
	cloneID := clone.runtimeState.SyncSprite.GetId()
	finalTexture := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "texture" && strings.HasSuffix(operation.path, "final.png")
	})
	visible := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	})
	if finalTexture < 0 || visible <= finalTexture {
		t.Fatalf("clone publication did not follow first-slice initialization: operations = %#v", operations)
	}
}

func TestDyingClonePublishesBeforeDeathAnimationCompletes(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	release := make(chan struct{})
	defer close(release)
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			// Die marks the sprite before AnimateAndWait reaches its first yield.
			// Model that first slice without depending on a concrete animation.
			clone.setDying()
			var signal struct{}
			engine.WaitForChan(release, &signal)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.isDestroyed() {
		t.Fatal("dying clone was rolled back before its death animation completed")
	}
	if clone.spriteState.IsProxyPublicationPending {
		t.Fatal("dying clone remained publication-pending after its first slice")
	}
	cloneID := clone.runtimeState.SyncSprite.GetId()
	if firstCloneOperation(mgr.recordedOperations(), cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	}) < 0 {
		t.Fatalf("dying clone was not published for its death animation: operations = %#v", mgr.recordedOperations())
	}
}

func TestCloneStopAllOtherScriptsStillPublishes(t *testing.T) {
	co, game, mgr := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			clone.SetCostume__0("final")
			clone.Stop(AllOtherScripts)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	creatorReturned := make(chan struct{})
	gco.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, false, nil)
		close(creatorReturned)
		return 0
	})

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && !clone.spriteState.IsProxyPublicationPending
	})

	clone := scenario.captured()
	if clone.isDestroyed() {
		t.Fatal("stop other scripts destroyed the initialized clone")
	}
	select {
	case <-creatorReturned:
		t.Fatal("stop other scripts did not cancel the source creator")
	default:
	}
	operations := mgr.recordedOperations()
	cloneID := clone.runtimeState.SyncSprite.GetId()
	if firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	}) < 0 {
		t.Fatalf("clone was not published after creator cancellation: operations = %#v", operations)
	}
}

func TestCloneStopAllRollsBackPendingClone(t *testing.T) {
	co, game, mgr := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			clone.SetCostume__0("final")
			clone.Stop(AllStop)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	gco.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, false, nil)
		return 0
	})

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && clone.isDestroyed() && len(game.getAllShapes()) == 1
	})

	clone := scenario.captured()
	cloneID := clone.runtimeState.SyncSprite.GetId()
	for _, operation := range mgr.recordedOperations() {
		if operation.object == cloneID &&
			(operation.kind == "visible" || operation.kind == "transform") && operation.visible {
			t.Fatalf("stop all exposed the pending clone: operations = %#v", mgr.recordedOperations())
		}
	}
	flushCloneProxyDestroys(game)
	if clone.runtimeState.SyncSprite != nil {
		t.Fatal("rolled-back clone proxy was not released")
	}
	if firstCloneOperation(mgr.recordedOperations(), cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "destroy"
	}) < 0 {
		t.Fatalf("rolled-back clone proxy was not destroyed: operations = %#v", mgr.recordedOperations())
	}
}

func TestCloneStopAllBeforeLifecycleStartSkipsHandlers(t *testing.T) {
	co, game, _ := setupCloneLifecycleTest(t)
	var handlerCalls atomic.Int32
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(*cloneLifecycleTestSprite) {
			handlerCalls.Add(1)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	gco.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, true, nil)
		// The detached lifecycle is registered but cannot start while this
		// creator still owns runMu.
		source.Stop(AllStop)
		return 0
	})

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && clone.isDestroyed() && len(game.getAllShapes()) == 1
	})
	if got := handlerCalls.Load(); got != 0 {
		t.Fatalf("stale clone handlers ran after Stop All: calls = %d", got)
	}
}

func TestCloneLifecycleCanceledBeforeStartRollsBack(t *testing.T) {
	co, game, _ := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{registerRollbackHandlers: true}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	gco.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, true, nil)
		// Cancel both creator and lifecycle before the latter enters its body.
		gco.AbortAll()
		return 0
	})

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && clone.isDestroyed() && len(game.getAllShapes()) == 1
	})
	clone := scenario.captured()
	for _, sink := range append(
		game.scriptEvents.manager.SnapshotCloned(),
		game.scriptEvents.manager.SnapshotIReceive()...,
	) {
		if sink.Owner == spriteOf(clone) {
			t.Fatal("pre-start cancellation left registered clone handlers")
		}
	}
}

func TestAbortAllBeforeCloneHandlerRegistrationRejectsLateChild(t *testing.T) {
	co, game, _ := setupCloneLifecycleTest(t)
	var handlerCalls atomic.Int32
	scenario := &cloneLifecycleScenario{
		abortAllWhileMatching: true,
		onClonedAction: func(*cloneLifecycleTestSprite) {
			handlerCalls.Add(1)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	co.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, false, nil)
		return 0
	})

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && clone.isDestroyed() && len(game.getAllShapes()) == 1
	})
	if got := handlerCalls.Load(); got != 0 {
		t.Fatalf("post-AbortAll clone child ran user code: calls = %d", got)
	}
	// AbortAllAndWait must not time out on the child registered after the
	// original AbortAll snapshot.
	if !co.AbortAllAndWait(time.Second) {
		t.Fatal("late clone child escaped the shutdown barrier")
	}
}

func TestAbortAllBeforeCloneLifecycleRegistrationRollsBack(t *testing.T) {
	co, game, _ := setupCloneLifecycleTest(t)
	mainEntered := make(chan struct{})
	releaseMain := make(chan struct{})
	var handlerCalls atomic.Int32
	var continued atomic.Bool
	scenario := &cloneLifecycleScenario{
		mainAction: func(*cloneLifecycleTestSprite) {
			close(mainEntered)
			<-releaseMain
		},
		onClonedAction: func(*cloneLifecycleTestSprite) {
			handlerCalls.Add(1)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	co.Create(spriteOf(source), func(coroutine.Thread) int {
		doClone(source, nil, true, nil)
		continued.Store(true)
		return 0
	})

	waitForCloneSignal(t, mainEntered, "clone Main did not start")
	// At this point AbortAll's registry snapshot contains the creator but the
	// detached clone lifecycle has not been registered yet.
	co.AbortAll()
	close(releaseMain)

	waitForCloneLifecycle(t, co, func() bool {
		clone := scenario.captured()
		return clone != nil && clone.isDestroyed() && len(game.getAllShapes()) == 1
	})
	if got := handlerCalls.Load(); got != 0 {
		t.Fatalf("clone handler escaped AbortAll registration barrier: calls = %d", got)
	}
	if continued.Load() {
		t.Fatal("canceled creator continued after the Clone block")
	}
}

func TestCanceledCreatorCannotStartCloneLifecycle(t *testing.T) {
	co, game, _ := setupCloneLifecycleTest(t)
	creatorEntered := make(chan struct{})
	releaseCreator := make(chan struct{})
	creatorFinished := make(chan struct{})
	var continued atomic.Bool
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(*cloneLifecycleTestSprite) {
			t.Fatal("clone handler ran from an already-canceled creator")
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))
	co.Create(spriteOf(source), func(coroutine.Thread) int {
		defer close(creatorFinished)
		close(creatorEntered)
		<-releaseCreator
		doClone(source, nil, true, nil)
		continued.Store(true)
		return 0
	})

	waitForCloneSignal(t, creatorEntered, "creator did not start")
	co.AbortAll()
	close(releaseCreator)
	waitForCloneSignal(t, creatorFinished, "canceled creator did not stop")

	if continued.Load() {
		t.Fatal("canceled creator continued after Clone")
	}
	if clone := scenario.captured(); clone != nil {
		t.Fatal("already-canceled creator initialized a clone candidate")
	}
	if shapes := game.getAllShapes(); len(shapes) != 1 || shapes[0] != spriteOf(source) {
		t.Fatalf("already-canceled creator changed active shapes: %v", shapes)
	}
}

func TestNestedCloneWithoutHatDoesNotYieldParentHandler(t *testing.T) {
	_, game, _ := setupCloneLifecycleTest(t)
	var parentPublishedEarly atomic.Bool
	scenario := new(cloneLifecycleScenario)
	scenario.onClonedAction = func(parent *cloneLifecycleTestSprite) {
		// Nested Main sees no clone hat, while this already-registered parent
		// handler keeps its captured action.
		scenario.onClonedAction = nil
		parent.Clone__0()
		if !parent.spriteState.IsProxyPublicationPending {
			parentPublishedEarly.Store(true)
		}
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	if parentPublishedEarly.Load() {
		t.Fatal("nested clone without a hat yielded and published its parent early")
	}
}

func TestCloneMainFailureRollsBackBeforeInsertion(t *testing.T) {
	for _, test := range []struct {
		name   string
		panic  any
		action func(*cloneLifecycleTestSprite)
	}{
		{name: "abort", panic: coroutine.ErrAbortThread, action: func(*cloneLifecycleTestSprite) { gco.Abort() }},
		{name: "panic", panic: "main failed", action: func(*cloneLifecycleTestSprite) { panic("main failed") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, game, mgr := setupCloneLifecycleTest(t)
			scenario := &cloneLifecycleScenario{
				mainAction:               test.action,
				registerRollbackHandlers: true,
			}
			source := newCloneLifecycleTestSprite(game, scenario, true)
			game.addShape(spriteOf(source))

			var recovered any
			func() {
				defer func() { recovered = recover() }()
				doClone(source, nil, false, nil)
			}()
			if recovered != test.panic {
				t.Fatalf("recovered panic = %v, want %v", recovered, test.panic)
			}

			clone := scenario.captured()
			if clone == nil || !clone.isDestroyed() {
				t.Fatal("failed clone Main did not roll back the candidate")
			}
			if len(game.getAllShapes()) != 1 || game.getAllShapes()[0] != spriteOf(source) {
				t.Fatalf("failed clone was inserted: shapes = %v", game.getAllShapes())
			}
			for _, sink := range append(
				game.scriptEvents.manager.SnapshotCloned(),
				game.scriptEvents.manager.SnapshotIReceive()...,
			) {
				if sink.Owner == spriteOf(clone) {
					t.Fatal("failed clone left registered event handlers")
				}
			}
			cloneID := clone.runtimeState.SyncSprite.GetId()
			flushCloneProxyDestroys(game)
			if clone.runtimeState.SyncSprite != nil {
				t.Fatal("failed clone proxy was not released")
			}
			if firstCloneOperation(mgr.recordedOperations(), cloneID, func(operation cloneProxyOperation) bool {
				return operation.kind == "destroy"
			}) < 0 {
				t.Fatalf("failed clone proxy was not destroyed: operations = %#v", mgr.recordedOperations())
			}
		})
	}
}

func TestStaleGenerationRollbackRemovesLateCloneHandlers(t *testing.T) {
	_, game, _ := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{
		mainAction: func(clone *cloneLifecycleTestSprite) {
			// Model Game.reset's ordering: generation and registry reset can happen
			// before a synchronous clone Main finishes registering handlers.
			game.resetBootstrapState()
			game.scriptEvents.manager.Reset()
			clone.OnMsg__1("stale-generation", func() {})
			clone.OnCloned__0(func() {})
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || !clone.isDestroyed() {
		t.Fatal("stale-generation clone was not rolled back")
	}
	if clone.runtimeState.SyncSprite != nil {
		t.Fatal("stale-generation clone retained its discarded proxy")
	}
	for _, sink := range append(
		game.scriptEvents.manager.SnapshotCloned(),
		game.scriptEvents.manager.SnapshotIReceive()...,
	) {
		if sink.Owner == spriteOf(clone) {
			t.Fatal("stale-generation clone left a handler in the reset registry")
		}
	}
}

func TestCloneFinalLayerAndTexturePrecedeVisibility(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			clone.SetCostume__0("final")
			clone.SetLayerTo(Front)
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	peer := newCloneLifecycleTestSprite(game, new(cloneLifecycleScenario), true)
	game.addShape(spriteOf(source))
	game.addShape(spriteOf(peer))
	game.shapeMgr.updateRenderLayers()

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.runtimeState.SyncSprite == nil {
		t.Fatal("clone was not published")
	}
	cloneID := clone.runtimeState.SyncSprite.GetId()
	operations := mgr.recordedOperations()
	finalTexture := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "texture" && strings.HasSuffix(operation.path, "final.png")
	})
	finalLayer := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "z" && operation.z == int64(clone.runtimeState.Layer)
	})
	visible := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	})
	if finalTexture < 0 || finalLayer < 0 || visible < 0 ||
		finalTexture >= visible || finalLayer >= visible {
		t.Fatalf("clone was exposed before final texture/layer: operations = %#v", operations)
	}
}

func TestPendingCloneNativePhysicsActivatesOnlyAtCommit(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	var observedVelocity mathf.Vec2
	var observedClearedVelocity mathf.Vec2
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			if err := clone.SetColliderShape(false, PolygonCollider, []float64{
				-1, -1, 1, -1, 0, 1,
			}); err != nil {
				panic(err)
			}
			if err := clone.SetColliderShape(true, RectCollider, []float64{3, 3}); err != nil {
				panic(err)
			}
			clone.SetCollisionEnabled(true)
			clone.SetTriggerEnabled(true)
			clone.SetVelocity(1, 2)
			clone.SetPhysicsMode(NoPhysics)
			clearedX, clearedY := clone.Velocity()
			observedClearedVelocity = mathf.NewVec2(clearedX, clearedY)
			clone.SetPhysicsMode(DynamicPhysics)
			clone.SetVelocity(4, 5)
			clone.AddImpulse(6, 7)
			velocityX, velocityY := clone.Velocity()
			observedVelocity = mathf.NewVec2(velocityX, velocityY)
			mgr.record(cloneProxyOperation{
				kind: "on-cloned-end", object: clone.runtimeState.SyncSprite.GetId(),
			})
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	configureCloneTestPhysics(source)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.spriteState.IsProxyPublicationPending {
		t.Fatal("clone did not publish")
	}
	if want := mathf.NewVec2(4, 5); observedVelocity != want {
		t.Fatalf("pending logical velocity = %v, want %v", observedVelocity, want)
	}
	if observedClearedVelocity != (mathf.Vec2{}) {
		t.Fatalf("NoPhysics did not clear pending velocity: %v", observedClearedVelocity)
	}
	cloneID := clone.runtimeState.SyncSprite.GetId()
	operations := mgr.recordedOperations()
	marker := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "on-cloned-end"
	})
	polygon := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "collision-polygon"
	})
	collisionEnabled := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "collision" && operation.enabled
	})
	triggerEnabled := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "trigger" && operation.enabled
	})
	dynamicMode := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "physics-mode" && operation.mode == DynamicPhysics
	})
	velocity := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "velocity" && operation.vec == mathf.NewVec2(4, 5)
	})
	impulse := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "impulse" && operation.vec == mathf.NewVec2(6, 7)
	})
	visible := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return (operation.kind == "visible" || operation.kind == "transform") && operation.visible
	})
	if polygon < 0 || marker <= polygon || collisionEnabled <= marker || triggerEnabled <= marker ||
		dynamicMode <= marker || velocity <= dynamicMode || impulse <= velocity ||
		visible <= collisionEnabled || visible <= triggerEnabled || visible <= impulse {
		t.Fatalf("native physics escaped the clone commit barrier: operations = %#v", operations)
	}
	for _, kind := range []string{"collision", "trigger"} {
		if firstCloneOperation(operations[:marker+1], cloneID, func(operation cloneProxyOperation) bool {
			return operation.kind == kind && operation.enabled
		}) >= 0 {
			t.Fatalf("%s was active while clone initialization ran: operations = %#v", kind, operations)
		}
	}
	if firstCloneOperation(operations[:marker+1], cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "physics-mode" && operation.mode != NoPhysics
	}) >= 0 {
		t.Fatalf("physics mode was active while clone initialization ran: operations = %#v", operations)
	}
	if firstCloneOperation(operations[:marker+1], cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "velocity" || operation.kind == "impulse"
	}) >= 0 {
		t.Fatalf("physics commands reached native state while clone initialization ran: operations = %#v", operations)
	}
}

func TestCloneCommitRestoresInheritedPhysicsMode(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	scenario := &cloneLifecycleScenario{
		onClonedAction: func(clone *cloneLifecycleTestSprite) {
			mgr.record(cloneProxyOperation{
				kind: "on-cloned-end", object: clone.runtimeState.SyncSprite.GetId(),
			})
		},
	}
	source := newCloneLifecycleTestSprite(game, scenario, true)
	configureCloneTestPhysics(source)
	game.addShape(spriteOf(source))

	doClone(source, nil, false, nil)

	clone := scenario.captured()
	if clone == nil || clone.spriteState.IsProxyPublicationPending {
		t.Fatal("clone did not publish")
	}
	cloneID := clone.runtimeState.SyncSprite.GetId()
	operations := mgr.recordedOperations()
	marker := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "on-cloned-end"
	})
	dynamicMode := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "physics-mode" && operation.mode == DynamicPhysics
	})
	if marker < 0 || dynamicMode <= marker {
		t.Fatalf("inherited physics mode was not restored at commit: operations = %#v", operations)
	}
}

func TestClonePublicationPanicRestoresPublishedLayers(t *testing.T) {
	_, game, baseMgr := setupCloneLifecycleTest(t)
	mgr := &panicVisibleCloneSpriteMgr{
		spyCloneSpriteMgr: baseMgr,
		panicValue:        "publication bridge failed",
	}
	pkgengine.SpriteMgr = mgr

	scenario := new(cloneLifecycleScenario)
	source := newCloneLifecycleTestSprite(game, scenario, true)
	peer := newCloneLifecycleTestSprite(game, new(cloneLifecycleScenario), true)
	configureCloneTestPhysics(source)
	source.initRuntimeProxy()
	peer.initRuntimeProxy()
	game.addShape(spriteOf(source))
	game.addShape(spriteOf(peer))
	game.shapeMgr.updateRenderLayers()
	game.shapeMgr.syncDirtySpriteLayers()

	sourceID := source.runtimeState.SyncSprite.GetId()
	peerID := peer.runtimeState.SyncSprite.GetId()
	sourceLayer, peerLayer := source.runtimeState.Layer, peer.runtimeState.Layer
	sourceZ, peerZ := baseMgr.zIndex(sourceID), baseMgr.zIndex(peerID)
	mgr.panicNextVisible.Store(true)

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		doClone(source, nil, false, nil)
	}()
	if recovered != mgr.panicValue {
		t.Fatalf("publication panic = %v, want %v", recovered, mgr.panicValue)
	}

	clone := scenario.captured()
	if clone == nil || !clone.isDestroyed() {
		t.Fatal("failed publication did not roll back the clone")
	}
	if source.runtimeState.Layer != sourceLayer || peer.runtimeState.Layer != peerLayer {
		t.Fatalf(
			"failed publication changed logical peer layers: source %d->%d, peer %d->%d",
			sourceLayer, source.runtimeState.Layer, peerLayer, peer.runtimeState.Layer,
		)
	}
	if got := baseMgr.zIndex(sourceID); got != sourceZ {
		t.Fatalf("failed publication changed source proxy layer: %d -> %d", sourceZ, got)
	}
	if got := baseMgr.zIndex(peerID); got != peerZ {
		t.Fatalf("failed publication changed peer proxy layer: %d -> %d", peerZ, got)
	}
	if shapes := game.getAllShapes(); len(shapes) != 2 ||
		shapes[0] != spriteOf(source) || shapes[1] != spriteOf(peer) {
		t.Fatalf("failed publication changed active shapes: %v", shapes)
	}

	cloneID := clone.runtimeState.SyncSprite.GetId()
	operations := baseMgr.recordedOperations()
	lastTransform := firstCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "transform"
	})
	for i := len(operations) - 1; i >= 0; i-- {
		if operations[i].object == cloneID && operations[i].kind == "transform" {
			lastTransform = i
			break
		}
	}
	if lastTransform < 0 || operations[lastTransform].visible {
		t.Fatalf("failed publication did not finish with a hidden proxy: operations = %#v", operations)
	}
	lastCollision := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "collision"
	})
	lastTrigger := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "trigger"
	})
	lastMode := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "physics-mode"
	})
	if lastCollision < 0 || operations[lastCollision].enabled ||
		lastTrigger < 0 || operations[lastTrigger].enabled ||
		lastMode < 0 || operations[lastMode].mode != NoPhysics {
		t.Fatalf("failed publication left native physics active: operations = %#v", operations)
	}
}

func TestPendingCloneDoesNotOccupyPublishedLayerOrReceivePhysics(t *testing.T) {
	_, game, mgr := setupCloneLifecycleTest(t)
	source := newCloneLifecycleTestSprite(game, new(cloneLifecycleScenario), true)
	peer := newCloneLifecycleTestSprite(game, new(cloneLifecycleScenario), true)
	configureCloneTestPhysics(source)
	source.initRuntimeProxy()
	peer.initRuntimeProxy()
	game.addShape(spriteOf(source))
	game.addShape(spriteOf(peer))
	game.shapeMgr.updateRenderLayers()
	game.shapeMgr.syncDirtySpriteLayers()

	sourceLayer, peerLayer := source.runtimeState.Layer, peer.runtimeState.Layer
	sourceZ := mgr.zIndex(source.runtimeState.SyncSprite.GetId())
	peerZ := mgr.zIndex(peer.runtimeState.SyncSprite.GetId())
	in := reflect.ValueOf(source).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface().(Sprite)
	clone := cloneSprite(out, outPtr, in, nil)
	game.addClonedShape(spriteOf(source), clone)
	cloneID := clone.runtimeState.SyncSprite.GetId()
	operations := mgr.recordedOperations()
	lastCollision := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "collision"
	})
	lastTrigger := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "trigger"
	})
	lastMode := lastCloneOperation(operations, cloneID, func(operation cloneProxyOperation) bool {
		return operation.kind == "physics-mode"
	})
	if lastCollision < 0 || operations[lastCollision].enabled ||
		lastTrigger < 0 || operations[lastTrigger].enabled ||
		lastMode < 0 || operations[lastMode].mode != NoPhysics {
		t.Fatalf("pending clone entered native physics: operations = %#v", operations)
	}

	if isSpriteTouchable(clone) {
		t.Fatal("publication-pending clone participated in physics")
	}
	if source.runtimeState.Layer != sourceLayer || peer.runtimeState.Layer != peerLayer {
		t.Fatalf(
			"pending clone changed peer layers: source %d->%d, peer %d->%d",
			sourceLayer, source.runtimeState.Layer, peerLayer, peer.runtimeState.Layer,
		)
	}
	game.shapeMgr.syncDirtySpriteLayers()
	if got := mgr.zIndex(source.runtimeState.SyncSprite.GetId()); got != sourceZ {
		t.Fatalf("pending clone changed source proxy layer: %d -> %d", sourceZ, got)
	}
	if got := mgr.zIndex(peer.runtimeState.SyncSprite.GetId()); got != peerZ {
		t.Fatalf("pending clone changed peer proxy layer: %d -> %d", peerZ, got)
	}
}
