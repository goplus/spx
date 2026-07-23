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

package spx

import "github.com/goplus/spx/v3/internal/engine"

// IsRunWithoutScreenRefresh reports whether the current coroutine is in
// Scratch-style "run without screen refresh" mode.
func IsRunWithoutScreenRefresh() bool {
	return engine.IsRunWithoutScreenRefresh()
}

// SetRunWithoutScreenRefresh enables or disables Scratch-style
// "run without screen refresh" mode for the current coroutine.
//
// Prefer Warp for scoped execution.
//
// It returns the previous value so callers can restore nested state with:
// `prev := spx.SetRunWithoutScreenRefresh(true); defer spx.SetRunWithoutScreenRefresh(prev)`.
func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	return engine.SetRunWithoutScreenRefresh(enabled)
}

// Warp runs call in Scratch-style "run without screen refresh" mode and
// restores the previous mode when call returns. It can be used as an XGo
// decorator:
//
//	@warp
//	func Update() { ... }
func Warp(call func()) {
	engine.RunWithoutScreenRefresh(call)
}

func waitNextFrameForControlFlow() {
	if engine.ShouldWaitNextFrame() {
		engine.WaitNextFrame()
	}
}
