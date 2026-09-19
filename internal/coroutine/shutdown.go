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
	stime "time"
)

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

// StopAllAndWait stops scripts and waits for scripts and workers to finish.
// The managed caller is excluded from the wait. A non-positive timeout waits indefinitely.
// Runtime callbacks must not drain; reentry panics with ErrReentrantWait.
func (p *Coroutines) StopAllAndWait(timeout stime.Duration) bool {
	p.requireDrainCaller()
	caller := p.callerThread()
	if caller != nil {
		p.StopAll()
		return p.waitForPeerDrain(timeout, caller)
	}
	return p.RunAfterStopAll(timeout, nil)
}

// RunAfterStopAll drains scripts and workers, then runs call with admission
// closed. Call it outside managed coroutines and runtime callbacks. A drain timeout
// leaves admission closed until a later successful call. The timeout covers
// barrier acquisition and draining, but not call.
func (p *Coroutines) RunAfterStopAll(timeout stime.Duration, call func()) bool {
	_, completed := p.RunAfterStopAllIf(timeout, nil, call)
	return completed
}

// RunAfterStopAllIf drains only if condition still holds after earlier barriers.
// selected is false if condition fails or the barrier cannot be acquired in time.
func (p *Coroutines) RunAfterStopAllIf(timeout stime.Duration, condition func() bool, call func()) (selected, completed bool) {
	p.requireDrainCaller()
	if p.callerThread() != nil {
		panic("coroutine: RunAfterStopAll requires an external caller")
	}
	remaining, locked := p.lockShutdown(timeout)
	if !locked {
		return false, false
	}
	defer p.shutdownMu.Unlock()
	if condition != nil && !condition() {
		return false, false
	}

	p.admissionMu.Lock()
	p.beginStoppingLocked()
	p.admissionMu.Unlock()
	if !p.waitForDrain(remaining, nil) {
		return true, false
	}
	defer func() {
		p.admissionMu.Lock()
		p.endStoppingLocked()
		p.admissionMu.Unlock()
	}()
	if call != nil {
		id, previous := p.enterCallback(callbackShutdown)
		defer p.leaveCallback(id, previous)
		call()
	}
	return true, true
}

func (p *Coroutines) beginStoppingLocked() {
	if !p.stopping {
		p.stopping = true
		p.closeAdmissionLocked()
	}
	p.cancelAllLocked()
}

func (p *Coroutines) endStoppingLocked() {
	if !p.stopping {
		return
	}
	p.openAdmissionLocked()
	p.stopping = false
}

func (p *Coroutines) cancelAllLocked() {
	for _, th := range p.snapshotThreads() {
		th.Cancel()
	}
}

func (p *Coroutines) waitForPeerDrain(timeout stime.Duration, caller Thread) bool {
	// Release runMu so canceled peers can unregister.
	p.setCurrent(nil)
	p.runMu.Unlock()
	completed := p.waitForDrain(timeout, caller)
	p.runMu.Lock()
	p.setCurrent(caller)
	return completed
}

func (p *Coroutines) waitForDrain(timeout stime.Duration, skip Thread) bool {
	var timedOut <-chan stime.Time
	if timeout > 0 {
		timer := stime.NewTimer(timeout)
		defer timer.Stop()
		timedOut = timer.C
	}

	for {
		p.threadsMu.Lock()
		if !p.hasPendingWorkLocked(skip) {
			p.threadsMu.Unlock()
			return true
		}
		// Subscribe while holding the predicate lock so completion cannot be lost.
		if p.lifecycleChanged == nil {
			p.lifecycleChanged = make(chan struct{})
		}
		changed := p.lifecycleChanged
		p.threadsMu.Unlock()

		if !p.waitForDrainChange(changed, timedOut) {
			// Completion may have raced the timeout; the registry is authoritative.
			return !p.hasPendingWork(skip)
		}
	}
}

func (p *Coroutines) hasPendingWork(skip Thread) bool {
	p.threadsMu.Lock()
	defer p.threadsMu.Unlock()
	return p.hasPendingWorkLocked(skip)
}

func (p *Coroutines) hasPendingWorkLocked(skip Thread) bool {
	remaining := len(p.allThreads)
	if _, registered := p.allThreads[skip]; registered {
		remaining--
	}
	return remaining != 0 || p.workerCount != 0
}
