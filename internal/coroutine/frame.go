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
	"runtime"
	stime "time"

	"github.com/goplus/spx/v3/internal/time"
)

// Scratch budgets 75% of the default stepping interval for script rounds.
const loopWorkBudget = stime.Second * 3 / (4 * time.DefaultFPS)

// WaitNextFrame suspends the current coroutine until the next frame.
func (p *Coroutines) WaitNextFrame() {
	p.WaitNextFrameFor(p.callerThread())
}

// WaitNextFrameFor suspends me until the next frame.
func (p *Coroutines) WaitNextFrameFor(me Thread) {
	p.yieldAtFrame(me, waitTypeFrame)
}

// YieldLoopFor yields until the next eligible script round.
func (p *Coroutines) YieldLoopFor(me Thread) {
	p.yieldAtFrame(me, waitTypeLoop)
}

func (p *Coroutines) yieldAtFrame(me Thread, kind int) {
	if me == nil || p.callerThread() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	job := p.newResumeWaitJob(me, kind)
	job.Frame = time.Frame()
	p.enqueueAndYield(me, job)
}

// Admit a new round only after all runnable scripts have yielded.
func (p *Coroutines) queueNextLoopRound(state *updateState) bool {
	if p.loopJobs.Count() == 0 || p.redrawFrame.Load() == state.frame || !stime.Now().Before(state.workDeadline) {
		return false
	}
	for p.loopJobs.Count() > 0 {
		job := p.loopJobs.PopFront()
		job.Type = waitTypeYield
		p.currentJobs.PushBack(job)
	}
	return true
}

// RequestRedraw ends additional script rounds after the current round finishes.
func (p *Coroutines) RequestRedraw() {
	p.redrawFrame.Store(time.Frame())
}

// ReadScriptState reads state on the engine thread, excluding script execution.
func (p *Coroutines) ReadScriptState(call func()) {
	for !p.runMu.TryLock() {
		// Service engine calls so a waiting script can release runMu.
		if job := p.takeMainThreadJob(); job != nil {
			job.Call()
		} else {
			runtime.Gosched()
		}
	}
	defer p.runMu.Unlock()
	call()
}

func (p *Coroutines) takeMainThreadJob() *WaitJob {
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	if p.currentJobs.Count() == 0 {
		return nil
	}
	job := p.currentJobs.PopFront()
	if job.Type == waitTypeMainThread {
		return job
	}
	p.currentJobs.PushFront(job)
	return nil
}
