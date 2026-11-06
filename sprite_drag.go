/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
)

// SetDragMode controls whether the user can drag this sprite with the left mouse
// button. Draggable enables dragging; NotDraggable (or any other value) disables
// it and ends a pending or active drag.
func (p *SpriteImpl) SetDragMode(mode DragMode) {
	p.spriteState.isDraggable = mode == Draggable
	if !p.spriteState.isDraggable && p.g != nil {
		p.g.inputMgr.cancelDrag(p)
	}
}

type spriteDragState struct {
	target *SpriteImpl
	start  mathf.Vec2
	offset mathf.Vec2
	active bool
	click  bool
}

func (p *SpriteImpl) isDragging() bool {
	return p.g != nil && p.g.inputMgr.drag.target == p && p.g.inputMgr.drag.active
}

func (p *Game) findDragTarget(point mathf.Vec2) *SpriteImpl {
	for _, item := range slices.Backward(p.shapeMgr.getTempShapes()) {
		sprite, ok := item.(*SpriteImpl)
		if ok && sprite.spriteState.isDraggable && !sprite.isDestroyed() && p.pointHitsClickTarget(sprite, point) {
			return sprite
		}
	}
	return nil
}

func (p *inputManager) cancelDrag(sprite *SpriteImpl) {
	if p.drag.target == sprite {
		p.drag = spriteDragState{}
		p.swipe.Finish(p.mousePos)
	}
}

func (p *inputManager) updateDrag(point mathf.Vec2) {
	drag := &p.drag
	sprite := drag.target
	if sprite == nil {
		return
	}
	if !sprite.spriteState.isDraggable || !sprite.Visible() || sprite.isDestroyed() {
		p.cancelDrag(sprite)
		return
	}
	if !drag.active {
		if point == drag.start {
			return
		}
		drag.active = true
		sprite.SetLayerTo(Front)
	}
	transform := sprite.transform()
	x, y := transform.fixWorldRange(point.X+drag.offset.X, point.Y+drag.offset.Y)
	if x == transform.x && y == transform.y {
		return
	}
	// Dragging bypasses script movement and pen drawing while still publishing
	// the position through the normal versioned proxy synchronization path.
	if pen := sprite.pen(); pen != nil && pen.isPenDown {
		pen.positionReset = true
	}
	transform.writePosition(x, y, engine.PositionXY)
}
