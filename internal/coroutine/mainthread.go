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

type mainThreadCall struct {
	caller  Thread
	call    func()
	done    chan callbackResult
	claimed atomic.Bool // Claimed by execution or cancellation.
}

// WaitMainThread runs call on the engine thread, directly when the platform allows.
// Managed callers retain their script slice while waiting.
func (p *Coroutines) WaitMainThread(call func()) {
	caller := p.Caller()
	if p.callbackScope()&callbackCleanup != 0 {
		// Engine wrappers nested in cleanup inherit its cancellation exemption.
		caller = nil
	}
	p.waitMainThread(call, caller)
}

// CleanupMainThread runs call on the engine thread, ignoring script cancellation.
// Call must not yield; shutdown waits for managed callers to finish.
func (p *Coroutines) CleanupMainThread(call func()) {
	p.waitMainThread(func() {
		id, previous := p.enterCallback(callbackExclusive | callbackCleanup)
		defer p.leaveCallback(id, previous)
		call()
	}, nil)
}

// RunBetweenScripts runs call on the caller between script slices while
// servicing queued engine-thread jobs. Native callers must use the engine thread.
// Calls that already hold a script slice are rejected.
func (p *Coroutines) RunBetweenScripts(call func()) {
	if p.Caller() != nil || p.callbackScope()&callbackExclusive != 0 {
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

func (p *Coroutines) waitMainThread(call func(), caller Thread) {
	if platform.TryCallEngineDirectly(func() {
		p.invokeMainThreadDirect(caller, call)
	}) {
		return
	}

	pending := &mainThreadCall{
		caller: caller,
		call:   call,
		done:   make(chan callbackResult, 1),
	}
	p.enqueueMainThreadCall(pending)
	pending.wait()
}

func (p *Coroutines) invokeMainThreadDirect(caller Thread, call func()) {
	if caller.canceled() {
		panic(ErrAbortThread)
	}
	func() {
		scope := callbackNoDrain
		if caller != nil {
			scope |= callbackExclusive
		}
		id, previous := p.enterCallback(scope)
		defer p.leaveCallback(id, previous)
		defer func() {
			// Let uncanceled panics propagate with their original fault stack.
			if caller.canceled() {
				recover()
			}
		}()
		call()
	}()
	// A canceled panic (including panic(nil)) reaches this check, but Goexit
	// never returns from the inner call and must not be replaced with a panic.
	checkMainThreadResult(caller, callbackResult{})
}

func (p *Coroutines) invokeMainThread(caller Thread, call func()) callbackResult {
	if caller.canceled() {
		return callbackResult{}
	}
	scope := callbackNoDrain
	if caller != nil {
		scope |= callbackExclusive
	}
	return p.invokeCallback(scope, call)
}

// Cancellation takes precedence over a completed callback, including its panic.
func checkMainThreadResult(caller Thread, result callbackResult) {
	if caller.canceled() {
		panic(ErrAbortThread)
	}
	if result.panicked {
		panic(result.panicValue)
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
	pending := p.takeMainThreadCall()
	if pending == nil {
		runtime.Gosched()
		return
	}
	if !pending.caller.canceled() {
		pending.run(p)
	}
}

func (p *Coroutines) enqueueMainThreadCall(pending *mainThreadCall) {
	p.schedulerMu.Lock()
	p.mainCalls = append(p.mainCalls, pending)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) takeMainThreadCall() *mainThreadCall {
	p.schedulerMu.Lock()
	defer p.schedulerMu.Unlock()
	return p.takeMainThreadCallLocked()
}

// Select and remove together so Update and engine waiters cannot claim the same call.
func (p *Coroutines) takeMainThreadCallLocked() *mainThreadCall {
	if len(p.mainCalls) == 0 {
		return nil
	}
	last := len(p.mainCalls) - 1
	pending := p.mainCalls[last]
	p.mainCalls[last] = nil
	p.mainCalls = p.mainCalls[:last]
	return pending
}

func (c *mainThreadCall) run(p *Coroutines) {
	if !c.claimed.CompareAndSwap(false, true) {
		return
	}
	result := callbackResult{}
	// Notify the waiter even if call exits the engine goroutine with Goexit.
	defer func() { c.done <- result }()
	result = p.invokeMainThread(c.caller, c.call)
}

func (c *mainThreadCall) wait() {
	var canceled <-chan struct{}
	if c.caller != nil {
		canceled = c.caller.Context().Done()
	}
	select {
	case result := <-c.done:
		checkMainThreadResult(c.caller, result)
	case <-canceled:
		if !c.claimed.CompareAndSwap(false, true) {
			// Keep script ownership until the running call returns.
			<-c.done
		}
		panic(ErrAbortThread)
	}
}
