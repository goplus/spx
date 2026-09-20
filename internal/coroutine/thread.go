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

	"github.com/goplus/spx/v3/internal/debug"
	"github.com/goplus/spx/v3/internal/time"
)

// ThreadObj is the owner associated with a Thread. String values and owners
// implementing Name() also provide the thread's diagnostic name.
type ThreadObj any

type suspendState uint8

const (
	suspendStateRunning suspendState = iota
	suspendStateSuspended
	suspendStateSignaled
)

type threadImpl struct {
	Obj ThreadObj

	id    int64
	name  string
	stack string

	// Active handler restarts inherit this position without changing identity.
	resumeOrder int64

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

	joinWaiters   waiterSet
	yieldWaiters  waiterSet
	yieldedOrDone chan struct{}
}

// Thread represents a coroutine.
type Thread = *threadImpl

func (p *Coroutines) newThread(obj ThreadObj) Thread {
	th := &threadImpl{
		Obj:           obj,
		id:            p.nextThreadID.Add(1),
		schedFrame:    -1,
		name:          resolveThreadName(obj),
		done:          make(chan struct{}),
		yieldedOrDone: make(chan struct{}),
	}
	th.resumeOrder = th.id
	th.ctx, th.cancelFunc = context.WithCancel(context.Background())
	if p.debug {
		th.stack = debug.GetStackTrace()
	}
	th.suspendCond = sync.NewCond(&th.suspendMu)
	return th
}

// Context returns the thread's cancellation context.
func (th *threadImpl) Context() context.Context {
	if th.ctx == nil {
		return context.Background()
	}
	return th.ctx
}

// Cancel requests a stop and wakes the thread if it is suspended.
func (th *threadImpl) Cancel() {
	th.suspendMu.Lock()
	defer th.suspendMu.Unlock()
	if th.stopped.Load() {
		return
	}
	th.stopped.Store(true)
	th.cancelContext()
	th.suspendCond.Signal()
}

func (th *threadImpl) cancelContext() {
	if th.cancelFunc != nil {
		th.cancelFunc()
	}
}

// String returns the thread's ID and name.
func (th *threadImpl) String() string {
	return fmt.Sprintf("id=%d name=%s ", th.id, th.name)
}

// Name returns the thread's resolved name.
func (th *threadImpl) Name() string {
	return th.name
}

// ID returns the thread's manager-local ID.
func (th *threadImpl) ID() int64 {
	return th.id
}

// Stack returns the creation stack, if one was captured.
func (th *threadImpl) Stack() string {
	return th.stack
}

// Stopped reports whether the thread has been requested to stop.
func (th *threadImpl) Stopped() bool {
	return th.stopped.Load()
}

// RunWithoutScreenRefresh reports whether the thread is in warp mode.
func (th *threadImpl) RunWithoutScreenRefresh() bool {
	return th.warp.Load()
}

// SetRunWithoutScreenRefresh changes warp mode and returns its previous value.
func (th *threadImpl) SetRunWithoutScreenRefresh(enabled bool) bool {
	previous := th.warp.Swap(enabled)
	if enabled != previous {
		th.warpStart.Store(0)
	}
	return previous
}

// ShouldWaitNextFrame reports whether the thread should yield at a loop edge.
// Warp mode suppresses the yield until budget is exhausted.
func (th *threadImpl) ShouldWaitNextFrame(budget stime.Duration) bool {
	if !th.RunWithoutScreenRefresh() {
		return true
	}

	now := stime.Now().UnixNano()
	startedAt := th.warpStart.Load()
	if startedAt == 0 {
		th.warpStart.Store(now)
		return false
	}
	if stime.Duration(now-startedAt) <= budget {
		return false
	}

	// Exhausting a warp budget forces one yield, then starts a new window.
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

type threadNamer interface {
	Name() string
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
