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
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/engine"
)

const (
	dragDistance = 3 // Viewport pixels.
	dragDelay    = 400 * time.Millisecond
)

const mouseClickInterval = 50 * time.Millisecond

const (
	clickTimerGlobal = -1
	clickTimerStage  = 0
)

// Click Dispatch
type clicker interface {
	doWhenClick()
	getProxy() *engine.Sprite
	Visible() bool
}

type spriteDragState struct {
	pressed       bool
	clickDeferred bool        // Click target was draggable on mouse down.
	target        *SpriteImpl // Fixed for this press.
	startView     mathf.Vec2
	pressedAt     time.Time
	offset        mathf.Vec2
	active        bool
}

// dragProjection preserves the input frame's world-to-viewport mapping.
type dragProjection struct {
	world, view mathf.Rect2
}

func (p *inputManager) onLeftButtonDown(point mathf.Vec2) {
	target := p.g.findClickTarget(point)
	sprite, _ := target.(*SpriteImpl)
	view := p.frame.dragProjection.viewportPoint(point)
	p.drag = spriteDragState{
		pressed:       true,
		clickDeferred: sprite != nil && sprite.spriteState.isDraggable,
		target:        p.g.findDragTarget(point),
		startView:     view,
		pressedAt:     p.g.inputClock(),
	}
	if p.drag.clickDeferred {
		p.swipe.Finish(point)
		return
	}
	p.swipe.Begin(point, sprite)
	p.click(target)
}

func (p *inputManager) onLeftButtonUp(point mathf.Vec2) {
	p.updateDrag(point)
	drag := p.drag
	p.drag = spriteDragState{}
	result, target, ok := p.swipe.Finish(point)
	if drag.active {
		return // Dragging suppresses clicks and swipes.
	}
	if drag.pressed {
		// Pick the release target independently of the drag candidate.
		if released, ok := p.g.findClickTarget(point).(*SpriteImpl); ok && released.spriteState.isDraggable {
			p.click(released)
		}
	}
	if !ok {
		return
	}
	if isDebugEventEnabled() {
		targetName := "stage"
		if target != nil {
			targetName = target.name
		}
		spxlog.Debug("Swipe detected: direction=%v, velocity=%.2f, distance=%.2f, target=%s",
			Direction(result.Direction), result.Velocity, result.Distance, targetName)
	}
	if target != nil {
		target.doWhenSwipe(Direction(result.Direction))
	} else {
		p.g.doWhenSwipe(Direction(result.Direction))
	}
}

func (p *inputManager) click(target clicker) {
	if !p.clickGate.Allow(clickTimerGlobal) {
		return
	}
	if target != nil {
		proxy := target.getProxy()
		if proxy == nil || !p.clickGate.Allow(proxy.GetId()) {
			return
		}
		target.doWhenClick()
		return
	}
	if p.clickGate.Allow(clickTimerStage) {
		p.g.doWhenClick()
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
	// Allow for v1 replay timestamp rounding.
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
		p.swipe.Finish(point) // Drag takes over the foreground swipe.
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

func (p *inputManager) cancelDrag(sprite *SpriteImpl) {
	if p.drag.target == sprite {
		// Preserve foreground clicks when cancelling a pending background drag.
		pressed := p.drag.pressed && !p.drag.active && !p.drag.clickDeferred
		p.drag = spriteDragState{pressed: pressed}
	}
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

func (p *Game) findClickTarget(point mathf.Vec2) clicker {
	for _, item := range slices.Backward(p.scene.getTempShapes()) {
		if target, ok := item.(clicker); ok && p.pointHitsClickTarget(target, point) {
			return target
		}
	}
	return nil
}

// Drag picking ignores non-draggable foreground sprites.
func (p *Game) findDragTarget(point mathf.Vec2) *SpriteImpl {
	for _, item := range slices.Backward(p.scene.getTempShapes()) {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.spriteState.isDraggable && p.pointHitsClickTarget(sprite, point) {
			return sprite
		}
	}
	return nil
}

func (p *Game) pointHitsClickTarget(target clicker, point mathf.Vec2) bool {
	proxy := target.getProxy()
	if proxy == nil || !target.Visible() {
		return false
	}

	sprite, ok := target.(*SpriteImpl)
	if ok {
		if sprite.isDestroyed() {
			return false
		}
		sprite.syncProxyForQuery()
		if sprite.isFullyGhosted() {
			return false
		}
	}

	return engine.Managers().SpriteMgr.CheckCollisionWithPoint(proxy.GetId(), point, true)
}

func (d *dragProjection) viewportPoint(point mathf.Vec2) mathf.Vec2 {
	if d == nil {
		return point
	}
	world, view := d.world, d.view
	return mathf.NewVec2((point.X-world.Position.X)*view.Size.X/world.Size.X,
		(-point.Y-world.Position.Y)*view.Size.Y/world.Size.Y)
}

func (d *dragProjection) worldPoint(point mathf.Vec2) mathf.Vec2 {
	if d == nil {
		return point
	}
	world, view := d.world, d.view
	return mathf.NewVec2(point.X*world.Size.X/view.Size.X+world.Position.X,
		-(point.Y*world.Size.Y/view.Size.Y + world.Position.Y))
}
