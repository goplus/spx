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
	"slices"
	"testing"
	"time"

	coreevent "github.com/goplus/spx/v4/internal/core/event"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
)

type stopAllAudioBackend struct {
	animationAudioBackend
	stopped   int
	destroyed []engine.Object
}

func TestProcedureConsumesStopThisScript(t *testing.T) {
	Procedure(func() {
		panic(coroutine.ErrStopThisScript)
	})
}

func TestProcedurePropagatesOtherPanics(t *testing.T) {
	tests := []struct {
		name  string
		panic any
	}{
		{name: "abort thread", panic: coroutine.ErrAbortThread},
		{name: "user panic", panic: "user panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != test.panic {
					t.Fatalf("panic = %v, want %v", got, test.panic)
				}
			}()
			Procedure(func() {
				panic(test.panic)
			})
		})
	}
}

func TestProcedureScopesStopThisScriptAcrossRepeat(t *testing.T) {
	co := setupRuntimeScheduler(t)

	done := make(chan string, 1)
	co.Create("stop-this-script", func(coroutine.Thread) {
		trace := "outer-before;"
		Procedure(func() {
			trace += "inner-before;"
			Repeat(1, func() {
				// Match a stop nested in a converted repeat callback.
				co.StopThisScript()
				trace += "inner-bad;"
			})
			trace += "inner-bad2;"
		})
		trace += "outer-after;"
		done <- trace
	})

	select {
	case got := <-done:
		if want := "outer-before;inner-before;outer-after;"; got != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("stop-this-script boundary did not return from procedure")
	}
}

func TestWarpSkipsControlFlowFrameWaits(t *testing.T) {
	co := setupRuntimeScheduler(t)

	type result struct {
		repeatCalls int
		condCalls   int
		flagDuring  bool
		flagAfter   bool
	}

	done := make(chan result, 1)
	co.Create("run-without-screen-refresh", func(me coroutine.Thread) {
		flagDuring := false
		repeatCalls := 0
		condCalls := 0
		Warp(func() {
			flagDuring = IsRunWithoutScreenRefresh()
			Repeat(3, func() {
				repeatCalls++
			})
			WaitUntil(func() bool {
				condCalls++
				return condCalls > 2
			})
		})

		done <- result{
			repeatCalls: repeatCalls,
			condCalls:   condCalls,
			flagDuring:  flagDuring,
			flagAfter:   IsRunWithoutScreenRefresh(),
		}
	})

	select {
	case got := <-done:
		if !got.flagDuring || got.flagAfter {
			t.Fatalf("flag state = during:%v after:%v, want true/false", got.flagDuring, got.flagAfter)
		}
		if got.repeatCalls != 3 {
			t.Fatalf("repeatCalls = %d, want 3", got.repeatCalls)
		}
		if got.condCalls != 3 {
			t.Fatalf("condCalls = %d, want 3", got.condCalls)
		}
	case <-time.After(time.Second):
		t.Fatal("control-flow wait points should not block when run without screen refresh is enabled")
	}
}

func TestWarpRestoresPreviousState(t *testing.T) {
	co := setupRuntimeScheduler(t)

	type result struct {
		prev        bool
		outerDuring bool
		callDuring  bool
		callAfter   bool
		restored    bool
	}

	done := make(chan result, 1)
	co.Create("run-without-screen-refresh-state", func(me coroutine.Thread) {
		prev := SetRunWithoutScreenRefresh(true)
		outerDuring := IsRunWithoutScreenRefresh()

		callDuring := false
		Warp(func() {
			callDuring = IsRunWithoutScreenRefresh()
		})

		callAfter := IsRunWithoutScreenRefresh()
		SetRunWithoutScreenRefresh(prev)
		done <- result{
			prev:        prev,
			outerDuring: outerDuring,
			callDuring:  callDuring,
			callAfter:   callAfter,
			restored:    IsRunWithoutScreenRefresh(),
		}
	})

	select {
	case got := <-done:
		if got.prev {
			t.Fatal("previous run-without-screen-refresh flag should default to false")
		}
		if !got.outerDuring || !got.callDuring || !got.callAfter {
			t.Fatalf("flag state = outer:%v call:%v after:%v, want true/true/true", got.outerDuring, got.callDuring, got.callAfter)
		}
		if got.restored {
			t.Fatal("run-without-screen-refresh flag should restore to false after outer reset")
		}
	case <-time.After(time.Second):
		t.Fatal("run-without-screen-refresh state should restore without blocking")
	}
}

func TestStopThisScriptAtEventBoundaryEndsThread(t *testing.T) {
	panicReported := make(chan struct{}, 1)
	co := coroutine.New(func(coroutine.PanicReport) {
		panicReported <- struct{}{}
	})
	installRuntimeScheduler(t, co, nil)

	var script scriptEventBindings
	script.bind(&scriptEventRegistry{}, "owner")
	thread := co.Create("event-stop-this-script", func(coroutine.Thread) {
		script.Stop(ThisScript)
		panic("stop-this-script should not continue")
	})

	select {
	case <-thread.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("event thread did not terminate after stop-this-script")
	}
	select {
	case <-panicReported:
		t.Fatal("stop-this-script reached the panic reporter")
	default:
	}
}

func TestStopAllCleansResourcesBeforeAbortingCaller(t *testing.T) {
	for _, target := range []string{"stage", "clone"} {
		t.Run(target, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			game.bindScriptEvents()
			backend := &stopAllAudioBackend{}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			source.transform().x = 42
			source.onInit = func(s *cloneLimitSprite) { s.OnKey__0(KeySpace, func() {}) }
			var clone *SpriteImpl
			doClone(source, nil, func(s *SpriteImpl) { clone = s })
			clone.sound().soundObj = 17
			playback := game.soundMgr.Play(17, "long.wav", false, false, 0, 0, 0)
			gate := gco.NewLatch()
			peerFinished := false
			peer := gco.Create(&source.SpriteImpl, func(coroutine.Thread) {
				gate.Wait()
				peerFinished = true
			})
			gco.JoinFirstSlice(peer)
			caller, stopScripts := any(game), game.Stop
			if target == "clone" {
				caller, stopScripts = clone, clone.Stop
			}
			afterStop := false
			stop := gco.Create(caller, func(coroutine.Thread) {
				stopScripts(AllStop)
				afterStop = true
			})
			gco.Join(stop)
			gco.Join(peer)
			if afterStop || peerFinished {
				t.Fatal("stopped script continued")
			}
			if !clone.isDestroyed() || game.shapeMgr.cloneCount != 0 || game.shapeMgr.findShapeIndex(clone) >= 0 {
				t.Fatal("stop all retained clone or its quota")
			}
			if len(game.scriptEvents.manager.Snapshot(coreevent.BucketKeyPressed)) != 0 {
				t.Fatal("clone event registration survived")
			}
			if backend.IsPlaying(playback) || backend.stopped != 1 {
				t.Fatal("stop all left audio playing")
			}
			if len(backend.destroyed) != 1 || backend.destroyed[0] != 17 {
				t.Fatalf("released sound objects = %v", backend.destroyed)
			}
			if source.isDestroyed() || source.transform().x != 42 || len(game.getAllShapes()) != 1 {
				t.Fatal("stop all reset or removed original")
			}
			// Repeated stops must not release resources twice.
			again := gco.Create(game, func(coroutine.Thread) { game.Stop(AllStop) })
			gco.Join(again)
			if game.shapeMgr.cloneCount != 0 || len(backend.destroyed) != 1 {
				t.Fatal("repeated stop duplicated cleanup")
			}
		})
	}
}

func TestStopAllCancelsWaitingCloneHandler(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "source")
	gate := gco.NewLatch()
	source.onClone = func(*cloneLimitSprite) {
		gate.Wait()
		t.Error("clone handler continued after stop all")
	}
	var clone *SpriteImpl
	parent := gco.Create(&source.SpriteImpl, func(coroutine.Thread) {
		doClone(source, nil, func(s *SpriteImpl) { clone = s })
		source.Stop(AllStop)
	})
	gco.Join(parent)
	gco.Update()
	if !clone.isDestroyed() || game.shapeMgr.cloneCount != 0 {
		t.Fatal("pending clone survived stop all")
	}
	flushCloneProxyUpdates(game)
	if clone.runtimeState.SyncSprite != nil {
		t.Fatal("stopped clone retained its proxy")
	}
}

func TestStopAllClearsExistingSoundEffectsWithoutAllocating(t *testing.T) {
	for _, target := range []string{"stage", "sprite"} {
		t.Run(target, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			backend := &fakeAudioBackend{pan: 0.5, pitch: 2}
			game.soundMgr.Init(backend)
			source := newCloneLimitSprite(game, "source")
			unused := newCloneLimitSprite(game, "unused")
			if target == "stage" {
				game.audioState.SoundObj = 9
			} else {
				source.sound().soundObj = 9
			}
			stop := co.Create(game, func(coroutine.Thread) { game.Stop(AllStop) })
			co.Join(stop)
			if backend.pan != 0 || backend.pitch != 1 || backend.createCalls != 0 {
				t.Fatalf("audio after stop: %+v", backend)
			}
			if unused.sound().soundObj != 0 {
				t.Fatal("stop allocated an unused sprite's sound object")
			}
		})
	}
}

func TestStopAllRemovesEveryCloneFromMixedShapes(t *testing.T) {
	game := setupCloneLimitGame(t)
	left := newCloneLimitSprite(game, "left")
	game.shapeMgr.add(&struct{}{})
	right := newCloneLimitSprite(game, "right")
	originals := slices.Clone(game.getAllShapes())
	var clones []*SpriteImpl
	for range 3 {
		for _, source := range []*cloneLimitSprite{left, right} {
			doClone(source, nil, func(clone *SpriteImpl) {
				clone.greffUniforms = map[EffectKind]float64{GhostEffect: 50}
				clones = append(clones, clone)
			})
		}
	}
	for range 2 {
		stop := gco.Create(&left.SpriteImpl, func(coroutine.Thread) { left.Stop(AllStop) })
		gco.Join(stop)
		if !slices.Equal(game.getAllShapes(), originals) || game.shapeMgr.cloneCount != 0 {
			t.Fatal("stop retained clones or changed original shape order")
		}
		for _, clone := range clones {
			if !clone.isDestroyed() || clone.greffUniforms[GhostEffect] != 0 {
				t.Fatal("stop skipped a clone's cleanup")
			}
		}
	}
}

func TestStopAllClearsGraphicEffects(t *testing.T) {
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

	g.shapeMgr.add(left)
	g.shapeMgr.add(right)

	g.stopAllResources()

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

func (b *stopAllAudioBackend) StopAll() {
	b.stopped++
	clear(b.playing)
}

func (b *stopAllAudioBackend) DestroyAudio(obj engine.Object) { b.destroyed = append(b.destroyed, obj) }
