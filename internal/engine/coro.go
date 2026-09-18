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
	"github.com/goplus/spx/v3/internal/engine/profiler"
)

var (
	gco        *coroutine.Coroutines
	gameObject any
)

func SetGame(game any) {
	gameObject = game
}

func GetGame() any {
	return gameObject
}

func IsInCoroutine() bool {
	return gco != nil && gco.IsInCoroutine()
}

func GetCoroutineOwner() any {
	if thread := currentThread(); thread != nil {
		return thread.Obj
	}
	return nil
}

func GetCurrentThreadContext() context.Context {
	if thread := currentThread(); thread != nil {
		return thread.Context()
	}
	return context.Background()
}

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
	profiler.SetGco(co)
}

func Go(tobj coroutine.ThreadObj, fn func(ctx context.Context)) {
	gco.Create(tobj, func(me coroutine.Thread) int {
		fn(me.Context())
		return 0
	})
}

func ResolveCoroutineOwner(owner any) any {
	if owner != nil {
		return owner
	}
	if thread := currentThread(); thread != nil {
		return thread.Obj
	}
	return GetGame()
}

func GoWithOwner(owner any, fn func(ctx context.Context, owner any)) {
	owner = ResolveCoroutineOwner(owner)
	Go(owner, func(ctx context.Context) {
		fn(ctx, owner)
	})
}

func currentThread() coroutine.Thread {
	co := gco
	if co == nil || !co.IsInCoroutine() {
		return nil
	}
	return co.Current()
}
