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
	"fmt"
	"log"
	"math"
	"strings"
	"sync/atomic"

	"github.com/goplus/spx/internal/engine"

	gdspx "github.com/realdream-ai/gdspx/pkg/engine"
	"github.com/realdream-ai/mathf"
)

var (
	cachedBounds map[string]mathf.Rect2
)

func (p *Game) OnEngineStart() {
	cachedBounds = make(map[string]mathf.Rect2)
	go p.onStartAsync()
}

func (p *Game) OnEngineDestroy() {
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.isRunned {
		return
	}
	// all these functions is called in main thread
	p.updateInput()
	p.updateCamera()
	p.updateLogic()
}
func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
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
	engine.OnGameStarted()
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
		gdspx.CameraMgr.SetCameraPosition(mathf.NewVec2(x, -y))
	}
}

func (p *Game) updateInput() {
	pos := gdspx.InputMgr.GetMousePos()
	posX, posY := engine.ScreenToWorld(float64(pos.X), float64(pos.Y))
	atomic.StoreInt64(&p.gMouseX, int64(posX))
	atomic.StoreInt64(&p.gMouseY, int64(posY))
}

func (sprite *SpriteImpl) checkInitProxy() {
	// bind proxy
	if sprite.proxy == nil && !sprite.HasDestroyed {
		sprite.proxy = engine.NewSpriteProxy(sprite)
		initSpritePhysicInfo(sprite, sprite.proxy)
		sprite.proxy.Name = sprite.name
		sprite.proxy.SetTypeName(sprite.name)
		sprite.proxy.SetVisible(sprite.isVisible)
	}
}

func (sprite *SpriteImpl) updateProxyTransform(isSync bool) {
	if sprite.proxy == nil {
		return
	}
	x, y := sprite.getXY()
	applyRenderOffset(sprite, &x, &y)
	rot, scale := calcRenderRotation(sprite)
	sprite.proxy.UpdateTransform(x, y, rot, scale, isSync)
}

func (p *Game) updateProxy() {
	count := 0
	items := p.getItems()
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if ok {
			sprite.checkInitProxy()
			if sprite.HasDestroyed {
				continue
			}
			proxy := sprite.proxy
			// sync position
			if sprite.isVisible {
				sprite.updateProxyTransform(false)
				checkUpdateCostume(&sprite.baseObj, false)
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

func checkUpdateCostume(p *baseObj, isSync bool) {
	if isSync {
		engine.WaitMainThread(func() {
			doCheckUpdateCostume(p)
		})
	} else {
		doCheckUpdateCostume(p)
	}
}
func doCheckUpdateCostume(p *baseObj) {
	if !p.isCostumeDirty {
		return
	}
	p.isCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	rect := p.getCostumeAltasRegion()
	isAltas := p.isCostumeAltas()
	pself := p.proxy
	if isAltas {
		pself.UpdateTextureAltas(path, rect, renderScale)
	} else {
		pself.UpdateTexture(path, renderScale)
	}
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
		proxy.SetColliderCircle(sprite.colliderCenter, math.Max(sprite.colliderRadius, 0.01))
	case physicColliderRect:
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderRect(sprite.colliderCenter, sprite.colliderSize)
	case physicColliderAuto:
		center, size := getCostumeBoundByAlpha(sprite, sprite.scale)
		proxy.SetCollisionEnabled(true)
		proxy.SetColliderRect(center, size)
	case physicColliderNone:
		proxy.SetCollisionEnabled(false)
	}

	switch sprite.triggerType {
	case physicColliderCircle:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerCircle(sprite.triggerCenter, math.Max(sprite.triggerRadius, 0.01))
		sprite.triggerSize = mathf.NewVec2(sprite.triggerRadius, sprite.triggerRadius)
	case physicColliderRect:
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(sprite.triggerCenter, sprite.triggerSize)
	case physicColliderAuto:
		sprite.triggerCenter, sprite.triggerSize = getCostumeBoundByAlpha(sprite, sprite.scale)
		proxy.SetTriggerEnabled(true)
		proxy.SetTriggerRect(sprite.triggerCenter, sprite.triggerSize)
	case physicColliderNone:
		proxy.SetTriggerEnabled(false)
	}
}

func getCostumeBoundByAlpha(p *SpriteImpl, pscale float64) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex_]
	var rect mathf.Rect2
	// GetBoundFromAlpha is very slow, so we should cache the result
	if cs.isAltas() {
		rect = p.getCostumeAltasRegion()
		rect.Position.X = 0
		rect.Position.Y = 0
	} else {
		if cache, ok := cachedBounds[cs.path]; ok {
			rect = cache
		} else {
			assetPath := engine.ToAssetPath(cs.path)
			rect = gdspx.ResMgr.GetBoundFromAlpha(assetPath)
		}
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

	center := mathf.NewVec2(offsetX, offsetY)
	size := mathf.NewVec2(sizeX, sizeY)
	return center, size
}

func calcRenderRotation(p *SpriteImpl) (float64, float64) {
	cs := p.costumes[p.costumeIndex_]
	degree := p.Heading() + cs.faceRight
	degree -= 90
	hScale := 1.0
	if p.rotationStyle == LeftRight {
		degree = 0
		isFlip := p.direction < 0
		if isFlip {
			hScale = -1.0
		}
	}
	return degree, hScale
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

func registerAnimToEngine(spriteName string, animName string, animCfg *aniConfig, costumes []*costume, isCostumeSet bool) {
	sb := strings.Builder{}
	from, to := animCfg.IFrameFrom, animCfg.IFrameTo
	if from >= len(costumes) {
		log.Panicf("animation key [%s] from [%d] is out of costumes length [%d]", animName, from, len(costumes))
		return
	}
	if isCostumeSet {
		assetPath := engine.ToAssetPath(costumes[0].path)
		sb.WriteString(assetPath)
		sb.WriteString(";")
		ary := make([]int, 0)
		if from <= to {
			for i := from; i <= to; i++ {
				ary = append(ary, i)
			}
		} else {
			for i := from; i >= to; i-- {
				ary = append(ary, i)
			}
		}
		for _, i := range ary {
			costume := costumes[i]
			sb.WriteString(fmt.Sprintf("%d,%d,%d,%d", costume.posX, costume.posY, costume.width, costume.height))
			if i != to {
				sb.WriteString(",")
			}
		}
	} else {
		for i := from; i <= to; i++ {
			assetPath := engine.ToAssetPath(costumes[i].path)
			sb.WriteString(assetPath)
			if i != to {
				sb.WriteString(";")
			}
		}
	}
	engine.SyncResCreateAnimation(spriteName, animName, sb.String(), int64(animCfg.FrameFps), isCostumeSet)
}
