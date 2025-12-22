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

import (
	"log"

	"github.com/goplus/spx/v2/internal/engine"
	gtime "github.com/goplus/spx/v2/internal/time"
)

// spriteManager manages the lifecycle of all sprites/shapes.
// It is responsible for:
//   - activation (delayed add)
//   - destruction (delayed remove)
//   - render layer grouping
//   - minimizing per-frame allocations
type spriteManager struct {
	// active shapes
	items []Shape
	// shapes waiting to be activated
	tempItems []Shape
	// shapes waiting to be destroyed
	destroyItems []Shape
}

// newSpriteManager creates a spriteManager with preallocated buffers.
func newSpriteManager() *spriteManager {
	return &spriteManager{
		items:        make([]Shape, 0, 64),
		tempItems:    make([]Shape, 0, 50),
		destroyItems: make([]Shape, 0, 16),
	}
}

// reset clears all internal state while keeping allocated memory.
// It is safe to call between scenes or rounds.
func (sm *spriteManager) reset() {
	sm.items = sm.items[:0]
	sm.tempItems = sm.tempItems[:0]
	sm.destroyItems = sm.destroyItems[:0]
}

//
// ========== lifecycle operations ==========
//

// add immediately adds a shape to the active list.
func (sm *spriteManager) add(s Shape) {
	sm.items = append(sm.items, s)
}

// remove schedules a shape for destruction at the end of the frame.
func (sm *spriteManager) remove(s Shape) {
	sm.destroyItems = append(sm.destroyItems, s)
}

// addShape is a legacy wrapper for add.
// Deprecated: Use add() instead.
func (sm *spriteManager) addShape(child Shape) {
	sm.add(child)
}

// addClonedShape inserts a cloned shape immediately after its source.
// This preserves rendering order and ensures clones appear behind their source.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) addClonedShape(src, clone Shape) {
	idx := sm.findShapeIndex(src)
	if idx < 0 {
		log.Println("addClonedShape: clone a deleted sprite")
		gco.Abort()
		return
	}

	sm.items = sm.insertAt(sm.items, idx, clone)
	sm.updateRenderLayers()
}

// removeShape removes a shape from the active list and schedules it for destruction.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) removeShape(child Shape) {
	idx := sm.findShapeIndex(child)
	if idx < 0 {
		return
	}

	sm.items = sm.deleteAt(sm.items, idx)
	sm.remove(child)
	sm.updateRenderLayers()
}

// activateShape moves a shape to the end of the active list (brings it to front).
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) activateShape(child Shape) {
	items := sm.items
	for idx, item := range items {
		if item == child {
			if idx == len(items)-1 {
				return
			}
			sm.items = sm.moveToEnd(sm.items, idx)
			sm.updateRenderLayers()
			return
		}
	}
}

// goBackLayers moves a sprite forward or backward by n layers.
// Positive n moves backward (deeper), negative n moves forward (shallower).
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) goBackLayers(spr *SpriteImpl, n int) {
	if engine.HasLayerSortMethod() {
		log.Println("Cannot manually set sprite layer when a layer sort mode is active.")
		return
	}

	if n == 0 {
		return
	}

	idx := sm.findShapeIndex(spr)
	if idx < 0 {
		return
	}

	newIdx := sm.calculateNewIndex(idx, n)
	if newIdx == idx {
		return
	}

	sm.items = sm.moveToIndex(sm.items, idx, newIdx)
	sm.updateRenderLayers()
}

//
// ========== flush phase ==========
//

// flushActivate updates all pending shapes in the active list.
func (sm *spriteManager) flushActivate() {
	if len(sm.tempItems) == 0 {
		return
	}

	for _, item := range sm.tempItems {
		if updater, ok := item.(interface{ onUpdate(float64) }); ok {
			updater.onUpdate(gtime.DeltaTime())
		}
	}
}

// flushDestroy performs cleanup for shapes that have been scheduled for destruction.
func (sm *spriteManager) flushDestroy() {
	if len(sm.destroyItems) == 0 {
		return
	}

	for _, item := range sm.destroyItems {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.syncSprite != nil {
			sprite.syncSprite.Destroy()
			sprite.syncSprite = nil
		}
	}

	sm.destroyItems = sm.destroyItems[:0]
}

//
// ========== render layer management ==========
//

// updateRenderLayers updates the layer index for all sprites.
// Only effective when no layer sort method is active.
func (sm *spriteManager) updateRenderLayers() {
	if engine.HasLayerSortMethod() {
		return
	}

	layer := 0
	for _, item := range sm.items {
		if sp, ok := item.(*SpriteImpl); ok {
			layer++
			sp.setLayer(layer)
		}
	}
}

//
// ========== query helpers ==========
//

// all returns all active shapes.
// Returns the internal slice - callers should not modify it.
func (sm *spriteManager) all() []Shape {
	return sm.items
}

// getTempShapes returns a copy of all active shapes in a temporary buffer.
func (sm *spriteManager) getTempShapes() []Shape {
	sm.tempItems = copyShapes(sm.tempItems, sm.items)
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
			if !sp.isCloned_ && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

//
// ========== internal helpers ==========
//

// findShapeIndex finds the index of a shape in the items slice.
// Returns -1 if not found.
func (sm *spriteManager) findShapeIndex(target Shape) int {
	for i, item := range sm.items {
		if item == target {
			return i
		}
	}
	return -1
}

// calculateNewIndex calculates the new index after moving n sprite layers.
// Positive n moves backward (toward index 0), negative n moves forward (toward end).
func (sm *spriteManager) calculateNewIndex(currentIdx, n int) int {
	items := sm.items
	newIdx := currentIdx

	if n > 0 {
		// Move backward (toward index 0)
		for newIdx > 0 && n > 0 {
			newIdx--
			if _, ok := items[newIdx].(*SpriteImpl); ok {
				n--
			}
		}
	} else if n < 0 {
		// Move forward (toward end)
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

// insertAt inserts an item at the specified index.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) insertAt(slice []Shape, idx int, item Shape) []Shape {
	n := len(slice)
	newSlice := make([]Shape, n+1)
	copy(newSlice[:idx], slice[:idx])
	newSlice[idx] = item
	copy(newSlice[idx+1:], slice[idx:])
	return newSlice
}

// deleteAt removes an item at the specified index.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) deleteAt(slice []Shape, idx int) []Shape {
	n := len(slice)
	newSlice := make([]Shape, n-1)
	copy(newSlice[:idx], slice[:idx])
	copy(newSlice[idx:], slice[idx+1:])
	return newSlice
}

// moveToEnd moves an item from idx to the end of the slice.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) moveToEnd(slice []Shape, idx int) []Shape {
	n := len(slice)
	item := slice[idx]
	newSlice := make([]Shape, n)
	copy(newSlice[:idx], slice[:idx])
	copy(newSlice[idx:n-1], slice[idx+1:])
	newSlice[n-1] = item
	return newSlice
}

// moveToIndex moves an item from oldIdx to newIdx.
// Creates a new slice to maintain immutability for concurrent access safety.
func (sm *spriteManager) moveToIndex(slice []Shape, oldIdx, newIdx int) []Shape {
	if oldIdx == newIdx {
		return slice
	}

	n := len(slice)
	item := slice[oldIdx]
	newSlice := make([]Shape, n)

	if oldIdx < newIdx {
		// Move forward: item moves toward end
		copy(newSlice[:oldIdx], slice[:oldIdx])
		copy(newSlice[oldIdx:newIdx], slice[oldIdx+1:newIdx+1])
		newSlice[newIdx] = item
		copy(newSlice[newIdx+1:], slice[newIdx+1:])
	} else {
		// Move backward: item moves toward start
		copy(newSlice[:newIdx], slice[:newIdx])
		newSlice[newIdx] = item
		copy(newSlice[newIdx+1:oldIdx+1], slice[newIdx:oldIdx])
		copy(newSlice[oldIdx+1:], slice[oldIdx+1:])
	}

	return newSlice
}

// copyShapes copies shapes from src to dst, reusing dst's capacity if possible.
func copyShapes(dst, src []Shape) []Shape {
	if dst == nil {
		dst = make([]Shape, 0, 50)
	}

	// Ensure capacity
	if cap(dst) < len(src) {
		dst = make([]Shape, len(src))
	} else {
		dst = dst[:len(src)]
	}

	copy(dst, src)
	return dst
}
