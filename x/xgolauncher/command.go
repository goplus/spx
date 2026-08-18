/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package xgolauncher

import "context"

// RunCommand owns the outer command lifecycle. It records host termination,
// cancels the complete operation, waits for run and all deferred cleanup, then
// returns the original signal/status for the caller's final Exit boundary.
// Library users that provide their own signal policy should call RunContext.
func RunCommand(parent context.Context, run func(context.Context) (ProcessStatus, error)) (ProcessStatus, error) {
	if parent == nil {
		parent = context.Background()
	}
	if run == nil {
		return ProcessStatus{Code: 1}, context.Canceled
	}
	return runCommand(parent, run)
}
