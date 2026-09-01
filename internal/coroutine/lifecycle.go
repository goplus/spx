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
	"reflect"
	"runtime"
	"sync"
	stime "time"

	"github.com/goplus/spx/v3/internal/debug"
	"github.com/goplus/spx/v3/internal/engine/platform"
	"github.com/visualfc/gid"
)

type threadNamer interface {
	Name() string
}

// Create creates a coroutine without explicitly yielding execution to it.
func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.createAndStart(false, obj, fn, nil, nil)
}

// CreateAndStart creates a coroutine. start controls eager scheduling: when it
// is true, the new coroutine gets an immediate opportunity to run before this
// method returns.
func (p *Coroutines) CreateAndStart(start bool, obj ThreadObj, fn func(me Thread) int) Thread {
	return p.createAndStart(start, obj, fn, nil, nil)
}

// CreateWithFinalizer creates a coroutine whose finalizer runs before the
// thread is unregistered, including when cancellation wins before fn starts.
// The finalizer is serialized with scripts and is detached from the finished
// thread's cancellation context, so it may perform synchronous main-thread
// cleanup. It must not wait for other coroutines to make progress.
func (p *Coroutines) CreateWithFinalizer(
	obj ThreadObj,
	fn func(me Thread) int,
	finalizer func(),
) Thread {
	return p.createAndStart(false, obj, fn, finalizer, nil)
}

// CreateChildWithFinalizer atomically validates a parent against AbortAll and
// registers its child. A nil result means cancellation won before handoff.
// expectedAbortEpoch must have been captured from AbortEpoch by the parent
// operation that owns the child lifecycle.
func (p *Coroutines) CreateChildWithFinalizer(
	parent Thread,
	expectedAbortEpoch uint64,
	obj ThreadObj,
	fn func(me Thread) int,
	finalizer func(),
) Thread {
	return p.createAndStart(false, obj, fn, finalizer, func() bool {
		return p.abortEpoch.Load() == expectedAbortEpoch &&
			!p.isThreadCanceled(parent)
	})
}

// AdmitChildHandoff atomically validates an inline child operation against
// AbortAll. It is the registration-free counterpart of
// CreateChildWithFinalizer: cancellation and handoff are ordered by
// creationMu, but the caller does not hold the barrier while running call.
func (p *Coroutines) AdmitChildHandoff(parent Thread, expectedAbortEpoch uint64) bool {
	p.creationMu.RLock()
	admitted := p.abortEpoch.Load() == expectedAbortEpoch &&
		!p.isThreadCanceled(parent)
	p.creationMu.RUnlock()
	return admitted
}

func (p *Coroutines) createAndStart(
	start bool,
	obj ThreadObj,
	fn func(me Thread) int,
	finalizer func(),
	admit func() bool,
) Thread {
	th := p.newThread(obj)
	th.onFinished = finalizer
	p.creationMu.RLock()
	if admit != nil && !admit() {
		p.creationMu.RUnlock()
		return nil
	}
	p.registerThread(th)
	if p.stopping {
		stopThreadIfRunning(th)
	}
	p.creationMu.RUnlock()
	go p.runThread(th, fn)

	if start {
		// Wait for this child so another yield job cannot reacquire runMu first.
		if p.hasInited.Load() {
			if caller := p.currentCoroutineThread(); caller != nil {
				p.JoinYieldedOrDone(th)
				return th
			}
		}
		runtime.Gosched()
	}
	return th
}

// LastThreadID returns the most recently allocated thread ID.
func (p *Coroutines) LastThreadID() int64 {
	return p.nextThreadID.Load()
}

// Abort stops the current coroutine by panicking with ErrAbortThread.
func (p *Coroutines) Abort() {
	panic(ErrAbortThread)
}

// AbortAll requests cancellation of every registered coroutine.
func (p *Coroutines) AbortAll() {
	p.creationMu.Lock()
	p.abortAllLocked()
	p.creationMu.Unlock()
}

// abortAllLocked requires creationMu to be write-locked, making the epoch,
// registry snapshot, and cancellation indivisible from guarded registration.
func (p *Coroutines) abortAllLocked() {
	p.abortEpoch.Add(1)
	for _, th := range p.snapshotThreads() {
		stopThreadIfRunning(th)
	}
}

// AbortEpoch returns a token that changes whenever AbortAll is requested.
// Work created from a canceled script can use it to reject late handoff.
func (p *Coroutines) AbortEpoch() uint64 {
	return p.abortEpoch.Load()
}

// AbortAllAndWait aborts all registered coroutines and waits for every thread
// other than the caller to stop. A non-positive timeout waits indefinitely.
func (p *Coroutines) AbortAllAndWait(timeout stime.Duration) bool {
	caller := p.currentCoroutineThread()
	p.AbortAll()
	serviceMainThread, unlockOSThread := lockCurrentMainThread()
	if unlockOSThread != nil {
		defer unlockOSThread()
	}
	if caller != nil {
		return p.waitForThreadsToStopFromCoroutine(timeout, caller, serviceMainThread)
	}
	return p.waitForThreadsToStopWithMainThreadJobs(timeout, nil, serviceMainThread)
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
		stopThread(th)
	}
}

// Stop cancels thread; nil and repeated calls are safe.
func (p *Coroutines) Stop(thread Thread) {
	if thread != nil {
		stopThreadIfRunning(thread)
	}
}

// IsInCoroutine reports whether the caller is running in this manager.
func (p *Coroutines) IsInCoroutine() bool {
	_, exists := p.goroutineIDs.Load(gid.Get())
	return exists
}

// RunSynchronizedCleanup runs call while excluding scripts and without
// inheriting the caller's coroutine cancellation. A managed caller already
// owns runMu; an external caller acquires it. Cleanup callbacks must not wait
// for another coroutine to make progress and must not nest this method.
func (p *Coroutines) RunSynchronizedCleanup(call func()) {
	if call == nil {
		return
	}
	gID := gid.Get()
	if _, managed := p.goroutineIDs.Load(gID); managed {
		p.runWithoutThreadContext(p.Current(), gID, call)
		return
	}
	serviceMainThread, unlockOSThread := lockCurrentMainThread()
	if unlockOSThread != nil {
		defer unlockOSThread()
	}
	if serviceMainThread {
		for !p.runMu.TryLock() {
			if !p.runQueuedMainThreadJobs() {
				runtime.Gosched()
			}
		}
	} else {
		p.runMu.Lock()
	}
	defer p.runMu.Unlock()
	call()
}

func stopThreadIfRunning(th Thread) {
	th.suspendMu.Lock()
	if th.stopped.Load() {
		th.suspendMu.Unlock()
		return
	}
	stopThreadLocked(th)
	th.suspendMu.Unlock()
}

func stopThread(th Thread) {
	th.suspendMu.Lock()
	stopThreadLocked(th)
	th.suspendMu.Unlock()
}

func stopThreadLocked(th Thread) {
	th.stopped.Store(true)
	th.Cancel()
	th.suspendCond.Signal()
}

func (p *Coroutines) currentCoroutineThread() Thread {
	if !p.IsInCoroutine() {
		return nil
	}
	return p.Current()
}

func (p *Coroutines) newThread(obj ThreadObj) Thread {
	th := &threadImpl{
		Obj:           obj,
		id:            p.nextThreadID.Add(1),
		schedFrame:    -1,
		name:          resolveThreadName(obj),
		done:          make(chan struct{}),
		yieldedOrDone: make(chan struct{}),
	}
	th.ctx, th.cancelFunc = context.WithCancel(context.Background())
	if p.debug {
		th.stack = debug.GetStackTrace()
	}
	th.suspendCond = sync.NewCond(&th.suspendMu)
	return th
}

func resolveThreadName(obj ThreadObj) string {
	if obj == nil {
		return ""
	}
	if name, ok := obj.(string); ok {
		return name
	}
	if named, ok := obj.(threadNamer); ok {
		return named.Name()
	}

	typ := reflect.TypeOf(obj)
	if typ.Kind() != reflect.Pointer || typ.Elem().Name() == "" {
		return ""
	}
	return "*" + typ.Elem().Name()
}

func (p *Coroutines) registerThread(th Thread) {
	p.threadsMu.Lock()
	p.allThreads[th] = struct{}{}
	p.threadsMu.Unlock()
	// Update treats synchronous registration as a spawn barrier.
	p.setThreadState(th, threadRunnable)
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.threadsMu.Lock()
	delete(p.allThreads, th)
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

func (p *Coroutines) hasThreadsOtherThan(skip Thread) bool {
	p.threadsMu.Lock()
	defer p.threadsMu.Unlock()
	for th := range p.allThreads {
		if th != skip {
			return true
		}
	}
	return false
}

func (p *Coroutines) waitForThreadsToStopFromCoroutine(
	timeout stime.Duration,
	caller Thread,
	serviceMainThread bool,
) bool {
	// Release runMu so canceled peers can unregister.
	p.setCurrent(nil)
	p.runMu.Unlock()
	completed := p.waitForThreadsToStopWithMainThreadJobs(timeout, caller, serviceMainThread)
	p.runMu.Lock()
	p.setCurrent(caller)
	return completed
}

func (p *Coroutines) waitForThreadsToStop(timeout stime.Duration, skip Thread) bool {
	return p.waitForThreadsToStopWithMainThreadJobs(timeout, skip, false)
}

func (p *Coroutines) waitForThreadsToStopWithMainThreadJobs(
	timeout stime.Duration,
	skip Thread,
	serviceMainThread bool,
) bool {
	hasTimeout := timeout > 0
	deadline := stime.Time{}
	if hasTimeout {
		deadline = stime.Now().Add(timeout)
	}

	for {
		if !p.hasThreadsOtherThan(skip) {
			return true
		}
		if serviceMainThread && p.runQueuedMainThreadJobs() {
			continue
		}

		sleepFor := 10 * stime.Millisecond
		if hasTimeout {
			remaining := stime.Until(deadline)
			if remaining <= 0 {
				return false
			}
			if remaining < sleepFor {
				sleepFor = remaining
			}
		}
		stime.Sleep(sleepFor)
	}
}

// runQueuedMainThreadJobs services main-thread callbacks while a shutdown
// barrier waits for finalizers. Other scheduler jobs retain their order and
// remain queued; no canceled script is resumed from this path.
func (p *Coroutines) runQueuedMainThreadJobs() bool {
	p.jobConsumerMu.Lock()
	defer p.jobConsumerMu.Unlock()
	count := p.currentJobs.Count()
	ran := false
	for range count {
		job := p.currentJobs.PopFront()
		if job.Type != waitTypeMainThread {
			p.currentJobs.PushBack(job)
			continue
		}
		if platform.TryCallEngineDirectly(job.Call) {
			ran = true
		} else {
			p.currentJobs.PushBack(job)
		}
	}
	return ran
}

// lockCurrentMainThread pins the caller across a shutdown wait. The native
// platform check and every serviced engine callback must observe the same OS
// thread.
func lockCurrentMainThread() (isMain bool, unlock func()) {
	runtime.LockOSThread()
	if platform.TryCallEngineDirectly(func() {}) {
		return true, runtime.UnlockOSThread
	}
	runtime.UnlockOSThread()
	return false, nil
}

func (p *Coroutines) runThread(th Thread, fn func(me Thread) int) {
	gid := gid.Get()
	p.goroutineIDs.Store(gid, struct{}{})
	p.runMu.Lock()
	p.setCurrent(th)
	defer func() {
		p.finishThread(th, gid, recover())
	}()

	if th.stopped.Load() {
		panic(ErrAbortThread)
	}
	fn(th)
}

func (p *Coroutines) finishThread(th Thread, gid uint64, recovered any) {
	finalizerPanic := p.runThreadFinalizer(th, gid)
	for _, waiter := range th.finishYieldWaiters() {
		p.markRunnableAndResume(waiter)
	}
	for _, waiter := range th.finishJoinWaiters() {
		p.markRunnableAndResume(waiter)
	}
	// Make waiters runnable before removing the target's scheduler state.
	th.Cancel()
	close(th.done)
	p.removeThreadState(th)
	p.setCurrent(nil)
	p.unregisterThread(th)
	p.runMu.Unlock()
	p.goroutineIDs.Delete(gid)
	if (recovered == nil || recovered == ErrAbortThread) && finalizerPanic != nil {
		recovered = finalizerPanic
	}
	p.handleThreadPanic(th, recovered)
}

// runThreadFinalizer temporarily removes the finished thread from coroutine
// identity while retaining runMu. This keeps cleanup serialized with scripts
// without inheriting a canceled context in WaitMainThread calls.
func (p *Coroutines) runThreadFinalizer(th Thread, gid uint64) (recovered any) {
	if th.onFinished == nil {
		return nil
	}
	defer func() {
		recovered = recover()
	}()
	p.runWithoutThreadContext(th, gid, th.onFinished)
	return nil
}

func (p *Coroutines) runWithoutThreadContext(th Thread, gid uint64, call func()) {
	p.setCurrent(nil)
	p.goroutineIDs.Delete(gid)
	defer func() {
		p.goroutineIDs.Store(gid, struct{}{})
		p.setCurrent(th)
	}()
	call()
}

func (p *Coroutines) handleThreadPanic(th Thread, recovered any) {
	if recovered == nil || recovered == ErrAbortThread {
		return
	}
	if p.onPanic != nil {
		p.onPanic(th.name, th.stack)
		return
	}
	panic(recovered)
}
