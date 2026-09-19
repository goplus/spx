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
	"context"

	"github.com/goplus/spx/v3/internal/coroutine"
)

func Execute(owner any, fn func(ctx context.Context, owner any)) {
	binding, ok := captureRuntimeWork()
	if !ok {
		return
	}
	co := gco
	if co.IsInCoroutine() {
		thread := co.Current()
		if isRuntimeWorkCurrent(binding) {
			fn(thread.Context(), owner)
		}
		return
	}

	owner = ResolveCoroutineOwner(owner)
	call := func() {
		if isRuntimeWorkCurrent(binding) {
			fn(co.Current().Context(), owner)
		}
	}
	if co.TryRunFromEngine(owner, call) {
		return
	}
	thread := co.Create(owner, func(coroutine.Thread) int {
		call()
		return 0
	})
	// Completion also covers cancellation before the callback starts.
	co.Join(thread)
}

func WaitMainThread(call func()) {
	gco.WaitMainThread(call)
}

func WaitToDo(call func()) {
	gco.WaitToDo(call)
}

func ExecuteNative(fn func(ctx context.Context, owner any)) {
	thread := currentThread()
	if thread == nil {
		fn(context.Background(), nil)
		return
	}
	ctx, owner := thread.Context(), thread.Obj
	WaitToDo(func() {
		fn(ctx, owner)
	})
}
