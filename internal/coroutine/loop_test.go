/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package coroutine

import (
	"sync/atomic"
	"testing"
	stime "time"

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestLoopCheckpointInlinesSingleScriptWithinBudget(t *testing.T) {
	co := New(nil)
	co.SetPerfDebug(true)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	const iterations = 128
	var completed atomic.Int64
	th := co.Create("single-loop", func(me Thread) {
		for range iterations {
			completed.Add(1)
			co.YieldLoopFor(me)
		}
	})
	co.JoinFirstSlice(th)
	co.Update()

	if got := completed.Load(); got != iterations {
		t.Fatalf("completed iterations = %d, want %d", got, iterations)
	}
	// 第一次继续需要真实轮次，后续检查点应当直接继续。
	if jobs := co.UpdateStats().JobCount; jobs > 3 {
		t.Fatalf("processed jobs = %d, want at most 3 for inlined loop", jobs)
	}
	stats := co.UpdateStats()
	if stats.LoopYieldCount != iterations-1 || stats.LoopInlineCount != iterations-1 ||
		stats.LoopScheduledCount != 0 || stats.LoopDeferredCount != 0 {
		t.Fatalf("loop stats = %+v, want all in-frame checkpoints inline", stats)
	}
}

func TestLoopCheckpointKeepsPeerRoundFairness(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	order := make([]string, 0, 6)
	newLoop := func(name string) Thread {
		return co.Create(name, func(me Thread) {
			for range 3 {
				order = append(order, name)
				co.YieldLoopFor(me)
			}
		})
	}
	first := newLoop("first")
	second := newLoop("second")
	joinFirstSliceAll(co, []Thread{first, second})
	co.Update()

	want := []string{"first", "second", "first", "second", "first", "second"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

func TestLoopCheckpointStopsAtFrameBudget(t *testing.T) {
	co := New(nil)
	co.SetPerfDebug(true)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	base := stime.Now()
	var elapsed atomic.Int64
	co.loopNow = func() stime.Time {
		return base.Add(stime.Duration(elapsed.Load()))
	}

	var completed atomic.Int64
	th := co.Create("budgeted-loop", func(me Thread) {
		for i := range 3 {
			completed.Add(1)
			if i == 1 {
				elapsed.Store(int64(loopWorkBudget))
			}
			co.YieldLoopFor(me)
		}
	})
	co.JoinFirstSlice(th)
	co.Update()
	if got := completed.Load(); got != 2 {
		t.Fatalf("first frame iterations = %d, want 2", got)
	}
	first := co.UpdateStats()
	if first.LoopYieldCount != 1 || first.LoopInlineCount != 0 || first.LoopScheduledCount != 1 ||
		first.LoopSameFrameCount != 1 || first.LoopDeferredCount != 1 {
		t.Fatalf("first frame loop stats = %+v, want one deferred checkpoint", first)
	}

	itime.Update(1.0/30, 30)
	co.Update()
	if got := completed.Load(); got != 3 {
		t.Fatalf("second frame iterations = %d, want 3", got)
	}
	second := co.UpdateStats()
	if second.LoopYieldCount != 1 || second.LoopInlineCount != 1 || second.LoopScheduledCount != 0 ||
		second.LoopDeferredCount != 0 || second.LoopNextFrameCount != 1 {
		t.Fatalf("second frame loop stats = %+v, want one resumed checkpoint", second)
	}
}

func TestLoopCheckpointStopsForRedraw(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	var completed atomic.Int64
	th := co.Create("redraw-loop", func(me Thread) {
		for i := range 3 {
			completed.Add(1)
			if i == 1 {
				co.RequestRedraw()
			}
			co.YieldLoopFor(me)
		}
	})
	co.JoinFirstSlice(th)
	co.Update()
	if got := completed.Load(); got != 2 {
		t.Fatalf("redraw frame iterations = %d, want 2", got)
	}

	itime.Update(1.0/30, 30)
	co.Update()
	if got := completed.Load(); got != 3 {
		t.Fatalf("next frame iterations = %d, want 3", got)
	}
}

func TestLoopBudgetYieldsWithoutStoppingScript(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })
	iterations := 0
	th := co.Create(nil, func(me Thread) {
		for {
			iterations++
			co.YieldLoopFor(me)
		}
	})
	co.JoinFirstSlice(th)
	co.Update()
	if iterations <= 1 || th.Stopped() {
		t.Fatalf("iterations = %d, stopped = %v", iterations, th.Stopped())
	}
	previous := iterations
	itime.Update(1.0/30, 30)
	co.Update()
	if iterations <= previous || th.Stopped() {
		t.Fatalf("script did not continue after budget: %d -> %d", previous, iterations)
	}
}

func TestScriptRoundAdvancesForSameFrameLoopRounds(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	var rounds []uint64
	th := co.Create(nil, func(me Thread) {
		for range 3 {
			rounds = append(rounds, co.ScriptRound())
			co.YieldLoopFor(me)
		}
	})
	co.JoinFirstSlice(th)
	co.Update()

	if len(rounds) != 3 || rounds[1] <= rounds[0] || rounds[2] <= rounds[1] {
		t.Fatalf("script rounds = %v, want three increasing same-frame rounds", rounds)
	}
}

func TestNextRoundWaitDoesNotAdmitRound(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	resumed := false
	th := co.Create(nil, func(me Thread) {
		co.YieldToNextRoundFor(me)
		resumed = true
	})
	co.JoinFirstSlice(th)
	co.Update()
	if resumed {
		t.Fatal("passive round wait admitted its own round")
	}

	itime.Update(0, 30)
	co.Update()
	if !resumed {
		t.Fatal("passive round wait did not resume in the next frame")
	}
}

func TestLoopContinuationAdmitsPassiveRoundWait(t *testing.T) {
	co := New(nil)
	itime.Start()
	t.Cleanup(func() { co.RunAfterStop(nil) })

	passiveResumed := false
	passive := co.Create(nil, func(me Thread) {
		co.YieldToNextRoundFor(me)
		passiveResumed = true
	})
	loopResumed := false
	loop := co.Create(nil, func(me Thread) {
		co.YieldLoopFor(me)
		loopResumed = true
	})
	joinFirstSliceAll(co, []Thread{passive, loop})
	co.Update()
	if !passiveResumed || !loopResumed {
		t.Fatalf("same-frame round resumed passive=%v loop=%v", passiveResumed, loopResumed)
	}
}

func TestRunBetweenScriptsSkipsCanceledMainThreadCall(t *testing.T) {
	co := New(nil)
	canceled := co.newThread("canceled")
	canceled.stopped.Store(true)
	var called atomic.Bool
	co.enqueuePriorityJob(&waitJob{
		thread: canceled,
		kind:   waitMainThread,
		call:   func() { called.Store(true) },
	})

	co.pumpMainThread()
	if called.Load() {
		t.Fatal("canceled coroutine's main-thread callback ran")
	}
}
