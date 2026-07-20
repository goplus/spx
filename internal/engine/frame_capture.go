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

	itime "github.com/goplus/spx/v2/internal/time"
)

var errCaptureHandlerNotConfigured = errors.New("spx: capture backend is not configured")

type captureQueue struct {
	mu       sync.Mutex
	sequence uint64
	requests []CaptureRequest
	handler  CaptureRequestHandler
}

func (q *captureQueue) setHandler(handler CaptureRequestHandler) {
	q.mu.Lock()
	q.handler = handler
	q.mu.Unlock()
}

func (q *captureQueue) submit(
	name string,
	inputTick *int64,
	enqueue bool,
) (CaptureRequest, CaptureRequestHandler) {
	q.mu.Lock()
	request := q.newRequestLocked(name, inputTick)
	if enqueue {
		q.requests = append(q.requests, request)
		q.mu.Unlock()
		return request, nil
	}
	handler := q.handler
	q.mu.Unlock()
	return request, handler
}

func (q *captureQueue) hasPending() bool {
	q.mu.Lock()
	pending := len(q.requests) != 0
	q.mu.Unlock()
	return pending
}

func (q *captureQueue) dispatch(request CaptureRequest) error {
	q.mu.Lock()
	handler := q.handler
	q.mu.Unlock()
	return callCaptureHandler(handler, request)
}

func (q *captureQueue) takeAll() []CaptureRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	requests := q.requests
	q.requests = nil
	return requests
}

func (q *captureQueue) newRequestLocked(name string, inputTick *int64) CaptureRequest {
	q.sequence++
	var tick *int64
	if inputTick != nil {
		value := *inputTick
		tick = &value
	}
	return CaptureRequest{
		Name:      name,
		InputTick: tick,
		Frame:     itime.Frame(),
		Sequence:  q.sequence,
	}
}

func (q *captureQueue) reset() {
	q.mu.Lock()
	q.requests = nil
	q.sequence = 0
	q.mu.Unlock()
}

func callCaptureHandler(handler CaptureRequestHandler, request CaptureRequest) error {
	if handler == nil {
		return errCaptureHandlerNotConfigured
	}
	return handler(request)
}
