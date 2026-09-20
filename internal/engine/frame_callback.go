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
	"cmp"
	"slices"
	"sync"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type frameCallback struct {
	frame  int64
	origin frameCallbackOrigin
	fn     func()
}

type frameCallbackOrigin struct {
	owner  any
	thread coroutine.Thread
}

type frameCallbackQueue struct {
	mu        sync.Mutex
	callbacks []frameCallback
}

func (q *frameCallbackQueue) schedule(
	frame int64,
	origin frameCallbackOrigin,
	fn func(),
) (frameCallback, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	callback := frameCallback{
		frame:  frame,
		origin: origin,
		fn:     fn,
	}
	if frame <= itime.Frame() {
		return callback, true
	}
	q.callbacks = append(q.callbacks, callback)
	return callback, false
}

func (q *frameCallbackQueue) takeDue(frame int64) []frameCallback {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.callbacks) == 0 {
		return nil
	}
	due := make([]frameCallback, 0, len(q.callbacks))
	future := q.callbacks[:0]
	for _, callback := range q.callbacks {
		if callback.frame <= frame {
			due = append(due, callback)
		} else {
			future = append(future, callback)
		}
	}
	clear(q.callbacks[len(future):])
	q.callbacks = future

	slices.SortStableFunc(due, func(a, b frameCallback) int {
		return cmp.Compare(a.frame, b.frame)
	})
	return due
}

func (q *frameCallbackQueue) reset() {
	q.mu.Lock()
	q.callbacks = nil
	q.mu.Unlock()
}

func (callback frameCallback) canceled() bool {
	// A callback expires when its source thread stops.
	return callback.origin.thread != nil && callback.origin.thread.Stopped()
}

func currentFrameCallbackOrigin() frameCallbackOrigin {
	origin := frameCallbackOrigin{owner: GetGame()}
	if thread := currentThread(); thread != nil {
		origin.thread = thread
		origin.owner = thread.Obj
	}
	return origin
}

func executeFrameCallbacks(callbacks []frameCallback) {
	if len(callbacks) == 0 {
		return
	}
	if gco != nil && !gco.IsInCoroutine() {
		gco.Create(GetGame(), func(coroutine.Thread) {
			executeFrameCallbacks(callbacks)
		})
		return
	}
	for _, callback := range callbacks {
		executeFrameCallback(callback)
	}
}

func executeFrameCallback(callback frameCallback) {
	if callback.canceled() {
		return
	}
	if gco == nil {
		callback.fn()
		return
	}
	thread := gco.Create(callback.origin.owner, func(coroutine.Thread) {
		if !callback.canceled() {
			callback.fn()
		}
	})
	// The engine thread cannot wait here because the callback may need
	// WaitMainThread. Coroutine callers still preserve immediate semantics.
	if gco.IsInCoroutine() {
		gco.JoinYieldedOrDone(thread)
	}
}
