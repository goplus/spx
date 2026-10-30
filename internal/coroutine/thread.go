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
	"sync"
	"sync/atomic"
	stime "time"

	"github.com/goplus/spx/v4/internal/debug"
	"github.com/goplus/spx/v4/internal/time"
)

// warpClockOrigin lets hot loop edges read only the monotonic clock.
var warpClockOrigin = stime.Now()

type suspendState uint8

const (
	suspendRunning suspendState = iota
	suspendWaiting
	suspendSignaled
)

type threadImpl struct {
	Owner any // Strings and Name() provide the diagnostic name.

	id    int64
	name  string
	stack string

	// Active handler restarts inherit this position without changing identity.
	resumeOrder int64
	published   bool // Under schedulerMu; registration is complete and published.
	// Ready-queue membership under schedulerMu; nil readyHeap means unqueued.
	readyHeap  *readyHeap
	readyIndex int

	stopped      atomic.Bool
	suspendMu    sync.Mutex
	suspendCond  *sync.Cond
	suspendState suspendState

	ctx        context.Context
	cancelFunc context.CancelFunc
	done       chan struct{}

	schedFrame     int64
	schedTimestamp stime.Time
	mainStartedAt  stime.Time

	warp            atomic.Bool
	warpStart       atomic.Int64
	stopAtNextYield atomic.Bool

	joinWaiters       waiterSet
	firstSliceWaiters waiterSet
	firstSliceDone    chan struct{}
}

// Thread represents a coroutine.
type Thread = *threadImpl

func (p *Coroutines) newThread(owner any) Thread {
	th := &threadImpl{
		Owner:          owner,
		id:             p.nextThreadID.Add(1),
		schedFrame:     -1,
		name:           threadName(owner),
		done:           make(chan struct{}),
		firstSliceDone: make(chan struct{}),
	}
	th.resumeOrder = th.id
	th.ctx, th.cancelFunc = context.WithCancel(context.Background())
	if p.debug {
		th.stack = debug.GetStackTrace()
	}
	th.suspendCond = sync.NewCond(&th.suspendMu)
	return th
}

// ID returns the thread's manager-local ID.
func (th Thread) ID() int64 {
	return th.id
}

// Name returns the thread's resolved name.
func (th Thread) Name() string {
	return th.name
}

// String returns the thread's ID and name.
func (th Thread) String() string {
	return fmt.Sprintf("id=%d name=%s ", th.id, th.name)
}

// Context returns the thread's cancellation context.
func (th Thread) Context() context.Context {
	return th.ctx
}

// Stopped reports whether the thread has been requested to stop.
func (th Thread) Stopped() bool {
	return th.stopped.Load()
}

// Cancel requests a stop and wakes the thread if it is suspended.
func (th Thread) Cancel() {
	th.suspendMu.Lock()
	defer th.suspendMu.Unlock()
	if th.stopped.Load() {
		return
	}
	th.stopped.Store(true)
	th.cancelFunc()
	th.suspendCond.Signal()
}

// Warp reports whether screen refresh is suppressed.
func (th Thread) Warp() bool {
	return th.warp.Load()
}

// SetWarp changes warp mode and returns its previous value.
func (th Thread) SetWarp(enabled bool) bool {
	previous := th.warp.Swap(enabled)
	if enabled != previous {
		th.warpStart.Store(0)
	}
	return previous
}

// ShouldYield checks a loop edge, resetting the warp budget when exhausted.
func (th Thread) ShouldYield(budget stime.Duration) bool {
	if !th.Warp() {
		return true
	}

	// Keep zero reserved for an execution window that has not started.
	now := int64(stime.Since(warpClockOrigin)) + 1
	startedAt := th.warpStart.Load()
	if startedAt == 0 {
		th.warpStart.Store(now)
		return false
	}
	if stime.Duration(now-startedAt) <= budget {
		return false
	}

	th.warpStart.Store(0)
	return true
}

// IsSchedTimeout reports whether ms has elapsed in the current scheduler frame.
func (th Thread) IsSchedTimeout(ms float64) bool {
	frame := time.Frame()
	if th.schedFrame < frame {
		th.schedFrame = frame
		th.schedTimestamp = stime.Now()
	}
	return stime.Since(th.schedTimestamp) > stime.Duration(ms)*stime.Millisecond
}

// BeginMain tracks this Main's start time and returns a function to restore the previous one.
func (th Thread) BeginMain(startedAt stime.Time) func() {
	previous := th.mainStartedAt
	th.mainStartedAt = startedAt
	return func() {
		th.mainStartedAt = previous
	}
}

// MainStartedAt returns the current Main's start time, or zero if its timeout is disabled.
func (th Thread) MainStartedAt() stime.Time {
	return th.mainStartedAt
}

// DisableMainTimeout stops timeout checks for the current Main.
func (th Thread) DisableMainTimeout() {
	th.mainStartedAt = stime.Time{}
}

// resume wakes th or records a signal for its next yield.
func (th Thread) resume() {
	th.suspendMu.Lock()
	defer th.suspendMu.Unlock()
	if th.canceled() {
		return
	}

	switch th.suspendState {
	case suspendWaiting:
		th.suspendState = suspendRunning
		th.suspendCond.Signal()
	case suspendRunning:
		th.suspendState = suspendSignaled
	}
}

// canceled also recognizes normal completion, which cancels the thread context.
func (th Thread) canceled() bool {
	if th == nil {
		return false
	}
	if th.stopped.Load() {
		return true
	}
	select {
	case <-th.Context().Done():
		return true
	default:
		return false
	}
}

func threadName(owner any) string {
	switch owner := owner.(type) {
	case nil:
		return ""
	case string:
		return owner
	case interface{ Name() string }:
		return owner.Name()
	}

	typ := reflect.TypeOf(owner)
	if typ.Kind() != reflect.Pointer || typ.Elem().Name() == "" {
		return ""
	}
	return "*" + typ.Elem().Name()
}
