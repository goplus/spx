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
	"slices"

	"github.com/goplus/spx/v4/internal/base"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/engine"
)

// Scratch renders the shared pen canvas above the backdrop and below every
// managed sprite. Keep the pen layer explicit so sprite ordering cannot reuse it.
const (
	penLayer         = 0
	firstSpriteLayer = penLayer + 1
)

// Scratch shares one limit across all sprites, including hidden clones.
const maxClones = 300

type sceneObjects struct {
	proxyQueue
	bubbleLayout
	layersDirty   bool
	cloneCount    int
	pendingClones int
	items         []Shape
	named         map[string][]*SpriteImpl
	tempItems     []Shape
}

// GetWidget finds a named widget or panics. The interpreter calls this
// non-generic helper directly from the XGo widget hook.
func GetWidget(sg ShapeGetter, name WidgetName) Widget {
	items := sg.getAllShapes()
	for _, item := range items {
		widget, ok := item.(Widget)
		if !ok {
			continue
		}
		if widget.GetName() == name {
			return widget
		}
	}
	panic("GetWidget: widget not found - " + name)
}

// XGot_Game_XGox_GetWidget implements typed widget lookup for XGo.
func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}

// Target.
// GetTarget returns the implementation of the first active, non-cloned sprite
// with the specified name. It returns nil if no matching sprite exists.
func (p *Game) GetTarget(target string) *SpriteImpl {
	return p.scene.findSprite(target)
}

func (p *Game) getAllShapes() []Shape {
	return p.scene.all()
}

func (p *SpriteImpl) getAllShapes() []Shape {
	return p.g.scene.all()
}

func (s *sceneObjects) init() {
	s.layersDirty = false
	s.cloneCount = 0
	s.pendingClones = 0
	s.named = nil
	s.items = base.Reset(s.items, 64)
	s.tempItems = base.Reset(s.tempItems, 50)
	s.proxyQueue.reset()
	s.bubbleLayout.reset()
}

func (s *sceneObjects) reset() {
	engine.ClearAllSprites()
	s.init()
}

func (s *sceneObjects) add(shape Shape) {
	if sprite, ok := shape.(*SpriteImpl); ok {
		s.layersDirty = true
		if sprite.IsCloned() {
			s.cloneCount++
		}
		s.trackExistingDirtySprite(sprite)
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
func (s *sceneObjects) addClonedShape(src, clone Shape) {
	idx := s.findShapeIndex(src)
	if idx < 0 {
		spxlog.Debug("AddClonedShape: cloning a deleted sprite")
		scheduler.StopCurrent()
		return
	}

	s.items = slices.Insert(s.items, idx, clone)
	clear(s.named)
	if sprite, ok := clone.(*SpriteImpl); ok && sprite.IsCloned() {
		s.cloneCount++
		s.trackExistingDirtySprite(sprite)
	}
	s.updateRenderLayers()
}

// reserveClone includes clones still running initialization code, which can
// yield or create more clones before they enter the active shape list.
func (s *sceneObjects) reserveClone() bool {
	if s.cloneCount+s.pendingClones >= maxClones {
		return false
	}

	s.pendingClones++
	return true
}

func (s *sceneObjects) removeShape(child Shape) {
	idx := s.findShapeIndex(child)
	if idx < 0 {
		return
	}

	s.items = slices.Delete(s.items, idx, idx+1)
	clear(s.named)
	if sprite, ok := child.(*SpriteImpl); ok {
		// Keep deletion dirty until its native destroy batch commits.
		if sprite.IsCloned() {
			s.cloneCount--
		}
		if sprite.runtimeState.proxy != nil {
			s.destroyItems = append(s.destroyItems, sprite)
		}
	}
	s.updateRenderLayers()
}

func (s *sceneObjects) activateShape(child Shape) {
	idx := s.findShapeIndex(child)
	if idx < 0 || idx == len(s.items)-1 {
		return
	}

	s.items = base.MoveToIndex(s.items, idx, len(s.items)-1)
	clear(s.named)
	s.updateRenderLayers()
}

func (s *sceneObjects) goBackLayers(spr *SpriteImpl, n int) {
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

	s.items = base.MoveToIndex(s.items, idx, newIdx)
	clear(s.named)
	if s.layersDirty {
		s.updateRenderLayers()
	} else {
		s.updateRenderLayerRange(min(idx, newIdx), max(idx, newIdx)+1)
	}
}

func (s *sceneObjects) updateRenderLayers() {
	// Automatic sorting may defer renumbering while shapes are added or removed.
	s.layersDirty = true
	if engine.HasLayerSortMethod() {
		return
	}

	s.updateRenderLayerRange(0, len(s.items))
	s.layersDirty = false
}

// Layer moves affect only the crossed sprites. The unchanged prefix supplies
// the first layer; non-sprite shapes do not occupy layers.
func (s *sceneObjects) updateRenderLayerRange(start, end int) {
	layer := firstSpriteLayer
	for i := start - 1; i >= 0; i-- {
		if sprite, ok := s.items[i].(*SpriteImpl); ok {
			layer = sprite.runtimeState.layer + 1
			break
		}
	}
	for _, item := range s.items[start:end] {
		if sp, ok := item.(*SpriteImpl); ok {
			sp.setLayer(layer)
			layer++
		}
	}
}

func (s *sceneObjects) all() []Shape {
	return s.items
}

// getTempShapes returns a copy of all active shapes in a temporary buffer.
func (s *sceneObjects) getTempShapes() []Shape {
	s.tempItems = base.CopyInto(s.tempItems, s.items, 50)
	return s.tempItems
}

func (s *sceneObjects) count() int {
	return len(s.items)
}

func (s *sceneObjects) spritesNamed(name string) []*SpriteImpl {
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
func (s *sceneObjects) findSprite(name SpriteName) *SpriteImpl {
	for _, item := range s.items {
		if sp, ok := item.(*SpriteImpl); ok {
			if !sp.spriteState.cloned && sp.name == name {
				return sp
			}
		}
	}
	return nil
}

func (s *sceneObjects) findShapeIndex(target Shape) int {
	return slices.Index(s.items, target)
}

func (s *sceneObjects) calculateNewIndex(currentIdx, n int) int {
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
