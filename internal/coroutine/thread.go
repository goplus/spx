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
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	stime "time"
	"unsafe"

	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/petermattis/goid"
)

type ThreadObj any

type threadImpl struct {
	Obj       ThreadObj
	stopped   atomic.Bool
	suspended atomic.Bool // Per-thread suspension state to avoid lock-order inversion
	frame     int
	mutex     sync.Mutex // Mutex for this thread's condition variable
	cond      *sync.Cond // Per-thread condition variable for targeted wake-up
	id        int64
	name      string
	stack     string

	schedFrame     int64
	schedTimestamp stime.Time

	runWithoutScreenRefresh      atomic.Bool
	runWithoutScreenRefreshStart atomic.Int64

	ctx        context.Context
	cancelFunc context.CancelFunc
	done       chan struct{}

	joinMu     sync.Mutex
	joinDone   bool
	joinWaiter map[Thread]struct{}
}

// Thread represents a coroutine id.
type Thread = *threadImpl

type threadNamer interface {
	Name() string
}

// -------------------------------------------------------------------------------------
// Thread Methods
// -------------------------------------------------------------------------------------

func (p *threadImpl) Context() context.Context {
	if p.ctx == nil {
		return context.Background()
	}
	return p.ctx
}

func (p *threadImpl) Cancel() {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
}

func (p *threadImpl) String() string {
	return fmt.Sprintf("id=%d name=%s ", p.id, p.name)
}

func (p *threadImpl) Name() string {
	return p.name
}

func (p *threadImpl) Stack() string {
	return p.stack
}

func (p *threadImpl) Stopped() bool {
	return p.stopped.Load()
}

func (p *threadImpl) RunWithoutScreenRefresh() bool {
	return p.runWithoutScreenRefresh.Load()
}

func (p *threadImpl) SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	previous = p.runWithoutScreenRefresh.Swap(enabled)
	if enabled != previous {
		p.runWithoutScreenRefreshStart.Store(0)
	}
	return
}

func (p *threadImpl) ShouldWaitNextFrame(runWithoutScreenRefreshBudget stime.Duration) bool {
	if !p.RunWithoutScreenRefresh() {
		return true
	}

	now := stime.Now().UnixNano()
	startedAt := p.runWithoutScreenRefreshStart.Load()
	if startedAt == 0 {
		p.runWithoutScreenRefreshStart.Store(now)
		return false
	}
	if stime.Duration(now-startedAt) <= runWithoutScreenRefreshBudget {
		return false
	}

	// Match Scratch warp behavior by forcing a yield once the time budget is up,
	// then restarting the budget window on the next loop edge.
	p.runWithoutScreenRefreshStart.Store(0)
	return true
}

func (p Thread) IsSchedTimeout(ms float64) bool {
	frame := time.Frame()
	if p.schedFrame < frame {
		p.schedFrame = frame
		p.schedTimestamp = stime.Now()
	}
	timeout := stime.Since(p.schedTimestamp) > stime.Duration(ms)*stime.Millisecond
	return timeout
}

// -------------------------------------------------------------------------------------
// Thread Creation & Management
// -------------------------------------------------------------------------------------

func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(false, obj, fn)
}

func (p *Coroutines) CreateAndStart(start bool, obj ThreadObj, fn func(me Thread) int) Thread {
	th := p.newThread(obj)
	p.registerThread(th)

	go func() {
		p.runThread(th, fn)
	}()

	if start {
		runtime.Gosched()
	}
	return th
}

func (p *Coroutines) Current() Thread {
	return Thread(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&p.current))))
}

func (p *Coroutines) Abort() {
	panic(ErrAbortThread)
}

func (p *Coroutines) AbortAll() {
	p.mutex.Lock()
	threads := make([]Thread, 0, len(p.allThreads))
	for th := range p.allThreads {
		threads = append(threads, th)
	}
	p.mutex.Unlock()

	for _, th := range threads {
		th.mutex.Lock()
		if !th.stopped.Load() {
			th.stopped.Store(true)
			th.Cancel()
			th.cond.Signal()
		}
		th.mutex.Unlock()
	}
}

func (p *Coroutines) AbortAllAndWait(timeout stime.Duration) bool {
	caller := p.currentCoroutineThread()
	p.AbortAll()

	if caller != nil {
		return p.waitForThreadsToStopFromCoroutine(timeout, caller)
	}

	return p.waitForThreadsToStop(timeout, nil)
}

func (p *Coroutines) Join(target Thread) {
	if target == nil {
		return
	}

	me := p.currentCoroutineThread()
	if me == nil {
		<-target.done
		return
	}
	if me == target {
		return
	}
	if !target.addJoinWaiter(me) {
		return
	}
	p.blockAndYield(me)
}

func (p *Coroutines) JoinAll(targets []Thread) {
	if len(targets) == 0 {
		return
	}

	seen := make(map[Thread]struct{}, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		p.Join(target)
	}
}

func (p *Coroutines) StopIf(filter func(th Thread) bool) {
	p.mutex.Lock()
	allThreads := make([]Thread, 0, len(p.allThreads))
	for th := range p.allThreads {
		allThreads = append(allThreads, th)
	}
	p.mutex.Unlock()

	threads := allThreads[:0]
	for _, th := range allThreads {
		if filter(th) {
			threads = append(threads, th)
		}
	}

	// Stop each thread with proper signaling
	for _, th := range threads {
		th.mutex.Lock()
		th.stopped.Store(true)
		th.Cancel()
		th.cond.Signal()
		th.mutex.Unlock()
	}
}

func (p *Coroutines) IsInCoroutine() bool {
	currentGID := goid.Get()
	_, exists := p.goroutineIDs.Load(currentGID)
	return exists
}

// -------------------------------------------------------------------------------------
// Internal Helpers (unexported)
// -------------------------------------------------------------------------------------

func (p *Coroutines) setCurrent(id Thread) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.current)), unsafe.Pointer(id))
}

func resolveThreadName(obj ThreadObj) string {
	if obj == nil {
		return ""
	}

	if str, ok := obj.(string); ok {
		return str
	}

	if named, ok := obj.(threadNamer); ok {
		return named.Name()
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer || t.Elem().Name() == "" {
		return ""
	}

	return "*" + t.Elem().Name()
}

func (p *Coroutines) newThread(obj ThreadObj) Thread {
	th := &threadImpl{
		Obj:        obj,
		frame:      p.frame,
		id:         atomic.AddInt64(&p.nextThreadID, 1),
		schedFrame: -1,
		name:       resolveThreadName(obj),
		done:       make(chan struct{}),
	}
	th.ctx, th.cancelFunc = context.WithCancel(context.Background())

	if p.debug {
		th.stack = debug.GetStackTrace()
	}

	th.cond = sync.NewCond(&th.mutex)
	return th
}

func (p *threadImpl) addJoinWaiter(waiter Thread) bool {
	if waiter == nil {
		return false
	}

	p.joinMu.Lock()
	defer p.joinMu.Unlock()
	if p.joinDone {
		return false
	}
	if p.joinWaiter == nil {
		p.joinWaiter = make(map[Thread]struct{})
	}
	p.joinWaiter[waiter] = struct{}{}
	return true
}

func (p *threadImpl) finishJoinWaiters() []Thread {
	p.joinMu.Lock()
	waiters := make([]Thread, 0, len(p.joinWaiter))
	for waiter := range p.joinWaiter {
		waiters = append(waiters, waiter)
	}
	p.joinWaiter = nil
	p.joinDone = true
	p.joinMu.Unlock()

	close(p.done)
	return waiters
}

func (p *Coroutines) registerThread(th Thread) {
	p.wg.Add(1)
	p.mutex.Lock()
	p.allThreads[th] = struct{}{}
	p.mutex.Unlock()
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.mutex.Lock()
	delete(p.suspended, th)
	delete(p.allThreads, th)
	p.mutex.Unlock()
	p.wg.Done()
}

func (p *Coroutines) currentCoroutineThread() Thread {
	if !p.IsInCoroutine() {
		return nil
	}
	return p.Current()
}

func (p *Coroutines) hasThreadsOtherThan(skip Thread) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for th := range p.allThreads {
		if th != skip {
			return true
		}
	}
	return false
}

func (p *Coroutines) waitForThreadsToStopFromCoroutine(timeout stime.Duration, caller Thread) bool {
	// Release the scheduler while waiting so aborted peer coroutines can
	// observe cancellation and unregister.
	p.sema.Unlock()
	completed := p.waitForThreadsToStop(timeout, caller)
	p.sema.Lock()
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

func (p *Coroutines) runThread(th Thread, fn func(me Thread) int) {
	gid := goid.Get()
	p.goroutineIDs.Store(gid, true)
	p.sema.Lock()
	p.setCurrent(th)
	defer func() {
		recovered := recover()
		p.unregisterThread(th)
		p.setWaitState(th, waitStatusDelete)
		th.Cancel()
		for _, waiter := range th.finishJoinWaiters() {
			p.markIdleAndResume(waiter)
		}
		p.sema.Unlock()
		p.goroutineIDs.Delete(gid)
		p.handleThreadPanic(th, recovered)
	}()
	if th.stopped.Load() {
		panic(ErrAbortThread)
	}
	p.setWaitState(th, waitStatusAdd)
	fn(th)
}
