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

// CaptureRequest identifies a screenshot and its timing.
type CaptureRequest struct {
	Name string
	// InputTick is nil when no input tick applies; zero is valid.
	InputTick *int64
	// Frame is the engine frame, independent of InputTick.
	Frame    int64
	Sequence uint64
}

// CaptureHandler handles a screenshot request.
type CaptureHandler func(CaptureRequest) error

var errCaptureHandlerNotConfigured = errors.New("spx: capture backend is not configured")

var captureState struct {
	mu       sync.Mutex
	sequence uint64
	requests []CaptureRequest
	handler  CaptureHandler
}

// SetCaptureHandler installs the screenshot backend used by EnqueueCapture.
func SetCaptureHandler(handler CaptureHandler) {
	captureState.mu.Lock()
	captureState.handler = handler
	captureState.mu.Unlock()
}

// HasPendingCaptures reports whether capture requests are queued.
func HasPendingCaptures() bool {
	captureState.mu.Lock()
	defer captureState.mu.Unlock()
	return len(captureState.requests) != 0
}

// FlushCaptures flushes the current batch. Requests enqueued by handlers remain
// queued for a subsequent flush.
func FlushCaptures() error {
	captureState.mu.Lock()
	requests := captureState.requests
	captureState.requests = nil
	captureState.mu.Unlock()

	var errs []error
	for _, request := range requests {
		// A handler may replace the backend for subsequent requests.
		captureState.mu.Lock()
		handler := captureState.handler
		captureState.mu.Unlock()
		if err := callCaptureHandler(handler, request); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ResetCaptures clears queued capture requests and resets their sequence,
// preserving the backend. It does not cancel an already flushing batch.
func ResetCaptures() {
	captureState.mu.Lock()
	captureState.requests = nil
	captureState.sequence = 0
	captureState.mu.Unlock()
}

// EnqueueCapture queues a screenshot request for the end of the current frame,
// or dispatches it immediately when no game is active. A non-nil inputTick is
// copied into the request to associate it with an input recording/replay tick.
func EnqueueCapture(name string, inputTick *int64) error {
	enqueue := GetGame() != nil
	captureState.mu.Lock()
	captureState.sequence++
	var tick *int64
	if inputTick != nil {
		value := *inputTick
		tick = &value
	}
	request := CaptureRequest{
		Name:      name,
		InputTick: tick,
		Frame:     itime.Frame(),
		Sequence:  captureState.sequence,
	}
	if enqueue {
		captureState.requests = append(captureState.requests, request)
		captureState.mu.Unlock()
		return nil
	}
	handler := captureState.handler
	captureState.mu.Unlock()
	return callCaptureHandler(handler, request)
}

func callCaptureHandler(handler CaptureHandler, request CaptureRequest) error {
	if handler == nil {
		return errCaptureHandlerNotConfigured
	}
	return handler(request)
}
