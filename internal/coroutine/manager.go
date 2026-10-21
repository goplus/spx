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
	// ErrNotCurrent signals a yield without execution ownership.
	ErrNotCurrent = errors.New("can not yield a non-running thread")

	// ErrAbortThread is the panic value used to stop the current coroutine.
	ErrAbortThread = errors.New("abort thread")

	// ErrStopThisScript signals Scratch-style termination to a procedure boundary.
	ErrStopThisScript = errors.New("stop this script")

	// ErrReentrantWait rejects a synchronous wait that would depend on its caller.
	ErrReentrantWait = errors.New("coroutine: caller cannot synchronously wait for itself")
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
	onPanic func(PanicReport)
	debug   bool

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

	// schedulerMu guards script selection, pendingExecution, readyQueue, and currentJobs.
	// A single Update call owns roundJobs and deferredJobs.
	schedulerMu      sync.Mutex
	schedulerCond    *sync.Cond
	pendingExecution map[Thread]struct{} // Scripts ready to run or finish cancellation cleanup.
	readyQueue       readyQueue          // Published scripts waiting for execution.
	selectedThread   Thread              // Scheduler permit owner; may still be waiting for runMu.
	currentJobs      *queue[*waitJob]
	deferredJobs     *queue[*waitJob]
	roundJobs        *queue[*waitJob]
	redrawFrame      atomic.Int64
	scriptRound      atomic.Uint64

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

// New creates a coroutine manager. onPanic is called when a coroutine exits
// with an unhandled panic other than ErrAbortThread or ErrStopThisScript.
func New(onPanic func(PanicReport)) *Coroutines {
	// Nodes move between these queues, so they share a recycling pool.
	jobs := newQueue[*waitJob]()
	p := &Coroutines{
		onPanic:           onPanic,
		shutdownGate:      make(chan struct{}, 1),
		allThreads:        make(map[Thread]struct{}),
		pendingExecution:  make(map[Thread]struct{}),
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
