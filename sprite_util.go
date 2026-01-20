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

// bounds returns the bounding rectangle of the sprite
func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if !p.isVisible {
		return nil
	}
	x, y, w, h := 0.0, 0.0, 0.0, 0.0
	c := p.costumes[p.costumeIndex_]
	// calc center
	x, y = p.getXY()
	applyRenderOffset(p, &x, &y)

	triggerInfo := p.physics().getTriggerInfo()
	if triggerInfo.Type != physicsColliderNone {
		if triggerInfo.Type == physicsColliderAuto && p.syncSprite == nil {
			// if sprite's proxy is not created, use the sync version to get the bound
			center, size := getCostumeBoundByAlpha(p, p.scale, false)
			// Update sprite state atomically to prevent race conditions
			triggerInfo.Pivot = center
			triggerInfo.Params = []float64{size.X, size.Y}
		}
		x += triggerInfo.Pivot.X
		y += triggerInfo.Pivot.Y
		// Calculate dimensions from triggerShape based on type
		w, h = triggerInfo.getDimensions()
	} else {
		// calc scale
		wi, hi := c.getSize()
		w, h = float64(wi)*p.scale, float64(hi)*p.scale
	}

	rect := mathf.NewRect2(x-w*0.5, y-h*0.5, w, h)
	return &rect
}

// touchingSprite checks if src sprite is touching dst sprite
func touchingSprite(dst, src *SpriteImpl) bool {
	if !src.isVisible || src.isDying {
		return false
	}
	ret := src.touchingSprite(dst)
	return ret
}

// touchPoint checks if a point touches the sprite
func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithPoint(p.syncSprite.GetId(), mathf.NewVec2(x, y), true)
}

// touchingColor checks if sprite is touching a specific color
func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if p.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionByColor(p.syncSprite.GetId(), color, colorThreshold, alphaThreshold)
}

// touchingSprite checks if sprite is touching another sprite
func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if p.syncSprite == nil || dst.syncSprite == nil {
		return false
	}
	return spriteMgr.CheckCollisionWithSprite(p.syncSprite.GetId(), dst.syncSprite.GetId(), alphaThreshold, !enabledPhysics)
}

// checkTouchingScreen checks which edges of the screen the sprite is touching
func (p *SpriteImpl) checkTouchingScreen(where int) (touching int) {
	if p.syncSprite == nil {
		return 0
	}
	touching = (int)(physicsMgr.CheckTouchedCameraBoundaries(p.syncSprite.GetId()))
	return touching & where
}

// checkNearestTouchedBoundary returns the nearest screen boundary that the sprite is touching.
func (p *SpriteImpl) checkNearestTouchedBoundary() int {
	if p.syncSprite == nil {
		return 0
	}
	return (int)(physicsMgr.CheckNearestTouchedCameraBoundary(p.syncSprite.GetId()))
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

// getRenderOffset calculates the render offset for the sprite
func getRenderOffset(p *SpriteImpl) (float64, float64) {
	cs := p.costumes[p.costumeIndex_]
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

// calcRenderRotation calculates the render rotation and horizontal scale
func calcRenderRotation(p *SpriteImpl) (float64, float64) {
	transform := p.transform()
	if transform.rotationStyle == None {
		return 0, 1.0
	}
	cs := p.costumes[p.costumeIndex_]
	degree := p.Heading() + cs.faceRight
	degree -= 90
	hScale := 1.0
	if transform.rotationStyle == LeftRight {
		degree = 0
		isFlip := transform.direction < 0
		if isFlip {
			hScale = -1.0
		}
	}
	return degree, hScale
}

// -----------------------------------------------------------------------------
// Costume Boundary Calculation
// -----------------------------------------------------------------------------

// syncGetCostumeBoundByAlpha gets costume boundary by alpha (sync version)
func syncGetCostumeBoundByAlpha(p *SpriteImpl, pscale float64) (mathf.Vec2, mathf.Vec2) {
	return getCostumeBoundByAlpha(p, pscale, true)
}

// getCostumeBoundByAlpha gets costume boundary by alpha channel detection
func getCostumeBoundByAlpha(p *SpriteImpl, pscale float64, isSync bool) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex_]
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
	scale := pscale / float64(cs.bitmapResolution)
	// top left
	posX := float64(rect.Position.X) * scale
	posY := float64(rect.Position.Y) * scale
	sizeX := float64(rect.Size.X) * scale
	sizeY := float64(rect.Size.Y) * scale

	w, h := p.getCostumeSize()
	w, h = w*pscale, h*pscale
	offsetX := float64(posX + sizeX/2 - w/2)
	offsetY := -float64(posY + sizeY/2 - h/2)

	center := mathf.NewVec2(offsetX, offsetY)
	size := mathf.NewVec2(sizeX, sizeY)
	return center, size
}

// -----------------------------------------------------------------------------
// Configuration Parsing Utilities
// -----------------------------------------------------------------------------

// parseDefaultValue parses int64 value with default
func parseDefaultValue(pval *int64, defaultValue int64) int64 {
	if pval == nil {
		return defaultValue
	}
	return *pval
}

// parseDefaultFloatValue parses float64 value with default
func parseDefaultFloatValue(pval *float64, defaultValue float64) float64 {
	if pval == nil {
		return defaultValue
	}
	return *pval
}

// parseLayerMaskValue parses layer mask value
func parseLayerMaskValue(pval *int64) int64 {
	return parseDefaultValue(pval, 1)
}

// parseColliderShapeType parses collider shape type from string
func parseColliderShapeType(typeName string, defaultValue int64) int64 {
	switch typeName {
	case "none":
		return physicsColliderNone
	case "auto":
		return physicsColliderAuto
	case "circle":
		return physicsColliderCircle
	case "rect":
		return physicsColliderRect
	case "capsule":
		return physicsColliderCapsule
	case "polygon":
		return physicsColliderPolygon
	}
	return defaultValue
}

// toRotationStyle converts a string representation to a RotationStyle constant
func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	}
	return Normal
}
