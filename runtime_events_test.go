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
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestOnCondWaitsUntilConditionBecomesTrue(t *testing.T) {
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

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	var condition atomic.Bool
	calls := 0
	game.OnCond(condition.Load, func() { calls++ })

	game.scriptEvents.doWhenCond()
	co.Update()
	if calls != 0 {
		t.Fatalf("calls before condition = %d, want 0", calls)
	}
	condition.Store(true)
	game.scriptEvents.doWhenCond()
	co.Update()
	if calls != 1 {
		t.Fatalf("calls after condition = %d, want 1", calls)
	}

	game.scriptEvents.doWhenCond()
	co.Update()
	if calls != 1 {
		t.Fatalf("calls while condition remains true = %d, want 1", calls)
	}

	condition.Store(false)
	game.scriptEvents.doWhenCond()
	co.Update()
	condition.Store(true)
	game.scriptEvents.doWhenCond()
	co.Update()
	if calls != 2 {
		t.Fatalf("calls after second rising edge = %d, want 2", calls)
	}
}

func TestStartHandlersRegisterAbsoluteEngineFrames(t *testing.T) {
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

	game.handleEvent(&eventStart{})
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

	var game Game
	game.scriptEventBindings.init(&game.scriptEvents, &game)
	game.lifecycleState.BootstrapDone.Store(true)
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
