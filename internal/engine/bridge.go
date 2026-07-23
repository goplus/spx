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
	"sync/atomic"

	"github.com/goplus/spx/v3/internal/enginewrap"
)

var (
	defaultManagers enginewrap.EngineManagers
	activeManagers  atomic.Pointer[enginewrap.EngineManagers]
)

func init() {
	activeManagers.Store(&defaultManagers)
}

// SetManagers injects the runtime-scoped manager set used by the Go bridge layer.
// Passing nil resets to the default zero-value manager set.
func SetManagers(managers *enginewrap.EngineManagers) {
	if managers == nil {
		activeManagers.Store(&defaultManagers)
		return
	}
	activeManagers.Store(managers)
}

// Managers returns the active runtime-scoped manager set for the current game.
func Managers() *enginewrap.EngineManagers {
	return activeManagers.Load()
}
