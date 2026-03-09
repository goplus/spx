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

	"github.com/goplus/spx/v2/internal/base/valueutil"
)

// XY returns the current position coordinates.
func (t *transformComponent) XY() (x, y float64) {
	return t.x, t.y
}

// Xpos returns the current x-coordinate.
func (t *transformComponent) Xpos() float64 {
	return t.x
}

// Ypos returns the current y-coordinate.
func (t *transformComponent) Ypos() float64 {
	return t.y
}

// SetXYpos sets both position coordinates to the specified values.
func (t *transformComponent) SetXYpos(x, y float64) {
	t.moveTo(x, y)
}

// SetXpos sets the x-coordinate to the specified value.
func (t *transformComponent) SetXpos(x float64) {
	t.moveTo(x, t.y)
}

// SetYpos sets the y-coordinate to the specified value.
func (t *transformComponent) SetYpos(y float64) {
	t.moveTo(t.x, y)
}

// ChangeXYpos changes both position coordinates by the specified deltas.
func (t *transformComponent) ChangeXYpos(dx, dy float64) {
	t.moveTo(t.x+dx, t.y+dy)
}

// ChangeXpos changes the x-coordinate by the specified delta.
func (t *transformComponent) ChangeXpos(dx float64) {
	t.moveTo(t.x+dx, t.y)
}

// ChangeYpos changes the y-coordinate by the specified delta.
func (t *transformComponent) ChangeYpos(dy float64) {
	t.moveTo(t.x, t.y+dy)
}

// DistanceTo calculates the Euclidean distance from this sprite to the specified object.
func (t *transformComponent) DistanceTo(obj any) float64 {
	x, y := t.x, t.y
	x2, y2 := t.sprite.g.objectPos(obj)
	dx := x - x2
	dy := y - y2
	return math.Sqrt(dx*dx + dy*dy)
}

// setXY sets position directly without triggering side effects.
// This is used for internal operations that need to bypass normal movement logic.
func (t *transformComponent) setXY(x, y float64) {
	t.x, t.y = x, y
}

// moveTo moves the sprite to the specified position, handling pen movement
// and transform updates.
func (t *transformComponent) moveTo(x, y float64) {
	x, y = t.fixWorldRange(x, y)
	t.sprite.movePen(x, y)
	t.x, t.y = x, y
	t.markDirty()
}

// fixWorldRange clamps sprite position within world boundaries.
func (t *transformComponent) fixWorldRange(x, y float64) (float64, float64) {
	rect := t.sprite.bounds()
	if rect == nil {
		return x, y
	}

	worldW, worldH := t.sprite.g.worldSize()
	maxW := float64(worldW)/2.0 + float64(rect.Size.X)
	maxH := float64(worldH)/2.0 + float64(rect.Size.Y)

	x = valueutil.ClampFloat64(x, -maxW, maxW)
	y = valueutil.ClampFloat64(y, -maxH, maxH)

	return x, y
}
