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
	"github.com/goplus/spx/v3/internal/time"
)

const (
	waitTypeFrame = iota
	waitTypeTime
	waitTypeMainThread
	waitTypeYield
	waitTypeLoop
	waitTypeNextRound
)

// WaitJob describes work consumed by Update.
type WaitJob struct {
	Th    Thread  // Coroutine waiting on the job, if any.
	Type  int     // Scheduler-internal job kind.
	Call  func()  // Action when eligible; nil resumes Th for non-main-thread jobs.
	Time  float64 // Level-time deadline for a time job.
	Frame int64   // Issuing frame for frame-gated jobs.
}

// Jobs without a thread sort first.
func (job *WaitJob) threadOrder() int64 {
	if job.Th == nil {
		return 0
	}
	return job.Th.resumeOrder
}

type taskResult struct {
	panicValue any
	panicked   bool
}

// Wait suspends the current coroutine for t seconds of level time.
func (p *Coroutines) Wait(t float64) {
	me := p.callerThread()
	if me == nil {
		return
	}
	p.enqueueAndYield(&WaitJob{
		Th:    me,
		Type:  waitTypeTime,
		Time:  time.TimeSinceLevelLoad() + max(t, 0),
		Frame: time.Frame(),
	})
}

// WaitYield suspends me until an Update pass processes its yield job.
func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(&WaitJob{Th: me, Type: waitTypeYield})
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
	results := make(chan taskResult, 1)
	p.setThreadState(me, threadBlocked)
	go func() {
		defer p.finishWorker()
		var result taskResult
		returned := false
		defer func() {
			if !returned {
				// Goexit ends the worker without returning a result to its script.
				me.Cancel()
			}
			// Publish before waking; buffering lets canceled waiters exit.
			results <- result
			p.markRunnableAndResume(me)
		}()
		result = p.runExternalTask(fn)
		returned = true
	}()
	p.Yield(me)
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

func (p *Coroutines) enqueueAndYield(job *WaitJob) {
	me := job.Th
	p.requireCurrent(me)
	// Publish blocking and its wake-up job atomically.
	p.schedulerMu.Lock()
	p.setThreadStateLocked(me, threadBlocked)
	p.currentJobs.PushBack(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.Yield(me)
}

func (p *Coroutines) enqueueJob(job *WaitJob) {
	p.schedulerMu.Lock()
	p.currentJobs.PushBack(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) enqueuePriorityJob(job *WaitJob) {
	p.schedulerMu.Lock()
	p.currentJobs.PushFront(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// runExternalTask calls fn here; nested calls retain the outer drain guard.
func (p *Coroutines) runExternalTask(fn func()) taskResult {
	id, previous := p.enterCallback(callbackExternal)
	defer p.leaveCallback(id, previous)
	return runTask(fn)
}

func runTask(fn func()) (result taskResult) {
	// Distinguish panic(nil) from a normal return.
	result.panicked = true
	defer func() {
		result.panicValue = recover()
	}()
	fn()
	result.panicked = false
	return result
}
