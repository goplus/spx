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
	"github.com/goplus/spx/v3/internal/engine/platform"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

const coroutineShutdownTimeout = 2 * stdtime.Second

func RequestExit(exitCode int64) {
	if platform.IsWeb() {
		resetWebRuntime(exitCode)
		return
	}
	Managers().ExtMgr.RequestExit(exitCode)
}

func resetWebRuntime(exitCode int64) {
	co := gco
	// Let the caller unwind while coroutines drain.
	go resetAfterCoroutinesStop(co, coroutineShutdownTimeout, func() {
		Managers().ExtMgr.RequestReset(exitCode)
	})
	if co.IsInCoroutine() {
		co.StopCurrent()
	}
}

func resetAfterCoroutinesStop(co *coroutine.Coroutines, timeout stdtime.Duration, reset func()) bool {
	if !co.RunAfterStopAll(timeout, func() {
		co.WaitMainThread(reset)
	}) {
		spxlog.Error("Coroutine shutdown timed out; engine reset was not requested.")
		return false
	}
	spxlog.Debug("Coroutine shutdown completed. Engine reset requested.")
	return true
}
