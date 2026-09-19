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
	"time"

	"github.com/goplus/spx/v3/internal/engine/platform"
)

// WaitMainThread runs call on the engine thread, directly when the platform allows.
// Managed callers retain their script slice while waiting.
func (p *Coroutines) WaitMainThread(call func()) {
	if platform.TryCallEngineDirectly(call) {
		return
	}

	pending := &mainThreadCall{
		caller: p.callerThread(),
		done:   make(chan taskResult, 1),
	}
	p.enqueuePriorityJob(&WaitJob{
		Th:   pending.caller,
		Type: waitTypeMainThread,
		Call: func() { pending.run(p, call) },
	})
	pending.wait(p)
}

type mainThreadCall struct {
	caller  Thread
	done    chan taskResult
	claimed atomic.Bool
}

func (c *mainThreadCall) run(p *Coroutines, call func()) {
	if !c.claimed.CompareAndSwap(false, true) {
		return
	}
	result := taskResult{}
	// Notify the waiter even if call exits the engine goroutine with Goexit.
	defer func() { c.done <- result }()
	if p.isThreadCanceled(c.caller) {
		return
	}
	if c.caller != nil {
		id, previous := p.enterCallback(callbackExclusive)
		defer p.leaveCallback(id, previous)
	}
	result = p.runExternalTask(call)
}

func (c *mainThreadCall) wait(p *Coroutines) {
	var canceled <-chan struct{}
	if c.caller != nil {
		canceled = c.caller.Context().Done()
	}
	select {
	case result := <-c.done:
		if p.isThreadCanceled(c.caller) {
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

// RunBetweenScripts runs call on the caller between script slices while
// servicing queued engine-thread jobs. Native callers must use the engine thread.
// Calls that already hold a script slice are rejected.
func (p *Coroutines) RunBetweenScripts(call func()) {
	if p.callerThread() != nil || p.currentCallback()&callbackExclusive != 0 {
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
func (p *Coroutines) TryRunFromEngine(owner ThreadObj, call func()) bool {
	if p.admissionClosed() {
		return true
	}
	if p.callerThread() != nil || p.currentCallback()&callbackExclusive != 0 {
		panic(ErrReentrantWait)
	}
	return platform.TryCallEngineDirectly(func() {
		// Shutdown may have closed admission after the initial check.
		if p.admissionClosed() {
			return
		}
		dispatcher := p.Create(owner, func(Thread) int {
			call()
			return 0
		})
		p.joinOnEngine(dispatcher)
	})
}

// joinOnEngine serves engine calls until thread completes, without advancing frames.
func (p *Coroutines) joinOnEngine(thread Thread) {
	if !hasMainThreadQueue {
		<-thread.done
		return
	}
	for {
		select {
		case <-thread.done:
			return
		default:
		}
		p.pumpMainThread()
	}
}

// lockShutdown keeps engine-thread work moving while it waits for the barrier.
func (p *Coroutines) lockShutdown(timeout time.Duration) (remaining time.Duration, locked bool) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	tryLock := func(wait func()) bool {
		for !p.shutdownMu.TryLock() {
			if !deadline.IsZero() && time.Now().After(deadline) {
				return false
			}
			wait()
		}
		return true
	}
	var acquired bool
	if hasMainThreadQueue && platform.TryCallEngineDirectly(func() {
		acquired = tryLock(p.pumpMainThread)
	}) {
		locked = acquired
	} else if deadline.IsZero() {
		p.shutdownMu.Lock()
		locked = true
	} else {
		locked = tryLock(func() {
			time.Sleep(min(time.Millisecond, time.Until(deadline)))
		})
	}
	if !locked || deadline.IsZero() {
		return 0, locked
	}
	remaining = time.Until(deadline)
	if remaining > 0 {
		return remaining, true
	}
	p.shutdownMu.Unlock()
	return 0, false
}

func (p *Coroutines) waitForDrainChange(changed <-chan struct{}, timedOut <-chan time.Time) bool {
	wasChanged := false
	if hasMainThreadQueue && platform.TryCallEngineDirectly(func() {
		for {
			select {
			case <-changed:
				wasChanged = true
				return
			case <-timedOut:
				return
			default:
				p.pumpMainThread()
			}
		}
	}) {
		return wasChanged
	}
	select {
	case <-changed:
		return true
	case <-timedOut:
		return false
	}
}

// pumpMainThread serves only engine jobs; it never advances script rounds or frames.
func (p *Coroutines) pumpMainThread() {
	if job := p.takeMainThreadJob(); job != nil {
		p.runMainThreadJob(job)
	} else {
		runtime.Gosched()
	}
}

// Skip canceled callers when serving engine calls outside Update.
func (p *Coroutines) runMainThreadJob(job *WaitJob) {
	if job == nil || (job.Th != nil && p.isThreadCanceled(job.Th)) {
		return
	}
	job.Call()
}

func (p *Coroutines) takeMainThreadJob() *WaitJob {
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	if job, ok := p.currentJobs.PeekFront(); ok && job.Type == waitTypeMainThread {
		return p.currentJobs.PopFront()
	}
	return nil
}
