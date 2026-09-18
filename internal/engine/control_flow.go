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
	stdtime "time"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
)

const runWithoutScreenRefreshBudget = 500 * stdtime.Millisecond

func IsAbortThreadError(err any) bool {
	return coroutine.IsAbortThreadError(err)
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
	thread := currentThread()
	return thread != nil && thread.RunWithoutScreenRefresh()
}

func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	if thread := currentThread(); thread != nil {
		return thread.SetRunWithoutScreenRefresh(enabled)
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
		if !coroutine.IsStopThisScriptError(recovered) {
			panic(recovered)
		}
	}()
	call()
	panicked = false
}

func ShouldWaitNextFrame() bool {
	if thread := currentThread(); thread != nil {
		return thread.ShouldWaitNextFrame(runWithoutScreenRefreshBudget)
	}
	return true
}

// NewControlFlowWaiter caches the calling thread for generated loop yields.
func NewControlFlowWaiter() func() {
	co := gco
	if co == nil || !co.IsInCoroutine() {
		return func() {
			if ShouldWaitNextFrame() {
				WaitNextFrame()
			}
		}
	}

	thread := co.Current()
	return func() {
		if !thread.RunWithoutScreenRefresh() {
			co.YieldLoopFor(thread)
		} else if thread.ShouldWaitNextFrame(runWithoutScreenRefreshBudget) {
			co.WaitNextFrameFor(thread)
		}
	}
}

// RequestRedraw marks a visual change for cooperative script scheduling.
func RequestRedraw() {
	if gco != nil {
		gco.RequestRedraw()
	}
}

func WaitForChan[T any](ch <-chan T) T {
	return coroutine.WaitForChan(gco, ch)
}
