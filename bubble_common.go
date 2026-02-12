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
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

// -------------------------------------------------------------------------------------
// Common Bubble System - Shared functionality for Say/Think/Quote bubbles
// -------------------------------------------------------------------------------------

// bubbleBase provides common functionality for all bubble types.
type bubbleBase struct {
	sprite  *SpriteImpl
	camera  *cameraImpl
	isDirty bool
}

// checkNeedsUpdate checks if the bubble needs to be refreshed.
func (b *bubbleBase) checkNeedsUpdate() bool {
	if !b.sprite.Visible() {
		return false
	}
	return b.isDirty || b.sprite.isDirty || b.camera.isDirty
}

// getBounds returns the sprite's bounds information.
func (b *bubbleBase) getBounds() (center, size mathf.Vec2) {
	bound := b.sprite.bounds()
	center = bound.Center()
	size = bound.Size
	return
}

// markClean marks the bubble as no longer needing a refresh.
func (b *bubbleBase) markClean() {
	b.isDirty = false
}

// markDirty marks the bubble as needing a refresh.
func (b *bubbleBase) markDirty() {
	b.isDirty = true
}

// waitAndStop is a helper function for waiting and then stopping a bubble.
func waitAndStop(secs float64, stopFunc func()) {
	engine.Wait(secs)
	stopFunc()
}
