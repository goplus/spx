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

// shapeManager manages the lifecycle of all runtime shapes.
// It is responsible for:
//   - activation (delayed add)
//   - destruction (delayed remove)
//   - render layer grouping
//   - minimizing per-frame allocations
type shapeManager struct {
	items        []Shape
	tempItems    []Shape
	destroyItems []Shape
}

// init prepares internal buffers while preserving existing allocations when possible.
func (s *shapeManager) init() {
	if s.items == nil {
		s.items = make([]Shape, 0, 64)
	} else {
		s.items = s.items[:0]
	}
	if s.tempItems == nil {
		s.tempItems = make([]Shape, 0, 50)
	} else {
		s.tempItems = s.tempItems[:0]
	}
	if s.destroyItems == nil {
		s.destroyItems = make([]Shape, 0, 16)
	} else {
		s.destroyItems = s.destroyItems[:0]
	}
}

// reset clears all internal state while keeping allocated memory.
// It is safe to call between scenes or rounds.
func (s *shapeManager) reset() {
	engine.ClearAllSprites()
	s.init()
}

// flushActivate updates all pending shapes in the active list.
func (s *shapeManager) flushActivate() {
	if len(s.tempItems) == 0 {
		return
	}

	delta := gtime.DeltaTime()
	for _, item := range s.tempItems {
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

func (s *shapeManager) collectProxyUpdates(buffer *engine.SpriteSyncBuffer) {
	for _, item := range s.getTempShapes() {
		if sprite, ok := item.(*SpriteImpl); ok {
			sprite.collectProxyUpdate(buffer)
		}
	}
}

// flushDestroy performs cleanup for shapes that have been scheduled for destruction.
func (s *shapeManager) flushDestroy(buffer *engine.SpriteSyncBuffer) {
	if len(s.destroyItems) == 0 {
		return
	}

	for _, item := range s.destroyItems {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.runtimeState.SyncSprite != nil {
			buffer.AddDelete(int64(sprite.runtimeState.SyncSprite.Id))
			sprite.runtimeState.SyncSprite = nil
		}
	}

	s.destroyItems = s.destroyItems[:0]
}

// add adds a shape immediately to the active list.
func (s *shapeManager) add(shape Shape) {
	s.items = append(s.items, shape)
}

// remove schedules a shape for destruction at the end of the frame.
func (s *shapeManager) remove(shape Shape) {
	s.destroyItems = append(s.destroyItems, shape)
}

// addShape delegates to add; kept for call-site consistency.
func (s *shapeManager) addShape(child Shape) {
	s.add(child)
}

// addClonedShape inserts a cloned shape immediately after its source.
// This preserves rendering order and ensures clones appear behind their source.
func (s *shapeManager) addClonedShape(src, clone Shape) {
	idx := s.findShapeIndex(src)
	if idx < 0 {
		spxlog.Debug("addClonedShape: clone a deleted sprite")
		gco.Abort()
		return
	}

	s.items = sliceutil.InsertAt(s.items, idx, clone)
	s.updateRenderLayers()
}

// removeShape removes a shape from the active list and schedules it for destruction.
func (s *shapeManager) removeShape(child Shape) {
	idx := s.findShapeIndex(child)
	if idx < 0 {
		return
	}

	s.items = sliceutil.DeleteAt(s.items, idx)
	s.remove(child)
	s.updateRenderLayers()
}

// activateShape moves a shape to the end of the active list.
func (s *shapeManager) activateShape(child Shape) {
	items := s.items
	for idx, item := range items {
		if item == child {
			if idx == len(items)-1 {
				return
			}
			s.items = sliceutil.MoveToEnd(s.items, idx)
			s.updateRenderLayers()
			return
		}
	}
}

// goBackLayers moves a sprite forward or backward by n layers.
func (s *shapeManager) goBackLayers(spr *SpriteImpl, n int) {
	if engine.HasLayerSortMethod() {
		spxlog.Debug("Cannot manually set sprite layer when a layer sort mode is active.")
		return
	}
	if n == 0 {
		return
	}

	idx := s.findShapeIndex(spr)
	if idx < 0 {
		return
	}
	newIdx := s.calculateNewIndex(idx, n)
	if newIdx == idx {
		return
	}

	s.items = sliceutil.MoveToIndex(s.items, idx, newIdx)
	s.updateRenderLayers()
}

// updateRenderLayers updates the layer index for all sprites.
func (s *shapeManager) updateRenderLayers() {
	if engine.HasLayerSortMethod() {
		return
	}

	layer := 0
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok {
			layer++
			sp.setLayer(layer)
		}
	}
}

// all returns all active shapes.
func (s *shapeManager) all() []Shape {
	return s.items
}

// getTempShapes returns a copy of all active shapes in a temporary buffer.
func (s *shapeManager) getTempShapes() []Shape {
	s.tempItems = sliceutil.CopyInto(s.tempItems, s.items, 50)
	return s.tempItems
}

// count returns the number of active shapes.
func (s *shapeManager) count() int {
	return len(s.items)
}

// findSprite finds a sprite by name (only non-cloned sprites).
func (s *shapeManager) findSprite(name SpriteName) *SpriteImpl {
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok {
			if !sp.spriteState.Cloned && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

// findShapeIndex finds the index of a shape in the items slice.
func (s *shapeManager) findShapeIndex(target Shape) int {
	for i, item := range s.items {
		if item == target {
			return i
		}
	}
	return -1
}

// calculateNewIndex calculates the new index after moving n sprite layers.
func (s *shapeManager) calculateNewIndex(currentIdx, n int) int {
	items := s.items
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
