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
	sdebug "runtime/debug"

	"github.com/visualfc/gid"
)

// Task describes a coroutine's owner, invocation registration, and execution.
type Task struct {
	Owner any
	// RegisterInvocation runs synchronously after the thread is registered and
	// returns optional cleanup.
	// Cleanup runs once on thread exit, even if Run never starts.
	RegisterInvocation func(Thread) (cleanup func())
	Run                func(Thread)
}

type threadAdmission struct {
	epoch   uint64
	allowed bool
}

// Create creates a coroutine without explicitly yielding execution to it.
func (p *Coroutines) Create(owner any, fn func(me Thread)) Thread {
	return p.createThread(p.captureAdmission(), Task{
		Owner: owner,
		Run:   fn,
	})
}

// LastThreadID returns the most recently allocated thread ID.
func (p *Coroutines) LastThreadID() int64 {
	return p.nextThreadID.Load()
}

func (p *Coroutines) createThread(admission threadAdmission, task Task) Thread {
	th := p.registerTask(admission, task)
	p.publishThreads([]Thread{th})
	return th
}

// registerTask admits a thread before its registration callback. Its body waits
// for publication; canceled work can unwind once registration returns.
func (p *Coroutines) registerTask(admission threadAdmission, task Task) Thread {
	th := p.newThread(task.Owner)
	p.admissionMu.RLock()
	if !admission.valid(p.admissionEpoch.Load()) {
		p.admissionMu.RUnlock()
		// The rejected thread has not been published, so it has no waiters to wake.
		th.Cancel()
		th.firstSliceWaiters.close(th.firstSliceDone)
		th.joinWaiters.close(th.done)
		return th
	}
	p.registerThread(th)
	if task.RegisterInvocation != nil {
		p.pendingSetups.Add(1)
	}
	p.admissionMu.RUnlock()

	var cleanup func()
	// Registered threads also need async cleanup if setup panics or calls Goexit.
	defer func() {
		if th.canceled() {
			// Update must observe executable cleanup before its goroutine starts.
			p.trackCleanup(th)
		}
		go p.runThread(th, task.Run, cleanup)
	}()
	if task.RegisterInvocation != nil {
		id, previous := p.enterCallback(callbackNoDrain)
		defer p.leaveCallback(id, previous)
		setupReturned := false
		defer func() {
			if !setupReturned {
				th.Cancel()
			}
			p.finishSetup()
		}()
		cleanup = task.RegisterInvocation(th)
		setupReturned = true
	}
	return th
}

// publishThreads publishes each newly registered thread once, as a batch.
func (p *Coroutines) publishThreads(threads []Thread) {
	p.schedulerMu.Lock()
	for _, th := range threads {
		th.published = true
		p.markPendingLocked(th)
	}
	p.scheduleLocked()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Coroutines) runThread(th Thread, fn func(Thread), cleanup func()) {
	gid := gid.Get()
	p.goroutineThreads.Store(gid, th)
	p.acquire(th)
	defer func() {
		p.finishThread(th, gid, recover())
	}()
	if cleanup != nil {
		// Keep cleanup a callback so it cannot recover the script's panic.
		defer func() {
			cleanup()
		}()
	}

	if th.stopped.Load() {
		panic(ErrAbortThread)
	}
	fn(th)
}

func (p *Coroutines) finishThread(th Thread, gid uint64, recovered any) {
	p.schedulerMu.Lock()
	// Both waiter groups participate in the same scheduler selection.
	for waiter := range th.firstSliceWaiters.close(th.firstSliceDone) {
		p.markPendingLocked(waiter)
	}
	for waiter := range th.joinWaiters.close(nil) {
		p.markPendingLocked(waiter)
	}
	th.cancelFunc()
	close(th.done)
	p.readyQueue.remove(th)
	delete(p.pendingExecution, th)
	p.releaseLocked(th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.current.Store(nil)
	// Draining also waits for the panic handler below.
	defer p.unregisterThread(th)
	p.runMu.Unlock()
	p.goroutineThreads.Delete(gid)
	p.handleThreadPanic(th, recovered)
}

func (p *Coroutines) handleThreadPanic(th Thread, recovered any) {
	if recovered == nil || recovered == ErrAbortThread || recovered == ErrStopThisScript {
		return
	}
	if p.onPanic != nil {
		id, previous := p.enterCallback(callbackNoDrain)
		defer p.leaveCallback(id, previous)
		p.onPanic(PanicReport{
			Value:         recovered,
			Name:          th.name,
			Stack:         string(sdebug.Stack()),
			CreationStack: th.stack,
		})
		return
	}
	panic(recovered)
}

func (p *Coroutines) captureAdmission() threadAdmission {
	epoch := p.admissionEpoch.Load()
	return threadAdmission{
		epoch:   epoch,
		allowed: epoch&1 == 0 && !p.callerThread().canceled(),
	}
}

func (p *Coroutines) finishSetup() {
	p.admissionMu.Lock()
	if p.pendingSetups.Add(-1) == 0 && !p.stopping {
		p.openAdmissionLocked()
	}
	p.admissionMu.Unlock()
}

func (p *Coroutines) closeAdmissionLocked() {
	if !p.admissionClosed() {
		p.admissionEpoch.Add(1)
	}
}

func (p *Coroutines) openAdmissionLocked() {
	if p.admissionClosed() {
		p.admissionEpoch.Add(1)
	}
}

func (p *Coroutines) admissionClosed() bool {
	return p.admissionEpoch.Load()&1 != 0
}

func (p *Coroutines) registerThread(th Thread) {
	p.threadsMu.Lock()
	p.allThreads[th] = struct{}{}
	p.threadsMu.Unlock()
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.threadsMu.Lock()
	delete(p.allThreads, th)
	p.notifyDrainLocked()
	p.threadsMu.Unlock()
}

func (p *Coroutines) snapshotThreads() []Thread {
	p.threadsMu.Lock()
	threads := make([]Thread, 0, len(p.allThreads))
	for th := range p.allThreads {
		threads = append(threads, th)
	}
	p.threadsMu.Unlock()
	return threads
}

func (p *Coroutines) notifyDrainLocked() {
	if p.drainDone != nil && !p.hasPendingWorkLocked() {
		close(p.drainDone)
		p.drainDone = nil
	}
}

func (a threadAdmission) valid(epoch uint64) bool {
	return a.allowed && a.epoch == epoch
}
