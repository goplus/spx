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
	"reflect"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/time"
)

// ======================== Sprite Utility Functions ========================
// This file contains utility functions for sprite operations,
// including type conversion, boundary calculations, render offset handling,
// collision detection helpers, and configuration parsing.

// -----------------------------------------------------------------------------
// Type Conversion and Reflection Utilities
// -----------------------------------------------------------------------------

// spriteOf extracts the SpriteImpl from a Sprite interface
func spriteOf(sprite Sprite) *SpriteImpl {
	vSpr := reflect.ValueOf(sprite)
	if vSpr.Kind() == reflect.Pointer {
		vSpr = vSpr.Elem()
	}
	if vSpr.Kind() != reflect.Struct {
		return nil
	}
	for i, n := 0, vSpr.NumField(); i < n; i++ {
		fld := vSpr.Field(i)
		if fld.Kind() == reflect.Struct {
			if fld.Type() == reflect.TypeOf(SpriteImpl{}) {
				return fld.Addr().Interface().(*SpriteImpl)
			}
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Boundary and Collision Detection Utilities
// -----------------------------------------------------------------------------

// bounds returns the bounding rectangle of the sprite.
func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if !p.isVisible {
		return nil
	}

	// Calculate base position with render offset
	x, y := p.getXYWithRenderOffset()

	// Calculate dimensions and adjust position for physics trigger offset
	w, h := p.adjustPositionAndGetDimensions(&x, &y)

	return &mathf.Rect2{
		Position: mathf.Vec2{X: x - w*0.5, Y: y - h*0.5},
		Size:     mathf.Vec2{X: w, Y: h},
	}
}

// adjustPositionAndGetDimensions calculates dimensions and adjusts position for physics trigger offset.
func (p *SpriteImpl) adjustPositionAndGetDimensions(x, y *float64) (width, height float64) {
	triggerInfo := p.physics().getTriggerInfo()

	if triggerInfo.Type == physicsColliderNone {
		// Use costume dimensions
		wi, hi := p.costumes[p.costumeIndex].getSize()
		return float64(wi) * p.scale, float64(hi) * p.scale
	}

	// Update auto collider parameters if needed
	if triggerInfo.Type == physicsColliderAuto && p.syncSprite == nil {
		center, size := getCostumeBoundByAlpha(p, false)
		triggerInfo.Pivot = center
		triggerInfo.Params = []float64{size.X, size.Y}
	}

	// Apply trigger pivot offset
	*x += triggerInfo.Pivot.X * p.scale
	*y += triggerInfo.Pivot.Y * p.scale

	// Get dimensions and apply scale
	w, h := triggerInfo.getDimensions()
	return w * p.scale, h * p.scale
}

// Touching checks if sprite is touching:
//   - another sprite (by name or Sprite object)
//   - spx.Mouse
//   - spx.Edge, spx.EdgeLeft, spx.EdgeTop, spx.EdgeRight, spx.EdgeBottom
func (p *SpriteImpl) touching(obj any) bool {
	if !p.isVisible || p.isDying {
		return false
	}
	switch v := obj.(type) {
	case SpriteName:
		if o := p.g.touchingSpriteBy(p, v); o != nil {
			return true
		}
		return false
	case specialObj:
		if v > 0 {
			return p.checkTouchingScreen(int(v)) != 0
		} else if v == Mouse {
			x, y := p.g.getMousePos()
			return p.g.touchingPoint(p, x, y)
		}
	case Sprite:
		return touchingSprite(p, spriteOf(v))
	}
	panic("Touching: unexpected input")
}

// touchingSprite checks if src sprite is touching dst sprite.
func touchingSprite(dst, src *SpriteImpl) bool {
	if !src.isVisible || src.isDying {
		return false
	}
	ret := src.touchingSprite(dst)
	return ret
}

// touchPoint checks if a point touches the sprite.
func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithPoint(p.syncSprite.GetId(), mathf.NewVec2(x, y), true)
}

// touchingColor checks if sprite is touching a specific color.
func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionByColor(p.syncSprite.GetId(), color, colorThreshold, alphaThreshold)
}

// touchingSprite checks if sprite is touching another sprite.
func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if p.syncSprite == nil || dst.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithSprite(p.syncSprite.GetId(), dst.syncSprite.GetId(), alphaThreshold, !enabledPhysics)
}

// checkTouchingScreen checks which edges of the screen the sprite is touching.
func (p *SpriteImpl) checkTouchingScreen(where int) (touching int) {
	if p.syncSprite == nil {
		return 0
	}
	touching = (int)(physicsMgr.CheckTouchedStageBoundaries(p.syncSprite.GetId()))
	return touching & where
}

// checkNearestTouchedBoundary returns the nearest screen boundary that the sprite is touching.
func (p *SpriteImpl) checkNearestTouchedBoundary() int {
	if p.syncSprite == nil {
		return 0
	}
	return (int)(physicsMgr.CheckNearestTouchedStageBoundary(p.syncSprite.GetId()))
}

// ============================================================================
// Monitor and Variable Display Methods
// ============================================================================

func (p *SpriteImpl) HideVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, false)
}

func (p *SpriteImpl) ShowVar(name string) {
	p.g.setStageMonitor(p.name, getVarPrefix+name, true)
}

// -----------------------------------------------------------------------------
// Render Offset and Transformation Utilities
// -----------------------------------------------------------------------------

// getRenderOffset calculates the render offset for the sprite.
func getRenderOffset(p *SpriteImpl) (float64, float64) {
	cs := p.costumes[p.costumeIndex]
	pivot := p.getPivot()
	x, y := -((cs.center.X)/float64(cs.bitmapResolution)+pivot.X)*p.scale,
		((cs.center.Y)/float64(cs.bitmapResolution)-pivot.Y)*p.scale

	// spx's start point is top left, gdspx's start point is center
	// so we should remove the offset to make the pivot point is the same
	w, h := p.getCostumeSize()
	x = x + float64(w)/2*p.scale
	y = y - float64(h)/2*p.scale

	return x, y
}

// getXYWithRenderOffset returns the sprite's XY position with render offset applied.
func (p *SpriteImpl) getXYWithRenderOffset() (x, y float64) {
	x, y = p.getXY()
	ox, oy := getRenderOffset(p)
	return x + ox, y + oy
}

// applyRenderOffset applies render offset to coordinates
func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getRenderOffset(p)
	*cx = *cx + x
	*cy = *cy + y
}

// revertRenderOffset reverts render offset from coordinates
func revertRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getRenderOffset(p)
	*cx = *cx - x
	*cy = *cy - y
}

// getRenderRotationAndScale calculates the render rotation and scale values (scaleX and scaleY).
func getRenderRotationAndScale(p *SpriteImpl) (rotation, scaleX, scaleY float64) {
	transform := p.transform()
	if transform.rotationStyle == None {
		return 0, 1.0, 1.0
	}
	cs := p.costumes[p.costumeIndex]
	rotation = p.Heading() + cs.faceRight
	rotation -= 90
	scaleX = 1.0
	scaleY = 1.0
	if transform.rotationStyle == LeftRight {
		rotation = 0
		isFlip := transform.direction < 0
		if isFlip {
			scaleX = -1.0
		}
	}
	return rotation, scaleX, scaleY
}

// -----------------------------------------------------------------------------
// Costume Boundary Calculation
// -----------------------------------------------------------------------------

// syncGetCostumeBoundByAlpha gets costume boundary by alpha (sync version).
func syncGetCostumeBoundByAlpha(p *SpriteImpl) (mathf.Vec2, mathf.Vec2) {
	return getCostumeBoundByAlpha(p, true)
}

// getCostumeBoundByAlpha gets costume boundary by alpha channel detection.
func getCostumeBoundByAlpha(p *SpriteImpl, isSync bool) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex]
	var rect mathf.Rect2
	// GetBoundFromAlpha is very slow, so we should cache the result
	if cs.isAtlas() {
		rect = p.getCostumeAtlasRegion()
		rect.Position.X = 0
		rect.Position.Y = 0
	} else {
		if cache, ok := cachedBounds_[cs.path]; ok {
			rect = cache
		} else {
			assetPath := engine.ToAssetPath(cs.path)
			if isSync {
				rect = engine.SyncGetBoundFromAlpha(assetPath)
			} else {
				rect = resMgr.GetBoundFromAlpha(assetPath)
			}
		}
		cachedBounds_[cs.path] = rect
	}
	scale := 1 / float64(cs.bitmapResolution)
	// top left
	posX := float64(rect.Position.X) * scale
	posY := float64(rect.Position.Y) * scale
	sizeX := float64(rect.Size.X) * scale
	sizeY := float64(rect.Size.Y) * scale

	w, h := p.getCostumeSize()
	offsetX := float64(posX + sizeX/2 - w/2)
	offsetY := -float64(posY + sizeY/2 - h/2)

	center := mathf.NewVec2(offsetX, offsetY)
	size := mathf.NewVec2(sizeX, sizeY)
	return center, size
}

// ============================================================================
// Time Methods
// ============================================================================

func (pself *SpriteImpl) DeltaTime() float64 {
	return time.DeltaTime()
}

func (pself *SpriteImpl) TimeSinceLevelLoad() float64 {
	return time.TimeSinceLevelLoad()
}
