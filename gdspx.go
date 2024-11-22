/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"sync/atomic"

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/math32"

	gdspx "github.com/realdream-ai/gdspx/pkg/engine"
)

var (
	cachedBounds map[string]gdspx.Rect2
)

func (p *Game) OnEngineStart() {
	cachedBounds = make(map[string]gdspx.Rect2)
	go p.onStartAsync()
}

func (p *Game) OnEngineDestroy() {
}

func (p *Game) OnEngineUpdate(delta float32) {
	if !p.isRunned {
		return
	}
	// all these functions is called in main thread
	p.updateInput()
	p.updateCamera()
	p.updateLogic()
	p.updateProxy()
	p.updatePhysic()
}

func (p *Game) onStartAsync() {
	initInput()
	gamer := p.gamer_
	if me, ok := gamer.(interface{ MainEntry() }); ok {
		me.MainEntry()
	}
	if !p.isRunned {
		Gopt_Game_Run(gamer, "assets")
	}
}

func (p *Game) updateLogic() error {
	p.startFlag.Do(func() {
		p.fireEvent(&eventStart{})
	})

	p.tickMgr.update()
	return nil
}

func (p *Game) updateCamera() {
	isOn, x, y := p.Camera.getFollowPos()
	if isOn {
		gdspx.CameraMgr.SetCameraPosition(engine.NewVec2(x, -y))
	}
}

func (p *Game) updateInput() {
	pos := gdspx.InputMgr.GetMousePos()
	posX, posY := engine.ScreenToWorld(float64(pos.X), float64(pos.Y))
	atomic.StoreInt64(&p.gMouseX, int64(posX))
	atomic.StoreInt64(&p.gMouseY, int64(posY))
}

func (p *Game) updateProxy() {
	count := 0
	items := p.getItems()
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if ok {
			var proxy *engine.ProxySprite
			// bind proxy
			if sprite.proxy == nil && !sprite.HasDestroyed {
				sprite.proxy = engine.NewSpriteProxy(sprite)
				initSpritePhysicInfo(sprite, sprite.proxy)
				//sprite.proxy.SetScale(engine.NewVec2(0.5, 0.5)) // TODO(tanjp) remove this hack
			}
			proxy = sprite.proxy
			if sprite.HasDestroyed {
				continue
			}
			proxy.Name = sprite.name
			// sync position
			if sprite.isVisible {
				x, y := sprite.getXY()
				applyRenderOffset(sprite, &x, &y)
				rot := calcRenderRotation(sprite)
				proxy.UpdatePosRot(x, y, rot)
				if sprite.isCostumeAltas() {
					proxy.UpdateTextureAltas(sprite.getCostumePath(), sprite.getCostumeAltasRegion().ToRect2(), sprite.getCostumeRenderScale())
				} else {
					proxy.UpdateTexture(sprite.getCostumePath(), sprite.getCostumeRenderScale())
				}

				count++
			}
			proxy.SetVisible(sprite.isVisible)
		}
	}

	// unbind proxy
	for _, item := range p.destroyItems {
		sprite, ok := item.(*SpriteImpl)
		if ok && sprite.proxy != nil {
			sprite.proxy.Destroy()
			sprite.proxy = nil
		}
	}
	p.destroyItems = nil
}

func (*Game) updatePhysic() {
	triggers := make([]engine.TriggerEvent, 0)
	triggers = engine.GetTriggerEvents(triggers)
	for _, pair := range triggers {
		src := pair.Src.Target
		dst := pair.Dst.Target
		srcSprite, ok1 := src.(*SpriteImpl)
		dstSrpite, ok2 := dst.(*SpriteImpl)
		if ok1 && ok2 {
			if srcSprite.isVisible && !srcSprite.isDying && dstSrpite.isVisible && !dstSrpite.isDying {
				srcSprite.hasOnTouchStart = true
				srcSprite.fireTouchStart(dstSrpite)
			}

		} else {
			panic("unexpected trigger pair ")
		}
	}
}

func initSpritePhysicInfo(sprite *SpriteImpl, proxy *engine.ProxySprite) {
	// update collision layers
	proxy.SetTriggerLayer(sprite.triggerLayer)
	proxy.SetTriggerMask(sprite.triggerMask)
	proxy.SetCollisionLayer(sprite.collisionLayer)
	proxy.SetCollisionMask(sprite.collisionMask)

	// set trigger & collider
	switch sprite.colliderType {
	case physicColliderCircle:
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderCircle(sprite.colliderCenter.ToVec2(), float32(math.Max(sprite.colliderRadius, 0.01)))
	case physicColliderRect:
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderRect(sprite.colliderCenter.ToVec2(), sprite.colliderSize.ToVec2())
	case physicColliderAuto:
		center, size := getCostumeBoundByAlpha(sprite, sprite.scale)
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderRect(center.ToVec2(), size.ToVec2())
	case physicColliderNone:
		proxy.SetCollisionEnabled(false)
	}

	switch sprite.triggerType {
	case physicColliderCircle:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerCircle(sprite.triggerCenter.ToVec2(), float32(math.Max(sprite.triggerRadius, 0.01)))
		sprite.triggerSize = *math32.NewVector2(sprite.triggerRadius, sprite.triggerRadius)
	case physicColliderRect:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(sprite.triggerCenter.ToVec2(), sprite.triggerSize.ToVec2())
	case physicColliderAuto:
		sprite.triggerCenter, sprite.triggerSize = getCostumeBoundByAlpha(sprite, sprite.scale)
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(sprite.triggerCenter.ToVec2(), sprite.triggerSize.ToVec2())
	case physicColliderNone:
		proxy.SetTriggerEnabled(false)
	}

}

func getCostumeBoundByAlpha(p *SpriteImpl, pscale float64) (math32.Vector2, math32.Vector2) {
	cs := p.costumes[p.costumeIndex_]
	var rect gdspx.Rect2
	// GetBoundFromAlpha is very slow, so we should cache the result
	if cache, ok := cachedBounds[cs.path]; ok {
		rect = cache
	} else {
		assetPath := engine.ToAssetPath(cs.path)
		rect = gdspx.ResMgr.GetBoundFromAlpha(assetPath)
		cachedBounds[cs.path] = rect
	}
	scale := pscale / float64(cs.bitmapResolution)
	// top left
	posX := float64(rect.Position.X) * scale
	posY := float64(rect.Position.Y) * scale
	sizeX := float64(rect.Size.X) * scale
	sizeY := float64(rect.Size.Y) * scale

	w, h := p.getCostumeSize()
	w, h = w*p.scale, h*p.scale
	offsetX := float64(posX + sizeX/2 - w/2)
	offsetY := -float64(posY + sizeY/2 - h/2)

	center := *math32.NewVector2(offsetX, offsetY)
	size := *math32.NewVector2(sizeX, sizeY)
	return center, size
}

func calcRenderRotation(p *SpriteImpl) float64 {
	cs := p.costumes[p.costumeIndex_]
	degree := p.Heading() + cs.faceRight
	degree -= 90
	if p.rotationStyle == LeftRight {
		degree = 0
		hScale := 1
		isFlip := p.direction < 0
		if isFlip {
			hScale = -1
		}
		p.proxy.SetScaleX(float32(hScale))
	}
	return degree
}

func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	cs := p.costumes[p.costumeIndex_]
	x, y := -((cs.center.X)/float64(cs.bitmapResolution)+p.pivot.X)*p.scale,
		((cs.center.Y)/float64(cs.bitmapResolution)-p.pivot.Y)*p.scale

	// spx's start point is top left, gdspx's start point is center
	// so we should remove the offset to make the pivot point is the same
	w, h := p.getCostumeSize()
	x = x + float64(w)/2*p.scale
	y = y - float64(h)/2*p.scale

	*cx = *cx + x
	*cy = *cy + y
}
