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

import "github.com/goplus/spbase/mathf"

func (p *SpriteImpl) bounds() *mathf.Rect2 {
	if !p.spriteState.IsVisible {
		return nil
	}

	x, y := p.getXYWithRenderOffset()
	w, h := p.adjustPositionAndGetDimensions(&x, &y)

	return &mathf.Rect2{
		Position: mathf.Vec2{X: x - w*0.5, Y: y - h*0.5},
		Size:     mathf.Vec2{X: w, Y: h},
	}
}

func (p *SpriteImpl) adjustPositionAndGetDimensions(x, y *float64) (width, height float64) {
	triggerInfo := p.physics().getTriggerInfo()

	if triggerInfo.Type == physicsColliderNone {
		wi, hi := p.costumes[p.costumeIndex].getSize()
		return float64(wi) * p.runtimeState.Scale, float64(hi) * p.runtimeState.Scale
	}

	if triggerInfo.Type == physicsColliderAuto && p.runtimeState.SyncSprite == nil {
		center, size := getCostumeBoundByAlpha(p, false)
		triggerInfo.Pivot = center
		triggerInfo.Params = []float64{size.X, size.Y}
	}

	*x += triggerInfo.Pivot.X * p.runtimeState.Scale
	*y += triggerInfo.Pivot.Y * p.runtimeState.Scale

	w, h := triggerInfo.getDimensions()
	return w * p.runtimeState.Scale, h * p.runtimeState.Scale
}

func (p *SpriteImpl) TouchingColor__0(color Color) bool {
	return p.touchingColor(toMathfColor(color))
}

func (p *SpriteImpl) TouchingColor__1(spriteColor, targetColor Color) bool {
	return p.touchingColors(toMathfColor(spriteColor), toMathfColor(targetColor))
}

func (p *SpriteImpl) Touching__0(sprite Sprite) bool {
	return p.touching(sprite, edgeAreaStage)
}

func (p *SpriteImpl) Touching__1(sprite SpriteName) bool {
	return p.touching(sprite, edgeAreaStage)
}

func (p *SpriteImpl) Touching__2(obj specialObj) bool {
	return p.touching(obj, edgeAreaStage)
}

func (p *SpriteImpl) TouchingWith(target Target) bool {
	return p.touching(target, edgeAreaStage)
}

func (p *SpriteImpl) touching(obj Target, area string) bool {
	if !p.spriteState.IsVisible || p.spriteState.IsDying {
		return false
	}
	switch v := obj.(type) {
	case SpriteName:
		return p.g.touchingSpriteBy(p, v) != nil
	case specialObj:
		if v > 0 {
			return p.checkTouchingScreen(int(v), area) != 0
		}
		if v == Mouse {
			x, y := p.g.getMousePos()
			return p.g.touchingPoint(p, x, y)
		}
	case Sprite:
		return touchingSprite(p, spriteOf(v))
	}
	panic("Touching: unexpected input")
}

func touchingSprite(dst, src *SpriteImpl) bool {
	if !src.spriteState.IsVisible || src.spriteState.IsDying {
		return false
	}
	return src.touchingSprite(dst)
}

func (p *SpriteImpl) touchPoint(x, y float64) bool {
	if p.runtimeState.SyncSprite == nil {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionWithPoint(p.runtimeState.SyncSprite.GetId(), mathf.NewVec2(x, y), true)
}

func (p *SpriteImpl) touchingColor(color mathf.Color) bool {
	if p.runtimeState.SyncSprite == nil {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionByColor(p.runtimeState.SyncSprite.GetId(), color, colorThreshold, alphaThreshold)
}

func (p *SpriteImpl) touchingColors(spriteColor, targetColor mathf.Color) bool {
	if p.runtimeState.SyncSprite == nil {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionByColors(
		p.runtimeState.SyncSprite.GetId(),
		spriteColor,
		targetColor,
		colorThreshold,
		alphaThreshold,
	)
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	if p.runtimeState.SyncSprite == nil || dst.runtimeState.SyncSprite == nil {
		return false
	}
	return p.engine().SpriteMgr.CheckCollisionWithSprite(p.runtimeState.SyncSprite.GetId(), dst.runtimeState.SyncSprite.GetId(), alphaThreshold, !isPhysicsEnabled())
}

func (p *SpriteImpl) checkTouchingScreen(where int, area string) (touching int) {
	if p.runtimeState.SyncSprite == nil {
		return 0
	}
	switch area {
	case edgeAreaCamera:
		touching = int(p.engine().PhysicsMgr.CheckTouchedCameraBoundaries(p.runtimeState.SyncSprite.GetId()))
	default:
		touching = int(p.engine().PhysicsMgr.CheckTouchedStageBoundaries(p.runtimeState.SyncSprite.GetId()))
	}
	return touching & where
}

func (p *SpriteImpl) checkNearestTouchedBoundary(area string) int {
	if p.runtimeState.SyncSprite == nil {
		return 0
	}
	switch area {
	case edgeAreaCamera:
		return int(p.engine().PhysicsMgr.CheckNearestTouchedCameraBoundary(p.runtimeState.SyncSprite.GetId()))
	default:
		return int(p.engine().PhysicsMgr.CheckNearestTouchedStageBoundary(p.runtimeState.SyncSprite.GetId()))
	}
}

func (p *SpriteImpl) HideVar(name PropertyName) {
	if !p.g.setStageMonitor(p.name, name, false) {
		p.g.setStageMonitor("", name, false)
	}
}

func (p *SpriteImpl) ShowVar(name PropertyName) {
	if !p.g.setStageMonitor(p.name, name, true) {
		p.g.setStageMonitor("", name, true)
	}
}
