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
	"sync/atomic"

	"github.com/goplus/spx/v4/internal/engine/platform"
)

// WaitMainThread runs call on the engine thread, directly when the platform allows.
// Managed callers retain their script slice while waiting.
func (p *Coroutines) WaitMainThread(call func()) {
	p.waitMainThread(call, p.callerThread())
}

// CleanupMainThread runs call on the engine thread, ignoring script cancellation.
// Call must not yield; shutdown waits for managed callers to finish.
func (p *Coroutines) CleanupMainThread(call func()) {
	p.waitMainThread(func() {
		id, previous := p.enterCallback(callbackExclusive)
		defer p.leaveCallback(id, previous)
		call()
	}, nil)
}

// RunBetweenScripts runs call on the caller between script slices while
// servicing queued engine-thread jobs. Native callers must use the engine thread.
// Calls that already hold a script slice are rejected.
func (p *Coroutines) RunBetweenScripts(call func()) {
	if p.callerThread() != nil || p.callbackScope()&callbackExclusive != 0 {
		panic(ErrReentrantWait)
	}
	if hasMainThreadQueue {
		for !p.runMu.TryLock() {
			// Service engine calls so a waiting script can release runMu.
			p.pumpMainThread()
		}
	} else {
		p.runMu.Lock()
	}
	defer p.runMu.Unlock()
	id, previous := p.enterCallback(callbackExclusive)
	defer p.leaveCallback(id, previous)
	call()
}

// TryRunFromEngine runs call in a managed coroutine when the caller
// has direct engine access. While waiting, it services engine jobs without
// advancing frames. It reports handled when shutdown skips the call.
func (p *Coroutines) TryRunFromEngine(owner any, call func()) bool {
	if p.admissionClosed() {
		return true
	}
	if p.callerThread() != nil || p.callbackScope()&callbackExclusive != 0 {
		panic(ErrReentrantWait)
	}
	return platform.TryCallEngineDirectly(func() {
		// Shutdown may have closed admission after the initial check.
		if p.admissionClosed() {
			return
		}
		dispatcher := p.Create(owner, func(Thread) {
			call()
		})
		p.waitForSignal(dispatcher.done)
	})
}

type mainThreadCall struct {
	caller  Thread
	done    chan callbackResult
	claimed atomic.Bool // Claimed by execution or cancellation.
}

func (p *Coroutines) waitMainThread(call func(), caller Thread) {
	if platform.TryCallEngineDirectly(call) {
		return
	}

	pending := &mainThreadCall{
		caller: caller,
		done:   make(chan callbackResult, 1),
	}
	p.enqueuePriorityJob(&waitJob{
		thread: pending.caller,
		kind:   waitMainThread,
		call:   func() { pending.run(p, call) },
	})
	pending.wait()
}

func (c *mainThreadCall) run(p *Coroutines, call func()) {
	if !c.claimed.CompareAndSwap(false, true) {
		return
	}
	result := callbackResult{}
	// Notify the waiter even if call exits the engine goroutine with Goexit.
	defer func() { c.done <- result }()
	if c.caller.canceled() {
		return
	}
	scope := callbackNoDrain
	if c.caller != nil {
		scope |= callbackExclusive
	}
	result = p.invokeCallback(scope, call)
}

func (c *mainThreadCall) wait() {
	var canceled <-chan struct{}
	if c.caller != nil {
		canceled = c.caller.Context().Done()
	}
	select {
	case result := <-c.done:
		if c.caller.canceled() {
			panic(ErrAbortThread)
		}
		if result.panicked {
			panic(result.panicValue)
		}
	case <-canceled:
		if !c.claimed.CompareAndSwap(false, true) {
			// Keep script ownership until the running call returns.
			<-c.done
		}
		panic(ErrAbortThread)
	}
}

// waitForSignal serves native engine jobs while waiting, without advancing frames.
// Other callers park so the host can progress.
func (p *Coroutines) waitForSignal(ready <-chan struct{}) {
	if hasMainThreadQueue && platform.TryCallEngineDirectly(func() {
		for {
			select {
			case <-ready:
				return
			default:
				p.pumpMainThread()
			}
		}
	}) {
		return
	}
	<-ready
}

// pumpMainThread serves only engine jobs; it never advances script rounds or frames.
func (p *Coroutines) pumpMainThread() {
	job := p.takeMainThreadJob()
	if job == nil {
		runtime.Gosched()
		return
	}
	if !job.thread.canceled() {
		job.call()
	}
}

func (p *Coroutines) enqueuePriorityJob(job *waitJob) {
	p.schedulerMu.Lock()
	p.currentJobs.pushFront(job)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) takeMainThreadJob() *waitJob {
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	if job, ok := p.currentJobs.peekFront(); ok && job.kind == waitMainThread {
		return p.currentJobs.popFront()
	}
	return nil
}
