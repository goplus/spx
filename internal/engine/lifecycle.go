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
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/platform"
	itime "github.com/goplus/spx/v4/internal/time"
)

func RequestExit(exitCode int64) {
	if platform.IsWeb() {
		resetWebRuntime(exitCode)
		return
	}
	current := scheduler
	exit := func() {
		bindingMu.Lock()
		if binding := activeGame.Load(); binding != nil && binding.loadPhase() != gameClosing {
			binding.storePhase(gameStopped)
		}
		bindingMu.Unlock()
		Managers().ExtMgr.RequestExit(exitCode)
		if current != nil {
			current.StopAll()
		}
	}
	if current == nil {
		exit()
		return
	}
	// Deliver exit on the engine thread before closing the frame gate.
	current.WaitMainThread(exit)
}

func onStart() {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	itime.Start()
	binding.callbacks.OnEngineStart()
}

func onPause(paused bool) {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	binding.callbacks.OnEnginePause(paused)
}

func onReset() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil || binding.callbacks == nil {
		return
	}
	directReset := beginGameClose(binding)
	waitForGameStart(binding)
	reset := func() {
		resetFrameState()
		binding.callbacks.OnEngineReset()
	}
	switch {
	case directReset:
		// Direct editor reset.
		drainAndReleaseGame(scheduler, binding, reset)
	case binding.resetReleaseDeferred.Load():
		// Web exit handles draining and release.
		reset()
	}
}

func onDestroy() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil || binding.callbacks == nil || !beginGameClose(binding) {
		return
	}
	waitForGameStart(binding)
	drainCoroutines(scheduler, binding.callbacks.OnEngineDestroy)
}

// Called after Go cleanup and backend teardown.
func onDestroyed() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil {
		return
	}
	releaseGame(binding)
}

func resetWebRuntime(exitCode int64) {
	current := scheduler
	binding, started := beginDeferredReset()
	if started {
		// Let the caller unwind while coroutines drain.
		go finishWebReset(current, binding, exitCode)
	}
	if current.IsInCoroutine() {
		current.StopCurrent()
	}
}

func finishWebReset(current *coroutine.Scheduler, binding *gameBinding, exitCode int64) {
	defer CheckPanic()
	waitForGameStart(binding)
	if binding.startDone != nil && binding.link == nil {
		// Initialization may have started work before the backend was linked.
		drainAndReleaseGame(current, binding, nil)
		return
	}
	drainAndReleaseGame(current, binding, func() {
		current.WaitMainThread(func() { Managers().ExtMgr.RequestReset(exitCode) })
	})
}

// Keep the binding until a completed drain reopens scheduler admission, even if
// cleanup panics or exits the goroutine. An interrupted drain must not release it.
func drainAndReleaseGame(current *coroutine.Scheduler, binding *gameBinding, cleanup func()) {
	drained := false
	defer func() {
		if drained {
			releaseGame(binding)
		}
	}()
	drainCoroutines(current, func() {
		drained = true
		if cleanup != nil {
			cleanup()
		}
	})
}

// Drain and clean up synchronously on the caller thread.
func drainCoroutines(current *coroutine.Scheduler, cleanup func()) {
	if current == nil {
		if cleanup != nil {
			cleanup()
		}
		return
	}
	current.RunAfterStop(cleanup)
}
