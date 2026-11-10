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
	"errors"
	"runtime"
	sdebug "runtime/debug"
	"sync"
	"sync/atomic"
	stime "time"

	"github.com/visualfc/gid"
)

var (
	// ErrNotCurrent signals a yield without execution ownership.
	ErrNotCurrent = errors.New("can not yield a non-running thread")

	// ErrForeignThread rejects waiting on a thread owned by another scheduler.
	ErrForeignThread = errors.New("coroutine: thread belongs to another manager")

	// ErrAbortThread is the panic value used to stop the current coroutine.
	ErrAbortThread = errors.New("abort thread")

	// ErrStopThisScript signals Scratch-style termination to a procedure boundary.
	ErrStopThisScript = errors.New("stop this script")

	// ErrReentrantWait rejects a synchronous wait that would depend on its caller.
	ErrReentrantWait = errors.New("coroutine: caller cannot synchronously wait for itself")
)

// PanicReport preserves an unhandled coroutine panic and its diagnostic context.
type PanicReport struct {
	Value any
	Name  string
	Stack string // Stack at the panic site, captured during recovery.
}

// Scheduler coordinates thread lifecycle and cooperative scheduling.
type Scheduler struct {
	onPanic func(PanicReport)

	// runMu serializes script slices; current identifies its owner.
	runMu   sync.Mutex
	current atomic.Pointer[threadImpl]

	// shutdownGate serializes external drains; admissionMu guards admission and registration.
	// Lock order: shutdownGate, runMu, admissionMu.
	shutdownGate chan struct{}
	admissionMu  sync.RWMutex
	stopping     bool // Implies closed admission when observed under admissionMu.RLock.

	// threadsMu protects lifecycle registration and the drain notification.
	threadsMu   sync.Mutex
	allThreads  map[Thread]struct{}
	workerCount int
	drainDone   chan struct{} // Created lazily by drains; closed when all work exits.

	// schedulerMu guards script selection, pendingExecution, readyQueue, currentJobs, and mainCalls.
	// A single Update call owns roundJobs and deferredJobs.
	schedulerMu    sync.Mutex
	schedulerCond  *sync.Cond
	*runnableState                   // Published scripts and cancellation cleanup.
	waitBatch      waitBatch         // Unpublished batch; guarded by schedulerMu.
	selectedThread Thread            // Scheduler permit owner; may still be waiting for runMu.
	mainCalls      []*mainThreadCall // Engine calls, served in reverse arrival order.
	currentJobs    *queue[*waitJob]
	deferredJobs   *queue[*waitJob]
	roundJobs      *queue[*waitJob]
	redrawFrame    atomic.Int64
	scriptRound    atomic.Uint64

	nextThreadID atomic.Int64

	// admissionEpoch is even while admission is open and odd while closed.
	admissionEpoch atomic.Uint64
	// Concurrent admission readers register setups; unfinished setups delay reopening.
	pendingSetups atomic.Int64

	perfDebug         atomic.Bool
	readGCStats       func(*sdebug.GCStats)
	updateWatchdogNow func() stime.Time
	statsMu           sync.RWMutex
	lastUpdateStats   UpdateStats

	// Caller identity is independent of the script owning runMu.
	goroutineThreads sync.Map // map[uint64]Thread

	// Callbacks cannot drain themselves; exclusive callbacks also retain runMu.
	callbacks sync.Map // map[uint64]callbackScope
}

// A runnableState keeps queue membership and pending cleanup together. It must
// not be copied because threads retain pointers into its ready heaps.
type runnableState struct {
	readyQueue       readyQueue
	pendingExecution map[Thread]struct{}
}

// New creates a coroutine scheduler. onPanic is called when a coroutine exits
// with an unhandled panic other than ErrAbortThread or ErrStopThisScript.
func New(onPanic func(PanicReport)) *Scheduler {
	// Nodes move between these queues, so they share a recycling pool.
	jobs := newQueue[*waitJob]()
	p := &Scheduler{
		onPanic:       onPanic,
		shutdownGate:  make(chan struct{}, 1),
		allThreads:    make(map[Thread]struct{}),
		runnableState: newRunnableState(),
		waitBatch: waitBatch{
			runnable:     newRunnableState(),
			deferredJobs: &queue[*waitJob]{pool: jobs.pool},
			roundJobs:    &queue[*waitJob]{pool: jobs.pool},
		},
		currentJobs:       jobs,
		deferredJobs:      &queue[*waitJob]{pool: jobs.pool},
		roundJobs:         &queue[*waitJob]{pool: jobs.pool},
		readGCStats:       sdebug.ReadGCStats,
		updateWatchdogNow: stime.Now,
	}
	p.shutdownGate <- struct{}{}
	p.schedulerCond = sync.NewCond(&p.schedulerMu)
	p.redrawFrame.Store(-1)
	return p
}

// Sched yields the current coroutine and resumes it without an Update pass.
func (p *Scheduler) Sched(me Thread) {
	p.requireCurrent(me)
	p.requireCanWait()
	// Requeue this script before advancing to the next runnable position.
	p.schedulerMu.Lock()
	p.blockThreadLocked(me)
	p.markPendingLocked(me)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.yield(me)
}

// StopAtNextYield cancels me when it next yields to the scheduler.
// It must be called by the currently running managed coroutine.
func (p *Scheduler) StopAtNextYield(me Thread) {
	p.requireCurrent(me)
	me.stopAtNextYield.Store(true)
}

// IsInCoroutine reports whether the caller is running in this scheduler.
func (p *Scheduler) IsInCoroutine() bool {
	return p.Caller() != nil
}

// Current returns the coroutine that currently owns runMu, or nil.
func (p *Scheduler) Current() Thread {
	return p.current.Load()
}

// Caller returns this scheduler's thread running on the calling goroutine, or nil.
// A nil scheduler has no calling thread.
// Unlike Current, it never borrows another goroutine's script ownership.
func (p *Scheduler) Caller() Thread {
	if p == nil {
		return nil
	}
	value, ok := p.goroutineThreads.Load(gid.Get())
	if !ok {
		return nil
	}
	return value.(Thread)
}

// yield releases the current slice after its blocking state has been published.
func (p *Scheduler) yield(me Thread) {
	p.requireCurrent(me)
	p.schedulerMu.Lock()
	p.releaseLocked(me)
	resumeSelf := p.selectedThread == me
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
	p.current.Store(nil)
	p.runMu.Unlock()
	if resumeSelf {
		// The permit stays with this script, but consuming its own signal
		// would not park the Go goroutine. Let workers progress on WASM too.
		runtime.Gosched()
	}
	p.acquire(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

// acquire consumes the scheduler's permit, then takes exclusive script access.
// Cancellation also wakes a thread so its defers can unwind under runMu.
func (p *Scheduler) acquire(me Thread) {
	me.suspendMu.Lock()
	if me.suspendState == suspendSignaled {
		me.suspendState = suspendRunning
	} else {
		me.suspendState = suspendWaiting
	}
	for me.suspendState == suspendWaiting && !me.canceled() {
		me.suspendCond.Wait()
	}
	me.suspendState = suspendRunning
	me.suspendMu.Unlock()
	if me.canceled() {
		// Cancellation can wake an unpublished or blocked script without
		// a scheduler permit. Its cleanup is now executable work for Update.
		p.trackCleanup(me)
	}
	p.runMu.Lock()
	p.current.Store(me)
}

func (p *Scheduler) trackCleanup(th Thread) {
	p.schedulerMu.Lock()
	p.pendingExecution[th] = struct{}{}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Scheduler) requireCurrent(me Thread) {
	if me == nil || p.Caller() != me || p.Current() != me {
		panic(ErrNotCurrent)
	}
}

// wakeThread marks th pending and schedules the next ready script.
func (p *Scheduler) wakeThread(th Thread) {
	p.schedulerMu.Lock()
	p.markPendingLocked(th)
	p.scheduleLocked()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

func (p *Scheduler) blockThread(th Thread) {
	p.schedulerMu.Lock()
	p.blockThreadLocked(th)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}

// Publish first-slice continuations together with the blocked script.
func (p *Scheduler) blockThreadLocked(th Thread) {
	if th.stopAtNextYield.Swap(false) {
		th.Cancel()
	}
	th.removeReadyLocked()
	if !th.canceled() {
		delete(p.pendingExecution, th)
	}
	p.wakeWaitersLocked(th.firstSliceWaiters.close(th.firstSliceDone))
}

// Canceled threads can unwind under runMu without owning a scheduler permit.
func (p *Scheduler) releaseLocked(th Thread) {
	if p.selectedThread == th {
		p.selectedThread = nil
	}
	p.scheduleLocked()
}

// Visit runnable scripts in their original order, wrapping after each round.
// New scripts append to the round; restarted handlers retain their old position.
func (p *Scheduler) scheduleLocked() {
	if p.selectedThread != nil {
		return
	}
	p.selectedThread = p.readyQueue.pop()
	if p.selectedThread != nil {
		p.selectedThread.resume()
	}
}

func (p *Scheduler) wakeWaitersLocked(waiters map[Thread]struct{}) {
	for waiter := range waiters {
		p.markPendingLocked(waiter)
	}
	p.scheduleLocked()
}

// Caller must hold schedulerMu.
func (p *Scheduler) hasPendingExecutionLocked() bool {
	// Canceled scripts remain pending until their executable cleanup finishes.
	return p.selectedThread != nil || len(p.pendingExecution) != 0
}

func newRunnableState() *runnableState {
	return &runnableState{pendingExecution: make(map[Thread]struct{})}
}

// Mark published work pending, queuing live scripts and retaining cancellation cleanup.
func (p *runnableState) markPendingLocked(th Thread) {
	if _, pending := p.pendingExecution[th]; pending || !th.published {
		return
	}
	select {
	case <-th.done:
		return
	default:
	}
	p.pendingExecution[th] = struct{}{}
	if !th.canceled() {
		p.readyQueue.add(th)
	}
}
