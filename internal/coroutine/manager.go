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
	// ErrCannotYieldANonrunningThread is the panic value used when a coroutine
	// tries to yield without owning the scheduler.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")

	// ErrAbortThread is the panic value used to stop the current coroutine.
	ErrAbortThread = errors.New("abort thread")

	// ErrStopThisScript signals Scratch-style termination to a procedure boundary.
	ErrStopThisScript = errors.New("stop this script")
)

// Coroutines coordinates thread lifecycle and cooperative scheduling.
type Coroutines struct {
	onPanic   func(name, stack string)
	hasInited atomic.Bool
	debug     bool

	// runMu ensures that only one coroutine executes at a time. current is valid
	// only while a coroutine owns runMu.
	runMu   sync.Mutex
	current atomic.Pointer[threadImpl]

	// shutdownMu serializes fatal shutdowns. creationMu protects stopping and
	// makes thread/native-task registration atomic with respect to shutdown
	// transitions.
	// Lock order is shutdownMu, runMu, then creationMu.
	shutdownMu sync.Mutex
	creationMu sync.RWMutex
	stopping   bool
	// reopenWhenDrained is set after a timed-out barrier. Admission remains
	// closed until the final managed thread/native task unregisters.
	reopenWhenDrained bool

	// threadsMu protects the thread and native-task registries.
	threadsMu   sync.Mutex
	allThreads  map[Thread]struct{}
	nativeTasks map[*nativeTask]struct{}

	// schedulerMu protects threadStates and the condition-variable predicate.
	// It also makes state changes and their corresponding enqueue atomic.
	schedulerMu   sync.Mutex
	schedulerCond *sync.Cond
	threadStates  map[Thread]threadState
	currentJobs   *Queue[*WaitJob]
	deferredJobs  *Queue[*WaitJob]

	nextJobID    atomic.Int64
	nextThreadID atomic.Int64
	nextNativeID atomic.Uint64
	// abortEpoch is even outside an abort registration barrier and odd while
	// one is active. Create captures it before admission so a registration that
	// overlaps AbortAll cannot escape the abort snapshot.
	abortEpoch atomic.Uint64

	perfDebug         atomic.Bool
	readGCStats       func(*sdebug.GCStats)
	updateWatchdogNow func() stime.Time
	statsMu           sync.RWMutex
	lastUpdateStats   UpdateJobsStats

	// goroutineThreads maps each managed goroutine to the exact coroutine it
	// executes. A scheduler-wide Current value is not sufficient to identify
	// the caller because external goroutines can observe it concurrently.
	goroutineThreads sync.Map // map[uint64]Thread
}

// New creates a coroutine manager. onPanic is called when a coroutine exits
// with an unhandled panic other than ErrAbortThread.
func New(onPanic func(name, stack string)) *Coroutines {
	p := &Coroutines{
		onPanic:           onPanic,
		allThreads:        make(map[Thread]struct{}),
		nativeTasks:       make(map[*nativeTask]struct{}),
		threadStates:      make(map[Thread]threadState),
		currentJobs:       NewQueue[*WaitJob](),
		deferredJobs:      NewQueue[*WaitJob](),
		readGCStats:       sdebug.ReadGCStats,
		updateWatchdogNow: stime.Now,
	}
	p.schedulerCond = sync.NewCond(&p.schedulerMu)
	return p
}

// OnRestart marks the scheduler as not yet initialized.
func (p *Coroutines) OnRestart() {
	p.hasInited.Store(false)
}

// OnInited marks the scheduler as initialized.
func (p *Coroutines) OnInited() {
	p.hasInited.Store(true)
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
