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
	"context"
	"fmt"
	"slices"
	"strings"
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

// RunAfterStop stops scripts and drains all scripts and workers with admission closed.
// It then runs call if non-nil; pass nil to only stop and wait.
// Call outside scripts and callbacks with an independent host context.
// Cancellation before stopping leaves work and admission unchanged.
// Failed drains keep admission closed until a successful retry.
// The context bounds waiting, not callbacks or engine calls.
func (p *Coroutines) RunAfterStop(ctx context.Context, call func()) error {
	_, err := p.RunAfterStopIf(ctx, nil, call)
	return err
}

// RunAfterStopIf checks condition under the RunAfterStop barrier.
// A false condition returns false, nil; stopStarted reports whether stopping began.
// Drain completion wins concurrent cancellation; errors.Is identifies context errors.
func (p *Coroutines) RunAfterStopIf(ctx context.Context, condition func() bool, call func()) (stopStarted bool, err error) {
	p.requireDrainCaller()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := p.waitForSignal(ctx, p.shutdownGate); err != nil {
		return false, err
	}
	defer func() { p.shutdownGate <- struct{}{} }()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	// Holding the gate forbids reentrant drains.
	id, previous := p.enterCallback(callbackNoDrain)
	defer p.leaveCallback(id, previous)
	if condition != nil && !condition() {
		return false, nil
	}

	p.admissionMu.Lock()
	if err := ctx.Err(); err != nil {
		p.admissionMu.Unlock()
		return false, err
	}
	p.beginStoppingLocked()
	p.admissionMu.Unlock()
	if err := p.drain(ctx); err != nil {
		return true, err
	}
	defer func() {
		p.admissionMu.Lock()
		p.endStoppingLocked()
		p.admissionMu.Unlock()
	}()
	if call != nil {
		call()
	}
	return true, nil
}

func (p *Coroutines) beginStoppingLocked() {
	p.stopping = true
	p.closeAdmissionLocked()
	p.cancelAllLocked()
}

func (p *Coroutines) endStoppingLocked() {
	p.openAdmissionLocked()
	p.stopping = false
}

func (p *Coroutines) cancelAllLocked() {
	for _, th := range p.snapshotThreads() {
		th.Cancel()
	}
}

// drain waits for registered work to exit; it does not cancel it.
// Callers must prevent new admissions while waiting.
func (p *Coroutines) drain(ctx context.Context) error {
	p.threadsMu.Lock()
	if !p.hasPendingWorkLocked() {
		p.threadsMu.Unlock()
		return nil
	}
	// Check and subscribe under one lock to avoid lost wakeups.
	if p.drainDone == nil {
		p.drainDone = make(chan struct{})
	}
	done := p.drainDone
	p.threadsMu.Unlock()

	if err := p.waitForSignal(ctx, done); err != nil {
		// Recheck completion before reporting cancellation.
		return p.drainError(err)
	}
	return nil
}

func (p *Coroutines) drainError(cause error) error {
	p.threadsMu.Lock()
	if !p.hasPendingWorkLocked() {
		p.threadsMu.Unlock()
		return nil
	}
	names := make([]string, 0, len(p.allThreads))
	for th := range p.allThreads {
		names = append(names, fmt.Sprintf("id=%d name=%q", th.ID(), th.Name()))
	}
	workers := p.workerCount
	p.threadsMu.Unlock()
	slices.Sort(names)
	return fmt.Errorf("coroutine: drain incomplete: %d threads [%s], %d workers: %w", len(names), strings.Join(names, ", "), workers, cause)
}

func (p *Coroutines) hasPendingWorkLocked() bool {
	return len(p.allThreads) != 0 || p.workerCount != 0
}
