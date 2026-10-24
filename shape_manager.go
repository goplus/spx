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
	"cmp"
	"slices"

	"github.com/goplus/spx/v4/internal/base/sliceutil"
	"github.com/goplus/spx/v4/internal/engine"
	spxlog "github.com/goplus/spx/v4/internal/log"
	itime "github.com/goplus/spx/v4/internal/time"
	"github.com/goplus/spx/v4/internal/ui"
)

// Scratch renders the shared pen canvas above the backdrop and below every
// managed sprite. Keep the pen layer explicit so sprite ordering cannot reuse it.
const (
	penLayer         = 0
	firstSpriteLayer = penLayer + 1
)

// Scratch shares one limit across all sprites, including hidden clones.
const maxClones = 300

type shapeManager struct {
	cloneCount             int
	pendingClones          int
	items                  []Shape
	named                  map[string][]*SpriteImpl
	tempItems              []Shape
	destroyItems           []*SpriteImpl
	proxyChangeBuffer      []proxyChange
	bubbles                []bubbleShape
	textBubbles            []*textBubble
	activeTextBubbles      []*textBubble
	sayLayouts             []ui.SayBubbleLayout
	nextTextBubbleLayoutID uint64
}

func (s *shapeManager) init() {
	s.cloneCount = 0
	s.pendingClones = 0
	s.named = nil
	s.items = sliceutil.Reset(s.items, 64)
	s.tempItems = sliceutil.Reset(s.tempItems, 50)
	s.destroyItems = sliceutil.Reset(s.destroyItems, 16)
	s.proxyChangeBuffer = sliceutil.Reset(s.proxyChangeBuffer, 0)
	s.bubbles = sliceutil.Reset(s.bubbles, 0)
	s.textBubbles = sliceutil.Reset(s.textBubbles, 0)
	s.activeTextBubbles = sliceutil.Reset(s.activeTextBubbles, 0)
	s.sayLayouts = sliceutil.Reset(s.sayLayouts, 0)
	s.nextTextBubbleLayoutID = 0
}

func (s *shapeManager) reset() {
	engine.ClearAllSprites()
	s.init()
}

func (s *shapeManager) flushBubbleVisuals(items []Shape) {
	s.collectBubbles(items)

	frame := itime.Frame()
	// Remove expired bubbles before resolving this frame's layouts.
	for _, bubble := range s.bubbles {
		bubble.flushPendingRemoval(frame)
	}

	s.layoutTextBubbles()

	for _, bubble := range s.bubbles {
		bubble.onUpdate()
	}
}

func (s *shapeManager) collectBubbles(items []Shape) {
	s.bubbles = sliceutil.Reset(s.bubbles, 0)
	s.textBubbles = sliceutil.Reset(s.textBubbles, 0)
	for _, item := range items {
		bubble, ok := item.(bubbleShape)
		if !ok {
			continue
		}
		s.bubbles = append(s.bubbles, bubble)
		if textBubble, ok := bubble.(*textBubble); ok {
			s.textBubbles = append(s.textBubbles, textBubble)
		}
	}
}

func (s *shapeManager) layoutTextBubbles() {
	s.sayLayouts = sliceutil.Reset(s.sayLayouts, 0)

	s.textBubbles = slices.DeleteFunc(s.textBubbles, func(bubble *textBubble) bool {
		return bubble.panel == nil || !bubble.sprite.Visible()
	})

	slices.SortStableFunc(s.textBubbles, func(a, b *textBubble) int {
		return cmp.Compare(a.layoutID, b.layoutID)
	})
	topologyChanged := !slices.Equal(s.textBubbles, s.activeTextBubbles)
	if topologyChanged {
		s.activeTextBubbles = sliceutil.CopyInto(s.activeTextBubbles, s.textBubbles, 0)
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
		}
		needsResolve = needsResolve || !bubble.hasLayout || !bubble.layout.SameInput(layout)
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

func (s *shapeManager) collectProxyChanges(items []Shape, buffer *engine.SpriteSyncBuffer, batch *proxyBatch) {
	// Prepare resources first, then capture the latest position and visibility.
	// Each sprite keeps the geometry paired with its prepared resources.
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			sprite.prepareProxyUpdate()
		}
	}
	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			if update := sprite.collectProxyUpdate(buffer); update.sprite != nil {
				batch.changes = append(batch.changes, update)
			}
		}
	}

	// Keep deletion requests pending until this batch commits.
	batch.destroyCount = len(s.destroyItems)
	for _, sprite := range s.destroyItems {
		proxy := sprite.runtimeState.SyncSprite
		if proxy == nil {
			continue
		}
		buffer.AddDelete(proxy.Id)
		batch.changes = append(batch.changes, proxyChange{sprite: sprite, proxy: proxy, destroy: true})
	}
}

func (s *shapeManager) commitProxyChanges(batch proxyBatch) {
	for _, change := range batch.changes {
		commitProxyChange(change)
	}
	// Deletions queued during submission belong to the next batch.
	s.destroyItems = slices.Delete(s.destroyItems, 0, batch.destroyCount)
}

func (s *shapeManager) add(shape Shape) {
	if sprite, ok := shape.(*SpriteImpl); ok && sprite.IsCloned() {
		s.cloneCount++
	}
	if bubble, ok := shape.(*textBubble); ok && bubble.layoutID == 0 {
		s.nextTextBubbleLayoutID++
		if s.nextTextBubbleLayoutID == 0 {
			s.nextTextBubbleLayoutID++
		}
		bubble.layoutID = s.nextTextBubbleLayoutID
	}
	s.items = append(s.items, shape)
	clear(s.named)
}

// addClonedShape inserts a clone immediately behind the target it copied. This
// matches Scratch: repeated clones of one target are ordered oldest to newest,
// with the original target still in front of all of them.
func (s *shapeManager) addClonedShape(src, clone Shape) {
	idx := s.findShapeIndex(src)
	if idx < 0 {
		spxlog.Debug("AddClonedShape: cloning a deleted sprite")
		gco.StopCurrent()
		return
	}

	s.items = sliceutil.InsertAt(s.items, idx, clone)
	clear(s.named)
	if sprite, ok := clone.(*SpriteImpl); ok && sprite.IsCloned() {
		s.cloneCount++
	}
	s.updateRenderLayers()
}

// reserveClone includes clones still running initialization code, which can
// yield or create more clones before they enter the active shape list.
func (s *shapeManager) reserveClone() bool {
	if s.cloneCount+s.pendingClones >= maxClones {
		return false
	}

	s.pendingClones++
	return true
}

func (s *shapeManager) removeShape(child Shape) {
	idx := s.findShapeIndex(child)
	if idx < 0 {
		return
	}

	s.items = sliceutil.DeleteAt(s.items, idx)
	clear(s.named)
	if sprite, ok := child.(*SpriteImpl); ok {
		if sprite.IsCloned() {
			s.cloneCount--
		}
		if sprite.runtimeState.SyncSprite != nil {
			s.destroyItems = append(s.destroyItems, sprite)
		}
	}
	s.updateRenderLayers()
}

func (s *shapeManager) activateShape(child Shape) {
	idx := s.findShapeIndex(child)
	if idx < 0 || idx == len(s.items)-1 {
		return
	}

	s.items = sliceutil.MoveToEnd(s.items, idx)
	clear(s.named)
	s.updateRenderLayers()
}

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
	clear(s.named)
	s.updateRenderLayers()
}

func (s *shapeManager) updateRenderLayers() {
	if engine.HasLayerSortMethod() {
		return
	}

	layer := firstSpriteLayer
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok {
			sp.setLayer(layer)
			layer++
		}
	}
}

func (s *shapeManager) all() []Shape {
	return s.items
}

// getTempShapes returns a copy of all active shapes in a temporary buffer.
func (s *shapeManager) getTempShapes() []Shape {
	s.tempItems = sliceutil.CopyInto(s.tempItems, s.items, 50)
	return s.tempItems
}

func (s *shapeManager) count() int {
	return len(s.items)
}

func (s *shapeManager) spritesNamed(name string) []*SpriteImpl {
	if len(s.items) == 0 {
		return nil
	}
	if sprites, ok := s.named[name]; ok {
		return sprites
	}
	var sprites []*SpriteImpl
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok && sp.name == name {
			sprites = append(sprites, sp)
		}
	}
	if s.named == nil {
		s.named = make(map[string][]*SpriteImpl)
	} else if len(s.named) > len(s.items) {
		clear(s.named)
	}
	s.named[name] = sprites
	return sprites
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

func (s *shapeManager) findShapeIndex(target Shape) int {
	return slices.Index(s.items, target)
}

func (s *shapeManager) calculateNewIndex(currentIdx, n int) int {
	direction := -1
	if n < 0 {
		direction = 1
	}
	for next := currentIdx + direction; n != 0 && next >= 0 && next < len(s.items); next += direction {
		currentIdx = next
		if _, ok := s.items[next].(*SpriteImpl); ok {
			n += direction
		}
	}
	return currentIdx
}
