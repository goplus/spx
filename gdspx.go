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

	gdspx "github.com/realdream-ai/gdspx/pkg/engine"
)

func (p *Game) OnEngineStart() {
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
	p.updateUI()
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
	atomic.StoreInt64(&p.gMouseX, int64(pos.X))
	atomic.StoreInt64(&p.gMouseY, int64(pos.Y))
}

func (p *Game) updateUI() {
	newItems := make([]Shape, len(p.items))
	copy(newItems, p.items)
	for _, item := range newItems {
		if result, ok := item.(interface{ OnUpdate(float32) }); ok {
			result.OnUpdate(0.01)
		}
	}
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
				proxy.UpdatePosRot(x, y, sprite.Heading()-sprite.initDirection)
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
		w, h := sprite.getCostumeSize()
		w, h = w*sprite.scale, h*sprite.scale
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderRect(engine.NewVec2(0, 0), engine.NewVec2(w, h))
	case physicColliderNone:
		proxy.SetCollisionEnabled(false)
	}

	switch sprite.triggerType {
	case physicColliderCircle:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerCircle(sprite.triggerCenter.ToVec2(), float32(math.Max(sprite.triggerRadius, 0.01)))
	case physicColliderRect:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(sprite.triggerCenter.ToVec2(), sprite.triggerSize.ToVec2())
	case physicColliderAuto:
		w, h := sprite.getCostumeSize()
		w, h = w*sprite.scale, h*sprite.scale
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(engine.NewVec2(0, 0), engine.NewVec2(w, h))
	case physicColliderNone:
		proxy.SetTriggerEnabled(false)
	}

}
func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	cs := p.costumes[p.costumeIndex_]
	x, y := -(cs.center.X+p.pivot.X)/float64(cs.bitmapResolution)*p.scale,
		(cs.center.Y+p.pivot.Y)/float64(cs.bitmapResolution)*p.scale

	// spx's start point is top left, gdspx's start point is center
	// so we should remove the offset to make the pivot point is the same
	w, h := p.getCostumeSize()
	x = x + float64(w)/2*p.scale
	y = y - float64(h)/2*p.scale

	*cx = *cx + x
	*cy = *cy + y
}
