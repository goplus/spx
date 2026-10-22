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

package engine

import (
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func resetRuntimeDeltaTestState() {
	delaySpriteCalls = delaySpriteCalls[:0]
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
	tweenInfos = tweenInfos[:0]
	tempTweenInfos = tempTweenInfos[:0]
	state.sprites = make(map[Object]gdx.ISpriter)
	state.uiNodes = make(map[Object]gdx.IUiNode)
	state.timeSinceStart = 0
	itime.SetFixedDeltaTime(0)
	itime.Start(nil)
}

func advanceRuntimeDeltaFrame(realDelta float64) {
	logicalDelta := itime.EffectiveLogicalDeltaTime(realDelta)
	itime.Update(logicalDelta, 60)
	runtimeBridge{}.InternalUpdateEngine(logicalDelta)
}

func TestInternalUpdateEngineUsesScriptDeltaForDelayCall(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	itime.SetFixedDeltaTime(0.1)

	firedAt := 0
	delayCall(0.25, func() {
		if firedAt == 0 {
			firedAt = int(itime.Frame())
		}
	})

	for _, realDelta := range []float64{0.05, 0.20, 0.01} {
		advanceRuntimeDeltaFrame(realDelta)
	}

	if firedAt != 3 {
		t.Fatalf("delayCall fired at frame %d, want 3 with fixed delta", firedAt)
	}
}

func TestInternalUpdateEngineUsesScriptDeltaForTweens(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	itime.SetFixedDeltaTime(0.1)

	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite
	tweenPos(sprite, mathf.NewVec2(1, 0), 1, func() {})

	advanceRuntimeDeltaFrame(0.05)
	advanceRuntimeDeltaFrame(0.20)

	if got := tweenInfos[0].timer; got != 0.2 {
		t.Fatalf("tween timer = %v, want 0.2 after two fixed-delta updates", got)
	}
}

func TestInternalUpdateEngineKeepsRealDeltaWithoutFixedTimestep(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite
	tweenPos(sprite, mathf.NewVec2(1, 0), 1, func() {})

	advanceRuntimeDeltaFrame(0.05)
	advanceRuntimeDeltaFrame(0.20)

	if got := tweenInfos[0].timer; got != 0.25 {
		t.Fatalf("tween timer = %v, want 0.25 with real delta", got)
	}
}

func TestRuntimeBridgeTimersRespectObjectLifetime(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	bridge := runtimeBridge{}
	alive, removed := &gdx.Sprite{Id: 1}, &gdx.Sprite{Id: 2}
	node := &gdx.UiNode{Id: 3}
	state.sprites[1], state.sprites[2], state.uiNodes[3] = alive, removed, node
	if bridge.Sprites()[1] != alive || bridge.UiNodes()[3] != node || bridge.GetUINode(3) != node || !IsNodeExist(3) {
		t.Fatal("runtime registry adapters returned wrong nodes")
	}
	var calls []string
	bridge.DelayCall(.25, func() { calls = append(calls, "global") })
	bridge.DelaySpriteCall(.25, alive, func() { calls = append(calls, "alive") })
	bridge.DelaySpriteCall(.25, removed, func() { calls = append(calls, "removed") })
	bridge.DeleteSprite(2)
	bridge.DeleteUINode(3)
	bridge.InternalUpdateEngine(.125)
	if len(calls) != 0 {
		t.Fatal("timers fired before their deadline")
	}
	bridge.InternalUpdateEngine(.125)
	bridge.InternalUpdateEngine(1)
	if !slices.Equal(calls, []string{"global", "alive"}) || bridge.GetUINode(3) != nil {
		t.Fatalf("timer calls=%v, deleted node=%v", calls, bridge.GetUINode(3))
	}
	if bridge.AdvanceTimeSinceGameStart(.5) != .5 || bridge.AdvanceTimeSinceGameStart(.25) != .75 || bridge.TimeSinceGameStarted() != .75 {
		t.Fatal("runtime elapsed time did not accumulate")
	}
}

func TestRuntimeBridgeTimersDeferCallbacksAddedDuringUpdate(t *testing.T) {
	isolateGameBinding(t)
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	bridge := runtimeBridge{}
	var calls []int
	bridge.DelayCall(.1, func() {
		calls = append(calls, 1)
		bridge.DelayCall(0, func() { calls = append(calls, 4) })
	})
	bridge.DelayCall(.2, func() { calls = append(calls, 3) })
	bridge.DelayCall(.1, func() { calls = append(calls, 2) })

	bridge.InternalUpdateEngine(.1)
	if !slices.Equal(calls, []int{1, 2}) {
		t.Fatalf("first frame callbacks = %v, want [1 2]", calls)
	}
	bridge.InternalUpdateEngine(.1)
	if !slices.Equal(calls, []int{1, 2, 3, 4}) {
		t.Fatalf("second frame callbacks = %v, want [1 2 3 4]", calls)
	}
}

type frameCallbackSprite struct {
	gdx.Sprite
	position   mathf.Vec2
	update     func()
	onPosition func(mathf.Vec2)
}

func (s *frameCallbackSprite) GetPosition() mathf.Vec2 { return s.position }
func (s *frameCallbackSprite) SetPosition(value mathf.Vec2) {
	s.position = value
	if s.onPosition != nil {
		s.onPosition(value)
	}
}
func (s *frameCallbackSprite) OnUpdate(float64) {
	if s.update != nil {
		s.update()
	}
}

func TestFrameRuntimeCallbacksCanWaitAcrossFrames(t *testing.T) {
	for _, source := range []string{"timer", "tween", "sprite update"} {
		for _, wait := range []string{"time", "frame", "batch"} {
			t.Run(source+"/"+wait, func(t *testing.T) {
				co := setupRuntimeCallbackTest(t)
				sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
				state.sprites[1] = sprite
				deleted := &frameCallbackSprite{
					Sprite: gdx.Sprite{Id: 2},
					update: func() { t.Error("OnUpdate ran after its sprite was deleted") },
				}
				deleted.onPosition = func(mathf.Vec2) {
					// A synchronous backend deletion during tween advancement
					// happens after callback collection but before batch startup.
					delete(state.sprites, deleted.Id)
				}
				state.sprites[2] = deleted
				delaySpriteCall(0, deleted, func() { t.Error("timer ran after its sprite was deleted") })
				tweenPos(deleted, mathf.NewVec2(1, 2), .1, func() { t.Error("tween callback ran after its sprite was deleted") })
				var started, completed, followingTimer atomic.Int32
				callback := func() {
					sprite.update = nil
					started.Add(1)
					delayCall(0, func() { followingTimer.Add(1) })
					switch wait {
					case "time":
						Wait(0)
					case "frame":
						WaitNextFrame()
					case "batch":
						// BroadcastAndWait uses the same joined batch path.
						co.StartBatch([]coroutine.Task{{Run: func(coroutine.Thread) {
							WaitNextFrame()
						}}}, coroutine.BatchWaitDone)
					}
					completed.Add(1)
				}
				switch source {
				case "timer":
					delayCall(0, callback)
				case "tween":
					tweenPos(sprite, mathf.NewVec2(20, 30), .1, callback)
				case "sprite update":
					sprite.update = callback
				}

				runRuntimeCallbackFrame(t, co)
				if started.Load() != 1 || completed.Load() != 0 || followingTimer.Load() != 0 {
					t.Fatalf("first frame: started=%d completed=%d following timer=%d",
						started.Load(), completed.Load(), followingTimer.Load())
				}
				if len(tempDelaySpriteCalls) != 0 || len(tempTweenInfos) != 0 {
					t.Fatal("suspended callback retained frame scratch buffers")
				}
				if source == "tween" && sprite.position != mathf.NewVec2(20, 30) {
					t.Fatalf("tween state was not committed before its callback: %v", sprite.position)
				}
				runRuntimeCallbackFrame(t, co)
				if started.Load() != 1 || completed.Load() != 1 || followingTimer.Load() != 1 {
					t.Fatalf("second frame: started=%d completed=%d following timer=%d",
						started.Load(), completed.Load(), followingTimer.Load())
				}
			})
		}
	}
}

func TestRuntimeCallbackBatchPreservesOrderAcrossWaits(t *testing.T) {
	setupRuntimeCallbackTest(t)
	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite

	for range 20 {
		var got []int
		var resumed bool
		sprite.update = func() { got = append(got, 0) }
		want := []int{0}
		for i := range 32 {
			want = append(want, i+1)
			delayCall(0, func() {
				got = append(got, i+1)
				if i == 0 {
					WaitNextFrame()
					resumed = true
				}
			})
		}
		for i := range 32 {
			want = append(want, i+33)
			tweenPos(sprite, mathf.NewVec2(float64(i), 0), .1, func() {
				got = append(got, i+33)
			})
		}

		onUpdate(.1)
		if !slices.Equal(got, want) || resumed {
			t.Fatalf("first frame callbacks=%v resumed=%v, want ordered %v with the first timer suspended", got, resumed, want)
		}
		got = nil
		onUpdate(.1)
		if !resumed || !slices.Equal(got, []int{0}) {
			t.Fatalf("second frame callbacks=%v resumed=%v, want only the next sprite update and timer continuation", got, resumed)
		}
	}
}

func setupRuntimeCallbackTest(t *testing.T) *coroutine.Coroutines {
	t.Helper()
	isolateGameBinding(t)
	co := setupExecuteTest(t)
	isolateCaptureState(t)
	ResetInputState()
	resetTriggerEvents()
	resetRuntimeDeltaTestState()
	t.Cleanup(func() {
		co.RunAfterStop(nil)
		resetRuntimeDeltaTestState()
		ResetInputState()
		resetTriggerEvents()
	})
	game := &frameRecordingGame{step: func(string) {}}
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}
	return co
}

func runRuntimeCallbackFrame(t *testing.T, co *coroutine.Coroutines) {
	t.Helper()
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		onUpdate(.1)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		co.StopAll()
		waitForEngineSignal(t, returned, time.Second, "frame did not unwind after cancellation")
		t.Fatal("runtime callback wait blocked the frame dispatcher")
	}
}

func TestRuntimeCallbackDeletionCancelsSuspendedWork(t *testing.T) {
	for _, source := range []string{"timer", "tween"} {
		t.Run(source, func(t *testing.T) {
			co := setupRuntimeCallbackTest(t)
			removed := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
			alive := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 2}}
			state.sprites[1], state.sprites[2] = removed, alive
			var started, completed, other, global atomic.Int32
			callback := func() {
				started.Add(1)
				WaitNextFrame()
				completed.Add(1)
			}
			if source == "timer" {
				delaySpriteCall(0, removed, callback)
			} else {
				tweenPos(removed, mathf.NewVec2(1, 0), .1, callback)
			}
			delaySpriteCall(0, alive, func() {
				WaitNextFrame()
				other.Add(1)
			})
			delayCall(0, func() {
				WaitNextFrame()
				global.Add(1)
			})
			runRuntimeCallbackFrame(t, co)
			if started.Load() != 1 || completed.Load() != 0 {
				t.Fatal("object callback did not suspend in its first frame")
			}
			runtimeBridge{}.DeleteSprite(removed.Id)
			runRuntimeCallbackFrame(t, co)
			if completed.Load() != 0 || other.Load() != 1 || global.Load() != 1 {
				t.Fatalf("after deletion: removed=%d other=%d global=%d", completed.Load(), other.Load(), global.Load())
			}
		})
	}
}

func TestRuntimeCallbackProxyUsesLogicalOwner(t *testing.T) {
	for _, deletion := range []string{"logical owner", "raw deletion"} {
		t.Run(deletion, func(t *testing.T) {
			co := setupRuntimeCallbackTest(t)
			owner := new(int)
			proxy := &Sprite{Sprite: gdx.Sprite{Id: 1}, Target: owner}
			state.sprites[1] = proxy
			var completed atomic.Bool
			delaySpriteCall(0, proxy, func() {
				if currentThread().Owner != owner {
					t.Error("callback did not retain the logical sprite owner")
				}
				WaitNextFrame()
				completed.Store(true)
			})
			runRuntimeCallbackFrame(t, co)
			if deletion == "raw deletion" {
				runtimeBridge{}.DeleteSprite(proxy.Id)
			} else {
				co.StopIf(func(thread coroutine.Thread) bool { return thread.Owner == owner })
			}
			runRuntimeCallbackFrame(t, co)
			if completed.Load() {
				t.Fatal("logical sprite callback resumed after deletion")
			}
		})
	}
}

func TestRuntimeCallbackDeletionBeforeDeadline(t *testing.T) {
	for _, deletion := range []string{"pending proxy", "raw deletion", "reused id"} {
		t.Run(deletion, func(t *testing.T) {
			isolateGameBinding(t)
			resetRuntimeDeltaTestState()
			t.Cleanup(resetRuntimeDeltaTestState)
			alive := true
			proxy := &Sprite{Sprite: gdx.Sprite{Id: 1}, CallbackAlive: func() bool { return alive }}
			state.sprites[1] = proxy
			callback := func() { t.Error("deleted object's callback ran") }
			delaySpriteCall(1, proxy, callback)
			// Avoid backend calls: neither tween should advance after deletion.
			tweenInfos = append(tweenInfos, &tweenCallInfo{
				sprite: proxy, callback: callback, infos: []posTweenInfo{{duration: 1}},
			})
			switch deletion {
			case "pending proxy":
				alive = false
			case "raw deletion":
				runtimeBridge{}.DeleteSprite(proxy.Id)
			case "reused id":
				runtimeBridge{}.DeleteSprite(proxy.Id)
				state.sprites[1] = &gdx.Sprite{Id: 1}
			}
			runtimeBridge{}.InternalUpdateEngine(.1)
			if len(delaySpriteCalls) != 0 || len(tweenInfos) != 0 {
				t.Fatal("deleted object's pending timer or tween was retained")
			}
			runtimeBridge{}.InternalUpdateEngine(1)
		})
	}
}

func TestRuntimeCallbackContinuousUpdatesOverlapAndCancel(t *testing.T) {
	co := setupRuntimeCallbackTest(t)
	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite
	var started, completed, inFlight atomic.Int32
	sprite.update = func() {
		started.Add(1)
		inFlight.Add(1)
		defer inFlight.Add(-1)
		WaitNextFrame()
		WaitNextFrame()
		completed.Add(1)
	}
	for frame, want := range [][3]int32{{1, 0, 1}, {2, 0, 2}, {3, 1, 2}} {
		runRuntimeCallbackFrame(t, co)
		got := [3]int32{started.Load(), completed.Load(), inFlight.Load()}
		if got != want {
			t.Fatalf("frame %d: started/completed/in-flight=%v, want %v", frame+1, got, want)
		}
	}
	runtimeBridge{}.DeleteSprite(sprite.Id)
	runRuntimeCallbackFrame(t, co)
	co.RunAfterStop(nil)
	if started.Load() != 3 || completed.Load() != 1 || inFlight.Load() != 0 {
		t.Fatalf("after deletion: started=%d completed=%d in-flight=%d", started.Load(), completed.Load(), inFlight.Load())
	}
}

func TestRuntimeCallbackStandaloneTimerPrecedesTween(t *testing.T) {
	for _, binding := range []bool{false, true} {
		t.Run(map[bool]string{false: "no binding", true: "owner only"}[binding], func(t *testing.T) {
			isolateGameBinding(t)
			resetRuntimeDeltaTestState()
			t.Cleanup(resetRuntimeDeltaTestState)
			if binding {
				SetGame(new(int))
			}
			sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
			state.sprites[1] = sprite
			var calls []string
			delayCall(.1, func() {
				calls = append(calls, "timer")
				tweenPos(sprite, mathf.NewVec2(1, 0), .1, func() { calls = append(calls, "tween") })
			})
			runtimeBridge{}.InternalUpdateEngine(.1)
			if !slices.Equal(calls, []string{"timer", "tween"}) || sprite.position.X != 1 {
				t.Fatalf("callbacks=%v position=%v, want timer then tween at its endpoint", calls, sprite.position)
			}
		})
	}
}

func TestRuntimeCallbacksAcceptEmbeddedSpriteHandles(t *testing.T) {
	for _, source := range []string{"timer", "tween", "two-stage tween"} {
		t.Run(source, func(t *testing.T) {
			isolateGameBinding(t)
			resetRuntimeDeltaTestState()
			t.Cleanup(resetRuntimeDeltaTestState)
			sprite := &frameCallbackSprite{}
			gdx.InitSpriteInstance(1, sprite, func(id Object, value gdx.ISpriter) {
				state.sprites[id] = value
			})
			completed := 0
			callback := func() { completed++ }
			endpoint := mathf.NewVec2(10, 20)
			switch source {
			case "timer":
				gdx.DelaySpriteCall(.2, &sprite.Sprite, callback)
			case "tween":
				gdx.TweenPos(&sprite.Sprite, endpoint, .2, callback)
			case "two-stage tween":
				gdx.TweenPos2(&sprite.Sprite, mathf.NewVec2(5, 10), .1, endpoint, .1, callback)
			}
			gdx.InternalUpdateEngine(.2)
			if completed != 1 {
				t.Fatalf("live embedded handle completed %d times, want 1", completed)
			}
			// Tween operations must retain the registered sprite's overrides,
			// not dispatch directly to the embedded engine Sprite.
			if source != "timer" && sprite.position != endpoint {
				t.Fatalf("registered sprite position=%v, want %v", sprite.position, endpoint)
			}
		})
	}
}

type callbackReusableBase struct {
	gdx.Sprite
	completed bool
}

func (s *callbackReusableBase) OnStart() {
	gdx.DelaySpriteCall(.1, s, func() { s.completed = true })
}

type callbackDerivedSprite struct{ callbackReusableBase }

func TestRuntimeCallbackInheritedStartRetainsBaseReceiver(t *testing.T) {
	isolateGameBinding(t)
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	sprite := &callbackDerivedSprite{}
	gdx.InitSpriteInstance(1, sprite, func(id Object, value gdx.ISpriter) {
		state.sprites[id] = value
	})
	gdx.InternalUpdateEngine(.1)
	if !sprite.completed {
		t.Fatal("timer registered by inherited OnStart was discarded")
	}
}

func TestRuntimeCallbackEmbeddedHandlesCannotFollowReusedIDs(t *testing.T) {
	for _, source := range []string{"timer", "tween"} {
		for _, change := range []string{"missing", "replaced before registration", "replaced after registration"} {
			t.Run(source+"/"+change, func(t *testing.T) {
				isolateGameBinding(t)
				resetRuntimeDeltaTestState()
				t.Cleanup(resetRuntimeDeltaTestState)
				original := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
				replacement := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
				if change == "replaced before registration" {
					state.sprites[1] = replacement
				} else if change == "replaced after registration" {
					state.sprites[1] = original
				}
				callback := func() { t.Error("stale sprite's callback executed") }
				if source == "timer" {
					gdx.DelaySpriteCall(.1, &original.Sprite, callback)
				} else {
					gdx.TweenPos(&original.Sprite, mathf.NewVec2(10, 20), .1, callback)
				}
				if change == "replaced after registration" {
					state.sprites[1] = replacement
				}
				global := false
				gdx.DelayCall(.1, func() { global = true })
				gdx.InternalUpdateEngine(.1)
				if !global || len(delaySpriteCalls) != 0 || len(tweenInfos) != 0 {
					t.Fatalf("global=%v pending timers=%d tweens=%d", global, len(delaySpriteCalls), len(tweenInfos))
				}
				if original.position != (mathf.Vec2{}) || replacement.position != (mathf.Vec2{}) {
					t.Fatalf("stale tween moved original=%v replacement=%v", original.position, replacement.position)
				}
			})
		}
	}
}

type opaqueRuntimeSprite struct{ gdx.ISpriter }

type nilBaseRuntimeSprite struct{ gdx.Sprite }

func (*nilBaseRuntimeSprite) BaseSprite() *gdx.Sprite { return nil }

func TestResolveRuntimeSpriteRequiresSharedBase(t *testing.T) {
	isolateGameBinding(t)
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	base := &gdx.Sprite{Id: 1}
	opaque := &opaqueRuntimeSprite{ISpriter: base}
	noBase := &nilBaseRuntimeSprite{Sprite: gdx.Sprite{Id: 1}}
	otherNoBase := &nilBaseRuntimeSprite{Sprite: gdx.Sprite{Id: 1}}
	for _, test := range []struct {
		name       string
		registered gdx.ISpriter
		handle     gdx.ISpriter
		accepted   bool
	}{
		{"nil handle", base, nil, false},
		{"unregistered", nil, base, false},
		{"canonical without accessor", opaque, opaque, true},
		{"registered without accessor", opaque, base, false},
		{"handle without accessor", base, opaque, false},
		{"canonical with nil base", noBase, noBase, true},
		{"different nil bases", noBase, otherNoBase, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state.sprites[1] = test.registered
			want := test.registered
			if !test.accepted {
				want = nil
			}
			if got := resolveRuntimeSprite(test.handle); got != want {
				t.Fatalf("resolved=%v, want %v", got, want)
			}
		})
	}
}

func TestRuntimeCallbackEmbeddedProxyRetainsLogicalOwner(t *testing.T) {
	co := setupRuntimeCallbackTest(t)
	owner := new(int)
	proxy := &Sprite{Sprite: gdx.Sprite{Id: 1}, Target: owner}
	state.sprites[1] = proxy
	var started, completed atomic.Bool
	gdx.DelaySpriteCall(0, &proxy.Sprite, func() {
		if currentThread().Owner != owner {
			t.Error("embedded proxy callback lost its logical sprite owner")
		}
		started.Store(true)
		WaitNextFrame()
		completed.Store(true)
	})
	runRuntimeCallbackFrame(t, co)
	if !started.Load() {
		t.Fatal("embedded proxy callback did not start")
	}
	runtimeBridge{}.DeleteSprite(proxy.Id)
	runRuntimeCallbackFrame(t, co)
	if completed.Load() {
		t.Fatal("embedded proxy callback resumed after deletion")
	}
}

func TestRuntimeCallbackNilSpriteRemainsGlobal(t *testing.T) {
	isolateGameBinding(t)
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	completed := 0
	gdx.DelaySpriteCall(.1, nil, func() { completed++ })
	gdx.InternalUpdateEngine(.1)
	if completed != 1 {
		t.Fatalf("nil-sprite callback completed %d times, want 1", completed)
	}
}
