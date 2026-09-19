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
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine/platform"
)

func RequestExit(exitCode int64) {
	if platform.IsWeb() {
		resetWebRuntime(exitCode)
		return
	}
	co := gco
	exit := func() {
		bindingMu.Lock()
		if binding := activeGame.Load(); binding != nil && binding.loadPhase() != gameClosing {
			binding.storePhase(gameStopped)
		}
		bindingMu.Unlock()
		Managers().ExtMgr.RequestExit(exitCode)
		if co != nil {
			co.StopAll()
		}
	}
	if co == nil {
		exit()
		return
	}
	// Deliver exit on the engine thread before closing the frame gate.
	co.WaitMainThread(exit)
}

func resetWebRuntime(exitCode int64) {
	co := gco
	binding, started := beginDeferredReset()
	if started {
		// Let the caller unwind while coroutines drain.
		go finishWebReset(co, binding, exitCode)
	}
	if co.IsInCoroutine() {
		co.StopCurrent()
	}
}

func finishWebReset(co *coroutine.Coroutines, binding *gameBinding, exitCode int64) {
	defer CheckPanic()
	waitForGameStart(binding)
	defer finishDeferredReset(binding)
	if binding.startDone != nil && binding.link == nil {
		return
	}
	co.RunAfterStopAll(0, func() {
		co.WaitMainThread(func() { Managers().ExtMgr.RequestReset(exitCode) })
	})
}
