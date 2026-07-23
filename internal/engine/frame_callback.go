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
	"sort"
	"sync"

	"github.com/goplus/spx/v3/internal/coroutine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type scheduledFrameCallback struct {
	frame   int64
	seq     uint64
	context frameCallbackContext
	fn      func()
}

type frameCallbackContext struct {
	owner  any
	source coroutine.Thread
}

type frameCallbackQueue struct {
	mu        sync.Mutex
	sequence  uint64
	callbacks []scheduledFrameCallback
}

func currentFrameCallbackContext() frameCallbackContext {
	callbackContext := frameCallbackContext{owner: GetGame()}
	if gco == nil || !gco.IsInCoroutine() {
		return callbackContext
	}
	callbackContext.source = gco.Current()
	callbackContext.owner = callbackContext.source.Obj
	return callbackContext
}

func (q *frameCallbackQueue) schedule(
	frame int64,
	callbackContext frameCallbackContext,
	fn func(),
) (scheduledFrameCallback, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.sequence++
	callback := scheduledFrameCallback{
		frame:   frame,
		seq:     q.sequence,
		context: callbackContext,
		fn:      fn,
	}
	if frame <= itime.Frame() {
		return callback, true
	}
	q.callbacks = append(q.callbacks, callback)
	return callback, false
}

func (q *frameCallbackQueue) takeDue(frame int64) []scheduledFrameCallback {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.callbacks) == 0 {
		return nil
	}
	due := make([]scheduledFrameCallback, 0, len(q.callbacks))
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

	sort.Slice(due, func(i, j int) bool {
		if due[i].frame == due[j].frame {
			return due[i].seq < due[j].seq
		}
		return due[i].frame < due[j].frame
	})
	return due
}

func (q *frameCallbackQueue) reset() {
	q.mu.Lock()
	q.callbacks = nil
	q.sequence = 0
	q.mu.Unlock()
}

func executeFrameCallbacks(callbacks []scheduledFrameCallback) {
	if len(callbacks) == 0 {
		return
	}
	if gco != nil && !gco.IsInCoroutine() {
		gco.CreateAndStart(false, GetGame(), func(coroutine.Thread) int {
			executeFrameCallbacks(callbacks)
			return 0
		})
		return
	}
	for _, callback := range callbacks {
		executeFrameCallback(callback)
	}
}

func executeFrameCallback(callback scheduledFrameCallback) {
	if callback.canceled() {
		return
	}
	if gco == nil {
		callback.fn()
		return
	}
	thread := gco.CreateAndStart(false, callback.context.owner, func(coroutine.Thread) int {
		if !callback.canceled() {
			callback.fn()
		}
		return 0
	})
	// The engine thread cannot wait here because the callback may need
	// WaitMainThread. Coroutine callers still preserve immediate semantics.
	if gco.IsInCoroutine() {
		gco.JoinYieldedOrDone(thread)
	}
}

func (callback scheduledFrameCallback) canceled() bool {
	// Normal coroutine completion does not cancel callbacks registered by it;
	// only an explicit Stop or Abort marks the source stopped.
	return callback.context.source != nil && callback.context.source.Stopped()
}
