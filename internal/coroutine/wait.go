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
	"sync/atomic"

	"github.com/goplus/spx/v3/internal/engine/platform"
	"github.com/goplus/spx/v3/internal/time"
)

const (
	waitTypeFrame = iota
	waitTypeTime
	waitTypeMainThread
	waitTypeYield
)

const (
	mainThreadCallPending uint32 = iota
	mainThreadCallRunning
	mainThreadCallDone
	mainThreadCallCanceled
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

// Wait suspends the current coroutine for the given amount of level time, in
// seconds.
func (p *Coroutines) Wait(t float64) {
	me := p.Current()
	deadline := time.TimeSinceLevelLoad() + t
	job := p.newResumeWaitJob(me, waitTypeTime)
	job.Time = deadline
	p.enqueueAndYield(me, job)
}

// WaitNextFrame suspends the current coroutine until the scheduler frame
// advances.
func (p *Coroutines) WaitNextFrame() {
	p.WaitNextFrameFor(p.Current())
}

// WaitNextFrameFor suspends me until the scheduler frame advances. Callers
// that capture an exact managed thread can reuse it without resolving the
// current goroutine identity at every generated loop edge.
func (p *Coroutines) WaitNextFrameFor(me Thread) {
	frame := time.Frame()
	job := p.newResumeWaitJob(me, waitTypeFrame)
	job.Frame = frame
	p.enqueueAndYield(me, job)
}

// WaitMainThread runs call on the main thread. It runs call immediately when
// the current platform can call the engine directly.
func (p *Coroutines) WaitMainThread(call func()) {
	me := p.currentCoroutineThread()
	if me == nil {
		if platform.TryCallEngineDirectly(call) {
			return
		}
		done := make(chan struct{})
		p.enqueuePriorityJob(&WaitJob{
			Id:   p.nextWaitJobID(),
			Type: waitTypeMainThread,
			Call: func() {
				defer close(done)
				call()
			},
		})
		<-done
		return
	}

	done := make(chan struct{})
	var state atomic.Uint32
	admittedCall := func() {
		// Direct and queued calls use the same admission protocol. Cancellation
		// and admission share suspendMu; whichever obtains it first wins.
		me.suspendMu.Lock()
		if p.isThreadCanceled(me) {
			canceled := state.CompareAndSwap(mainThreadCallPending, mainThreadCallCanceled)
			me.suspendMu.Unlock()
			// If the queued callback observes cancellation first, it owns the
			// terminal transition and must wake the waiter. The running path owns
			// the only other close, so the channel is closed exactly once.
			if canceled {
				close(done)
			}
			return
		}
		if !state.CompareAndSwap(mainThreadCallPending, mainThreadCallRunning) {
			me.suspendMu.Unlock()
			return
		}
		me.suspendMu.Unlock()
		defer func() {
			state.Store(mainThreadCallDone)
			close(done)
		}()
		call()
	}

	if platform.TryCallEngineDirectly(admittedCall) {
		if state.Load() != mainThreadCallDone || p.isThreadCanceled(me) {
			panic(ErrAbortThread)
		}
		return
	}

	p.enqueuePriorityJob(&WaitJob{
		Th:   me,
		Id:   p.nextWaitJobID(),
		Type: waitTypeMainThread,
		Call: admittedCall,
	})
	select {
	case <-done:
		if p.isThreadCanceled(me) {
			panic(ErrAbortThread)
		}
	case <-me.Context().Done():
		if !state.CompareAndSwap(mainThreadCallPending, mainThreadCallCanceled) {
			// Once admitted, a main-thread callback must finish before the
			// coroutine unwinds and runs teardown against the same state.
			<-done
		}
		panic(ErrAbortThread)
	}
}

// WaitToDo runs fn in a separate goroutine and suspends the current coroutine
// until fn returns.
func (p *Coroutines) WaitToDo(fn func()) {
	me := p.Current()
	p.setThreadState(me, threadBlocked)
	go func() {
		fn()
		p.markRunnableAndResume(me)
	}()
	p.Yield(me)
}

// WaitYield suspends me until an Update pass processes its yield job.
func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(me, p.newResumeWaitJob(me, waitTypeYield))
}

// WaitForChan receives one value from ch. Inside a coroutine it yields while
// waiting; otherwise it blocks the caller directly.
func WaitForChan[T any](p *Coroutines, ch <-chan T, data *T) {
	me := p.Current()
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
