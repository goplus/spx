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

import "github.com/visualfc/gid"

// Sched yields the current coroutine and resumes it without an Update pass.
func (p *Coroutines) Sched(me Thread) {
	p.requireCurrent(me)
	// Publish blocked first so a fast wake-up cannot be lost.
	p.removeRunnable(me)
	go p.wakeThread(me)
	p.Yield(me)
}

// Yield suspends me until resumed or canceled; me must be the current coroutine.
func (p *Coroutines) Yield(me Thread) {
	p.requireCurrent(me)
	if me.stopAtNextYield.Swap(false) {
		me.Cancel()
	}

	// Clear current before releasing the execution lock.
	p.current.Store(nil)
	p.runMu.Unlock()

	// Publish suspension before notifying waiters; consume an early Resume.
	me.suspendMu.Lock()
	if me.suspendState == suspendSignaled {
		me.suspendState = suspendRunning
	} else {
		me.suspendState = suspendWaiting
	}
	me.suspendMu.Unlock()

	p.wakeWaiters(me.firstSliceWaiters.close(me.firstSliceDone))

	me.suspendMu.Lock()
	for me.suspendState == suspendWaiting && !me.canceled() {
		me.suspendCond.Wait()
	}
	if me.suspendState == suspendWaiting {
		me.suspendState = suspendRunning
	}
	me.suspendMu.Unlock()

	p.signalScheduler()
	p.runMu.Lock()
	p.current.Store(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

// Resume wakes me or records a signal for its next Yield.
func (p *Coroutines) Resume(me Thread) {
	me.suspendMu.Lock()
	defer me.suspendMu.Unlock()
	if me.canceled() {
		return
	}

	switch me.suspendState {
	case suspendWaiting:
		me.suspendState = suspendRunning
		me.suspendCond.Signal()
	case suspendRunning:
		me.suspendState = suspendSignaled
	}
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

// Current returns the coroutine that currently owns the scheduler, or nil.
func (p *Coroutines) Current() Thread {
	return p.current.Load()
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

// wakeThread marks th runnable before resuming it.
func (p *Coroutines) wakeThread(th Thread) {
	p.schedulerMu.Lock()
	if th.canceled() {
		p.schedulerMu.Unlock()
		return
	}
	p.runnableThreads[th] = struct{}{}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.Resume(th)
}

func (p *Coroutines) signalScheduler() {
	p.schedulerMu.Lock()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) removeRunnable(th Thread) {
	p.schedulerMu.Lock()
	delete(p.runnableThreads, th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// Caller must hold schedulerMu.
func (p *Coroutines) hasRunnableLocked() bool {
	for th := range p.runnableThreads {
		if !th.canceled() {
			return true
		}
	}
	return false
}
