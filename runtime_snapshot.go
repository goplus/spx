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

// Snapshot runs fn and, if it succeeds, queues a screenshot for the end of the
// current engine frame. Without an active game, the request is dispatched
// immediately. After an input recording or replay session has consumed its
// first effective tick, the request is also associated with the most recently
// consumed input tick.
//
// The fn parameter is the decorated function body supplied by XGo. It may be
// nil when Snapshot is called directly. If fn returns an error, the runtime
// reports that error and does not request a screenshot. Snapshot only declares
// the screenshot point; the platform decides whether to store or compare it.
func Snapshot(name string, fn func() error) {
	if fn != nil {
		if err := fn(); err != nil {
			engine.Panic(err)
			return
		}
	}

	inputTick, hasInputTick := currentInputSessionTickForCapture()
	var err error
	if hasInputTick {
		err = engine.EnqueueCaptureAtInputTick(name, inputTick)
	} else {
		err = engine.EnqueueCapture(name)
	}
	if err != nil {
		engine.Panic(err)
	}
}
