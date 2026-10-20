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
	"slices"
	"testing"

	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
)

// Stamp-driven games broadcast to many clones each frame.
func BenchmarkTargetOrder(b *testing.B) {
	game := &Game{}
	game.initShapeMgr()
	sinks := make([]eventSink, 0, 202)
	for range 100 {
		sprite := &SpriteImpl{g: game}
		game.shapeMgr.add(sprite)
		sinks = append(sinks, eventSink{Owner: sprite}, eventSink{Owner: sprite})
	}
	sinks = append(sinks, eventSink{Owner: game}, eventSink{Owner: "external"})
	b.ReportAllocs()
	for b.Loop() {
		if got := sinksInTargetOrder(game, sinks); len(got) != len(sinks) {
			b.Fatal(len(got))
		}
	}
}

func BenchmarkTargetDispatch(b *testing.B) {
	previous := gco
	gco = nil
	defer func() { gco = previous }()
	for _, scenario := range []struct {
		name           string
		count, matches int
	}{
		{"1/missing", 1, 0},
		{"1/present", 1, 1},
		{"100/missing", 100, 0},
		{"100/present", 100, 1},
		{"300/missing", 300, 0},
		{"300/present", 300, 1},
		{"300/mixed-64", 300, 64},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			var registry scriptEventRegistry
			owner := &SpriteImpl{}
			for i := range scenario.count {
				sink := eventSink{Owner: &SpriteImpl{}}
				if i >= scenario.count-scenario.matches {
					sink.Owner = owner
					if scenario.matches > 1 && i%2 == 0 {
						sink.Cond = func(any) bool { return false }
					}
				}
				registry.manager.Add(coreevent.BucketTouchStart, sink)
			}
			event := scriptEventDispatch{run: func(coroutine.Thread, *eventSink) {}}
			b.ReportAllocs()
			for b.Loop() {
				registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
			}
		})
	}
}

func BenchmarkTargetDispatchMissingManaged(b *testing.B) {
	var registry scriptEventRegistry
	for range 300 {
		registry.manager.Add(coreevent.BucketTouchStart, eventSink{Owner: &SpriteImpl{}})
	}
	owner := &SpriteImpl{}
	event := scriptEventDispatch{mode: coroutine.BatchAsync}
	b.ReportAllocs()
	for b.Loop() {
		registry.dispatchTarget(coreevent.BucketTouchStart, owner, event)
	}
}

func TestTargetDispatchMutation(t *testing.T) {
	previous := gco
	gco = nil
	defer func() { gco = previous }()

	var registry scriptEventRegistry
	owner, other := &SpriteImpl{}, &SpriteImpl{}
	var calls []string
	enabled, nested := true, false
	var event scriptEventDispatch
	event.run = func(_ coroutine.Thread, sink *eventSink) {
		name := sink.Handler.(string)
		calls = append(calls, name)
		if name == "first" && !nested {
			nested = true
			registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "second"})
			registry.dispatchTarget(coreevent.BucketClick, owner, event)
		}
		if name == "delete" {
			registry.manager.DeleteOwner(owner)
		}
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: other, Handler: "other"})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: owner, Handler: "first", Cond: func(any) bool { return enabled },
	})

	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("nested dispatch = %v, want %v", calls, want)
	}
	enabled = false
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second"}; !slices.Equal(calls, want) {
		t.Fatalf("condition change = %v, want %v", calls, want)
	}
	registry.manager.DeleteOwner(owner)
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 4 {
		t.Fatalf("deleted owner received event: %v", calls)
	}
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "delete"})
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "after"})
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if want := []string{"first", "first", "second", "second", "delete", "after"}; !slices.Equal(calls, want) {
		t.Fatalf("deleted during dispatch = %v, want %v", calls, want)
	}
	registry.dispatchTarget(coreevent.BucketClick, owner, event)
	if len(calls) != 6 {
		t.Fatalf("deleted handlers received another event: %v", calls)
	}
}

func TestTargetDispatchCompletesMatchingBeforeHandlers(t *testing.T) {
	setupRuntimeScheduler(t)

	var registry scriptEventRegistry
	owner, other := &SpriteImpl{}, &SpriteImpl{}
	enabled := true
	var steps []string
	registry.manager.Add(coreevent.BucketClick, eventSink{Owner: owner, Handler: "first"})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: other,
		Cond: func(any) bool {
			steps = append(steps, "check-other")
			return false
		},
	})
	registry.manager.Add(coreevent.BucketClick, eventSink{
		Owner: owner,
		Cond: func(any) bool {
			steps = append(steps, "check-second")
			return enabled
		},
		Handler: "second",
	})

	registry.dispatchTarget(coreevent.BucketClick, owner, scriptEventDispatch{
		mode: coroutine.BatchWaitDone,
		run: func(_ coroutine.Thread, sink *eventSink) {
			name := sink.Handler.(string)
			steps = append(steps, "run-"+name)
			if name == "first" {
				enabled = false
			}
		},
	})

	if want := []string{"check-second", "run-first", "run-second"}; !slices.Equal(steps, want) {
		t.Fatalf("dispatch steps = %v, want %v", steps, want)
	}
}

func TestStartBatchCleansPendingThreadCanceledBeforeRun(t *testing.T) {
	co := setupRuntimeScheduler(t)
	var registry scriptEventRegistry
	var canceled coroutine.Thread
	var pendingBeforeCancel, runningMarkedPending, canceledRan bool
	dispatcher := co.Create("dispatcher", func(coroutine.Thread) {
		registry.dispatchStartEventBatch([]eventSink{{Owner: "first"}, {Owner: "canceled"}}, scriptEventDispatch{
			mode: coroutine.BatchWaitFirstSlice,
			run: func(thread coroutine.Thread, sink *eventSink) {
				if sink.Owner == "canceled" {
					canceledRan = true
					return
				}
				runningMarkedPending = registry.isPendingStartThread(thread)
				co.StopIf(func(thread coroutine.Thread) bool {
					if thread.Owner != "canceled" {
						return false
					}
					canceled = thread
					pendingBeforeCancel = registry.isPendingStartThread(thread)
					return true
				})
			},
		})
	})
	co.Join(dispatcher)
	if canceled == nil || !pendingBeforeCancel {
		t.Fatal("later start thread was not registered as pending before cancellation")
	}
	co.Join(canceled)
	if canceledRan {
		t.Fatal("canceled start handler ran")
	}
	if registry.isPendingStartThread(canceled) {
		t.Fatal("canceled start thread retained its pending marker after cleanup")
	}
	if runningMarkedPending {
		t.Fatal("running start handler retained its pending marker")
	}
}

func TestTargetOrderPreservesGroupsAndSnapshot(t *testing.T) {
	game := &Game{}
	game.initShapeMgr()
	back, front := &SpriteImpl{g: game}, &SpriteImpl{g: game}
	removed := &SpriteImpl{g: game}
	game.shapeMgr.add(back)
	game.shapeMgr.add(front)
	sinks := []eventSink{
		{Owner: game, Handler: "stage-1"},
		{Owner: back, Handler: "back-1"},
		{Owner: removed, Handler: "removed"},
		{Owner: front, Handler: "front-1"},
		{Owner: []int{1}, Handler: "external"},
		{Owner: back, Handler: "back-2"},
		{Owner: game, Handler: "stage-2"},
		{Owner: front, Handler: "front-2"},
		{Owner: &Game{}, Handler: "other-game"},
	}
	snapshot := slices.Clone(sinks)
	ordered := sinksInTargetOrder(game, sinks)
	names := make([]string, len(ordered))
	for i, sink := range ordered {
		names[i] = sink.Handler.(string)
	}
	want := []string{"front-1", "front-2", "back-1", "back-2", "removed", "external", "other-game", "stage-1", "stage-2"}
	if !slices.Equal(names, want) {
		t.Fatalf("order = %v, want %v", names, want)
	}
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("sorting modified the shared event snapshot")
	}
	ordered[0].Handler = "changed"
	if !reflect.DeepEqual(sinks, snapshot) {
		t.Fatal("ordered result aliases the shared event snapshot")
	}
	// A later broadcast must observe the new layer order, not a cached grouping.
	game.shapeMgr.items[0], game.shapeMgr.items[1] = front, back
	if got := sinksInTargetOrder(game, sinks)[0].Handler; got != "back-1" {
		t.Fatalf("first handler after layer change = %v", got)
	}
}

func TestEventHandlerNonMatchingTriggerPreservesInvocation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*Game, func())
		fire     func(*Game, bool)
	}{
		{"key", func(g *Game, run func()) { g.OnKey__0(KeySpace, run) }, func(g *Game, match bool) {
			key := KeyA
			if match {
				key = KeySpace
			}
			g.scriptEvents.doWhenKeyPressed(key)
		}},
		{"click", func(g *Game, run func()) { g.OnClick(run) }, func(g *Game, match bool) {
			if match {
				g.scriptEvents.doWhenClick(g)
			} else {
				g.scriptEvents.doWhenClick(&SpriteImpl{g: g})
			}
		}},
		{"backdrop", func(g *Game, run func()) { g.OnBackdrop__1("scene", run) }, func(g *Game, match bool) {
			name := "other"
			if match {
				name = "scene"
			}
			g.scriptEvents.doWhenBackdropChanged(name, false)
		}},
		{"message", func(g *Game, run func()) { g.OnMsg__1("msg", run) }, func(g *Game, match bool) {
			msg := "other"
			if match {
				msg = "msg"
			}
			g.Broadcast__0(msg)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			tc.register(game, func() { started++; gate.Wait(); finished++ })
			tc.fire(game, true)
			co.Update()
			tc.fire(game, false)
			co.Update()
			gate.Open()
			co.Update()
			if started != 1 || finished != 1 {
				t.Fatalf("non-matching trigger affected invocation: started=%d finished=%d", started, finished)
			}
		})
	}
}

func TestKeyHandlerSharesAdmissionAcrossRegisteredKeys(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	var received []Key
	keys := []Key{KeySpace, KeyA}
	game.OnKey__1(keys, func(key Key) {
		received = append(received, key)
		gate.Wait()
	})
	keys[1] = KeyB
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyA)
	co.Update()
	if len(received) != 1 || received[0] != KeySpace {
		t.Fatalf("keys did not share one registration: %v", received)
	}
	gate.Open()
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyB)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeyA)
	co.Update()
	if len(received) != 2 || received[1] != KeyA {
		t.Fatalf("registration did not retain its original keys: %v", received)
	}
}

func TestTargetEventHandlersStayWithTheirOwner(t *testing.T) {
	for _, tc := range []struct {
		name     string
		register func(*SpriteImpl, func())
		fire     func(*SpriteImpl, *SpriteImpl)
	}{
		{"click", func(s *SpriteImpl, run func()) { s.OnClick(run) }, func(s, _ *SpriteImpl) {
			s.doWhenClick(s)
		}},
		{"swipe", func(s *SpriteImpl, run func()) { s.OnSwipe__0(90, run) }, func(s, _ *SpriteImpl) {
			s.doWhenSwipe(90, s)
		}},
		{"cloned", func(s *SpriteImpl, run func()) { s.OnCloned__0(run) }, func(s, _ *SpriteImpl) {
			s.doWhenCloned(s, nil)
		}},
		{"touch", func(s *SpriteImpl, run func()) { s.addTouchStartHandler(func(Sprite) { run() }) }, func(s, other *SpriteImpl) {
			s.fireTouchStart(other)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			first := &newTouchEventSprite(game, "first").SpriteImpl
			second := &newTouchEventSprite(game, "second").SpriteImpl
			var calls [2]int
			tc.register(first, func() { calls[0]++ })
			tc.register(second, func() { calls[1]++ })
			tc.fire(first, second)
			co.Update()
			if calls != [2]int{1, 0} {
				t.Fatalf("first target: calls=%v, want [1 0]", calls)
			}
			tc.fire(second, first)
			co.Update()
			if calls != [2]int{1, 1} {
				t.Fatalf("second target: calls=%v, want [1 1]", calls)
			}
		})
	}
}

func TestEventHandlerRetriggerPolicies(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		wantStarted  int
		wantFinished int
	}{
		{"key", 1, 1},
		{"any key", 1, 1},
		{"click", 2, 1},
		{"backdrop", 2, 1},
		{"any backdrop", 2, 1},
		{"message", 2, 1},
		{"any message", 2, 1},
		{"timer", 2, 2},
		{"swipe", 2, 2},
		{"cloned", 2, 2},
		{"touch", 2, 2},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			var fire func()
			switch tc.kind {
			case "key":
				game.OnKey__0(KeySpace, run)
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			case "any key":
				game.OnAnyKey(func(Key) { run() })
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			case "click":
				game.OnClick(run)
				fire = func() { game.scriptEvents.doWhenClick(game) }
			case "backdrop":
				game.OnBackdrop__1("scene", run)
				fire = func() { game.scriptEvents.doWhenBackdropChanged("scene", false) }
			case "any backdrop":
				game.OnBackdrop__0(func(BackdropName) { run() })
				fire = func() { game.scriptEvents.doWhenBackdropChanged("scene", false) }
			case "message":
				game.OnMsg__1("msg", run)
				fire = func() { game.Broadcast__0("msg") }
			case "any message":
				game.OnMsg__0(func(MsgName, any) { run() })
				fire = func() { game.Broadcast__0("msg") }
			case "timer":
				itime.ClearTimers()
				t.Cleanup(itime.ClearTimers)
				game.OnTimer(0, run)
				fire = func() { game.scriptEvents.doWhenTimer(0) }
			case "swipe":
				game.OnSwipe__0(90, run)
				fire = func() { game.scriptEvents.doWhenSwipe(90, game) }
			case "cloned":
				sprite := newTouchEventSprite(game, "clone")
				sprite.OnCloned__0(run)
				fire = func() { sprite.doWhenCloned(&sprite.SpriteImpl, nil) }
			case "touch":
				sprite := newTouchEventSprite(game, "source")
				other := newTouchEventSprite(game, "target")
				sprite.addTouchStartHandler(func(Sprite) { run() })
				fire = func() { sprite.fireTouchStart(&other.SpriteImpl) }
			}
			fire()
			co.Update()
			if started != 1 || finished != 0 {
				t.Fatalf("first trigger: started=%d finished=%d, want 1/0", started, finished)
			}
			fire()
			co.Update()
			if started != tc.wantStarted || finished != 0 {
				t.Fatalf("retrigger: started=%d finished=%d, want %d/0", started, finished, tc.wantStarted)
			}
			gate.Open()
			co.Update()
			if finished != tc.wantFinished {
				t.Fatalf("finished %d scripts, want %d", finished, tc.wantFinished)
			}
			fire()
			co.Update()
			if started != tc.wantStarted+1 || finished != tc.wantFinished+1 {
				t.Fatalf("after completion: started=%d finished=%d, want %d/%d", started, finished, tc.wantStarted+1, tc.wantFinished+1)
			}
		})
	}
}

func TestKeyHandlerIdentityIncludesRegistrationAndOwner(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	started := 0
	run := func() { started++; gate.Wait() }
	game.OnKey__0(KeySpace, run)
	game.OnKey__0(KeySpace, run)
	clone := &SpriteImpl{g: game}
	clone.spriteState.Cloned = true
	clone.scriptEventBindings.bind(&game.scriptEvents, clone)
	clone.OnKey__0(KeySpace, run)
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	if started != 3 {
		t.Fatalf("started %d scripts, want one per registration and owner", started)
	}
	co.StopIf(func(th coroutine.Thread) bool { return th.Owner == clone })
	game.scriptEvents.doWhenKeyPressed(KeySpace)
	co.Update()
	if started != 4 {
		t.Fatalf("canceled clone handler did not restart independently: %d", started)
	}
	gate.Open()
	co.Update()
}

// Retriggers may arrive before a handler gets its first slice. Replacing a
// canceled registration must not let its delayed cleanup clear the new one.
func TestEventHandlerQueuedRetriggers(t *testing.T) {
	for _, key := range []bool{false, true} {
		t.Run(map[bool]string{false: "restart", true: "ignore"}[key], func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			gate := co.NewLatch()
			started, finished := 0, 0
			run := func() { started++; gate.Wait(); finished++ }
			fire := func() { game.scriptEvents.doWhenClick(game) }
			if key {
				game.OnKey__0(KeySpace, run)
				fire = func() { game.scriptEvents.doWhenKeyPressed(KeySpace) }
			} else {
				game.OnClick(run)
			}
			parent := co.Create(game, func(coroutine.Thread) {
				fire()
				fire()
				fire()
			})
			co.Join(parent)
			co.Update()
			if started != 1 {
				t.Fatalf("queued retriggers started %d handlers", started)
			}
			fire()
			co.Update()
			want := 2
			if key {
				want = 1
			}
			if started != want {
				t.Fatalf("retrigger started %d, want %d", started, want)
			}
			gate.Open()
			co.Update()
			if finished != 1 {
				t.Fatalf("finished %d handlers", finished)
			}
		})
	}
}

func TestIndependentBroadcastTreeDoesNotInheritReceiverTurn(t *testing.T) {
	for _, fromScript := range []bool{false, true} {
		name := "external"
		if fromScript {
			name = "script"
		}
		t.Run(name, func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			calls := 0
			game.OnMsg__1("cycle", func() {
				calls++
				if calls == 1 {
					game.Broadcast__0("cycle")
				}
			})

			game.Broadcast__0("cycle")
			co.Update()
			if calls != 1 {
				t.Fatalf("nested broadcast calls = %d, want repeated receiver deferred", calls)
			}

			// A new root restarts the deferred receiver without advancing the
			// frame or sharing the first tree's claimed receiver turn.
			if fromScript {
				threads := co.StartBatch([]coroutine.Task{{
					Owner: game,
					Run:   func(coroutine.Thread) { game.Broadcast__0("cycle") },
				}}, coroutine.BatchAsync)
				co.JoinAll(threads)
			} else {
				game.Broadcast__0("cycle")
			}
			co.Update()
			if calls != 2 {
				t.Fatalf("independent broadcast calls = %d, want new root to run in the same frame", calls)
			}
		})
	}
}

func TestMessageExecutionContextReleasedOnCancelAndReset(t *testing.T) {
	co, game := setupRuntimeEventGame(t)
	gate := co.NewLatch()
	started, cleaned := 0, 0
	for range 3 {
		game.OnMsg__1("held", func() {
			started++
			defer func() { cleaned++ }()
			gate.Wait()
		})
	}

	game.Broadcast__0("held")
	co.Update()
	if started != 3 {
		t.Fatalf("started receivers = %d, want 3", started)
	}
	co.RunAfterStop(nil)
	if cleaned != 3 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after cancellation: cleaned = %d, retained contexts = %d", cleaned, len(game.scriptEvents.messageExecutions))
	}

	game.scriptEvents.manager.Reset()
	calls := 0
	game.OnMsg__1("held", func() { calls++ })
	game.BroadcastAndWait__0("held")
	if calls != 1 || len(game.scriptEvents.messageExecutions) != 0 {
		t.Fatalf("after reset and completion: calls = %d, retained contexts = %d", calls, len(game.scriptEvents.messageExecutions))
	}
}
