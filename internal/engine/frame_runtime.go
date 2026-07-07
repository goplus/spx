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
	"fmt"
	"sort"
	"sync"

	itime "github.com/goplus/spx/v2/internal/time"
)

type frameCallback struct {
	frame int64
	seq   uint64
	fn    func()
}

type captureRequest struct {
	name  string
	check bool
}

type captureHandler func(string, bool) error

var frameRuntime struct {
	mu              sync.Mutex
	seq             uint64
	callbacks       []frameCallback
	captureRequests []captureRequest
	captureHandler  captureHandler
}

// CurrentFrame returns the current engine frame number.
func CurrentFrame() int64 {
	return itime.Frame()
}

// ScheduleFrame schedules fn to run once when the engine reaches frame.
func ScheduleFrame(frame int64, fn func()) {
	if fn == nil {
		return
	}
	if GetGame() == nil || frame <= CurrentFrame() {
		fn()
		return
	}

	frameRuntime.mu.Lock()
	frameRuntime.seq++
	frameRuntime.callbacks = append(frameRuntime.callbacks, frameCallback{
		frame: frame,
		seq:   frameRuntime.seq,
		fn:    fn,
	})
	frameRuntime.mu.Unlock()
}

// RunFrameCallbacks runs all callbacks due at or before frame.
func RunFrameCallbacks(frame int64) {
	callbacks := takeFrameCallbacks(frame)
	for _, cb := range callbacks {
		cb.fn()
	}
}

func takeFrameCallbacks(frame int64) []frameCallback {
	frameRuntime.mu.Lock()
	defer frameRuntime.mu.Unlock()

	if len(frameRuntime.callbacks) == 0 {
		return nil
	}

	due := make([]frameCallback, 0, len(frameRuntime.callbacks))
	future := frameRuntime.callbacks[:0]
	for _, cb := range frameRuntime.callbacks {
		if cb.frame <= frame {
			due = append(due, cb)
			continue
		}
		future = append(future, cb)
	}

	clear(frameRuntime.callbacks[len(future):])
	frameRuntime.callbacks = future

	sort.SliceStable(due, func(i, j int) bool {
		if due[i].frame == due[j].frame {
			return due[i].seq < due[j].seq
		}
		return due[i].frame < due[j].frame
	})
	return due
}

// SetCaptureHandler installs the screenshot backend used by RequestCapture.
func SetCaptureHandler(handler func(string, bool) error) {
	frameRuntime.mu.Lock()
	frameRuntime.captureHandler = handler
	frameRuntime.mu.Unlock()
}

// RequestCapture queues a screenshot request for the end of the current frame.
func RequestCapture(name string, check bool) error {
	if GetGame() == nil {
		return runCaptureHandler(name, check)
	}

	frameRuntime.mu.Lock()
	frameRuntime.captureRequests = append(frameRuntime.captureRequests, captureRequest{name: name, check: check})
	frameRuntime.mu.Unlock()
	return nil
}

// RunCaptureRequests flushes screenshot requests queued during the frame.
func RunCaptureRequests() error {
	requests := takeCaptureRequests()
	for _, req := range requests {
		if err := runCaptureHandler(req.name, req.check); err != nil {
			return err
		}
	}
	return nil
}

func takeCaptureRequests() []captureRequest {
	frameRuntime.mu.Lock()
	defer frameRuntime.mu.Unlock()
	if len(frameRuntime.captureRequests) == 0 {
		return nil
	}
	requests := append([]captureRequest(nil), frameRuntime.captureRequests...)
	clear(frameRuntime.captureRequests)
	frameRuntime.captureRequests = frameRuntime.captureRequests[:0]
	return requests
}

func runCaptureHandler(name string, check bool) error {
	frameRuntime.mu.Lock()
	handler := frameRuntime.captureHandler
	frameRuntime.mu.Unlock()
	if handler == nil {
		return fmt.Errorf("spx: capture backend is not configured")
	}
	return handler(name, check)
}

// ResetFrameRuntime clears frame-scoped callbacks and capture requests.
func ResetFrameRuntime() {
	frameRuntime.mu.Lock()
	clear(frameRuntime.callbacks)
	frameRuntime.callbacks = nil
	frameRuntime.seq = 0
	clear(frameRuntime.captureRequests)
	frameRuntime.captureRequests = nil
	frameRuntime.mu.Unlock()
}
