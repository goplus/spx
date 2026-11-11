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
	"maps"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type frameCallbackSprite struct {
	gdx.Sprite
	position   mathf.Vec2
	update     func()
	onPosition func(mathf.Vec2)
}

type callbackReusableBase struct {
	gdx.Sprite
	completed bool
}

type callbackDerivedSprite struct{ callbackReusableBase }

type opaqueRuntimeSprite struct{ gdx.ISpriter }

type nilBaseRuntimeSprite struct{ gdx.Sprite }

func TestInternalUpdateFrameUsesScriptDeltaForDelayCall(t *testing.T) {
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)

	clock.SetFixedDeltaTime(0.1)

	firedAt := 0
	delayCall(0.25, func() {
		if firedAt == 0 {
			firedAt = int(clock.Frame())
		}
	})

	for _, realDelta := range []float64{0.05, 0.20, 0.01} {
		advanceTestFrame(realDelta)
	}

	if firedAt != 3 {
		t.Fatalf("delayCall fired at frame %d, want 3 with fixed delta", firedAt)
	}
}

func TestInternalUpdateFrameUsesScriptDeltaForTweens(t *testing.T) {
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)

	clock.SetFixedDeltaTime(0.1)

	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite
	tweenPos(sprite, mathf.NewVec2(1, 0), 1, func() {})

	advanceTestFrame(0.05)
	advanceTestFrame(0.20)

	if got := state.jobs.tweens[0].elapsed; got != 0.2 {
		t.Fatalf("tween timer = %v, want 0.2 after two fixed-delta updates", got)
	}
}

func TestInternalUpdateFrameKeepsRealDeltaWithoutFixedTimestep(t *testing.T) {
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)

	sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	state.sprites[1] = sprite
	tweenPos(sprite, mathf.NewVec2(1, 0), 1, func() {})

	advanceTestFrame(0.05)
	advanceTestFrame(0.20)

	if got := state.jobs.tweens[0].elapsed; got != 0.25 {
		t.Fatalf("tween timer = %v, want 0.25 with real delta", got)
	}
}

func TestRuntimeBridgeTimersRespectObjectLifetime(t *testing.T) {
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
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
	bridge.InternalUpdateFrame(.125, nil)
	if len(calls) != 0 {
		t.Fatal("timers fired before their deadline")
	}
	bridge.InternalUpdateFrame(.125, nil)
	bridge.InternalUpdateFrame(1, nil)
	if !slices.Equal(calls, []string{"global", "alive"}) || bridge.GetUINode(3) != nil {
		t.Fatalf("timer calls=%v, deleted node=%v", calls, bridge.GetUINode(3))
	}
	if bridge.AdvanceTimeSinceGameStart(.5) != .5 || bridge.AdvanceTimeSinceGameStart(.25) != .75 || bridge.TimeSinceGameStarted() != .75 {
		t.Fatal("runtime elapsed time did not accumulate")
	}
}

func TestRuntimeBridgeTimersDeferCallbacksAddedDuringUpdate(t *testing.T) {
	isolateGameBinding(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
	bridge := runtimeBridge{}
	var calls []int
	bridge.DelayCall(.1, func() {
		calls = append(calls, 1)
		bridge.DelayCall(0, func() { calls = append(calls, 4) })
	})
	bridge.DelayCall(.2, func() { calls = append(calls, 3) })
	bridge.DelayCall(.1, func() { calls = append(calls, 2) })

	bridge.InternalUpdateFrame(.1, nil)
	if !slices.Equal(calls, []int{1, 2}) {
		t.Fatalf("first frame callbacks = %v, want [1 2]", calls)
	}
	bridge.InternalUpdateFrame(.1, nil)
	if !slices.Equal(calls, []int{1, 2, 3, 4}) {
		t.Fatalf("second frame callbacks = %v, want [1 2 3 4]", calls)
	}
}

func TestFrameRuntimeCallbacksCanWaitAcrossFrames(t *testing.T) {
	for _, source := range []string{"timer", "tween", "sprite update"} {
		for _, wait := range []string{"time", "frame", "batch"} {
			t.Run(source+"/"+wait, func(t *testing.T) {
				current := setupRuntimeCallbackTest(t)
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
						current.StartBatch([]coroutine.Task{{Run: func(coroutine.Thread) {
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

				runRuntimeCallbackFrame(t, current)
				if started.Load() != 1 || completed.Load() != 0 || followingTimer.Load() != 0 {
					t.Fatalf("first frame: started=%d completed=%d following timer=%d",
						started.Load(), completed.Load(), followingTimer.Load())
				}
				if len(state.jobs.timerScratch) != 0 || len(state.jobs.tweenScratch) != 0 {
					t.Fatal("suspended callback retained frame scratch buffers")
				}
				if source == "tween" && sprite.position != mathf.NewVec2(20, 30) {
					t.Fatalf("tween state was not committed before its callback: %v", sprite.position)
				}
				runRuntimeCallbackFrame(t, current)
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

func TestRuntimeCallbackDeletionCancelsSuspendedWork(t *testing.T) {
	for _, source := range []string{"timer", "tween"} {
		t.Run(source, func(t *testing.T) {
			current := setupRuntimeCallbackTest(t)
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
			runRuntimeCallbackFrame(t, current)
			if started.Load() != 1 || completed.Load() != 0 {
				t.Fatal("object callback did not suspend in its first frame")
			}
			runtimeBridge{}.DeleteSprite(removed.Id)
			runRuntimeCallbackFrame(t, current)
			if completed.Load() != 0 || other.Load() != 1 || global.Load() != 1 {
				t.Fatalf("after deletion: removed=%d other=%d global=%d", completed.Load(), other.Load(), global.Load())
			}
		})
	}
}

func TestRuntimeCallbackProxyUsesLogicalOwner(t *testing.T) {
	for _, deletion := range []string{"logical owner", "raw deletion", "embedded proxy"} {
		t.Run(deletion, func(t *testing.T) {
			current := setupRuntimeCallbackTest(t)
			owner := new(int)
			proxy := &Sprite{Sprite: gdx.Sprite{Id: 1}, Target: owner}
			state.sprites[1] = proxy
			var handle gdx.ISpriter = proxy
			if deletion == "embedded proxy" {
				handle = &proxy.Sprite
			}
			var started, completed atomic.Bool
			gdx.DelaySpriteCall(0, handle, func() {
				if scheduler.Caller().Owner != owner {
					t.Error("callback did not retain the logical sprite owner")
				}
				started.Store(true)
				WaitNextFrame()
				completed.Store(true)
			})
			runRuntimeCallbackFrame(t, current)
			if !started.Load() || completed.Load() {
				t.Fatal("proxy callback did not suspend in its first frame")
			}
			if deletion == "logical owner" {
				current.StopIf(func(thread coroutine.Thread) bool { return thread.Owner == owner })
			} else {
				runtimeBridge{}.DeleteSprite(proxy.Id)
			}
			runRuntimeCallbackFrame(t, current)
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
			resetFrameTestState()
			t.Cleanup(resetFrameTestState)
			alive := true
			proxy := &Sprite{Sprite: gdx.Sprite{Id: 1}, CallbackAlive: func() bool { return alive }}
			state.sprites[1] = proxy
			callback := func() { t.Error("deleted object's callback ran") }
			delaySpriteCall(1, proxy, callback)
			// Avoid backend calls: neither tween should advance after deletion.
			state.jobs.tweens = append(state.jobs.tweens, &positionTween{
				sprite: proxy, callback: callback, steps: []tweenStep{{duration: 1}},
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
			runtimeBridge{}.InternalUpdateFrame(.1, nil)
			if len(state.jobs.timers) != 0 || len(state.jobs.tweens) != 0 {
				t.Fatal("deleted object's pending timer or tween was retained")
			}
			runtimeBridge{}.InternalUpdateFrame(1, nil)
		})
	}
}

func TestRuntimeCallbackContinuousUpdatesOverlapAndCancel(t *testing.T) {
	current := setupRuntimeCallbackTest(t)
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
		runRuntimeCallbackFrame(t, current)
		got := [3]int32{started.Load(), completed.Load(), inFlight.Load()}
		if got != want {
			t.Fatalf("frame %d: started/completed/in-flight=%v, want %v", frame+1, got, want)
		}
	}
	runtimeBridge{}.DeleteSprite(sprite.Id)
	runRuntimeCallbackFrame(t, current)
	current.RunAfterStop(nil)
	if started.Load() != 3 || completed.Load() != 1 || inFlight.Load() != 0 {
		t.Fatalf("after deletion: started=%d completed=%d in-flight=%d", started.Load(), completed.Load(), inFlight.Load())
	}
}

func TestInternalUpdateFramePreservesStandaloneOrder(t *testing.T) {
	for _, binding := range []bool{false, true} {
		t.Run(map[bool]string{false: "no binding", true: "owner only"}[binding], func(t *testing.T) {
			isolateGameBinding(t)
			resetFrameTestState()
			t.Cleanup(resetFrameTestState)
			if binding {
				SetGame(new(int))
			}

			var calls []string
			sprite := &frameCallbackSprite{
				Sprite: gdx.Sprite{Id: 1},
				update: func() { calls = append(calls, "sprite") },
			}
			state.sprites[sprite.Id] = sprite
			bridge := runtimeBridge{}
			bridge.InternalUpdateFrame(.25, func(delta float64) {
				if delta != .25 {
					t.Errorf("user callback delta = %v, want .25", delta)
				}
				calls = append(calls, "user")
				bridge.DelayCall(.25, func() {
					calls = append(calls, "timer")
					bridge.TweenPos(sprite, mathf.NewVec2(1, 0), .25, func() {
						calls = append(calls, "tween")
					})
				})
				bridge.DelayCall(.5, func() { calls = append(calls, "next-timer") })
			})
			want := []string{"sprite", "user", "timer", "tween"}
			if !slices.Equal(calls, want) || sprite.position.X != 1 {
				t.Fatalf("frame callbacks=%v position=%v, want %v and completed tween", calls, sprite.position, want)
			}

			// Each frame updates the sprite and advances timers once.
			bridge.InternalUpdateFrame(.25, nil)
			want = append(want, "sprite", "next-timer")
			if !slices.Equal(calls, want) {
				t.Fatalf("next frame callbacks = %v, want %v", calls, want)
			}
		})
	}
}

func TestInternalUpdateFramePreservesStandaloneSpriteSnapshotAcrossNestedFrames(t *testing.T) {
	isolateGameBinding(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)

	bridge := runtimeBridge{}
	var calls []Object
	added := &frameCallbackSprite{
		Sprite: gdx.Sprite{Id: 4},
		update: func() { calls = append(calls, 4) },
	}
	for _, id := range []Object{1, 2, 3} {
		state.sprites[id] = &frameCallbackSprite{
			Sprite: gdx.Sprite{Id: id},
			update: func() {
				calls = append(calls, id)
				if len(calls) == 1 {
					// Mutate the registry from whichever snapshot member runs first.
					for otherID := range state.sprites {
						bridge.DeleteSprite(otherID)
					}
					state.sprites[added.Id] = added
					// A nested frame must not overwrite the outer frame's snapshot.
					bridge.InternalUpdateFrame(.25, nil)
				}
			},
		}
	}

	bridge.InternalUpdateFrame(.25, nil)
	slices.Sort(calls)
	if !slices.Equal(calls, []Object{1, 2, 3, 4}) {
		t.Fatalf("first frame updated sprites = %v, want original snapshot plus nested frame [1 2 3 4]", calls)
	}

	calls = nil
	bridge.InternalUpdateFrame(.25, nil)
	if !slices.Equal(calls, []Object{4}) {
		t.Fatalf("next frame updated sprites = %v, want newly registered sprite [4]", calls)
	}
}

func TestFrameJobsPreservePendingCallbacksAcrossNestedFrames(t *testing.T) {
	for _, source := range []string{"timer", "tween"} {
		t.Run(source, func(t *testing.T) {
			isolateGameBinding(t)
			resetFrameTestState()
			t.Cleanup(resetFrameTestState)
			bridge := runtimeBridge{}
			sprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
			registerSprite(1, sprite)
			add := func(delay float64, callback func()) {
				if source == "timer" {
					bridge.DelayCall(delay, callback)
				} else {
					bridge.TweenPos(sprite, mathf.NewVec2(10, 0), delay, callback)
				}
			}
			var calls []string
			add(1, func() {
				calls = append(calls, "first")
				add(0, func() { calls = append(calls, "nested") })
				bridge.InternalUpdateFrame(.5, nil)
			})
			add(1.5, func() { calls = append(calls, "pending") })
			add(1, func() { calls = append(calls, "second") })
			bridge.InternalUpdateFrame(1, nil)
			bridge.InternalUpdateFrame(1, nil)
			want := []string{"first", "pending", "nested", "second"}
			if !slices.Equal(calls, want) {
				t.Fatalf("nested frame callbacks = %v, want %v", calls, want)
			}
		})
	}
}

func TestRuntimeCallbacksAcceptEmbeddedSpriteHandles(t *testing.T) {
	for _, source := range []string{"timer", "tween", "two-stage tween"} {
		t.Run(source, func(t *testing.T) {
			isolateGameBinding(t)
			resetFrameTestState()
			t.Cleanup(resetFrameTestState)
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
			gdx.InternalUpdateFrame(.2, nil)
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

func TestRuntimeCallbackInheritedStartRetainsBaseReceiver(t *testing.T) {
	isolateGameBinding(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
	sprite := &callbackDerivedSprite{}
	gdx.InitSpriteInstance(1, sprite, func(id Object, value gdx.ISpriter) {
		state.sprites[id] = value
	})
	gdx.InternalUpdateFrame(.1, nil)
	if !sprite.completed {
		t.Fatal("timer registered by inherited OnStart was discarded")
	}
}

func TestRuntimeCallbackEmbeddedHandlesCannotFollowReusedIDs(t *testing.T) {
	for _, source := range []string{"timer", "tween"} {
		for _, change := range []string{"missing", "replaced before registration", "replaced after registration"} {
			t.Run(source+"/"+change, func(t *testing.T) {
				isolateGameBinding(t)
				resetFrameTestState()
				t.Cleanup(resetFrameTestState)
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
				gdx.InternalUpdateFrame(.1, nil)
				if !global || len(state.jobs.timers) != 0 || len(state.jobs.tweens) != 0 {
					t.Fatalf("global=%v pending timers=%d tweens=%d", global, len(state.jobs.timers), len(state.jobs.tweens))
				}
				if original.position != (mathf.Vec2{}) || replacement.position != (mathf.Vec2{}) {
					t.Fatalf("stale tween moved original=%v replacement=%v", original.position, replacement.position)
				}
			})
		}
	}
}

func TestResolveRuntimeSpriteRequiresSharedBase(t *testing.T) {
	isolateGameBinding(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
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

func TestRuntimeCallbackNilSpriteRemainsGlobal(t *testing.T) {
	isolateGameBinding(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
	completed := 0
	gdx.DelaySpriteCall(.1, nil, func() { completed++ })
	gdx.InternalUpdateFrame(.1, nil)
	if completed != 1 {
		t.Fatalf("nil-sprite callback completed %d times, want 1", completed)
	}
}

func TestRuntimeStateResetPreservesRegistrationsAndMapViews(t *testing.T) {
	isolateGameBinding(t)
	previous, previousTypes := state, spriteTypes
	state = runtimeState{
		sprites: make(map[Object]gdx.ISpriter),
		uiNodes: make(map[Object]gdx.IUiNode),
	}
	spriteTypes = maps.Clone(spriteTypes)
	t.Cleanup(func() { state, spriteTypes = previous, previousTypes })

	bridge := runtimeBridge{}
	sprites, nodes := bridge.Sprites(), bridge.UiNodes()
	spriteType := reflect.TypeFor[frameCallbackSprite]()
	bridge.RegisterSpriteType(spriteType)
	oldSprite := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	registerSprite(1, oldSprite)
	registerUINode(2, &gdx.UiNode{Id: 2})
	bridge.DelayCall(1, func() { t.Error("previous session's global timer ran") })
	bridge.TweenPos(oldSprite, mathf.NewVec2(10, 0), 1, func() {
		t.Error("previous session's tween callback ran")
	})
	bridge.InternalUpdateFrame(.25, nil)
	bridge.AdvanceTimeSinceGameStart(.25)
	state.layerSortMode = layerSortModeVertical
	oldPosition := oldSprite.position

	resetRuntimeState()
	if len(sprites) != 0 || len(nodes) != 0 || bridge.TimeSinceGameStarted() != 0 || HasLayerSortMethod() {
		t.Fatal("session reset retained objects, elapsed time, or layer sorting")
	}
	if len(state.jobs.timers) != 0 || len(state.jobs.tweens) != 0 {
		t.Fatal("session reset retained queued frame jobs")
	}
	if spriteTypes[spriteType.Name()] != spriteType {
		t.Fatal("session reset lost the process type registration")
	}

	replacement := &frameCallbackSprite{Sprite: gdx.Sprite{Id: 1}}
	node := &gdx.UiNode{Id: 2}
	registerSprite(1, replacement)
	registerUINode(2, node)
	if sprites[1] != replacement || nodes[2] != node {
		t.Fatal("session reset replaced legacy live registry maps")
	}
	var calls []string
	bridge.DelayCall(1, func() { calls = append(calls, "timer") })
	bridge.TweenPos(replacement, mathf.NewVec2(20, 0), 1, func() { calls = append(calls, "tween") })
	bridge.InternalUpdateFrame(1, nil)
	if !slices.Equal(calls, []string{"timer", "tween"}) || replacement.position.X != 20 || oldSprite.position != oldPosition {
		t.Fatalf("new session callbacks=%v, positions old=%v new=%v", calls, oldSprite.position, replacement.position)
	}

	resetRuntimeState()
	resetRuntimeState()
	if len(sprites) != 0 || len(nodes) != 0 {
		t.Fatal("repeated reset did not leave live map views empty")
	}
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

func (s *callbackReusableBase) OnStart() {
	gdx.DelaySpriteCall(.1, s, func() { s.completed = true })
}

func (*nilBaseRuntimeSprite) BaseSprite() *gdx.Sprite { return nil }

func resetFrameTestState() {
	resetRuntimeState()
	clock.SetFixedDeltaTime(0)
	clock.Start()
}

func advanceTestFrame(realDelta float64) {
	logicalDelta := clock.EffectiveLogicalDeltaTime(realDelta)
	clock.Update(logicalDelta, 60)
	runtimeBridge{}.InternalUpdateFrame(logicalDelta, nil)
}

func setupRuntimeCallbackTest(t *testing.T) *coroutine.Scheduler {
	t.Helper()
	isolateGameBinding(t)
	current := setupExecuteTest(t)
	isolateCaptureState(t)
	resetInputState()
	resetTriggerEvents()
	resetFrameTestState()
	t.Cleanup(func() {
		current.RunAfterStop(nil)
		resetFrameTestState()
		resetInputState()
		resetTriggerEvents()
	})
	game := &frameRecordingGame{step: func(string) {}}
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}
	return current
}

func runRuntimeCallbackFrame(t *testing.T, current *coroutine.Scheduler) {
	t.Helper()
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		onUpdate(.1)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		current.StopAll()
		waitForEngineSignal(t, returned, time.Second, "frame did not unwind after cancellation")
		t.Fatal("runtime callback wait blocked the frame dispatcher")
	}
}
