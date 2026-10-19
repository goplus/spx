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

// StopCurrent exits the current coroutine with ErrAbortThread.
func (p *Coroutines) StopCurrent() {
	panic(ErrAbortThread)
}

// StopThisScript unwinds to the nearest procedure or event boundary.
// A procedure boundary returns to its caller; an event boundary ends the coroutine.
func (p *Coroutines) StopThisScript() {
	panic(ErrStopThisScript)
}

// Stop cancels thread; nil and repeated calls are safe.
func (p *Coroutines) Stop(thread Thread) {
	if thread != nil {
		thread.Cancel()
	}
}

// StopIf requests cancellation of every thread accepted by filter. Filters are
// evaluated without holding the thread registry lock.
func (p *Coroutines) StopIf(filter func(th Thread) bool) {
	allThreads := p.snapshotThreads()
	threads := allThreads[:0]
	for _, th := range allThreads {
		if filter(th) {
			threads = append(threads, th)
		}
	}
	for _, th := range threads {
		th.Cancel()
	}
}

// StopAll requests cancellation of every registered coroutine.
func (p *Coroutines) StopAll() {
	p.admissionMu.Lock()
	p.closeAdmissionLocked()
	p.cancelAllLocked()
	if !p.stopping && p.pendingSetups.Load() == 0 {
		p.openAdmissionLocked()
	}
	p.admissionMu.Unlock()
}

// RunAfterStop stops scripts and drains all scripts and workers with admission closed.
// It then runs call if non-nil.
// Call outside scripts and callbacks. An interrupted drain keeps admission closed;
// a completed drain restores admission even if call panics or exits its goroutine.
func (p *Coroutines) RunAfterStop(call func()) {
	p.requireDrainCaller()
	p.waitForSignal(p.shutdownGate)
	defer func() { p.shutdownGate <- struct{}{} }()
	// Reject nested drains while holding the gate.
	id, previous := p.enterCallback(callbackNoDrain)
	defer p.leaveCallback(id, previous)
	p.admissionMu.Lock()
	p.stopping = true
	p.closeAdmissionLocked()
	p.cancelAllLocked()
	p.admissionMu.Unlock()
	p.drain()
	// Do not reopen admission if a pumped engine call interrupted the drain.
	defer func() {
		p.admissionMu.Lock()
		p.openAdmissionLocked()
		p.stopping = false
		p.admissionMu.Unlock()
	}()
	if call != nil {
		call()
	}
}

func (p *Coroutines) cancelAllLocked() {
	for _, th := range p.snapshotThreads() {
		th.Cancel()
	}
}

// drain waits for registered work to exit; it does not cancel it.
// Callers must prevent new admissions while waiting.
func (p *Coroutines) drain() {
	p.threadsMu.Lock()
	if !p.hasPendingWorkLocked() {
		p.threadsMu.Unlock()
		return
	}
	// Check and subscribe under one lock to avoid lost wakeups.
	if p.drainDone == nil {
		p.drainDone = make(chan struct{})
	}
	done := p.drainDone
	p.threadsMu.Unlock()

	p.waitForSignal(done)
}

func (p *Coroutines) hasPendingWorkLocked() bool {
	return len(p.allThreads) != 0 || p.workerCount != 0
}
