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

import "runtime"

type threadState uint8

const (
	threadRunnable threadState = iota
	threadBlocked
)

// Sched yields the current coroutine and arranges for it to resume without an
// Update pass.
func (p *Coroutines) Sched(me Thread) {
	// Publish blocked first so a fast wake-up cannot be lost.
	p.setThreadState(me, threadBlocked)
	go p.markRunnableAndResume(me)
	p.Yield(me)
}

// Yield suspends me until it is resumed or canceled. It panics if me is not the
// current coroutine.
func (p *Coroutines) Yield(me Thread) {
	if p.Current() != me {
		panic(ErrCannotYieldANonrunningThread)
	}

	// Clear current before releasing the execution lock.
	p.setCurrent(nil)
	p.runMu.Unlock()

	// Publish suspension before the handshake to preserve early wake-ups.
	me.suspended.Store(true)
	for _, waiter := range me.finishYieldWaiters() {
		p.markRunnableAndResume(waiter)
	}
	p.threadsMu.Lock()
	p.suspended[me] = true
	p.threadsMu.Unlock()

	me.suspendMu.Lock()
	for me.suspended.Load() && !p.isThreadCanceled(me) {
		me.suspendCond.Wait()
	}
	me.suspendMu.Unlock()

	p.signalScheduler()
	p.runMu.Lock()
	p.setCurrent(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

// Resume waits for me to publish its suspended state, then wakes it. It returns
// early if me is canceled before that state is published.
func (p *Coroutines) Resume(me Thread) {
	for {
		resumed := false
		p.threadsMu.Lock()
		if p.suspended[me] {
			p.suspended[me] = false
			resumed = true
		}
		p.threadsMu.Unlock()

		if resumed {
			me.suspended.Store(false)
			me.suspendMu.Lock()
			me.suspendCond.Signal()
			me.suspendMu.Unlock()
			return
		}
		if p.isThreadCanceled(me) {
			return
		}
		runtime.Gosched()
	}
}

// Current returns the coroutine that currently owns the scheduler, or nil.
func (p *Coroutines) Current() Thread {
	return p.current.Load()
}

func (p *Coroutines) setCurrent(th Thread) {
	p.current.Store(th)
}

func (p *Coroutines) markRunnableAndResume(th Thread) {
	p.setThreadState(th, threadRunnable)
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
	p.threadStates[th] = state
}

func (p *Coroutines) removeThreadState(th Thread) {
	p.schedulerMu.Lock()
	delete(p.threadStates, th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// runnableThreadCountLocked returns the number of threads that can make
// progress without an external wake-up. The caller must hold schedulerMu.
func (p *Coroutines) runnableThreadCountLocked() int {
	runnable := 0
	for th, state := range p.threadStates {
		if state == threadRunnable && !p.isThreadCanceled(th) {
			runnable++
		}
	}
	return runnable
}
