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

const (
	dragDistance = 3 // Viewport pixels.
	dragDelay    = 400 * time.Millisecond
)

type spriteDragState struct {
	pressed       bool
	clickDeferred bool        // The clicked sprite was draggable, so it did not click on mouse down.
	target        *SpriteImpl // Captured on mouse down; never replaced during this press.
	startView     mathf.Vec2
	pressedAt     time.Time
	offset        mathf.Vec2
	active        bool
}

// Input (including v1 recordings) is in world space. Freeze its projection
// with the frame: camera following can change the camera before dispatch.
// Viewport pixels need not match browser CSS pixels.
type dragProjection struct {
	world, view mathf.Rect2
}

// SetDragMode controls whether the user can drag this sprite with the left mouse
// button. Draggable enables dragging; NotDraggable (or any other value) disables
// it and ends a pending or active drag.
func (p *SpriteImpl) SetDragMode(mode DragMode) {
	p.spriteState.isDraggable = mode == Draggable
	if !p.spriteState.isDraggable && p.g != nil {
		p.g.inputMgr.cancelDrag(p)
	}
}

func (p *SpriteImpl) isDragging() bool {
	return p.g != nil && p.g.inputMgr.drag.target == p && p.g.inputMgr.drag.active
}

// Drag picking filters candidates before hit testing, unlike ordinary clicks.
// It runs only on mouse down: an empty press cannot acquire a target later.
func (p *Game) findDragTarget(point mathf.Vec2) *SpriteImpl {
	for _, item := range slices.Backward(p.shapeMgr.getTempShapes()) {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.spriteState.isDraggable && p.pointHitsClickTarget(sprite, point) {
			return sprite
		}
	}
	return nil
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

func (p *inputManager) cancelDrag(sprite *SpriteImpl) {
	if p.drag.target == sprite {
		// Cancelling a pending background candidate must not cancel the
		// ordinary foreground click. Clear the candidate so it cannot revive.
		pressed := p.drag.pressed && !p.drag.active && !p.drag.clickDeferred
		p.drag = spriteDragState{pressed: pressed}
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
	view := p.frame.dragProjection.viewportPoint(point)
	distance := view.Sub(drag.startView)
	// Keep v1 timestamp tolerance when checking the activation threshold.
	if !drag.active && distance.LengthSquared() <= dragDistance*dragDistance && p.g.inputClock().Sub(drag.pressedAt) < dragDelay-time.Microsecond {
		return
	}
	// Account for camera changes after sampling, including on release.
	projection := p.sampleDragProjection()
	if (p.frame.dragProjection == nil) != (projection == nil) {
		return // Do not mix world and viewport coordinates.
	}
	point = projection.worldPoint(view)
	if !drag.active {
		// Rebase the grab offset after scene changes while waiting.
		start := projection.worldPoint(drag.startView)
		x, y := sprite.getXY()
		drag.offset = mathf.NewVec2(x-start.X, y-start.Y)
		drag.active = true
		p.swipe.Finish(point) // A drag takes over any pending foreground swipe.
		sprite.SetLayerTo(Front)
	}
	transform := sprite.transform()
	x, y := transform.fixWorldRange(point.X+drag.offset.X, point.Y+drag.offset.Y)
	if x == transform.x && y == transform.y {
		return
	}
	// Resume drawing from the drop position on the next scripted move.
	if pen := sprite.pen(); pen != nil && pen.isPenDown {
		pen.needsPositionReset = true
	}
	transform.writePosition(x, y, engine.PositionXY)
}

func (projection *dragProjection) viewportPoint(point mathf.Vec2) mathf.Vec2 {
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
