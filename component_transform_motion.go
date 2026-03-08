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
	"math"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// MoveForward moves the sprite forward by the specified step size
// in the direction it's currently facing.
func (t *transformComponent) MoveForward(step float64) {
	sin, cos := math.Sincos(toRadian(t.direction))
	t.moveTo(t.x+step*sin, t.y+step*cos)
}

// Glide moves the sprite smoothly to the specified position over the given duration.
func (t *transformComponent) Glide(x, y float64, secs float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Glide: sprite=%s, x=%v, y=%v, secs=%v", t.sprite.name, x, y, secs)
	}

	x0, y0 := t.XY()
	from := mathf.NewVec2(x0, y0)
	to := mathf.NewVec2(x, y)

	aniCopy := aniConfig{
		Duration: secs,
		From:     &from,
		To:       &to,
		AniType:  aniTypeGlide,
		IsLoop:   true,
	}

	animName := t.sprite.getStateAnimName(StateGlide)
	t.sprite.animation().doTween(animName, &aniCopy)
}

// GlideTo moves the sprite smoothly to the specified object's position
// over the given duration.
func (t *transformComponent) GlideTo(obj any, secs float64) {
	x, y := t.sprite.g.objectPos(obj)
	t.Glide(x, y, secs)
}

// Step moves the sprite forward by the specified step size using a stepping animation.
func (t *transformComponent) Step(step, speed float64, animation SpriteAnimationName) {
	dirSin, dirCos := math.Sincos(toRadian(t.direction))
	diff := mathf.NewVec2(step*dirSin, step*dirCos)
	to := mathf.NewVec2(t.x, t.y).Add(diff)
	t.StepToPos(to.X, to.Y, speed, animation)
}

// StepToPos moves the sprite to the specified position using a stepping animation.
func (t *transformComponent) StepToPos(x, y, speed float64, animation SpriteAnimationName) {
	if animation == "" {
		animation = t.sprite.getStateAnimName(StateStep)
	}

	// If no animation exists, move to target immediately.
	if !t.sprite.hasAnim(animation) {
		t.SetXYpos(x, y)
		return
	}

	from := mathf.NewVec2(t.x, t.y)
	to := mathf.NewVec2(x, y)
	distance := from.DistanceTo(to)

	ani, ok := t.sprite.getAnimation(animation)
	if !ok {
		return
	}

	duration := math.Abs(distance) * ani.StepDuration / math.Max(speed, minSpeed)
	t.doAnimatedTween(animation, ani, &from, &to, aniTypeMove, duration, speed)
}

// StepTo moves the sprite to the specified object's position using a stepping animation.
func (t *transformComponent) StepTo(obj any, speed float64, animation SpriteAnimationName) {
	x, y := t.sprite.g.objectPos(obj)
	t.StepToPos(x, y, speed, animation)
}

// SetSize sets the sprite's scale to the specified size.
func (t *transformComponent) SetSize(size float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("SetSize: sprite=%s, size=%v", t.sprite.name, size)
	}

	t.sprite.scale = size
	t.sprite.isCostumeDirty = true
	t.markDirty()
	t.sprite.updatePhysicsShapesScale()
}

// ChangeSize changes the sprite's scale by the specified delta.
func (t *transformComponent) ChangeSize(delta float64) {
	if isDebugInstrEnabled() {
		spxlog.Debug("ChangeSize: sprite=%s, delta=%v", t.sprite.name, delta)
	}
	t.SetSize(t.sprite.scale + delta)
}
