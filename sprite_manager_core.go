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

package spx

import "github.com/goplus/spx/v2/internal/engine"

// spriteManager manages the lifecycle of all sprites/shapes.
// It is responsible for:
//   - activation (delayed add)
//   - destruction (delayed remove)
//   - render layer grouping
//   - minimizing per-frame allocations
type spriteManager struct {
	items        []Shape
	tempItems    []Shape
	destroyItems []Shape
}

// init prepares internal buffers while preserving existing allocations when possible.
func (sm *spriteManager) init() {
	if sm.items == nil {
		sm.items = make([]Shape, 0, 64)
	} else {
		sm.items = sm.items[:0]
	}
	if sm.tempItems == nil {
		sm.tempItems = make([]Shape, 0, 50)
	} else {
		sm.tempItems = sm.tempItems[:0]
	}
	if sm.destroyItems == nil {
		sm.destroyItems = make([]Shape, 0, 16)
	} else {
		sm.destroyItems = sm.destroyItems[:0]
	}
}

// reset clears all internal state while keeping allocated memory.
// It is safe to call between scenes or rounds.
func (sm *spriteManager) reset() {
	engine.ClearAllSprites()
	sm.init()
}
