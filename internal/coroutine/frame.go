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

	"github.com/goplus/spx/v3/internal/engine/platform"
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

// RunBetweenScripts runs call between script slices while servicing queued
// engine-thread jobs.
func (p *Coroutines) RunBetweenScripts(call func()) {
	for !p.runMu.TryLock() {
		// Service engine calls so a waiting script can release runMu.
		if job := p.takeMainThreadJob(); job != nil {
			p.runMainThreadJob(job)
		} else {
			runtime.Gosched()
		}
	}
	defer p.runMu.Unlock()
	call()
}

// TryRunManagedBetweenScripts runs call in a managed coroutine when the caller
// can access the engine directly. It services queued engine calls while the
// managed callback waits for the script execution lock. It returns true when
// the call was handled here, including when shutdown admission intentionally
// skips it.
func (p *Coroutines) TryRunManagedBetweenScripts(owner ThreadObj, call func()) bool {
	if p.abortEpoch.Load()&1 != 0 {
		return true
	}
	return platform.TryCallEngineDirectly(func() {
		// Match Create's admission barrier without taking creationMu. Shutdown
		// callbacks may invoke this while the barrier holds that lock.
		if p.abortEpoch.Load()&1 != 0 {
			return
		}
		dispatcher := p.Create(owner, func(Thread) int {
			call()
			return 0
		})
		p.waitForThreadOnEngine(dispatcher)
	})
}

// waitForThreadOnEngine keeps the engine callback responsive while a managed
// dispatcher waits behind another script or an engine-thread job.
func (p *Coroutines) waitForThreadOnEngine(thread Thread) {
	for {
		select {
		case <-thread.done:
			return
		default:
		}
		if job := p.takeMainThreadJob(); job != nil {
			p.runMainThreadJob(job)
		} else {
			runtime.Gosched()
		}
	}
}

// runMainThreadJob mirrors Update's cancellation check for engine callbacks
// serviced outside the scheduler loop. A canceled script must not receive a
// successful WaitMainThread result merely because another engine callback
// drained its queued job.
func (p *Coroutines) runMainThreadJob(job *WaitJob) {
	if job == nil || (job.Th != nil && p.isThreadCanceled(job.Th)) {
		return
	}
	job.Call()
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
