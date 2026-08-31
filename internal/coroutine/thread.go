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
	"sync"
	"sync/atomic"
	stime "time"

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

	stopped      atomic.Bool
	suspended    atomic.Bool
	suspendMu    sync.Mutex
	suspendCond  *sync.Cond
	suspendState suspendState

	ctx        context.Context
	cancelFunc context.CancelFunc
	done       chan struct{}

	schedFrame     int64
	schedTimestamp stime.Time

	runWithoutScreenRefresh      atomic.Bool
	runWithoutScreenRefreshStart atomic.Int64
	stopAtNextYield              atomic.Bool

	waitersMu   sync.Mutex
	joinDone    bool
	joinWaiters map[Thread]struct{}

	yieldedOrDoneOnce sync.Once
	yieldedOrDone     chan struct{}
	yieldWaiters      map[Thread]struct{}
}

// Thread represents a coroutine.
type Thread = *threadImpl

// Context returns the thread's cancellation context.
func (th *threadImpl) Context() context.Context {
	if th.ctx == nil {
		return context.Background()
	}
	return th.ctx
}

// Cancel cancels the thread's context.
func (th *threadImpl) Cancel() {
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
	return th.runWithoutScreenRefresh.Load()
}

// SetRunWithoutScreenRefresh changes warp mode and returns its previous value.
func (th *threadImpl) SetRunWithoutScreenRefresh(enabled bool) bool {
	previous := th.runWithoutScreenRefresh.Swap(enabled)
	if enabled != previous {
		th.runWithoutScreenRefreshStart.Store(0)
	}
	return previous
}

// ShouldWaitNextFrame reports whether the thread should yield at a loop edge.
// Warp mode suppresses the yield until budget is exhausted.
func (th *threadImpl) ShouldWaitNextFrame(runWithoutScreenRefreshBudget stime.Duration) bool {
	if !th.RunWithoutScreenRefresh() {
		return true
	}

	now := stime.Now().UnixNano()
	startedAt := th.runWithoutScreenRefreshStart.Load()
	if startedAt == 0 {
		th.runWithoutScreenRefreshStart.Store(now)
		return false
	}
	if stime.Duration(now-startedAt) <= runWithoutScreenRefreshBudget {
		return false
	}

	// Exhausting a warp budget forces one yield, then starts a new window.
	th.runWithoutScreenRefreshStart.Store(0)
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
