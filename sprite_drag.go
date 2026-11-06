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
	pressed        bool
	target         *SpriteImpl
	startView      mathf.Vec2
	lastView       mathf.Vec2
	timeoutPending bool
	pressedAt      time.Time
	offset         mathf.Vec2
	active         bool
}

func (p *SpriteImpl) isDragging() bool {
	return p.g != nil && p.g.inputMgr.drag.target == p && p.g.inputMgr.drag.active
}

// Input (including v1 recordings) is in world space. Freeze its projection
// with the frame: camera following can change the camera before dispatch.
// Viewport pixels need not match browser CSS pixels.
type dragProjection struct {
	world, view mathf.Rect2
}

func (p *inputManager) sampleDragProjection() *dragProjection {
	if p.g.camera == nil {
		return nil
	}
	camera := engine.Managers().CameraMgr
	world, view := camera.GetGlobalCameraRect(), camera.GetViewportRect()
	if world.Size.X == 0 || world.Size.Y == 0 || view.Size.X == 0 || view.Size.Y == 0 {
		return nil
	}
	return &dragProjection{world: world, view: view}
}

func (p *inputManager) dragViewportPoint(point mathf.Vec2) mathf.Vec2 {
	projection := p.frame.dragProjection
	if projection == nil {
		return point
	}
	world, view := projection.world, projection.view
	return mathf.NewVec2((point.X-world.Position.X)*view.Size.X/world.Size.X,
		(-point.Y-world.Position.Y)*view.Size.Y/world.Size.Y)
}

// Resolve a screen-space point against a camera projection.
func (projection *dragProjection) worldPoint(point mathf.Vec2) mathf.Vec2 {
	if projection == nil {
		return point
	}
	world, view := projection.world, projection.view
	return mathf.NewVec2(point.X*world.Size.X/view.Size.X+world.Position.X,
		-(point.Y*world.Size.Y/view.Size.Y + world.Position.Y))
}

// Drag picking filters candidates before hit testing, unlike ordinary clicks.
func (p *Game) findDragTarget(point mathf.Vec2) *SpriteImpl {
	for _, item := range slices.Backward(p.shapeMgr.getTempShapes()) {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.spriteState.isDraggable && p.pointHitsClickTarget(sprite, point) {
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
	if !drag.pressed && sprite == nil {
		return
	}
	if sprite != nil && (!sprite.spriteState.isDraggable || !sprite.Visible() || sprite.isDestroyed()) {
		p.cancelDrag(sprite)
		return
	}
	if !drag.active {
		view := p.dragViewportPoint(point)
		// Ignore projection roundoff, not scene movement or frame timing.
		moved := !view.Sub(drag.lastView).IsApproximatelyZero()
		drag.lastView = view
		distance := view.Sub(drag.startView)
		movementTrigger := moved && distance.LengthSquared() > dragDistance*dragDistance
		// Consume the one-shot timeout even if picking fails. Later pointer
		// movement can retry; scene changes alone must not capture a sprite.
		if !movementTrigger && (!drag.timeoutPending || p.g.inputClock().Sub(drag.pressedAt) < dragDelay-time.Microsecond) {
			return
		}
		drag.timeoutPending = false
		// Picking sees the scene at activation, not the scene at sampling.
		currentProjection := p.sampleDragProjection()
		start := currentProjection.worldPoint(drag.startView)
		sprite = p.g.findDragTarget(start)
		if sprite == nil {
			return
		}
		x, y := sprite.getXY()
		drag.target = sprite
		drag.offset = mathf.NewVec2(x-start.X, y-start.Y)
		point = currentProjection.worldPoint(view)
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
