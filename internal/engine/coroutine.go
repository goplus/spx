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

package engine

import (
	"context"
	stdtime "time"

	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine/profiler"
	itime "github.com/goplus/spx/v4/internal/time"
)

const warpBudget = 500 * stdtime.Millisecond

var scheduler *coroutine.Scheduler

func SetScheduler(next *coroutine.Scheduler) {
	scheduler = next
	profiler.SetScheduler(next)
}

func IsInCoroutine() bool {
	return coroutine.Caller(scheduler) != nil
}

func Go(owner any, fn func(ctx context.Context, owner any)) {
	binding, ok := captureRuntimeWork()
	if !ok {
		return
	}
	owner = resolveOwner(owner)
	scheduler.Create(owner, func(me coroutine.Thread) {
		if isRuntimeWorkCurrent(binding) {
			fn(me.Context(), owner)
		}
	})
}

func Execute(owner any, fn func(ctx context.Context, owner any)) {
	binding, ok := captureRuntimeWork()
	if !ok {
		return
	}
	current := scheduler
	if current.Caller() == nil {
		owner = resolveOwner(owner)
	}
	current.RunSync(owner, func(thread coroutine.Thread) {
		if isRuntimeWorkCurrent(binding) {
			fn(thread.Context(), owner)
		}
	})
}

func ExecuteNative(fn func(ctx context.Context, owner any)) {
	thread := coroutine.Caller(scheduler)
	if thread == nil {
		fn(context.Background(), nil)
		return
	}
	ctx, owner := thread.Context(), thread.Owner
	scheduler.WaitToDo(func() {
		fn(ctx, owner)
	})
}

func WaitMainThread(call func()) {
	scheduler.WaitMainThread(call)
}

func Wait(secs float64) float64 {
	startTime := itime.TimeSinceLevelLoad()
	scheduler.Wait(secs)
	return itime.TimeSinceLevelLoad() - startTime
}

func WaitYield() {
	scheduler.WaitYield(scheduler.Caller())
}

func WaitNextFrame() float64 {
	scheduler.WaitNextFrame()
	return itime.DeltaTime()
}

func WaitNextFrameIfNeeded() float64 {
	if ShouldWaitNextFrame() {
		return WaitNextFrame()
	}
	return itime.DeltaTime()
}

func WaitForChan[T any](ch <-chan T) T {
	return coroutine.WaitForChan(scheduler, ch)
}

func IsRunWithoutScreenRefresh() bool {
	thread := coroutine.Caller(scheduler)
	return thread != nil && thread.Warp()
}

func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	if thread := coroutine.Caller(scheduler); thread != nil {
		return thread.SetWarp(enabled)
	}
	return false
}

func RunWithoutScreenRefresh(call func()) {
	if call == nil {
		return
	}
	previous := SetRunWithoutScreenRefresh(true)
	defer SetRunWithoutScreenRefresh(previous)
	call()
}

func ShouldWaitNextFrame() bool {
	if thread := coroutine.Caller(scheduler); thread != nil {
		return thread.ShouldYield(warpBudget)
	}
	return true
}

// NewControlFlowWaiter caches the calling thread for generated loop yields.
func NewControlFlowWaiter() func() {
	current := scheduler
	thread := coroutine.Caller(current)
	if thread == nil {
		return func() {
			if ShouldWaitNextFrame() {
				WaitNextFrame()
			}
		}
	}

	return func() {
		if !thread.Warp() {
			current.YieldLoopFor(thread)
		} else if thread.ShouldYield(warpBudget) {
			current.WaitNextFrameFor(thread)
		}
	}
}

// RequestRedraw marks a visual change for cooperative script scheduling.
func RequestRedraw() {
	if scheduler != nil {
		scheduler.RequestRedraw()
	}
}

func IsAbortThreadError(err any) bool {
	return err == coroutine.ErrAbortThread
}

// RunStopThisScript consumes stop-this-script signals at a procedure boundary.
func RunStopThisScript(call func()) {
	if call == nil {
		return
	}

	panicked := true
	defer func() {
		recovered := recover()
		if !panicked {
			return
		}
		if recovered != coroutine.ErrStopThisScript {
			panic(recovered)
		}
	}()
	call()
	panicked = false
}

func resolveOwner(owner any) any {
	if owner != nil {
		return owner
	}
	if thread := coroutine.Caller(scheduler); thread != nil {
		return thread.Owner
	}
	return GetGame()
}
