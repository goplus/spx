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
	sdebug "runtime/debug"
	"sync"
	"sync/atomic"
	stime "time"
)

var (
	// ErrCannotYieldANonrunningThread signals a yield without execution ownership.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")

	// ErrAbortThread is the panic value used to stop the current coroutine.
	ErrAbortThread = errors.New("abort thread")

	// ErrStopThisScript signals Scratch-style termination to a procedure boundary.
	ErrStopThisScript = errors.New("stop this script")

	// ErrReentrantWait rejects a synchronous wait that would depend on its caller.
	ErrReentrantWait = errors.New("coroutine: callback cannot synchronously wait for itself")
)

// PanicReport preserves an unhandled coroutine panic and its diagnostic context.
type PanicReport struct {
	Value         any
	Name          string
	Stack         string // Stack at the panic site, captured during recovery.
	CreationStack string // Optional stack captured when the coroutine was created.
}

// Coroutines coordinates thread lifecycle and cooperative scheduling.
type Coroutines struct {
	onPanic     func(PanicReport)
	initialized atomic.Bool
	debug       bool

	// runMu serializes script slices; current identifies its owner.
	runMu   sync.Mutex
	current atomic.Pointer[threadImpl]

	// shutdownMu serializes external drains; admissionMu guards admission and registration.
	// Lock order: shutdownMu, runMu, admissionMu.
	shutdownMu  sync.Mutex
	admissionMu sync.RWMutex
	stopping    bool // Implies closed admission when observed under admissionMu.RLock.

	// threadsMu protects lifecycle registration and the drain notification.
	threadsMu        sync.Mutex
	allThreads       map[Thread]struct{}
	workerCount      int
	lifecycleChanged chan struct{} // Created lazily by drains; closed on removal.

	// schedulerMu guards runnable state and atomic state/queue publication.
	schedulerMu     sync.Mutex
	schedulerCond   *sync.Cond
	runnableThreads map[Thread]struct{} // Cancellation is checked before execution.
	currentJobs     *Queue[*WaitJob]
	deferredJobs    *Queue[*WaitJob]
	roundJobs       *Queue[*WaitJob]
	redrawFrame     atomic.Int64
	scriptRound     atomic.Uint64

	nextThreadID atomic.Int64

	// admissionEpoch is even while admission is open and odd while closed. Pending
	// setup calls delay reopening after StopAll.
	admissionEpoch atomic.Uint64
	pendingSetups  atomic.Int64

	perfDebug         atomic.Bool
	readGCStats       func(*sdebug.GCStats)
	updateWatchdogNow func() stime.Time
	statsMu           sync.RWMutex
	lastUpdateStats   UpdateJobsStats

	// Caller identity is independent of the script owning runMu.
	goroutineThreads sync.Map // map[uint64]Thread

	// Callbacks cannot drain themselves; exclusive callbacks also retain runMu.
	callbacks sync.Map // map[uint64]callbackScope
}

// New creates a coroutine manager. onPanic is called when a coroutine exits
// with an unhandled panic other than ErrAbortThread or ErrStopThisScript.
func New(onPanic func(PanicReport)) *Coroutines {
	// Nodes move between these queues, so they share a recycling pool.
	jobs := NewQueue[*WaitJob]()
	p := &Coroutines{
		onPanic:           onPanic,
		allThreads:        make(map[Thread]struct{}),
		runnableThreads:   make(map[Thread]struct{}),
		currentJobs:       jobs,
		deferredJobs:      &Queue[*WaitJob]{pool: jobs.pool},
		roundJobs:         &Queue[*WaitJob]{pool: jobs.pool},
		readGCStats:       sdebug.ReadGCStats,
		updateWatchdogNow: stime.Now,
	}
	p.schedulerCond = sync.NewCond(&p.schedulerMu)
	p.redrawFrame.Store(-1)
	return p
}

// ScriptRound counts additional script rounds within engine frames.
// Pair it with the frame number to identify a scheduling round.
func (p *Coroutines) ScriptRound() uint64 {
	return p.scriptRound.Load()
}

// OnRestart marks the scheduler as not yet initialized.
func (p *Coroutines) OnRestart() {
	p.initialized.Store(false)
}

// OnInited marks the scheduler as initialized.
func (p *Coroutines) OnInited() {
	p.initialized.Store(true)
}

// SetPerfDebug enables or disables GC statistics collection during Update.
func (p *Coroutines) SetPerfDebug(enabled bool) {
	p.perfDebug.Store(enabled)
}

// IsAbortThreadError reports whether err is the coroutine abort sentinel.
func IsAbortThreadError(err any) bool {
	return err == ErrAbortThread
}

// IsStopThisScriptError reports whether err is the stop-this-script sentinel.
func IsStopThisScriptError(err any) bool {
	return err == ErrStopThisScript
}
