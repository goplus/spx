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

package runtime

import "github.com/goplus/spx/v2/internal/engine/platform"

// These wrappers keep the main-thread marker API available in the runtime
// package while delegating to the shared platform-level implementation used by
// coroutine/enginewrap/gdengine.
// Prefer RunOnMainThread unless you need to span the marker across multiple
// calls, and always pair EnterMainThread with defer ExitMainThread in the same
// stack frame.
func EnterMainThread() {
	platform.EnterMainThread()
}

// ExitMainThread clears the manual main-thread marker set by EnterMainThread.
func ExitMainThread() {
	platform.ExitMainThread()
}

// IsMainThread reports whether the current goroutine is marked as executing
// engine main-thread work.
func IsMainThread() bool {
	return platform.IsMainThread()
}

// RunOnMainThread marks the current goroutine as main-thread work for the
// duration of call.
func RunOnMainThread(call func()) {
	platform.RunOnMainThread(call)
}
