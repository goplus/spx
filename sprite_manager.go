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
	"github.com/goplus/spx/v2/internal/base/sliceutil"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	gtime "github.com/goplus/spx/v2/internal/time"
)

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

// flushActivate updates all pending shapes in the active list.
func (sm *spriteManager) flushActivate() {
	if len(sm.tempItems) == 0 {
		return
	}

	delta := gtime.DeltaTime()
	for _, item := range sm.tempItems {
		if _, ok := item.(*SpriteImpl); ok {
			continue
		}

		switch v := item.(type) {
		case *quoterBubble:
			v.onUpdate(delta)
		case *textBubble:
			v.onUpdate(delta)
		case *Monitor:
			v.onUpdate(delta)
		default:
			if updater, ok := item.(interface{ onUpdate(float64) }); ok {
				updater.onUpdate(delta)
			}
		}
	}
}

func (sm *spriteManager) collectProxyUpdates(buffer *engine.SpriteSyncBuffer) {
	for _, item := range sm.getTempShapes() {
		if sprite, ok := item.(*SpriteImpl); ok {
			sprite.collectProxyUpdate(buffer)
		}
	}
}

// flushDestroy performs cleanup for shapes that have been scheduled for destruction.
func (sm *spriteManager) flushDestroy(buffer *engine.SpriteSyncBuffer) {
	if len(sm.destroyItems) == 0 {
		return
	}

	for _, item := range sm.destroyItems {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.runtimeState.SyncSprite != nil {
			buffer.AddDelete(int64(sprite.runtimeState.SyncSprite.Id))
			sprite.runtimeState.SyncSprite = nil
		}
	}

	sm.destroyItems = sm.destroyItems[:0]
}

// add adds a shape immediately to the active list.
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
func (sm *spriteManager) addClonedShape(src, clone Shape) {
	idx := sm.findShapeIndex(src)
	if idx < 0 {
		spxlog.Debug("addClonedShape: clone a deleted sprite")
		gco.Abort()
		return
	}

	sm.items = sliceutil.InsertAt(sm.items, idx, clone)
	sm.updateRenderLayers()
}

// removeShape removes a shape from the active list and schedules it for destruction.
func (sm *spriteManager) removeShape(child Shape) {
	idx := sm.findShapeIndex(child)
	if idx < 0 {
		return
	}

	sm.items = sliceutil.DeleteAt(sm.items, idx)
	sm.remove(child)
	sm.updateRenderLayers()
}

// activateShape moves a shape to the end of the active list.
func (sm *spriteManager) activateShape(child Shape) {
	items := sm.items
	for idx, item := range items {
		if item == child {
			if idx == len(items)-1 {
				return
			}
			sm.items = sliceutil.MoveToEnd(sm.items, idx)
			sm.updateRenderLayers()
			return
		}
	}
}

// goBackLayers moves a sprite forward or backward by n layers.
func (sm *spriteManager) goBackLayers(spr *SpriteImpl, n int) {
	if engine.HasLayerSortMethod() {
		spxlog.Debug("Cannot manually set sprite layer when a layer sort mode is active.")
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

	sm.items = sliceutil.MoveToIndex(sm.items, idx, newIdx)
	sm.updateRenderLayers()
}

// updateRenderLayers updates the layer index for all sprites.
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
			if !sp.spriteState.Cloned && sp.name == name {
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
