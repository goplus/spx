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

import "github.com/goplus/spx/v2/internal/engine"

// CurrentFrame returns the absolute frame number of the current engine
// session. The counter starts at zero before project loading, advances once per
// engine update, and is not reset when the game reloads.
func CurrentFrame() int64 {
	return engine.CurrentFrame()
}

// AtFrame schedules fn to run once when CurrentFrame reaches frame while a game
// is active. The target is an absolute frame number, not a delay; if it has
// already been reached, fn starts running immediately. Without an active game,
// fn runs synchronously regardless of frame.
//
// With an active game, the callback runs in a coroutine owned by the object
// that registered it, so it may use wait/yield APIs safely.
//
// It can be used as an XGo decorator. Decorator registration still uses the
// absolute engine-session frame, so loading may have already consumed a small
// target such as 10:
//
//	@atFrame(10)
//	func Step() { ... }
func AtFrame(frame int64, fn func()) {
	engine.ScheduleFrame(frame, fn)
}
