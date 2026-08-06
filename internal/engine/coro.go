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

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine/profiler"
	itime "github.com/goplus/spx/v3/internal/time"
)

var (
	gco   *coroutine.Coroutines
	pgame any // pointer to the current game object
)

const runWithoutScreenRefreshBudget = 500 * stdtime.Millisecond

func SetGame(game any) {
	pgame = game
}

func GetGame() any {
	return pgame
}

func IsInCoroutine() bool {
	if gco == nil {
		return false
	}
	return gco.IsInCoroutine()
}

func IsAbortThreadError(err any) bool {
	return coroutine.IsAbortThreadError(err)
}

func GetCoroutineOwner() any {
	if IsInCoroutine() {
		return gco.Current().Obj
	}
	return nil
}

func GetCurrentThreadContext() context.Context {
	if IsInCoroutine() {
		return gco.Current().Context()
	}
	return context.Background()
}

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
	profiler.SetGco(co)
}

func Go(tobj coroutine.ThreadObj, fn func(ctx context.Context)) {
	gco.CreateAndStart(false, tobj, func(me coroutine.Thread) int {
		fn(me.Context())
		return 0
	})
}

func ResolveCoroutineOwner(owner any) any {
	if owner != nil {
		return owner
	}
	if IsInCoroutine() {
		return GetCoroutineOwner()
	}
	return GetGame()
}

func GoWithOwner(owner any, fn func(ctx context.Context, owner any)) {
	owner = ResolveCoroutineOwner(owner)
	Go(owner, func(ctx context.Context) {
		fn(ctx, owner)
	})
}

func Execute(owner any, fn func(ctx context.Context, owner any)) {
	if IsInCoroutine() {
		fn(GetCurrentThreadContext(), owner)
		return
	}

	done := make(chan struct{}, 1)
	GoWithOwner(owner, func(ctx context.Context, owner any) {
		defer close(done)
		fn(ctx, owner)
	})
	<-done
}

func Wait(secs float64) float64 {
	startTime := itime.TimeSinceLevelLoad()
	gco.Wait(secs)
	return itime.TimeSinceLevelLoad() - startTime
}

func WaitYield() {
	gco.WaitYield(gco.Current())
}

func WaitNextFrame() float64 {
	gco.WaitNextFrame()
	return itime.DeltaTime()
}

func WaitNextFrameIfNeeded() float64 {
	if ShouldWaitNextFrame() {
		return WaitNextFrame()
	}
	return itime.DeltaTime()
}

func IsRunWithoutScreenRefresh() bool {
	if !IsInCoroutine() {
		return false
	}
	return gco.Current().RunWithoutScreenRefresh()
}

func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	if !IsInCoroutine() {
		return false
	}
	return gco.Current().SetRunWithoutScreenRefresh(enabled)
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
	if !IsInCoroutine() {
		return true
	}
	return gco.Current().ShouldWaitNextFrame(runWithoutScreenRefreshBudget)
}

// NewControlFlowWaiter captures the exact managed thread once for a generated
// Forever/Repeat/RepeatUntil/WaitUntil invocation. Reusing that thread avoids
// resolving goroutine identity at every loop edge. This is especially
// important on js/wasm, where goid.Get falls back to runtime.Stack.
func NewControlFlowWaiter() func() {
	co := gco
	if co == nil || !co.IsInCoroutine() {
		// Preserve the existing behavior for unsupported calls outside a managed
		// coroutine, including its validation and error path.
		return func() {
			if ShouldWaitNextFrame() {
				WaitNextFrame()
			}
		}
	}

	thread := co.Current()
	return func() {
		if thread.ShouldWaitNextFrame(runWithoutScreenRefreshBudget) {
			co.WaitNextFrameFor(thread)
		}
	}
}

func WaitMainThread(call func()) {
	gco.WaitMainThread(call)
}

func WaitToDo(fn func()) {
	gco.WaitToDo(fn)
}

func ExecuteNative(fn func(ctx context.Context, owner any)) {
	ctx := GetCurrentThreadContext()
	if !IsInCoroutine() {
		fn(ctx, nil)
		return
	}
	owner := GetCoroutineOwner()
	WaitToDo(func() {
		fn(ctx, owner)
	})
}

func WaitForChan[T any](done <-chan T, data *T) {
	coroutine.WaitForChan(gco, done, data)
}
