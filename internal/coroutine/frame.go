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

package coroutine

import (
	stime "time"

	"github.com/goplus/spx/v4/internal/time"
)

// Scratch budgets 75% of the default stepping interval for script rounds.
const loopWorkBudget = stime.Second * 3 / (4 * time.DefaultFPS)

// ScriptRound counts additional script rounds within engine frames.
// Pair it with the frame number to identify a scheduling round.
func (p *Coroutines) ScriptRound() uint64 {
	return p.scriptRound.Load()
}

// WaitNextFrame suspends the current coroutine until the next frame.
func (p *Coroutines) WaitNextFrame() {
	p.WaitNextFrameFor(p.callerThread())
}

// WaitNextFrameFor suspends me until the next frame.
func (p *Coroutines) WaitNextFrameFor(me Thread) {
	p.yieldAtFrame(me, waitFrame)
}

// YieldLoopFor yields until the next eligible script round.
func (p *Coroutines) YieldLoopFor(me Thread) {
	p.yieldAtFrame(me, waitLoop)
}

// YieldToNextRoundFor waits for the next frame or a round admitted by a loop continuation.
func (p *Coroutines) YieldToNextRoundFor(me Thread) {
	p.yieldAtFrame(me, waitNextRound)
}

// RequestRedraw ends additional script rounds after the current round finishes.
func (p *Coroutines) RequestRedraw() {
	p.redrawFrame.Store(time.Frame())
}

func (p *Coroutines) yieldAtFrame(me Thread, kind waitKind) {
	p.enqueueAndYield(&waitJob{thread: me, kind: kind, frame: time.Frame()})
}

// Admit a new round only after all runnable scripts have yielded.
func (p *Coroutines) queueNextRound(state *updateState) bool {
	if p.redrawFrame.Load() == state.frame ||
		!stime.Now().Before(state.workDeadline) || !p.hasLoopContinuation() {
		return false
	}
	// A script round does not advance the frame clock.
	p.scriptRound.Add(1)
	for n := p.roundJobs.head; n != nil; n = n.next {
		n.value.kind = waitYield
	}
	p.schedulerMu.Lock()
	p.currentJobs.appendFrom(p.roundJobs)
	p.schedulerMu.Unlock()
	return true
}

func (p *Coroutines) hasLoopContinuation() bool {
	return p.roundJobs.any(func(job *waitJob) bool {
		return job.kind == waitLoop && !job.thread.canceled()
	})
}
