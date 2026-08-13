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
	"runtime"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func setupRuntimeEventScheduler(t *testing.T) *coroutine.Coroutines {
	t.Helper()

	co := coroutine.New(nil)
	co.OnInited()
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
		gco = original
		engine.SetCoroutines(original)
	})
	return co
}

func updateRuntimeEventSchedulerUntil(t *testing.T, co *coroutine.Coroutines, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("coroutine scheduler did not reach the expected state")
		}
		co.Update()
		runtime.Gosched()
	}
}

func TestStartHandlersRegisterAbsoluteEngineFrames(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	engine.SetGame(&game)
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()

	var callbacks []string
	base := itime.Frame()
	game.OnStart(func() {
		AtFrame(base+1, func() { callbacks = append(callbacks, "first") })
		engine.WaitYield()
	})
	game.OnStart(func() {
		AtFrame(base+1, func() { callbacks = append(callbacks, "second") })
	})

	game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
	co.Update()
	if !game.lifecycleState.StartDispatched.Load() {
		t.Fatal("start event was not marked dispatched")
	}

	itime.Update(0, 0)
	engine.RunFrameCallbacks()
	co.Update()
	want := []string{"first", "second"}
	if !reflect.DeepEqual(callbacks, want) {
		t.Fatalf("frame callbacks = %v, want %v", callbacks, want)
	}
}

func TestOnStartReachesCoroutineBoundaryBeforePostBootstrapFrames(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	game.markBootstrapDoneFor(game.currentBootstrapGeneration())
	engine.SetGame(&game)
	defer engine.SetGame(nil)
	engine.ResetFrameRuntime()
	defer engine.ResetFrameRuntime()

	var phases []string
	base := itime.Frame()
	AtFrame(base+1, func() {
		phases = append(phases, "frame")
		engine.WaitYield()
	})
	game.OnStart(func() {
		phases = append(phases, "start")
	})

	itime.Update(0, 0)
	game.runScriptFramePhase()
	co.Update()
	game.runScriptFramePhase()
	co.Update()

	if want := []string{"start", "frame"}; !reflect.DeepEqual(phases, want) {
		t.Fatalf("frame/start phases = %v, want %v", phases, want)
	}
}

func TestOnCondRisingEdgeAndOwnerIsolation(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var registry scriptEventRegistry
	var left, right scriptEventBindings
	left.init(&registry, "left")
	right.init(&registry, "right")

	var (
		leftValue, rightValue bool
		leftEvaluations       int
		rightEvaluations      int
		calls                 []string
	)
	left.OnCond(func() bool {
		leftEvaluations++
		return leftValue
	}, func() {
		calls = append(calls, "left")
	})
	right.OnCond(func() bool {
		rightEvaluations++
		return rightValue
	}, func() {
		calls = append(calls, "right")
	})

	states := [][2]bool{
		{false, false},
		{true, false},
		{true, true},
		{false, true},
		{true, false},
	}
	for _, state := range states {
		leftValue, rightValue = state[0], state[1]
		registry.doWhenCondition()
		co.Update()
	}

	if want := []string{"left", "right", "left"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("condition calls = %v, want %v", calls, want)
	}
	if leftEvaluations != len(states) || rightEvaluations != len(states) {
		t.Fatalf("condition evaluations = %d/%d, want %d/%d", leftEvaluations, rightEvaluations, len(states), len(states))
	}

	left.doDeleteClone()
	leftValue = false
	rightValue = false
	registry.doWhenCondition()
	leftValue = true
	registry.doWhenCondition()
	co.Update()
	if leftEvaluations != len(states) {
		t.Fatalf("deleted owner evaluated %d times, want %d", leftEvaluations, len(states))
	}
	if want := []string{"left", "right", "left"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls after owner deletion = %v, want %v", calls, want)
	}
}

func TestOnCondIgnoresNilCallbacks(t *testing.T) {
	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	game.OnCond(nil, func() {})
	game.OnCond(func() bool { return true }, nil)

	if got := game.scriptEvents.manager.SnapshotCondition(); len(got) != 0 {
		t.Fatalf("condition sinks = %d, want 0", len(got))
	}
}

func TestOnCondStartsAfterOnStartPhase(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	var order []string
	evaluations := 0
	game.OnCond(func() bool {
		evaluations++
		order = append(order, "evaluate")
		return true
	}, func() {
		order = append(order, "condition")
	})
	game.OnStart(func() {
		order = append(order, "start")
	})

	game.pollConditions()
	if evaluations != 0 {
		t.Fatalf("condition evaluated %d times during bootstrap, want 0", evaluations)
	}

	game.markBootstrapDoneFor(game.currentBootstrapGeneration())
	game.pollConditions()
	if evaluations != 0 {
		t.Fatalf("condition evaluated %d times before OnStart, want 0", evaluations)
	}

	game.runScriptFramePhase()
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	if want := []string{"start"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order before condition polling = %v, want %v", order, want)
	}

	game.pollConditions()
	co.Update()
	if evaluations != 1 {
		t.Fatalf("condition evaluated %d times after OnStart, want 1", evaluations)
	}
	if want := []string{"start", "evaluate", "condition"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}

	game.pollConditions()
	co.Update()
	if evaluations != 2 {
		t.Fatalf("condition evaluated %d times after two polls, want 2", evaluations)
	}
	if want := []string{"start", "evaluate", "condition", "evaluate"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("sustained-true order = %v, want %v", order, want)
	}
}

func TestOnCondObservesTopLevelAndOnStartInitialization(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	var order []string
	ready := false
	topLevelBlocked := make(chan struct{})
	onStartBlocked := make(chan struct{})

	generation := game.currentBootstrapGeneration()
	game.deferBootstrapFor(generation, func() {
		game.runBootstrapMainUntilYield(&game, func() {
			order = append(order, "top-level")
			game.OnCond(func() bool {
				order = append(order, "evaluate")
				return ready
			}, func() {
				order = append(order, "condition")
			})
			game.OnStart(func() {
				ready = true
				order = append(order, "start")
				var signal struct{}
				engine.WaitForChan(onStartBlocked, &signal)
				order = append(order, "start-resumed")
			})

			var signal struct{}
			engine.WaitForChan(topLevelBlocked, &signal)
			order = append(order, "top-level-resumed")
		})
	})

	game.runBootstrapTasksFor(generation)
	if want := []string{"top-level"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order after top-level first yield = %v, want %v", order, want)
	}
	game.pollConditions()
	if want := []string{"top-level"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("condition ran before OnStart: %v", order)
	}

	game.markBootstrapDoneFor(generation)
	game.runScriptFramePhase()
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	if want := []string{"top-level", "start"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order after OnStart = %v, want %v", order, want)
	}

	game.pollConditions()
	co.Update()
	if want := []string{"top-level", "start", "evaluate", "condition"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("startup order = %v, want %v", order, want)
	}

	close(topLevelBlocked)
	close(onStartBlocked)
	co.Update()
}

func TestOnStartIgnoresStaleBootstrapGeneration(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	stale := game.currentBootstrapGeneration()
	game.resetBootstrapState()

	calls := 0
	game.OnStart(func() { calls++ })
	game.handleEvent(&eventStart{generation: stale})
	co.Update()
	if calls != 0 || game.lifecycleState.StartDispatched.Load() {
		t.Fatal("stale start event affected the current lifecycle")
	}

	game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	if calls != 1 {
		t.Fatalf("current start calls = %d, want 1", calls)
	}
}

func TestOnStartCompletionDoesNotCrossReset(t *testing.T) {
	co := setupRuntimeEventScheduler(t)

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	entered := make(chan struct{})
	release := make(chan struct{})
	game.OnStart(func() {
		close(entered)
		<-release
	})

	game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("OnStart did not begin")
	}

	game.resetBootstrapState()
	game.scriptEvents.manager.Reset()
	close(release)
	co.Update()
	if game.lifecycleState.StartDispatched.Load() {
		t.Fatal("stale OnStart completion reopened the start gate")
	}

	calls := 0
	game.OnStart(func() { calls++ })
	game.handleEvent(&eventStart{generation: game.currentBootstrapGeneration()})
	updateRuntimeEventSchedulerUntil(t, co, game.lifecycleState.StartDispatched.Load)
	if calls != 1 {
		t.Fatalf("new lifecycle start calls = %d, want 1", calls)
	}
}

func TestGameRunBootstrapTasksExecutesQueuedHooksOnce(t *testing.T) {
	var g Game

	var got []string
	g.deferBootstrap(func() {
		got = append(got, "first")
	})
	g.deferBootstrap(func() {
		got = append(got, "second")
	})

	g.runBootstrapTasks()
	g.runBootstrapTasks()

	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runBootstrapTasks got %v, want %v", got, want)
	}
}

type clickThroughSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	hits          map[pkgengine.Object]bool
	lastIsTrigger map[pkgengine.Object]bool
}

func (s *clickThroughSpriteMgr) CheckCollisionWithPoint(obj pkgengine.Object, point mathf.Vec2, isTrigger bool) bool {
	if s.lastIsTrigger != nil {
		s.lastIsTrigger[obj] = isTrigger
	}
	return s.hits[obj]
}

func setupClickThroughSpriteMgr(t *testing.T, hits map[pkgengine.Object]bool) *clickThroughSpriteMgr {
	t.Helper()

	enginewrap.Init(func(call func()) {
		call()
	})

	original := pkgengine.SpriteMgr
	mgr := &clickThroughSpriteMgr{
		hits:          hits,
		lastIsTrigger: make(map[pkgengine.Object]bool),
	}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
	return mgr
}

func newClickTestSprite(g *Game, name string, id pkgengine.Object, registerClick bool) *SpriteImpl {
	sprite := &SpriteImpl{}
	sprite.g = g
	sprite.name = name
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(id)
	sprite.scriptEventBindings.init(&g.scriptEvents, sprite)
	if registerClick {
		sprite.OnClick(func() {})
	}
	return sprite
}

func TestFindClickTargetKeepsTopmostSpriteWithoutClickHandler(t *testing.T) {
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
		2: true,
	})

	var g Game
	g.initShapeMgr()

	bottom := newClickTestSprite(&g, "bottom", 1, true)
	top := newClickTestSprite(&g, "top", 2, false)
	g.addShape(bottom)
	g.addShape(top)

	selection, ok := g.findClickTarget(mathf.NewVec2(0, 0))
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != top {
		t.Fatalf("target = %p, want top %p", selection.Target, top)
	}
	if selection.SwipeTarget != top {
		t.Fatalf("swipe target = %p, want top %p", selection.SwipeTarget, top)
	}
}

func TestFindClickTargetKeepsTopmostClickableSprite(t *testing.T) {
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
		2: true,
	})

	var g Game
	g.initShapeMgr()

	bottom := newClickTestSprite(&g, "bottom", 1, true)
	top := newClickTestSprite(&g, "top", 2, true)
	g.addShape(bottom)
	g.addShape(top)

	selection, ok := g.findClickTarget(mathf.NewVec2(0, 0))
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != top {
		t.Fatalf("target = %p, want top %p", selection.Target, top)
	}
	if selection.SwipeTarget != top {
		t.Fatalf("swipe target = %p, want top %p", selection.SwipeTarget, top)
	}
}

func TestFindClickTargetSkipsFullyGhostedSpriteEvenWithClickHandler(t *testing.T) {
	setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
		2: true,
	})

	var g Game
	g.initShapeMgr()

	bottom := newClickTestSprite(&g, "bottom", 1, true)
	top := newClickTestSprite(&g, "top", 2, true)
	top.greffUniforms = map[EffectKind]float64{GhostEffect: 100}
	g.addShape(bottom)
	g.addShape(top)

	selection, ok := g.findClickTarget(mathf.NewVec2(0, 0))
	if !ok {
		t.Fatal("expected click target")
	}
	if selection.Target != bottom {
		t.Fatalf("target = %p, want bottom %p", selection.Target, bottom)
	}
	if selection.SwipeTarget != bottom {
		t.Fatalf("swipe target = %p, want bottom %p", selection.SwipeTarget, bottom)
	}
}

func TestTouchPointUsesScratchSensingQuery(t *testing.T) {
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
	})

	var g Game
	sprite := newClickTestSprite(&g, "probe", 1, false)
	sprite.spriteState.IsVisible = false

	if !sprite.touchPoint(12, 34) {
		t.Fatal("expected touchPoint hit")
	}
	if got := mgr.lastIsTrigger[1]; got {
		t.Fatalf("touchPoint used click query, want sensing query")
	}
}

func TestPointHitsClickTargetUsesClickQuery(t *testing.T) {
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{
		1: true,
	})

	var g Game
	sprite := newClickTestSprite(&g, "button", 1, true)

	if !g.pointHitsClickTarget(sprite, mathf.NewVec2(0, 0)) {
		t.Fatal("expected click hit")
	}
	if got := mgr.lastIsTrigger[1]; !got {
		t.Fatalf("pointHitsClickTarget used sensing query, want click query")
	}
}

func TestResetGraphicEffectsOnStopAllClearsGraphicEffects(t *testing.T) {
	var g Game
	g.initShapeMgr()
	g.greffUniforms = map[EffectKind]float64{GhostEffect: 100}

	left := &SpriteImpl{g: &g, name: "left"}
	left.greffUniforms = map[EffectKind]float64{GhostEffect: 100}

	right := &SpriteImpl{g: &g, name: "right"}
	right.greffUniforms = map[EffectKind]float64{
		GhostEffect:      50,
		BrightnessEffect: 25,
	}

	g.addShape(left)
	g.addShape(right)

	g.resetGraphicEffectsOnStopAll()

	if got := g.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("stage ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := left.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("left ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.greffUniforms[GhostEffect]; got != 0 {
		t.Fatalf("right ghost effect = %v, want 0 after stop all reset", got)
	}
	if got := right.greffUniforms[BrightnessEffect]; got != 0 {
		t.Fatalf("right brightness effect = %v, want 0 after stop all reset", got)
	}
}
