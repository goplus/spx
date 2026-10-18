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

var gco *coroutine.Coroutines

// SetGame installs a non-runtime owner binding and cannot replace Main.
func SetGame(game any) {
	bindingMu.Lock()
	defer bindingMu.Unlock()
	if binding := activeGame.Load(); binding != nil && binding.callbacks != nil {
		panic("engine: SetGame cannot replace an active runtime")
	}
	if game == nil {
		activeGame.Store(nil)
		return
	}
	binding := &gameBinding{owner: game}
	binding.storePhase(gameRunning)
	activeGame.Store(binding)
}

func GetGame() any {
	if binding := activeGame.Load(); binding != nil {
		return binding.owner
	}
	return nil
}

func IsInCoroutine() bool {
	return gco != nil && gco.IsInCoroutine()
}

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
	profiler.SetGco(co)
}

func Go(owner any, fn func(ctx context.Context, owner any)) {
	binding, ok := captureRuntimeWork()
	if !ok {
		return
	}
	owner = resolveOwner(owner)
	gco.Create(owner, func(me coroutine.Thread) {
		if isRuntimeWorkCurrent(binding) {
			fn(me.Context(), owner)
		}
	})
}

func resolveOwner(owner any) any {
	if owner != nil {
		return owner
	}
	if thread := currentThread(); thread != nil {
		return thread.Owner
	}
	return GetGame()
}

func currentThread() coroutine.Thread {
	co := gco
	if co == nil || !co.IsInCoroutine() {
		return nil
	}
	return co.Current()
}
