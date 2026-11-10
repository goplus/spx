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

	itime "github.com/goplus/spx/v4/internal/time"
)

func TestLoopBudgetYieldsWithoutStoppingScript(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })
	iterations := 0
	th := scheduler.Create(nil, func(me Thread) {
		for {
			iterations++
			scheduler.YieldLoopFor(me)
		}
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	if iterations <= 1 || th.Stopped() {
		t.Fatalf("iterations = %d, stopped = %v", iterations, th.Stopped())
	}
	previous := iterations
	itime.Update(1.0/30, 30)
	scheduler.Update()
	if iterations <= previous || th.Stopped() {
		t.Fatalf("script did not continue after budget: %d -> %d", previous, iterations)
	}
}

func TestScriptRoundAdvancesForSameFrameLoopRounds(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })

	var rounds []uint64
	th := scheduler.Create(nil, func(me Thread) {
		for range 3 {
			rounds = append(rounds, scheduler.ScriptRound())
			scheduler.YieldLoopFor(me)
		}
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()

	if len(rounds) != 3 || rounds[1] <= rounds[0] || rounds[2] <= rounds[1] {
		t.Fatalf("script rounds = %v, want three increasing same-frame rounds", rounds)
	}
}

func TestNextRoundWaitDoesNotAdmitRound(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })

	resumed := false
	th := scheduler.Create(nil, func(me Thread) {
		scheduler.YieldToNextRoundFor(me)
		resumed = true
	})
	scheduler.JoinFirstSlice(th)
	scheduler.Update()
	if resumed {
		t.Fatal("passive round wait admitted its own round")
	}

	itime.Update(0, 30)
	scheduler.Update()
	if !resumed {
		t.Fatal("passive round wait did not resume in the next frame")
	}
}

func TestLoopContinuationAdmitsPassiveRoundWait(t *testing.T) {
	scheduler := New(nil)
	itime.Start()
	t.Cleanup(func() { scheduler.RunAfterStop(nil) })

	passiveResumed := false
	passive := scheduler.Create(nil, func(me Thread) {
		scheduler.YieldToNextRoundFor(me)
		passiveResumed = true
	})
	loopResumed := false
	loop := scheduler.Create(nil, func(me Thread) {
		scheduler.YieldLoopFor(me)
		loopResumed = true
	})
	joinFirstSliceAll(scheduler, []Thread{passive, loop})
	scheduler.Update()
	if !passiveResumed || !loopResumed {
		t.Fatalf("same-frame round resumed passive=%v loop=%v", passiveResumed, loopResumed)
	}
}

func TestRunBetweenScriptsSkipsCanceledMainThreadCall(t *testing.T) {
	scheduler := New(nil)
	canceled := scheduler.newThread("canceled")
	canceled.stopped.Store(true)
	var called atomic.Bool
	scheduler.queueMainThreadCallForTest(canceled, func() { called.Store(true) })

	scheduler.pumpMainThread()
	if called.Load() {
		t.Fatal("canceled coroutine's main-thread callback ran")
	}
}
