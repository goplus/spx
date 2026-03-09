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
)

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
