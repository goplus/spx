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
	"github.com/goplus/spx/v3/internal/engine/platform"
	"github.com/goplus/spx/v3/internal/time"
)

const (
	waitTypeFrame = iota
	waitTypeTime
	waitTypeMainThread
	waitTypeYield
)

// WaitJob describes work consumed by Update.
type WaitJob struct {
	Th    Thread  // Coroutine waiting on the job, if any.
	Id    int64   // Manager-local sequence number.
	Type  int     // Scheduler-internal job kind.
	Call  func()  // Action invoked when the job becomes eligible.
	Time  float64 // Level-time deadline for a time job.
	Frame int64   // Scheduler frame recorded for a frame job.
}

type taskResult struct {
	panicValue any
	panicked   bool
}

func runTask(fn func()) (result taskResult) {
	result.panicked = true
	defer func() {
		result.panicValue = recover()
	}()
	fn()
	result.panicked = false
	return result
}

// Wait suspends the current coroutine for the given amount of level time, in
// seconds.
func (p *Coroutines) Wait(t float64) {
	me := p.callerThread()
	if me == nil {
		return
	}
	deadline := time.TimeSinceLevelLoad() + t
	job := p.newResumeWaitJob(me, waitTypeTime)
	job.Time = deadline
	p.enqueueAndYield(me, job)
}

// WaitNextFrame suspends the current coroutine until the scheduler frame
// advances.
func (p *Coroutines) WaitNextFrame() {
	p.WaitNextFrameFor(p.callerThread())
}

// WaitNextFrameFor suspends me until the scheduler frame advances. Callers
// that capture an exact managed thread can reuse it without resolving the
// current goroutine identity at every generated loop edge.
func (p *Coroutines) WaitNextFrameFor(me Thread) {
	if me == nil || p.callerThread() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	frame := time.Frame()
	job := p.newResumeWaitJob(me, waitTypeFrame)
	job.Frame = frame
	p.enqueueAndYield(me, job)
}

// WaitMainThread runs call on the main thread. It runs call immediately when
// the current platform can call the engine directly.
func (p *Coroutines) WaitMainThread(call func()) {
	if platform.TryCallEngineDirectly(call) {
		return
	}

	jobID := p.nextWaitJobID()
	done := make(chan taskResult, 1)
	me := p.currentCoroutineThread()
	job := &WaitJob{
		Th:   me,
		Id:   jobID,
		Type: waitTypeMainThread,
		Call: func() {
			result := taskResult{}
			defer func() { done <- result }()
			if p.isThreadCanceled(me) {
				return
			}
			result = runTask(call)
		},
	}
	p.enqueuePriorityJob(job)

	if me == nil {
		result := <-done
		if result.panicked {
			panic(result.panicValue)
		}
		return
	}
	select {
	case result := <-done:
		if result.panicked {
			panic(result.panicValue)
		}
	case <-me.Context().Done():
		panic(ErrAbortThread)
	}
}

// WaitToDo runs fn in a separate goroutine and suspends the current coroutine
// until fn returns.
func (p *Coroutines) WaitToDo(fn func()) {
	me := p.callerThread()
	if me == nil {
		fn()
		return
	}
	results := make(chan taskResult, 1)
	p.setThreadState(me, threadBlocked)
	go func() {
		result := runTask(fn)
		// Publish the result before waking the waiter. The buffered channel also
		// lets a canceled waiter finish without leaking this worker goroutine.
		results <- result
		p.markRunnableAndResume(me)
	}()
	p.Yield(me)
	result := <-results
	if result.panicked {
		panic(result.panicValue)
	}
}

// WaitYield suspends me until an Update pass processes its yield job.
func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(me, p.newResumeWaitJob(me, waitTypeYield))
}

// WaitForChan receives one value from ch. Inside a coroutine it yields while
// waiting; otherwise it blocks the caller directly.
func WaitForChan[T any](p *Coroutines, ch <-chan T, data *T) {
	me := p.callerThread()
	if me == nil {
		*data = <-ch
		return
	}

	p.setThreadState(me, threadBlocked)
	go func() {
		select {
		case value := <-ch:
			if p.isThreadCanceled(me) {
				return
			}
			*data = value
			p.markRunnableAndResume(me)
		case <-me.Context().Done():
		}
	}()
	p.Yield(me)
}

func (p *Coroutines) nextWaitJobID() int64 {
	return p.nextJobID.Add(1)
}

func (p *Coroutines) newResumeWaitJob(me Thread, waitType int) *WaitJob {
	return &WaitJob{
		Th:   me,
		Id:   p.nextWaitJobID(),
		Type: waitType,
		Call: func() {
			p.markRunnableAndResume(me)
		},
	}
}

func (p *Coroutines) enqueueAndYield(me Thread, job *WaitJob) {
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
