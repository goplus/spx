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
	"slices"
	"testing"
	"time"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func setupRuntimeScheduler(t *testing.T) *coroutine.Coroutines {
	t.Helper()
	co := coroutine.New(nil)
	installRuntimeScheduler(t, co, nil)
	return co
}

// beforeRestore runs after the test's coroutines stop, while their scheduler
// remains bound. Register other resource cleanup before installing the scheduler.
func installRuntimeScheduler(t testing.TB, co *coroutine.Coroutines, beforeRestore func()) {
	t.Helper()
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		if beforeRestore != nil {
			beforeRestore()
		}
		gco = original
		engine.SetCoroutines(original)
	})
}

func TestRuntimeSchedulerFixtureDrainsBeforeRestoringBindings(t *testing.T) {
	original := setupRuntimeScheduler(t)
	stopped := make(chan struct{})
	cleaned := false
	t.Run("replacement", func(t *testing.T) {
		co := coroutine.New(nil)
		installRuntimeScheduler(t, co, func() {
			if gco != co {
				t.Error("scheduler restored before resource cleanup")
			}
			select {
			case <-stopped:
			default:
				t.Error("resource cleanup ran before coroutine completion")
			}
			cleaned = true
		})
		started := make(chan struct{})
		co.Create("fixture-drain", func(coroutine.Thread) {
			defer close(stopped)
			if !engine.IsInCoroutine() {
				t.Error("engine scheduler binding does not match the test scheduler")
			}
			close(started)
			engine.WaitNextFrame()
		})
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("test coroutine did not start")
		}
	})
	if !cleaned || gco != original {
		t.Fatal("scheduler cleanup did not complete and restore its previous binding")
	}
	bound := make(chan bool, 1)
	original.Create("fixture-restored", func(coroutine.Thread) {
		bound <- engine.IsInCoroutine()
	})
	select {
	case ok := <-bound:
		if !ok {
			t.Fatal("engine scheduler binding was not restored")
		}
	case <-time.After(time.Second):
		t.Fatal("restored scheduler did not start a coroutine")
	}
}

func TestNonVisualLoopsShareFrame(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	counts := [2]int{}
	for i := range counts {
		th := co.Create(nil, func(coroutine.Thread) {
			Repeat(5, func() { counts[i]++ })
		})
		co.JoinFirstSlice(th)
	}
	co.Update()
	if counts != [2]int{5, 5} {
		t.Fatalf("same-frame iterations = %v, want [5 5]", counts)
	}
}

func TestRotationStyleChangesControlRedrawBoundary(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "hidden", true: "visible"}[visible], func(t *testing.T) {
			co, _ := setupRuntimeEventGame(t)
			itime.Start(nil)
			sprite := new(SpriteImpl)
			sprite.spriteState.IsVisible = visible
			sprite.components.transform = &transformComponent{
				sprite:    sprite,
				direction: 45, rotationStyle: Normal,
			}
			var frames []int64
			th := co.Create(sprite, func(coroutine.Thread) {
				Repeat(3, func() {
					if len(frames)%2 == 0 {
						sprite.SetRotationStyle(None)
					} else {
						sprite.SetRotationStyle(Normal)
					}
					frames = append(frames, itime.Frame())
				})
			})
			co.JoinFirstSlice(th)
			for range 3 {
				co.Update()
				itime.Update(1.0/30, 30)
			}
			if len(frames) != 3 {
				t.Fatalf("iterations = %d, want 3", len(frames))
			}
			for i, frame := range frames {
				want := int64(0)
				if visible {
					want = int64(i)
				}
				if frame != want {
					t.Fatalf("iteration %d ran at frame %d, want %d", i, frame, want)
				}
			}
			if sprite.spriteState.DirtyVersion == 0 {
				t.Fatal("rotation style did not mark the render proxy dirty")
			}
		})
	}
}

func TestReplayFinalTickConditionBeforePause(t *testing.T) {
	resetInputSessionTest(t)
	co, game := setupRuntimeEventGame(t)
	itime.Start(nil)
	engine.ResetCaptures()
	t.Cleanup(engine.ResetCaptures)
	setupImmediateEngineCalls(t)
	originalInputMgr, originalExtMgr := pkgengine.InputMgr, pkgengine.ExtMgr
	pkgengine.InputMgr = &gameTestInputMgr{}
	extMgr := &replayPauseTestExtMgr{}
	pkgengine.ExtMgr = extMgr
	t.Cleanup(func() {
		pkgengine.InputMgr, pkgengine.ExtMgr = originalInputMgr, originalExtMgr
		engine.SetCaptureHandler(nil)
	})
	replay := validRuntimeReplay(
		InputReplayFrame{Frame: 0, Time: 0},
		InputReplayFrame{Frame: 1, Time: 0.05,
			State:     InputReplayState{KeysDown: []int64{int64(KeyA)}},
			KeyEvents: []InputReplayKeyEvent{{Key: int64(KeyA), Pressed: true}}},
	)
	replay.FixedTimestep = 0.05
	if _, err := PrepareInputReplay(replay, InputSessionOptions{CaptureKey: KeyA}); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	game.inputMgr.init(game)
	game.camera = &cameraImpl{g: game}
	game.initShapeMgr()
	game.syncBuffer = engine.NewSpriteSyncBuffer(1)
	game.lifecycleState.IsRunned.Store(true)
	game.lifecycleState.StartDispatched.Store(true)
	var sampled, handled float64
	var conditionCalls, keyCalls int
	game.OnCond(func() bool {
		if keyCalls != 0 {
			t.Error("key handler ran before condition sampling")
		}
		sampled = game.Timer()
		return game.KeyPressed(KeyA)
	}, func() {
		conditionCalls++
		handled = game.Timer()
	})
	game.OnKey__0(KeyA, func() { keyCalls++ })
	var captures []engine.CaptureRequest
	engine.SetCaptureHandler(func(request engine.CaptureRequest) error {
		captures = append(captures, request)
		return nil
	})
	for range replay.Frames {
		game.OnEngineBeforeUpdate(0.05)
		if engine.HasPendingCaptures() {
			t.Fatal("input capture was queued before the clock advanced")
		}
		itime.Update(0.05, 30)
		game.OnEngineUpdate(0.05)
		co.Update()
		game.OnEngineRender(0.05)
		if err := engine.FlushCaptures(); err != nil {
			t.Fatal(err)
		}
		game.OnEngineFrameEnd()
	}
	if conditionCalls != 1 || keyCalls != 1 {
		t.Fatalf("condition/key calls = %d/%d, want 1/1", conditionCalls, keyCalls)
	}
	if sampled != 0.05 || handled != 0.1 {
		t.Fatalf("predicate/handler clock = %v/%v, want %v/%v", sampled, handled, 0.05, 0.1)
	}
	if len(captures) != 1 || captures[0].Frame != 2 || captures[0].InputTick == nil || *captures[0].InputTick != 1 {
		t.Fatalf("final tick captures = %+v", captures)
	}
	if extMgr.pauses != 1 || session.status().Phase != InputSessionPhaseCompleted {
		t.Fatalf("EOF pauses = %d, session = %+v", extMgr.pauses, session.status())
	}
}

func TestRedrawFinishesWholeRound(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	var visual, nonVisual int
	th := co.Create(nil, func(coroutine.Thread) {
		Forever(func() {
			visual++
			engine.RequestRedraw()
		})
	})
	co.JoinFirstSlice(th)
	th = co.Create(nil, func(coroutine.Thread) {
		Forever(func() { nonVisual++ })
	})
	co.JoinFirstSlice(th)
	co.Update()
	for frame := 1; frame <= 4; frame++ {
		if visual != frame || nonVisual != frame {
			t.Fatalf("frame %d: visual/nonvisual = %d/%d", frame, visual, nonVisual)
		}
		itime.Update(1.0/30, 30)
		co.Update()
	}
}

func TestExplicitWaitStillCrossesFrame(t *testing.T) {
	co, _ := setupRuntimeEventGame(t)
	itime.Start(nil)
	count := 0
	th := co.Create(nil, func(coroutine.Thread) {
		Repeat(3, func() {
			count++
			engine.Wait(0)
		})
	})
	co.JoinFirstSlice(th)
	for want := 1; want <= 3; want++ {
		co.Update()
		if count != want {
			t.Fatalf("iterations = %d, want %d", count, want)
		}
		itime.Update(1.0/30, 30)
	}
}

func TestSpriteVisibilityControlsRedrawBoundary(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(map[bool]string{false: "hidden", true: "visible"}[visible], func(t *testing.T) {
			co, _ := setupRuntimeEventGame(t)
			itime.Start(nil)
			sprite := new(SpriteImpl)
			sprite.spriteState.IsVisible = visible
			count := 0
			th := co.Create(sprite, func(coroutine.Thread) {
				Repeat(5, func() { count++; sprite.markProxyDirty() })
			})
			co.JoinFirstSlice(th)
			co.Update()
			want := 5
			if visible {
				want = 1
			}
			if count != want {
				t.Fatalf("iterations = %d, want %d", count, want)
			}
		})
	}
}

func TestCostumeChangesControlRedrawBoundary(t *testing.T) {
	for _, tt := range []struct {
		name       string
		visible    bool
		costume    any
		wantFrames [3]int64
	}{
		{"visible", true, Next, [3]int64{0, 1, 2}},
		{"hidden", false, Next, [3]int64{0, 0, 0}},
		{"invalid", true, SpriteCostumeName("missing"), [3]int64{0, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			co, _ := setupRuntimeEventGame(t)
			itime.Start(nil)
			sprite := newTestRenderSprite()
			sprite.spriteState.IsVisible = tt.visible
			var frames [3]int64
			count := 0
			th := co.Create(sprite, func(coroutine.Thread) {
				Repeat(len(frames), func() {
					sprite.setCostume(tt.costume)
					frames[count] = itime.Frame()
					count++
				})
			})
			co.JoinFirstSlice(th)
			for range frames {
				co.Update()
				itime.Update(1.0/30, 30)
			}
			if count != len(frames) || frames != tt.wantFrames {
				t.Fatalf("costume iterations = %d at frames %v, want %d at %v", count, frames, len(frames), tt.wantFrames)
			}
		})
	}
}

func TestTimerTrackingHatDoesNotFire(t *testing.T) {
	for _, redraw := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonvisual", true: "redraw-every-round"}[redraw], func(t *testing.T) {
			co, game, _ := setupInputSourceTest(t)
			game.markGameStarted(game.bootstrapGeneration())
			itime.Start(nil)
			var tracked float64
			fired := 0
			game.OnStart(func() {
				Forever(func() {
					tracked = game.Timer()
					if redraw {
						engine.RequestRedraw()
					}
				})
			})
			game.OnCond(func() bool { return game.Timer() > tracked+0.01 }, func() { fired++ })
			game.handleEvent(&eventStart{generation: game.bootstrapGeneration()})
			co.Update()
			// Slow frames must preserve the same sampling order.
			for _, delta := range []float64{1.0 / 30, 1.0 / 60, 0.25, 1.0 / 30} {
				game.OnEngineBeforeUpdate(1.0 / 30)
				itime.Update(delta, 30)
				withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
				co.Update()
			}
			if fired != 0 {
				t.Fatalf("tracking hat fired %d times", fired)
			}
			// Stopping tracking allows a timer edge to fire.
			co.StopIf(func(th coroutine.Thread) bool { return th.Owner == game })
			game.OnEngineBeforeUpdate(1.0 / 30)
			itime.Update(0.1, 30)
			withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
			co.Update()
			game.OnEngineBeforeUpdate(1.0 / 30)
			itime.Update(0.1, 30)
			withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
			co.Update()
			if fired != 1 {
				t.Fatalf("stopped tracking hat fired %d times, want 1", fired)
			}
		})
	}
}

func TestConditionSamplingSkipsBootstrapWhileScriptIsRunning(t *testing.T) {
	for _, startDispatched := range []bool{false, true} {
		t.Run(fmt.Sprintf("start_dispatched=%t", startDispatched), func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			game.lifecycleState.StartDispatched.Store(startDispatched)
			started, release := make(chan struct{}), make(chan struct{})
			var returned chan struct{}
			t.Cleanup(func() {
				close(release)
				if returned != nil {
					<-returned
				}
			})
			co.Create(game, func(coroutine.Thread) {
				close(started)
				<-release
			})
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("bootstrap script did not start")
			}
			samples := 0
			game.OnCond(func() bool { samples++; return true }, func() {})
			returned = make(chan struct{})
			go func() {
				game.OnEngineBeforeUpdate(1.0 / 30)
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(time.Second):
				t.Fatal("unstarted frame waited for a script sampling barrier")
			}
			if samples != 0 {
				t.Fatalf("bootstrap condition samples = %d, want 0", samples)
			}
		})
	}
}

func TestConditionSamplesBeforeClockAndHandlerUsesNewClock(t *testing.T) {
	co, game, _ := setupInputSourceTest(t)
	game.markGameStarted(game.bootstrapGeneration())
	itime.Start(nil)
	game.lifecycleState.StartDispatched.Store(true)
	var sampled, handled float64
	game.OnCond(func() bool { sampled = game.Timer(); return sampled > 0.01 }, func() { handled = game.Timer() })
	itime.Update(0.05, 30)
	game.OnEngineBeforeUpdate(1.0 / 30)
	if handled != 0 {
		t.Fatal("handler ran in predicate phase")
	}
	itime.Update(0.05, 30)
	withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
	co.Update()
	if sampled != 0.05 || handled != 0.1 {
		t.Fatalf("predicate/handler time = %v/%v, want 0.05/0.1", sampled, handled)
	}
}

func TestConditionDoesNotReenterRunningHandler(t *testing.T) {
	co, game, _ := setupInputSourceTest(t)
	game.markGameStarted(game.bootstrapGeneration())
	itime.Start(nil)
	game.lifecycleState.StartDispatched.Store(true)
	value := true
	evaluations, calls := 0, 0
	release := make(chan struct{}, 2)
	completed := make(chan struct{}, 2)
	game.OnCond(func() bool { evaluations++; return value }, func() {
		calls++
		defer func() { completed <- struct{}{} }()
		engine.WaitForChan(release)
	})
	poll := func() {
		game.OnEngineBeforeUpdate(1.0 / 30)
		itime.Update(1.0/30, 30)
		withEventRegistrationBarrier(game, game.scriptEvents.dispatchConditions)
		co.Update()
	}
	poll()
	value = false
	poll()
	value = true
	poll()
	if calls != 1 || evaluations != 1 {
		t.Fatalf("running hat: calls/evaluations = %d/%d, want 1/1", calls, evaluations)
	}
	release <- struct{}{}
	<-completed
	poll()
	if calls != 1 {
		t.Fatal("sustained true condition restarted a completed handler")
	}
	value = false
	poll()
	value = true
	poll()
	if calls != 2 {
		t.Fatalf("new rising edge calls = %d, want 2", calls)
	}
	release <- struct{}{}
}

func TestLogicFramePreservesPhasesAndShapeSnapshot(t *testing.T) {
	setupRuntimeScheduler(t)
	itime.ClearTimers()
	itime.Start(nil)
	t.Cleanup(itime.ClearTimers)
	game := &Game{sounds: make(map[string]sound)}
	backend := &animationAudioBackend{}
	game.soundMgr.Init(backend)

	newSprite := func(name string) *SpriteImpl {
		anim := newTestAnimationComponent()
		sprite := anim.sprite
		sprite.name, sprite.g = name, game
		game.sounds[name] = &coreproject.SoundConfig{Path: name + ".wav"}
		sprite.sound().pendingAudios = []string{name}
		anim.curAnimState = &animState{Name: name}
		anim.doneAnimations = []*animState{anim.curAnimState}
		return sprite
	}
	first, second, late := newSprite("first"), newSprite("second"), newSprite("late")
	destroyed, unbound := newSprite("destroyed"), newSprite("unbound")
	destroyed.markDestroyed()
	unbound.runtimeState.SyncSprite = nil
	game.shapeMgr.items = []Shape{first, struct{}{}, destroyed, second, unbound}
	var steps []string
	backend.onPlay = func(id int64) {
		path := backend.plays[len(backend.plays)-1].path
		steps = append(steps, "play:"+path)
		switch id {
		case 1:
			first.animation().curAnimState.OnPlayAudioPlaybackID = id
			// Both passes must finish the original snapshot, even if playback
			// changes the live shape list.
			game.shapeMgr.items = []Shape{late}
		case 2:
			if first.animation().curAnimState == nil {
				t.Error("first animation completed before second audio played")
			}
			second.animation().curAnimState.OnPlayAudioPlaybackID = id
		case 3:
			late.animation().curAnimState.OnPlayAudioPlaybackID = id
		}
	}
	backend.onStop = func(id int64) {
		steps = append(steps, fmt.Sprintf("complete:%d", id))
	}
	itime.RegisterTimer(0.2)
	itime.RegisterTimer(0.1)
	itime.RegisterTimer(0.3)
	itime.Update(0.2, 30)
	audios, animations := make([]string, 0, 4), make([]*animState, 0, 4)
	audioStorage, animationStorage := &audios[:cap(audios)][0], &animations[:cap(animations)][0]
	audios, animations = game.processLogicFrame(audios, animations)
	want := []string{"play:" + engine.ToAssetPath("first.wav"), "play:" + engine.ToAssetPath("second.wav"), "complete:1", "complete:2"}
	if !slices.Equal(steps, want) {
		t.Fatalf("logic phases = %v, want %v", steps, want)
	}
	if first.animation().curAnimState != nil || second.animation().curAnimState != nil {
		t.Fatal("original snapshot did not complete both animations")
	}
	if len(late.sound().pendingAudios) != 1 || len(late.animation().doneAnimations) != 1 {
		t.Fatal("newly added sprite was processed in the old snapshot")
	}
	for _, sprite := range []*SpriteImpl{destroyed, unbound} {
		if len(sprite.sound().pendingAudios) != 0 || len(sprite.animation().doneAnimations) != 0 || sprite.animation().curAnimState == nil {
			t.Fatalf("inactive sprite %s was not drained without playback", sprite.name)
		}
	}
	// Timers belong to the frame event phase, not the audio/animation loop.
	for _, timestamp := range []int64{100, 200} {
		if timer, ok := itime.NextTimer(); !ok || timer != timestamp {
			t.Fatalf("pending timer = %d, %v, want %d", timer, ok, timestamp)
		}
	}
	if _, ok := itime.NextTimer(); ok {
		t.Fatal("future timer became due early")
	}
	steps = nil
	audios, animations = game.processLogicFrame(audios, animations)
	want = []string{"play:" + engine.ToAssetPath("late.wav"), "complete:3"}
	if !slices.Equal(steps, want) {
		t.Fatalf("next frame = %v, want %v", steps, want)
	}
	if len(audios) != 0 || cap(audios) != 4 || &audios[:cap(audios)][0] != audioStorage ||
		len(animations) != 0 || cap(animations) != 4 || &animations[:cap(animations)][0] != animationStorage {
		t.Fatal("logic frames did not preserve both reusable scratch buffers")
	}
}

func TestLogicFramePreservesEmptyScratch(t *testing.T) {
	itime.ClearTimers()
	t.Cleanup(itime.ClearTimers)
	for _, items := range [][]Shape{nil, {struct{}{}}} {
		game := &Game{}
		game.shapeMgr.items = items
		audios, animations := game.processLogicFrame([]string{}, []*animState{})
		if audios == nil || animations == nil || len(audios) != 0 || len(animations) != 0 {
			t.Fatalf("scratch = %#v, %#v, want non-nil empty slices", audios, animations)
		}
		audios, animations = game.processLogicFrame(nil, nil)
		if audios != nil || animations != nil {
			t.Fatalf("nil scratch = %#v, %#v, want nil slices", audios, animations)
		}
	}
}
