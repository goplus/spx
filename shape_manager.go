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
	"github.com/goplus/spx/v3/internal/base/sliceutil"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"
	"github.com/goplus/spx/v3/internal/ui"
)

// The Godot runtime renders the shared pen canvas at layer zero. Managed
// sprites start at one so they always render above it; the backdrop stays at -1.
const firstSpriteLayer = 1

// shapeManager manages the lifecycle of all runtime shapes.
// It is responsible for:
//   - activation (delayed add)
//   - destruction (delayed remove)
//   - render layer grouping
//   - minimizing per-frame allocations
type shapeManager struct {
	items                  []Shape
	tempItems              []Shape
	destroyItems           []Shape
	textBubbles            []*textBubble
	activeTextBubbles      []*textBubble
	sayLayouts             []ui.SayBubbleLayout
	nextTextBubbleLayoutID uint64
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
	clear(s.textBubbles)
	s.textBubbles = s.textBubbles[:0]
	clear(s.activeTextBubbles)
	s.activeTextBubbles = s.activeTextBubbles[:0]
	clear(s.sayLayouts)
	s.sayLayouts = s.sayLayouts[:0]
	s.nextTextBubbleLayoutID = 0
}

// reset clears all internal state while keeping allocated memory.
// It is safe to call between scenes or rounds.
func (s *shapeManager) reset() {
	engine.ClearAllSprites()
	s.init()
}

// flushActivate updates all active non-sprite shapes for the current frame.
func (s *shapeManager) flushActivate(items []Shape) {
	s.layoutTextBubbles(items)
	if len(items) == 0 {
		return
	}

	delta := itime.DeltaTime()
	for _, item := range items {
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

func (s *shapeManager) layoutTextBubbles(items []Shape) {
	clear(s.textBubbles)
	s.textBubbles = s.textBubbles[:0]
	clear(s.sayLayouts)
	s.sayLayouts = s.sayLayouts[:0]

	for _, item := range items {
		bubble, ok := item.(*textBubble)
		if !ok || bubble.panel == nil || !bubble.sprite.effectiveProxyVisibility() {
			continue
		}
		s.textBubbles = append(s.textBubbles, bubble)
	}

	sortTextBubblesByLayoutID(s.textBubbles)
	topologyChanged := len(s.textBubbles) != len(s.activeTextBubbles)
	if !topologyChanged {
		for i, bubble := range s.textBubbles {
			if bubble != s.activeTextBubbles[i] {
				topologyChanged = true
				break
			}
		}
	}
	if topologyChanged {
		clear(s.activeTextBubbles)
		s.activeTextBubbles = append(s.activeTextBubbles[:0], s.textBubbles...)
	}
	if len(s.textBubbles) == 0 {
		return
	}

	winSize := s.textBubbles[0].sprite.g.getWindowSize()
	context := ui.NewSayBubbleLayoutContext(winSize)
	needsResolve := topologyChanged
	for _, bubble := range s.textBubbles {
		center, size := bubble.getBounds()
		layout := context.NewLayout(bubble.layoutID, center, size, bubble.content)
		if bubble.hasLayout {
			layout = layout.WithPreviousDirection(bubble.layout)
			if !bubble.layout.SameInput(layout) {
				needsResolve = true
			}
		} else {
			needsResolve = true
		}
		s.sayLayouts = append(s.sayLayouts, layout)
	}
	if !needsResolve {
		return
	}

	ui.ResolveSayBubbleLayouts(s.sayLayouts)
	for i, bubble := range s.textBubbles {
		bubble.setLayout(s.sayLayouts[i])
	}
}

func sortTextBubblesByLayoutID(bubbles []*textBubble) {
	// Bubble counts are normally tiny and already ordered. Insertion sort keeps
	// the unchanged-frame path allocation-free while making activation order
	// irrelevant to layout.
	for i := 1; i < len(bubbles); i++ {
		bubble := bubbles[i]
		j := i
		for j > 0 && bubbles[j-1].layoutID > bubble.layoutID {
			bubbles[j] = bubbles[j-1]
			j--
		}
		bubbles[j] = bubble
	}
}

func (s *shapeManager) collectProxyUpdates(items []Shape, buffer *engine.SpriteSyncBuffer) {
	for _, item := range items {
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
	if bubble, ok := shape.(*textBubble); ok && bubble.layoutID == 0 {
		s.nextTextBubbleLayoutID++
		if s.nextTextBubbleLayoutID == 0 {
			s.nextTextBubbleLayoutID++
		}
		bubble.layoutID = s.nextTextBubbleLayoutID
	}
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

// addClonedShape inserts a clone immediately behind the target it copied. This
// matches Scratch: repeated clones of one target are ordered oldest to newest,
// with the original target still in front of all of them.
func (s *shapeManager) addClonedShape(src, clone Shape) {
	if s.tryAddClonedShape(src, clone) {
		return
	}
	spxlog.Debug("AddClonedShape: cloning a deleted sprite")
	gco.Abort()
}

func (s *shapeManager) tryAddClonedShape(src, clone Shape) bool {
	idx := s.findShapeIndex(src)
	if idx < 0 {
		return false
	}

	s.items = sliceutil.InsertAt(s.items, idx, clone)
	s.updateRenderLayers()
	return true
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
	s.updateRenderLayersIncludingPending(nil)
}

func (s *shapeManager) updateRenderLayersIncludingPending(include *SpriteImpl) {
	if engine.HasLayerSortMethod() {
		return
	}

	layer := firstSpriteLayer
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok {
			// A clone does not occupy a visible layer until its first initialization
			// slice commits. This prevents rollback from transiently moving already
			// published peers in the engine.
			if sp.spriteState.IsProxyPublicationPending && sp != include {
				continue
			}
			sp.setLayer(layer)
			layer++
		}
	}
}

// syncDirtySpriteLayers applies all layer changes as one publication barrier.
// Clone initialization can move multiple sprites even when only the clone is
// about to become visible.
func (s *shapeManager) syncDirtySpriteLayers() {
	for _, item := range s.items {
		if sprite, ok := item.(*SpriteImpl); ok && !sprite.isDestroyed() {
			sprite.baseObj.applyLayerUpdate()
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
