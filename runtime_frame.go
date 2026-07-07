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

// CurrentFrame returns the current engine frame number.
func CurrentFrame() int64 {
	return engine.CurrentFrame()
}

// Frame schedules fn to run once when the engine reaches frame i.
//
// It is primarily intended as an XGo decorator for deterministic E2E scripts:
//
//	@frame(10)
//	func Step() { ... }
func Frame(i int, fn func()) {
	engine.ScheduleFrame(int64(i), fn)
}

// Capture runs fn and then invokes the configured screenshot capture backend.
//
// The xgo decorator expansion passes fn as a func() error closure, which allows
// Capture to surface errors from the decorated function before requesting the
// screenshot.
func Capture(name string, fn func() error) {
	if err := runCaptureBody(fn); err != nil {
		engine.Panic(err)
		return
	}
	if err := queueCapture(name, false); err != nil {
		engine.Panic(err)
	}
}

// CaptureAndCheck runs fn and then invokes the configured screenshot comparison
// backend. The comparison implementation is supplied by the test harness or
// platform bridge.
func CaptureAndCheck(name string, fn func() error) {
	if err := runCaptureBody(fn); err != nil {
		engine.Panic(err)
		return
	}
	if err := queueCapture(name, true); err != nil {
		engine.Panic(err)
	}
}

// SetCaptureHandler installs the screenshot backend used by Capture and
// CaptureAndCheck. Passing nil disables capture.
func SetCaptureHandler(handler func(string, bool) error) {
	engine.SetCaptureHandler(handler)
}

func runCaptureBody(fn func() error) error {
	if fn == nil {
		return nil
	}
	return fn()
}

func queueCapture(name string, check bool) error {
	return engine.RequestCapture(name, check)
}
