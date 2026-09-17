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

type threadState uint8

const (
	threadRunnable threadState = iota
	threadBlocked
)

// Sched yields the current coroutine and arranges for it to resume without an
// Update pass.
func (p *Coroutines) Sched(me Thread) {
	p.requireCurrent(me)
	// Publish blocked first so a fast wake-up cannot be lost.
	p.setThreadState(me, threadBlocked)
	go p.markRunnableAndResume(me)
	p.Yield(me)
}

// Yield suspends me until it is resumed or canceled. It panics if me is not the
// current coroutine.
func (p *Coroutines) Yield(me Thread) {
	p.requireCurrent(me)
	if me.stopAtNextYield.Swap(false) {
		me.Cancel()
	}

	// Clear current before releasing the execution lock.
	p.setCurrent(nil)
	p.runMu.Unlock()

	// Publish suspension before notifying yield waiters. Resume may have
	// arrived before this point; consume that signal instead of blocking.
	me.suspendMu.Lock()
	if me.suspendState == suspendStateSignaled {
		me.suspendState = suspendStateRunning
	} else {
		me.suspendState = suspendStateSuspended
	}
	me.suspendMu.Unlock()

	for waiter := range me.yieldWaiters.close(me.yieldedOrDone) {
		p.markRunnableAndResume(waiter)
	}

	me.suspendMu.Lock()
	for me.suspendState == suspendStateSuspended && !p.isThreadCanceled(me) {
		me.suspendCond.Wait()
	}
	if me.suspendState == suspendStateSuspended {
		me.suspendState = suspendStateRunning
	}
	me.suspendMu.Unlock()

	p.signalScheduler()
	p.runMu.Lock()
	p.setCurrent(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

// Resume wakes me if it is suspended. If Yield has not published suspension
// yet, Resume records a signal for that Yield to consume without blocking.
func (p *Coroutines) Resume(me Thread) {
	me.suspendMu.Lock()
	defer me.suspendMu.Unlock()
	if p.isThreadCanceled(me) {
		return
	}

	switch me.suspendState {
	case suspendStateSuspended:
		me.suspendState = suspendStateRunning
		me.suspendCond.Signal()
	case suspendStateRunning:
		me.suspendState = suspendStateSignaled
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
		panic(ErrCannotYieldANonrunningThread)
	}
}

func (p *Coroutines) setCurrent(th Thread) {
	p.current.Store(th)
}

func (p *Coroutines) markRunnableAndResume(th Thread) {
	p.schedulerMu.Lock()
	if p.isThreadCanceled(th) {
		p.schedulerMu.Unlock()
		return
	}
	p.setThreadStateLocked(th, threadRunnable)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.Resume(th)
}

func (p *Coroutines) isThreadCanceled(th Thread) bool {
	if th == nil {
		return false
	}
	if th.stopped.Load() {
		return true
	}
	select {
	case <-th.Context().Done():
		return true
	default:
		return false
	}
}

func (p *Coroutines) signalScheduler() {
	p.schedulerMu.Lock()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) setThreadState(th Thread, state threadState) {
	p.schedulerMu.Lock()
	p.setThreadStateLocked(th, state)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) setThreadStateLocked(th Thread, state threadState) {
	if state == threadRunnable {
		p.runnableThreads[th] = struct{}{}
	} else {
		delete(p.runnableThreads, th)
	}
}

func (p *Coroutines) removeThreadState(th Thread) {
	p.schedulerMu.Lock()
	delete(p.runnableThreads, th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// Caller must hold schedulerMu.
func (p *Coroutines) hasRunnableThreadLocked() bool {
	for th := range p.runnableThreads {
		if !p.isThreadCanceled(th) {
			return true
		}
	}
	return false
}
