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
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestLoopBudgetYieldsWithoutStoppingScript(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() { co.StopAllAndWait(time.Second) })
	iterations := 0
	th := co.Create(nil, func(me Thread) {
		for {
			iterations++
			co.YieldLoopFor(me)
		}
	})
	co.JoinYieldedOrDone(th)
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
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() { co.StopAllAndWait(time.Second) })

	var rounds []uint64
	th := co.Create(nil, func(me Thread) {
		for range 3 {
			rounds = append(rounds, co.ScriptRound())
			co.YieldLoopFor(me)
		}
	})
	co.JoinYieldedOrDone(th)
	co.Update()

	if len(rounds) != 3 || rounds[1] <= rounds[0] || rounds[2] <= rounds[1] {
		t.Fatalf("script rounds = %v, want three increasing same-frame rounds", rounds)
	}
}

func TestNextRoundWaitDoesNotAdmitRound(t *testing.T) {
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() { co.StopAllAndWait(time.Second) })

	resumed := false
	th := co.Create(nil, func(me Thread) {
		co.YieldToNextRoundFor(me)
		resumed = true
	})
	co.JoinYieldedOrDone(th)
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
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() { co.StopAllAndWait(time.Second) })

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
	co.JoinYieldedOrDoneAll([]Thread{passive, loop})
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
	co.enqueuePriorityJob(&WaitJob{
		Th:   canceled,
		Type: waitTypeMainThread,
		Call: func() { called.Store(true) },
	})

	co.runMainThreadJob(co.takeMainThreadJob())
	if called.Load() {
		t.Fatal("canceled coroutine's main-thread callback ran")
	}
}
