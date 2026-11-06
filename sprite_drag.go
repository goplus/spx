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
	"time"

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

const (
	dragDistance = 3 // Viewport pixels.
	dragDelay    = 400 * time.Millisecond
)

type spriteDragState struct {
	pressed   bool
	target    *SpriteImpl
	start     mathf.Vec2
	startView mathf.Vec2
	pressedAt time.Time
	offset    mathf.Vec2
	active    bool
}

func (p *SpriteImpl) isDragging() bool {
	return p.g != nil && p.g.inputMgr.drag.target == p && p.g.inputMgr.drag.active
}

// Input (including recordings) is in world space. Project it into the viewport
// using the actual camera rect, so camera movement and smoothing do not count
// as pointer movement. Viewport pixels need not match browser CSS pixels.
func (p *inputManager) dragViewportPoint(point mathf.Vec2) mathf.Vec2 {
	if p.g.camera == nil {
		return point
	}
	camera := engine.Managers().CameraMgr
	world, view := camera.GetGlobalCameraRect(), camera.GetViewportRect()
	if world.Size.X == 0 || world.Size.Y == 0 {
		return point
	}
	// Camera rectangles use Godot's downward-positive Y, unlike SPX points.
	return mathf.Vec2{
		X: (point.X - world.Position.X) * view.Size.X / world.Size.X,
		Y: (-point.Y - world.Position.Y) * view.Size.Y / world.Size.Y,
	}
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
	if !drag.pressed && sprite == nil {
		return
	}
	if sprite != nil && (!sprite.spriteState.isDraggable || !sprite.Visible() || sprite.isDestroyed()) {
		p.cancelDrag(sprite)
		return
	}
	if !drag.active {
		delta := p.dragViewportPoint(point).Sub(drag.startView)
		// A microsecond tolerance keeps accumulated v1 replay timestamps from
		// delaying the 400 ms boundary by an entire frame.
		if delta.LengthSquared() <= dragDistance*dragDistance && p.g.inputClock().Sub(drag.pressedAt) < dragDelay-time.Microsecond {
			return
		}
		// Scripts may have moved or reordered sprites while the press waited.
		sprite, _ = p.g.findClickTarget(drag.start).(*SpriteImpl)
		if sprite == nil || !sprite.spriteState.isDraggable {
			return
		}
		x, y := sprite.getXY()
		drag.target = sprite
		drag.offset = mathf.NewVec2(x-drag.start.X, y-drag.start.Y)
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
		pen.needsPositionReset = true
	}
	transform.writePosition(x, y, engine.PositionXY)
}
