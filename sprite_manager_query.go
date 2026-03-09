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

import "github.com/goplus/spx/v2/internal/base/sliceutil"

// all returns all active shapes.
func (sm *spriteManager) all() []Shape {
	return sm.items
}

// getTempShapes returns a copy of all active shapes in a temporary buffer.
func (sm *spriteManager) getTempShapes() []Shape {
	sm.tempItems = sliceutil.CopyInto(sm.tempItems, sm.items, 50)
	return sm.tempItems
}

// count returns the number of active shapes.
func (sm *spriteManager) count() int {
	return len(sm.items)
}

// findSprite finds a sprite by name (only non-cloned sprites).
func (sm *spriteManager) findSprite(name SpriteName) *SpriteImpl {
	for _, item := range sm.items {
		if sp, ok := item.(*SpriteImpl); ok {
			if !sp.Cloned && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

// findShapeIndex finds the index of a shape in the items slice.
func (sm *spriteManager) findShapeIndex(target Shape) int {
	for i, item := range sm.items {
		if item == target {
			return i
		}
	}
	return -1
}

// calculateNewIndex calculates the new index after moving n sprite layers.
func (sm *spriteManager) calculateNewIndex(currentIdx, n int) int {
	items := sm.items
	newIdx := currentIdx

	if n > 0 {
		for newIdx > 0 && n > 0 {
			newIdx--
			if _, ok := items[newIdx].(*SpriteImpl); ok {
				n--
			}
		}
	} else if n < 0 {
		lastIdx := len(items) - 1
		for newIdx < lastIdx && n < 0 {
			newIdx++
			if _, ok := items[newIdx].(*SpriteImpl); ok {
				n++
			}
		}
	}

	return newIdx
}
