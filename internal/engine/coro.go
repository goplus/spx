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

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine/profiler"
	"github.com/goplus/spx/v2/internal/time"
)

var (
	gco   *coroutine.Coroutines
	pgame any // pointer to the current game object
)

func SetGame(game any) {
	pgame = game
}

func GetGame() any {
	return pgame
}

func IsInCoroutine() bool {
	if gco == nil {
		return false
	}
	return gco.IsInCoroutine()
}

func IsAbortThreadError(err any) bool {
	return coroutine.IsAbortThreadError(err)
}

func GetCoroutineOwner() any {
	if IsInCoroutine() {
		return gco.Current().Obj
	}
	return nil
}

func GetCurrentThreadContext() context.Context {
	if IsInCoroutine() {
		return gco.Current().Context()
	}
	return context.Background()
}

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
	profiler.SetGco(co)
}

func Go(tobj coroutine.ThreadObj, fn func(ctx context.Context)) {
	gco.CreateAndStart(false, tobj, func(me coroutine.Thread) int {
		fn(me.Context())
		return 0
	})
}

func Wait(secs float64) float64 {
	startTime := time.TimeSinceLevelLoad()
	gco.Wait(secs)
	return time.TimeSinceLevelLoad() - startTime
}

func WaitYield() {
	gco.WaitYield(gco.Current())
}

func WaitNextFrame() float64 {
	gco.WaitNextFrame()
	return time.DeltaTime()
}

func WaitMainThread(call func()) {
	gco.WaitMainThread(call)
}

func WaitToDo(fn func()) {
	gco.WaitToDo(fn)
}

func WaitForChan[T any](done <-chan T, data *T) {
	coroutine.WaitForChan(gco, done, data)
}
