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

type frameRuntimeState struct {
	mu          sync.Mutex
	callbackSeq uint64
	captureSeq  uint64
	callbacks   []frameCallback
	captures    []CaptureRequest
	handler     CaptureHandler
}

var frameRuntime frameRuntimeState

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
	frameRuntime.schedule(frame, fn)
}

// RunFrameCallbacks runs all callbacks due at or before frame.
func RunFrameCallbacks(frame int64) {
	callbacks := frameRuntime.drainDueCallbacks(frame)
	for _, cb := range callbacks {
		cb.fn()
	}
}

func (rt *frameRuntimeState) schedule(frame int64, fn func()) {
	rt.mu.Lock()
	rt.callbackSeq++
	rt.callbacks = append(rt.callbacks, frameCallback{
		frame: frame,
		seq:   rt.callbackSeq,
		fn:    fn,
	})
	rt.mu.Unlock()
}

func (rt *frameRuntimeState) drainDueCallbacks(frame int64) []frameCallback {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if len(rt.callbacks) == 0 {
		return nil
	}

	due := make([]frameCallback, 0, len(rt.callbacks))
	future := rt.callbacks[:0]
	for _, cb := range rt.callbacks {
		if cb.frame <= frame {
			due = append(due, cb)
			continue
		}
		future = append(future, cb)
	}

	clear(rt.callbacks[len(future):])
	rt.callbacks = future

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
	frameRuntime.setCaptureHandler(handler)
}

// EnqueueCapture queues a screenshot request for the end of the current frame.
func EnqueueCapture(name string, intent CaptureIntent) error {
	return frameRuntime.enqueueCapture(name, intent, CurrentFrame(), GetGame() == nil)
}

// FlushCaptures flushes screenshot requests queued during the frame.
func FlushCaptures() error {
	captures := frameRuntime.drainCaptures()
	for _, capture := range captures {
		if err := dispatchCapture(capture); err != nil {
			return err
		}
	}
	return nil
}

func (rt *frameRuntimeState) setCaptureHandler(handler CaptureHandler) {
	rt.mu.Lock()
	rt.handler = handler
	rt.mu.Unlock()
}

func (rt *frameRuntimeState) enqueueCapture(name string, intent CaptureIntent, frame int64, immediate bool) error {
	rt.mu.Lock()
	req := rt.nextCaptureRequest(name, intent, frame)
	if !immediate {
		rt.captures = append(rt.captures, req)
		rt.mu.Unlock()
		return nil
	}
	handler := rt.handler
	rt.mu.Unlock()
	return callCaptureHandler(handler, req)
}

func (rt *frameRuntimeState) nextCaptureRequest(name string, intent CaptureIntent, frame int64) CaptureRequest {
	rt.captureSeq++
	return CaptureRequest{
		Name:     name,
		Intent:   intent,
		Frame:    frame,
		Sequence: rt.captureSeq,
	}
}

func (rt *frameRuntimeState) drainCaptures() []CaptureRequest {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.captures) == 0 {
		return nil
	}
	captures := append([]CaptureRequest(nil), rt.captures...)
	clear(rt.captures)
	rt.captures = rt.captures[:0]
	return captures
}

func dispatchCapture(req CaptureRequest) error {
	frameRuntime.mu.Lock()
	handler := frameRuntime.handler
	frameRuntime.mu.Unlock()
	return callCaptureHandler(handler, req)
}

func callCaptureHandler(handler CaptureHandler, req CaptureRequest) error {
	if handler == nil {
		return fmt.Errorf("spx: capture backend is not configured")
	}
	return handler(req)
}

// ResetFrameRuntime clears frame-scoped callbacks and capture requests.
func ResetFrameRuntime() {
	frameRuntime.reset()
}

func (rt *frameRuntimeState) reset() {
	rt.mu.Lock()
	clear(rt.callbacks)
	rt.callbacks = nil
	rt.callbackSeq = 0
	clear(rt.captures)
	rt.captures = nil
	rt.captureSeq = 0
	rt.mu.Unlock()
}
