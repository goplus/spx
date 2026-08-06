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
	"github.com/visualfc/gid"
)

type threadNamer interface {
	Name() string
}

// Create creates a coroutine without explicitly yielding execution to it.
func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(false, obj, fn)
}

// CreateAndStart creates a coroutine. start controls eager scheduling: when it
// is true, the new coroutine gets an immediate opportunity to run before this
// method returns.
func (p *Coroutines) CreateAndStart(start bool, obj ThreadObj, fn func(me Thread) int) Thread {
	th := p.newThread(obj)
	p.registerThread(th)
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
	for _, th := range p.snapshotThreads() {
		stopThreadIfRunning(th)
	}
}

// AbortAllAndWait aborts all registered coroutines and waits for every thread
// other than the caller to stop. A non-positive timeout waits indefinitely.
func (p *Coroutines) AbortAllAndWait(timeout stime.Duration) bool {
	caller := p.currentCoroutineThread()
	p.AbortAll()
	if caller != nil {
		return p.waitForThreadsToStopFromCoroutine(timeout, caller)
	}
	return p.waitForThreadsToStop(timeout, nil)
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

// IsInCoroutine reports whether the caller is running in this manager.
func (p *Coroutines) IsInCoroutine() bool {
	_, exists := p.goroutineIDs.Load(gid.Get())
	return exists
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

func (p *Coroutines) waitForThreadsToStopFromCoroutine(timeout stime.Duration, caller Thread) bool {
	// Release runMu so canceled peers can unregister.
	p.setCurrent(nil)
	p.runMu.Unlock()
	completed := p.waitForThreadsToStop(timeout, caller)
	p.runMu.Lock()
	p.setCurrent(caller)
	return completed
}

func (p *Coroutines) waitForThreadsToStop(timeout stime.Duration, skip Thread) bool {
	hasTimeout := timeout > 0
	deadline := stime.Time{}
	if hasTimeout {
		deadline = stime.Now().Add(timeout)
	}

	for {
		if !p.hasThreadsOtherThan(skip) {
			return true
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
	p.handleThreadPanic(th, recovered)
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
