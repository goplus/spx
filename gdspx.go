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

	"github.com/goplus/spx/internal/engine"
	"github.com/goplus/spx/internal/enginewrap"

	"github.com/realdream-ai/mathf"
)

// copy these variable to any namespace you want
var (
	audioMgr    enginewrap.AudioMgrImpl
	cameraMgr   enginewrap.CameraMgrImpl
	inputMgr    enginewrap.InputMgrImpl
	physicMgr   enginewrap.PhysicMgrImpl
	platformMgr enginewrap.PlatformMgrImpl
	resMgr      enginewrap.ResMgrImpl
	sceneMgr    enginewrap.SceneMgrImpl
	spriteMgr   enginewrap.SpriteMgrImpl
	uiMgr       enginewrap.UiMgrImpl
	extMgr      enginewrap.ExtMgrImpl
)

var (
	cachedBounds_ map[string]mathf.Rect2
)

func (p *Game) OnEngineStart() {
	cachedBounds_ = make(map[string]mathf.Rect2)
	onStart := func() {
		initInput()
		gamer := p.gamer_
		if me, ok := gamer.(interface{ MainEntry() }); ok {
			runMain(me.MainEntry)
		}
		if !p.isRunned {
			Gopt_Game_Run(gamer, "assets")
		}
		engine.OnGameStarted()
	}
	go onStart()
}

func (p *Game) OnEngineDestroy() {
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.isRunned {
		return
	}
	// all these functions is called in main thread
	p.syncUpdateInput()
	p.syncUpdateCamera()
	p.syncUpdateLogic()
}
func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
	p.syncUpdateProxy()
	p.syncUpdatePhysic()
}

func (p *Game) syncUpdateLogic() error {
	p.startFlag.Do(func() {
		p.fireEvent(&eventStart{})
	})

	return nil
}

func (p *Game) syncUpdateCamera() {
	isOn, pos := p.Camera.getFollowPos()
	if isOn {
		engine.SyncSetCameraPosition(pos)
	}
}

func (p *Game) syncUpdateInput() {
	pos := engine.SyncGetMousePos()
	wpos := engine.SyncScreenToWorld(pos)
	p.mousePos = wpos
	p.mousePos = p.mousePos.Divf(p.windowScale)
}

func (sprite *SpriteImpl) syncCheckInitProxy() {
	// bind syncSprite
	if sprite.syncSprite == nil && !sprite.HasDestroyed {
		sprite.syncSprite = engine.SyncNewSprite(sprite)
		syncInitSpritePhysicInfo(sprite, sprite.syncSprite)
		sprite.syncSprite.Name = sprite.name
		sprite.syncSprite.SetTypeName(sprite.name)
		sprite.syncSprite.SetVisible(sprite.isVisible)
	}
}

func (sprite *SpriteImpl) updateProxyTransform(isSync bool) {
	if sprite.syncSprite == nil {
		return
	}
	x, y := sprite.getXY()
	applyRenderOffset(sprite, &x, &y)
	rot, scale := calcRenderRotation(sprite)
	sprite.syncSprite.UpdateTransform(x, y, rot, scale, isSync)
}

func (p *Game) syncUpdateProxy() {
	count := 0
	items := p.getItems()
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if ok {
			sprite.syncCheckInitProxy()
			if sprite.HasDestroyed {
				continue
			}
			syncSprite := sprite.syncSprite
			// sync position
			if sprite.isVisible {
				sprite.updateProxyTransform(true)
				syncCheckUpdateCostume(&sprite.baseObj)
				count++
			}
			syncSprite.SetVisible(sprite.isVisible)
		}
	}

	// unbind syncSprite
	for _, item := range p.destroyItems {
		sprite, ok := item.(*SpriteImpl)
		if ok && sprite.syncSprite != nil {
			sprite.syncSprite.Destroy()
			sprite.syncSprite = nil
		}
	}
	p.destroyItems = nil
}

func checkUpdateCostume(p *baseObj) {
	engine.WaitMainThread(func() {
		syncCheckUpdateCostume(p)
	})
}

func syncCheckUpdateCostume(p *baseObj) {
	syncSprite := p.syncSprite
	if p.isLayerDirty {
		syncSprite.SetZIndex(int64(p.layer))
		p.isLayerDirty = false
	}
	if !p.isCostumeDirty {
		return
	}
	p.isCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	rect := p.getCostumeAltasRegion()
	isAltas := p.isCostumeAltas()
	if isAltas {
		syncSprite.UpdateTextureAltas(path, rect, renderScale)
	} else {
		syncSprite.UpdateTexture(path, renderScale)
	}
}

func (*Game) syncUpdatePhysic() {
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

func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	// update collision layers
	syncProxy.SetTriggerLayer(sprite.triggerLayer)
	syncProxy.SetTriggerMask(sprite.triggerMask)
	syncProxy.SetCollisionLayer(sprite.collisionLayer)
	syncProxy.SetCollisionMask(sprite.collisionMask)

	// set trigger & collider
	switch sprite.colliderType {
	case physicColliderCircle:
		syncProxy.SetCollisionEnabled(true)
		syncProxy.SetColliderCircle(sprite.colliderCenter, math.Max(sprite.colliderRadius, 0.01))
	case physicColliderRect:
		syncProxy.SetCollisionEnabled(true)
		syncProxy.SetColliderRect(sprite.colliderCenter, sprite.colliderSize)
	case physicColliderAuto:
		center, size := syncGetCostumeBoundByAlpha(sprite, sprite.scale)
		syncProxy.SetCollisionEnabled(true)
		syncProxy.SetColliderRect(center, size)
	case physicColliderNone:
		syncProxy.SetCollisionEnabled(false)
	}

	switch sprite.triggerType {
	case physicColliderCircle:
		syncProxy.SetTriggerEnabled(true)
		syncProxy.SetTriggerCircle(sprite.triggerCenter, math.Max(sprite.triggerRadius, 0.01))
		sprite.triggerSize = mathf.NewVec2(sprite.triggerRadius, sprite.triggerRadius)
	case physicColliderRect:
		syncProxy.SetTriggerEnabled(true)
		syncProxy.SetTriggerRect(sprite.triggerCenter, sprite.triggerSize)
	case physicColliderAuto:
		sprite.triggerCenter, sprite.triggerSize = syncGetCostumeBoundByAlpha(sprite, sprite.scale)
		syncProxy.SetTriggerEnabled(true)
		syncProxy.SetTriggerRect(sprite.triggerCenter, sprite.triggerSize)
	case physicColliderNone:
		syncProxy.SetTriggerEnabled(false)
	}
}

func syncGetCostumeBoundByAlpha(p *SpriteImpl, pscale float64) (mathf.Vec2, mathf.Vec2) {
	cs := p.costumes[p.costumeIndex_]
	var rect mathf.Rect2
	// GetBoundFromAlpha is very slow, so we should cache the result
	if cs.isAltas() {
		rect = p.getCostumeAltasRegion()
		rect.Position.X = 0
		rect.Position.Y = 0
	} else {
		if cache, ok := cachedBounds_[cs.path]; ok {
			rect = cache
		} else {
			assetPath := engine.ToAssetPath(cs.path)
			rect = engine.SyncGetBoundFromAlpha(assetPath)
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
	resMgr.CreateAnimation(spriteName, animName, sb.String(), int64(animCfg.FrameFps), isCostumeSet)
}
