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
	"errors"
	"sync"

	itime "github.com/goplus/spx/v3/internal/time"
)

// CaptureRequest identifies one end-of-frame screenshot.
type CaptureRequest struct {
	Name string
	// InputTick is nil until recording or replay consumes its first tick.
	InputTick *int64
	// Frame is render metadata independent of InputTick.
	Frame    int64
	Sequence uint64
}

// CaptureRequestHandler handles one screenshot request.
type CaptureRequestHandler func(CaptureRequest) error

// CurrentFrame returns the engine-session frame counter.
func CurrentFrame() int64 {
	return itime.Frame()
}

// ScheduleFrame schedules fn to run once when the engine reaches frame.
func ScheduleFrame(frame int64, fn func()) {
	if fn == nil {
		return
	}
	if GetGame() == nil {
		fn()
		return
	}
	frameRuntime.scheduleCallback(frame, fn)
}

// RunFrameCallbacks dispatches callbacks due at or before the current frame.
func RunFrameCallbacks() {
	executeFrameCallbacks(frameRuntime.callbacks.takeDue(itime.Frame()))
}

// SetCaptureHandler installs the screenshot backend used by EnqueueCapture.
func SetCaptureHandler(handler CaptureRequestHandler) {
	frameRuntime.captures.setHandler(handler)
}

// EnqueueCapture queues a screenshot request for the end of the current frame.
func EnqueueCapture(name string) error {
	return frameRuntime.submitCapture(name, nil, GetGame() != nil)
}

// EnqueueCaptureAtInputTick queues a screenshot request associated with an
// input recording/replay tick while retaining the current engine frame as
// diagnostic metadata.
func EnqueueCaptureAtInputTick(name string, inputTick int64) error {
	return frameRuntime.submitCapture(name, &inputTick, GetGame() != nil)
}

// HasPendingCaptures reports whether capture requests are queued.
func HasPendingCaptures() bool {
	return frameRuntime.captures.hasPending()
}

// FlushCaptures flushes screenshot requests queued during the frame.
func FlushCaptures() error {
	return frameRuntime.flushCaptures()
}

// ResetFrameRuntime clears pending callbacks and capture requests.
func ResetFrameRuntime() {
	frameRuntime.reset()
}

type frameRuntimeState struct {
	lifecycle sync.RWMutex
	callbacks frameCallbackQueue
	captures  captureQueue
}

var frameRuntime frameRuntimeState

func (rt *frameRuntimeState) scheduleCallback(frame int64, fn func()) {
	rt.lifecycle.RLock()
	callback, due := rt.callbacks.schedule(
		frame,
		currentFrameCallbackContext(),
		fn,
	)
	rt.lifecycle.RUnlock()
	if due {
		executeFrameCallback(callback)
	}
}

func (rt *frameRuntimeState) submitCapture(name string, inputTick *int64, enqueue bool) error {
	rt.lifecycle.RLock()
	request, handler := rt.captures.submit(name, inputTick, enqueue)
	rt.lifecycle.RUnlock()
	if enqueue {
		return nil
	}
	return callCaptureHandler(handler, request)
}

func (rt *frameRuntimeState) flushCaptures() error {
	captures := rt.captures.takeAll()
	var errs []error
	for _, request := range captures {
		if err := rt.captures.dispatch(request); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (rt *frameRuntimeState) reset() {
	rt.lifecycle.Lock()
	rt.callbacks.reset()
	rt.captures.reset()
	rt.lifecycle.Unlock()
}
