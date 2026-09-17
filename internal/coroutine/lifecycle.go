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
	sdebug "runtime/debug"

	"github.com/visualfc/gid"
)

// Task describes a coroutine's owner, setup, and execution.
type Task struct {
	Owner ThreadObj
	// Setup runs synchronously after registration and returns optional cleanup.
	// Cleanup runs once on thread exit, even if Run never starts.
	Setup func(Thread) (cleanup func())
	Run   func(Thread)
}

// Create creates a coroutine without explicitly yielding execution to it.
func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.createThread(p.captureAdmission(), Task{
		Owner: obj,
		Run:   func(th Thread) { fn(th) },
	})
}

// CreateAndStart creates a coroutine and requests eager scheduling.
// Initialized managed callers then wait for the child's first yield or completion.
func (p *Coroutines) CreateAndStart(obj ThreadObj, fn func(me Thread) int) Thread {
	th := p.Create(obj, fn)
	if p.initialized.Load() && p.callerThread() != nil {
		// Wait for this child so another yield job cannot reacquire runMu first.
		p.JoinYieldedOrDone(th)
	} else {
		runtime.Gosched()
	}
	return th
}

// LastThreadID returns the most recently allocated thread ID.
func (p *Coroutines) LastThreadID() int64 {
	return p.nextThreadID.Load()
}

type threadAdmission struct {
	epoch   uint64
	allowed bool
}

func (a threadAdmission) valid(epoch uint64) bool {
	return a.allowed && a.epoch == epoch
}

func (p *Coroutines) captureAdmission() threadAdmission {
	epoch := p.admissionEpoch.Load()
	return threadAdmission{
		epoch:   epoch,
		allowed: epoch&1 == 0 && !p.isThreadCanceled(p.callerThread()),
	}
}

// createThread admits a thread before setup or execution.
func (p *Coroutines) createThread(admission threadAdmission, task Task) Thread {
	th := p.newThread(task.Owner)
	var cleanup func()
	defer func() { go p.runThread(th, task.Run, cleanup) }()

	p.admissionMu.RLock()
	rejected := !admission.valid(p.admissionEpoch.Load())
	if rejected {
		th.Cancel()
	} else {
		p.registerThread(th)
		if task.Setup != nil {
			p.pendingSetups.Add(1)
		}
	}
	p.admissionMu.RUnlock()

	if !rejected && task.Setup != nil {
		id, previous := p.enterCallback(callbackSetup)
		defer p.leaveCallback(id, previous)
		setupReturned := false
		defer func() {
			if !setupReturned {
				th.Cancel()
			}
			p.finishSetup()
		}()
		cleanup = task.Setup(th)
		setupReturned = true
	}
	return th
}

func (p *Coroutines) runThread(th Thread, fn func(Thread), cleanup func()) {
	gid := gid.Get()
	p.goroutineThreads.Store(gid, th)
	p.runMu.Lock()
	p.setCurrent(th)
	defer func() {
		p.finishThread(th, gid, recover())
	}()
	if cleanup != nil {
		defer func() {
			id, previous := p.enterCallback(callbackCleanup)
			defer p.leaveCallback(id, previous)
			cleanup()
		}()
	}

	if th.stopped.Load() {
		panic(ErrAbortThread)
	}
	fn(th)
}

func (p *Coroutines) finishThread(th Thread, gid uint64, recovered any) {
	for waiter := range th.yieldWaiters.close(th.yieldedOrDone) {
		p.markRunnableAndResume(waiter)
	}
	for waiter := range th.joinWaiters.close(nil) {
		p.markRunnableAndResume(waiter)
	}
	// Make waiters runnable before removing the target's scheduler state.
	th.cancelContext()
	close(th.done)
	p.removeThreadState(th)
	p.setCurrent(nil)
	// Draining also waits for the panic handler below.
	defer p.unregisterThread(th)
	p.runMu.Unlock()
	id, previous := p.enterCallback(callbackFinalizing)
	defer p.leaveCallback(id, previous)
	p.goroutineThreads.Delete(gid)
	p.handleThreadPanic(th, recovered)
}

func (p *Coroutines) handleThreadPanic(th Thread, recovered any) {
	if recovered == nil || recovered == ErrAbortThread || recovered == ErrStopThisScript {
		return
	}
	if p.onPanic != nil {
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
	// Update must observe the new runnable thread before its goroutine starts.
	p.setThreadState(th, threadRunnable)
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.threadsMu.Lock()
	if _, registered := p.allThreads[th]; registered {
		delete(p.allThreads, th)
		p.notifyDrainLocked()
	}
	p.threadsMu.Unlock()
}

// admitWorker registers external work under the admission barrier.
func (p *Coroutines) admitWorker(me Thread) bool {
	p.admissionMu.RLock()
	defer p.admissionMu.RUnlock()
	if p.admissionClosed() || p.isThreadCanceled(me) {
		return false
	}
	p.threadsMu.Lock()
	p.workerCount++
	p.threadsMu.Unlock()
	return true
}

// WaitToDo pairs each admission with one deferred finish in the worker.
func (p *Coroutines) finishWorker() {
	p.threadsMu.Lock()
	p.workerCount--
	p.notifyDrainLocked()
	p.threadsMu.Unlock()
}

func (p *Coroutines) notifyDrainLocked() {
	if p.lifecycleChanged != nil {
		close(p.lifecycleChanged)
		p.lifecycleChanged = nil
	}
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
