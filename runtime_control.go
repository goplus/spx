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

// IsRunWithoutScreenRefresh reports whether the current coroutine is in warp mode.
func IsRunWithoutScreenRefresh() bool {
	return engine.IsRunWithoutScreenRefresh()
}

// SetRunWithoutScreenRefresh changes warp mode and returns its previous value.
func SetRunWithoutScreenRefresh(enabled bool) (previous bool) {
	return engine.SetRunWithoutScreenRefresh(enabled)
}

// Warp runs call in warp mode and restores the previous mode afterward.
func Warp(call func()) {
	engine.RunWithoutScreenRefresh(call)
}

// StopThisScript catches a stop-this-script signal at a custom procedure boundary.
func StopThisScript(call func()) {
	engine.RunStopThisScript(call)
}
