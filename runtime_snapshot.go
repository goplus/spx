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

import "github.com/goplus/spx/v4/internal/engine"

// Snapshot runs fn and requests a screenshot if it succeeds. fn is the decorated
// XGo function body and may be nil when called directly. Body errors are reported
// without requesting a screenshot.
// The input tick is read after fn returns. Dispatch occurs at engine frame end,
// or immediately without an active game. The platform decides whether to store
// or compare the screenshot.
func Snapshot(name string, fn func() error) {
	if fn != nil {
		if err := fn(); err != nil {
			engine.Panic(err)
			return
		}
	}

	if err := engine.EnqueueCapture(name, captureInputTick()); err != nil {
		engine.Panic(err)
	}
}
