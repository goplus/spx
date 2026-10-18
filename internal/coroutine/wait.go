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

import "github.com/goplus/spx/v3/internal/time"

// Wait suspends the current coroutine for t seconds of level time.
func (p *Coroutines) Wait(t float64) {
	me := p.callerThread()
	if me == nil {
		return
	}
	p.enqueueAndYield(&waitJob{
		thread:   me,
		kind:     waitTime,
		deadline: time.TimeSinceLevelLoad() + max(t, 0),
		frame:    time.Frame(),
	})
}

// WaitYield suspends me until an Update pass processes its yield job.
func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(&waitJob{thread: me, kind: waitYield})
}

// WaitToDo runs fn in a worker and yields when called from a coroutine.
// Other callers run fn directly.
func (p *Coroutines) WaitToDo(fn func()) {
	me := p.callerThread()
	if me == nil {
		fn()
		return
	}
	if !p.admitWorker(me) {
		panic(ErrAbortThread)
	}
	results := make(chan callbackResult, 1)
	p.removeRunnable(me)
	go func() {
		defer p.finishWorker()
		var result callbackResult
		returned := false
		defer func() {
			if !returned {
				// Goexit cancels the waiting script.
				me.Cancel()
			}
			// Publish before waking; buffering lets canceled waiters exit.
			results <- result
			p.wakeThread(me)
		}()
		result = p.invokeCallback(callbackNoDrain, fn)
		returned = true
	}()
	p.yield(me)
	result := <-results
	if result.panicked {
		panic(result.panicValue)
	}
}

// WaitForChan receives one value from ch. Inside a coroutine it yields while
// waiting; otherwise it blocks the caller directly.
func WaitForChan[T any](p *Coroutines, ch <-chan T) T {
	me := p.callerThread()
	if me == nil {
		return <-ch
	}

	var value T
	p.WaitToDo(func() {
		select {
		case value = <-ch:
		case <-me.Context().Done():
		}
	})
	// Only return the result after the script resumes normally.
	return value
}

// admitWorker registers external work under the admission barrier.
func (p *Coroutines) admitWorker(me Thread) bool {
	p.admissionMu.RLock()
	defer p.admissionMu.RUnlock()
	if p.admissionClosed() || me.canceled() {
		return false
	}
	p.threadsMu.Lock()
	p.workerCount++
	p.threadsMu.Unlock()
	return true
}

// WaitToDo pairs each admission with one deferred finish in the worker.
func (p *Coroutines) finishWorker() {
	p.threadsMu.Lock()
	p.workerCount--
	p.notifyDrainLocked()
	p.threadsMu.Unlock()
}

func (p *Coroutines) enqueueAndYield(job *waitJob) {
	me := job.thread
	p.requireCurrent(me)
	// Publish blocking and its wake-up job atomically.
	p.schedulerMu.Lock()
	delete(p.runnableThreads, me)
	p.currentJobs.pushBack(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.yield(me)
}
