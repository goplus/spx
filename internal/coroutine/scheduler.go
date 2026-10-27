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

	"github.com/visualfc/gid"
)

// Sched yields the current coroutine and resumes it without an Update pass.
func (p *Coroutines) Sched(me Thread) {
	p.requireCurrent(me)
	// Requeue this script before advancing to the next runnable position.
	p.schedulerMu.Lock()
	p.blockThreadLocked(me)
	p.markPendingLocked(me)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.yield(me)
}

// StopAtNextYield cancels me when it next yields to the scheduler.
// It must be called by the currently running managed coroutine.
func (p *Coroutines) StopAtNextYield(me Thread) {
	p.requireCurrent(me)
	me.stopAtNextYield.Store(true)
}

// IsInCoroutine reports whether the caller is running in this manager.
func (p *Coroutines) IsInCoroutine() bool {
	return p.callerThread() != nil
}

// Current returns the coroutine that currently owns runMu, or nil.
func (p *Coroutines) Current() Thread {
	return p.current.Load()
}

// yield releases the current slice after its blocking state has been published.
func (p *Coroutines) yield(me Thread) {
	p.requireCurrent(me)
	p.schedulerMu.Lock()
	p.releaseLocked(me)
	resumeSelf := p.selectedThread == me
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.current.Store(nil)
	p.runMu.Unlock()
	if resumeSelf {
		// The permit stays with this script, but consuming its own signal
		// would not park the Go goroutine. Let workers progress on WASM too.
		runtime.Gosched()
	}
	p.acquire(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

// acquire consumes the scheduler's permit, then takes exclusive script access.
// Cancellation also wakes a thread so its defers can unwind under runMu.
func (p *Coroutines) acquire(me Thread) {
	me.suspendMu.Lock()
	if me.suspendState == suspendSignaled {
		me.suspendState = suspendRunning
	} else {
		me.suspendState = suspendWaiting
	}
	for me.suspendState == suspendWaiting && !me.canceled() {
		me.suspendCond.Wait()
	}
	me.suspendState = suspendRunning
	me.suspendMu.Unlock()
	if me.canceled() {
		// Cancellation can wake an unpublished or blocked script without
		// a scheduler permit. Its cleanup is now executable work for Update.
		p.trackCleanup(me)
	}
	p.runMu.Lock()
	p.current.Store(me)
}

func (p *Coroutines) trackCleanup(th Thread) {
	p.schedulerMu.Lock()
	p.pendingExecution[th] = struct{}{}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// callerThread identifies the calling goroutine; Current identifies the runMu owner.
func (p *Coroutines) callerThread() Thread {
	value, ok := p.goroutineThreads.Load(gid.Get())
	if !ok {
		return nil
	}
	return value.(Thread)
}

func (p *Coroutines) requireCurrent(me Thread) {
	if me == nil || p.callerThread() != me || p.Current() != me {
		panic(ErrNotCurrent)
	}
}

// wakeThread marks th pending and schedules the next ready script.
func (p *Coroutines) wakeThread(th Thread) {
	p.schedulerMu.Lock()
	p.markPendingLocked(th)
	p.scheduleLocked()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// Mark published work pending, queuing live scripts and retaining cancellation cleanup.
func (p *Coroutines) markPendingLocked(th Thread) {
	if _, pending := p.pendingExecution[th]; pending || !th.published {
		return
	}
	select {
	case <-th.done:
		return
	default:
	}
	p.pendingExecution[th] = struct{}{}
	if !th.canceled() {
		p.readyQueue.add(th)
	}
}

func (p *Coroutines) blockThread(th Thread) {
	p.schedulerMu.Lock()
	p.blockThreadLocked(th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// Publish first-slice continuations together with the blocked script.
func (p *Coroutines) blockThreadLocked(th Thread) {
	if th.stopAtNextYield.Swap(false) {
		th.Cancel()
	}
	p.readyQueue.remove(th)
	if !th.canceled() {
		delete(p.pendingExecution, th)
	}
	p.wakeWaitersLocked(th.firstSliceWaiters.close(th.firstSliceDone))
}

// Canceled threads can unwind under runMu without owning a scheduler permit.
func (p *Coroutines) releaseLocked(th Thread) {
	if p.selectedThread == th {
		p.selectedThread = nil
	}
	p.scheduleLocked()
}

// Visit runnable scripts in their original order, wrapping after each round.
// New scripts append to the round; restarted handlers retain their old position.
func (p *Coroutines) scheduleLocked() {
	if p.selectedThread != nil {
		return
	}
	p.selectedThread = p.readyQueue.pop()
	if p.selectedThread != nil {
		p.selectedThread.resume()
	}
}

func (p *Coroutines) wakeWaitersLocked(waiters map[Thread]struct{}) {
	for waiter := range waiters {
		p.markPendingLocked(waiter)
	}
	p.scheduleLocked()
}

// Caller must hold schedulerMu.
func (p *Coroutines) hasPendingExecutionLocked() bool {
	// Canceled scripts remain pending until their executable cleanup finishes.
	return p.selectedThread != nil || len(p.pendingExecution) != 0
}
