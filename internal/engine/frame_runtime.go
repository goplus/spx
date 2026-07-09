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

type CaptureIntent string

const (
	CaptureIntentSnapshot CaptureIntent = "snapshot"
	CaptureIntentCheck    CaptureIntent = "check"
)

type CaptureRequest struct {
	Name     string
	Intent   CaptureIntent
	Frame    int64
	Sequence uint64
}

func (req CaptureRequest) IsCheck() bool {
	return req.Intent == CaptureIntentCheck
}

type CaptureHandler func(CaptureRequest) error

var frameRuntime struct {
	mu                    sync.Mutex
	callbackSeq           uint64
	captureSeq            uint64
	pendingFrameCallbacks []frameCallback
	pendingCaptures       []CaptureRequest
	captureHandler        CaptureHandler
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
	frameRuntime.callbackSeq++
	frameRuntime.pendingFrameCallbacks = append(frameRuntime.pendingFrameCallbacks, frameCallback{
		frame: frame,
		seq:   frameRuntime.callbackSeq,
		fn:    fn,
	})
	frameRuntime.mu.Unlock()
}

// RunFrameCallbacks runs all callbacks due at or before frame.
func RunFrameCallbacks(frame int64) {
	callbacks := drainDueFrameCallbacks(frame)
	for _, cb := range callbacks {
		cb.fn()
	}
}

func drainDueFrameCallbacks(frame int64) []frameCallback {
	frameRuntime.mu.Lock()
	defer frameRuntime.mu.Unlock()

	if len(frameRuntime.pendingFrameCallbacks) == 0 {
		return nil
	}

	due := make([]frameCallback, 0, len(frameRuntime.pendingFrameCallbacks))
	future := frameRuntime.pendingFrameCallbacks[:0]
	for _, cb := range frameRuntime.pendingFrameCallbacks {
		if cb.frame <= frame {
			due = append(due, cb)
			continue
		}
		future = append(future, cb)
	}

	clear(frameRuntime.pendingFrameCallbacks[len(future):])
	frameRuntime.pendingFrameCallbacks = future

	sort.SliceStable(due, func(i, j int) bool {
		if due[i].frame == due[j].frame {
			return due[i].seq < due[j].seq
		}
		return due[i].frame < due[j].frame
	})
	return due
}

// SetCaptureHandler installs the screenshot backend used by EnqueueCapture.
func SetCaptureHandler(handler CaptureHandler) {
	frameRuntime.mu.Lock()
	frameRuntime.captureHandler = handler
	frameRuntime.mu.Unlock()
}

// EnqueueCapture queues a screenshot request for the end of the current frame.
func EnqueueCapture(name string, intent CaptureIntent) error {
	frameRuntime.mu.Lock()
	req := CaptureRequest{
		Name:     name,
		Intent:   intent,
		Frame:    CurrentFrame(),
		Sequence: frameRuntime.captureSeq + 1,
	}
	frameRuntime.captureSeq++
	if GetGame() == nil {
		handler := frameRuntime.captureHandler
		frameRuntime.mu.Unlock()
		if handler == nil {
			return fmt.Errorf("spx: capture backend is not configured")
		}
		return handler(req)
	}
	frameRuntime.pendingCaptures = append(frameRuntime.pendingCaptures, req)
	frameRuntime.mu.Unlock()
	return nil
}

// FlushCaptures flushes screenshot requests queued during the frame.
func FlushCaptures() error {
	captures := drainPendingCaptures()
	for _, capture := range captures {
		if err := dispatchCapture(capture); err != nil {
			return err
		}
	}
	return nil
}

func drainPendingCaptures() []CaptureRequest {
	frameRuntime.mu.Lock()
	defer frameRuntime.mu.Unlock()
	if len(frameRuntime.pendingCaptures) == 0 {
		return nil
	}
	captures := append([]CaptureRequest(nil), frameRuntime.pendingCaptures...)
	clear(frameRuntime.pendingCaptures)
	frameRuntime.pendingCaptures = frameRuntime.pendingCaptures[:0]
	return captures
}

func dispatchCapture(req CaptureRequest) error {
	frameRuntime.mu.Lock()
	handler := frameRuntime.captureHandler
	frameRuntime.mu.Unlock()
	if handler == nil {
		return fmt.Errorf("spx: capture backend is not configured")
	}
	return handler(req)
}

// ResetFrameRuntime clears frame-scoped callbacks and capture requests.
func ResetFrameRuntime() {
	frameRuntime.mu.Lock()
	clear(frameRuntime.pendingFrameCallbacks)
	frameRuntime.pendingFrameCallbacks = nil
	frameRuntime.callbackSeq = 0
	clear(frameRuntime.pendingCaptures)
	frameRuntime.pendingCaptures = nil
	frameRuntime.captureSeq = 0
	frameRuntime.mu.Unlock()
}
