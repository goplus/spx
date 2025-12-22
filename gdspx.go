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
	"encoding/json"
	"fmt"

	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"

	"github.com/goplus/spbase/mathf"
)

// copy these variable to any namespace you want
var (
	audioMgr      enginewrap.AudioMgrImpl
	cameraMgr     enginewrap.CameraMgrImpl
	inputMgr      enginewrap.InputMgrImpl
	physicMgr     enginewrap.PhysicMgrImpl
	platformMgr   enginewrap.PlatformMgrImpl
	resMgr        enginewrap.ResMgrImpl
	sceneMgr      enginewrap.SceneMgrImpl
	spriteMgr     enginewrap.SpriteMgrImpl
	uiMgr         enginewrap.UiMgrImpl
	extMgr        enginewrap.ExtMgrImpl
	penMgr        enginewrap.PenMgrImpl
	debugMgr      enginewrap.DebugMgrImpl
	navigationMgr enginewrap.NavigationMgrImpl
	tilemapMgr    enginewrap.TilemapMgrImpl
)

var (
	cachedBounds_ map[string]mathf.Rect2
)

func (p *Game) OnEngineStart() {
	cachedBounds_ = make(map[string]mathf.Rect2)
	onStart := func() {
		defer engine.CheckPanic()
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

func (p *Game) OnEngineReset() {
	p.reset()
	p.isRunned = false
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.isRunned {
		return
	}
	// all these functions is called in main thread
	p.syncUpdateInput()
	p.syncUpdateCamera()
	p.syncUpdateLogic()
	p.syncEnginePositions()
}
func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
	p.syncUpdateProxy()
	p.syncUpdatePhysic()
}

func (p *Game) OnEnginePause(isPaused bool) {
	if !p.isRunned {
		return
	}
}

func (p *Game) syncUpdateLogic() error {
	p.startFlag.Do(func() {
		p.fireEvent(&eventStart{})
	})

	return nil
}
func (p *Game) syncEnginePositions() error {
	items := p.getTempShapes()
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if ok && sprite.syncSprite != nil {
			if sprite.physicsMode != NoPhysics {
				sprite.x, sprite.y = sprite.syncGetEnginePosition(true)
			}
		}
	}
	return nil
}

func (p *Game) syncUpdateCamera() {
	isOn, pos := p.camera.getFollowPos()
	if isOn {
		engine.SyncSetCameraPosition(pos)
	}
}

func (p *Game) syncUpdateInput() {
	p.mousePos = engine.SyncGetMousePos()
}

func (sprite *SpriteImpl) syncCheckInitProxy() {
	// bind syncSprite
	if sprite.syncSprite == nil && !sprite.HasDestroyed {
		sprite.syncSprite = engine.SyncNewSprite(sprite, mathf.NewVec2(sprite.x, sprite.y))
		syncInitSpritePhysicInfo(sprite, sprite.syncSprite)
		sprite.syncSprite.Name = sprite.name
		sprite.syncSprite.SetTypeName(sprite.name)
		sprite.syncSprite.SetVisible(sprite.isVisible)
		sprite.applyGraphicEffects(true)
		sprite.syncSprite.RegisterOnAnimationLooped(sprite.syncOnAnimationLooped)
		sprite.syncSprite.RegisterOnAnimationFinished(sprite.syncOnAnimationFinished)
		sprite.updateProxyTransform(true)
	}
}
func (sprite *SpriteImpl) syncOnAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.curAnimState
	if state != nil && state.Name != "" && sprite.syncSprite != nil {
		curAnimName := sprite.syncSprite.GetCurrentAnimName()
		sprite.donedAnimations = append(sprite.donedAnimations, curAnimName)
	}
}
func (sprite *SpriteImpl) syncOnAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.curTweenState
	if state != nil && state.AudioName != "" {
		sprite.pendingAudios = append(sprite.pendingAudios, state.AudioName)
	}
}
func (sprite *SpriteImpl) updateProxyTransform(isSync bool) {
	if sprite.syncSprite == nil {
		return
	}
	x, y := sprite.getXY()
	applyRenderOffset(sprite, &x, &y)
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scale := calcRenderRotation(sprite)
	sprite.syncSprite.UpdateTransform(x, y, rot, scale, offsetX, offsetY, isSync)
}

func (sprite *SpriteImpl) syncGetEnginePosition(isSync bool) (float64, float64) {
	if sprite.syncSprite == nil {
		return sprite.x, sprite.y
	}
	pos := sprite.syncSprite.GetPosition()
	x, y := pos.X, pos.Y
	revertRenderOffset(sprite, &x, &y)
	return x, y
}

func (p *Game) syncUpdateProxy() {
	count := 0
	items := p.getItems()
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if ok {
			if sprite.HasDestroyed {
				continue
			}

			syncSprite := sprite.syncSprite
			// sync position
			if sprite.isVisible {
				syncCheckUpdateCostume(&sprite.baseObj)
				count++
			}
			syncSprite.SetVisible(sprite.isVisible)
		}
	}

	// unbind syncSprite
	p.spriteMgr.flushDestroy()
}

func checkUpdateCostume(p *baseObj) {
	engine.WaitMainThread(func() {
		syncCheckUpdateCostume(p)
	})
}

func syncCheckUpdateCostume(p *baseObj) {
	syncSprite := p.syncSprite
	if p.isLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.layer))
		}
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
		// if is animating, will not update render texture
		syncSprite.UpdateTextureAltas(path, rect, renderScale, !p.isAnimating)
		syncOnAltasChanged(p)
	} else {
		syncSprite.UpdateTexture(path, renderScale, !p.isAnimating)
	}
}
func syncOnAltasChanged(p *baseObj) {
	key := "atlas_uv_rect2"
	uvRemap := p.getCostumeAltasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4(key, val, true)
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
			fmt.Printf("Physics error: unexpected trigger pair - invalid sprite types\n")
		}
	}
}

func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	sprite.initCollisionParams()
	sprite.collisionInfo.syncToProxy(syncProxy, false, sprite)
	sprite.triggerInfo.syncToProxy(syncProxy, true, sprite)
	syncProxy.SetGravityScale(sprite.gravity)
	syncProxy.SetPhysicsMode(sprite.physicsMode)
}
func syncGetCostumeBoundByAlpha(p *SpriteImpl, pscale float64) (mathf.Vec2, mathf.Vec2) {
	return getCostumeBoundByAlpha(p, pscale, true)
}

func getCostumeBoundByAlpha(p *SpriteImpl, pscale float64, isSync bool) (mathf.Vec2, mathf.Vec2) {
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

func calcRenderRotation(p *SpriteImpl) (float64, float64) {
	if p.rotationStyle == None {
		return 0, 1.0
	}
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
func getRenderOffset(p *SpriteImpl) (float64, float64) {
	cs := p.costumes[p.costumeIndex_]
	x, y := -((cs.center.X)/float64(cs.bitmapResolution)+p.pivot.X)*p.scale,
		((cs.center.Y)/float64(cs.bitmapResolution)-p.pivot.Y)*p.scale

	// spx's start point is top left, gdspx's start point is center
	// so we should remove the offset to make the pivot point is the same
	w, h := p.getCostumeSize()
	x = x + float64(w)/2*p.scale
	y = y - float64(h)/2*p.scale

	return x, y
}

func applyRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getRenderOffset(p)
	*cx = *cx + x
	*cy = *cy + y
}

func revertRenderOffset(p *SpriteImpl, cx, cy *float64) {
	x, y := getRenderOffset(p)
	*cx = *cx - x
	*cy = *cy - y
}

func createAnimation(
	spriteName string,
	animName string,
	cfg *aniConfig,
	costumes []*costume,
	isAtlas bool,
) {
	payload := buildAnimPayload(cfg, costumes, isAtlas)
	cfg.AdaptAnimBitmapResolution = int(payload.MaxBitmap)

	bin, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	if cfg.IFrameFrom < 0 || cfg.IFrameFrom >= len(costumes) || cfg.IFrameTo < 0 || cfg.IFrameTo >= len(costumes) {
		panic(fmt.Sprintf("animation frame index out of bounds: from %d, to %d, costumes len %d", cfg.IFrameFrom, cfg.IFrameTo, len(costumes)))
	}

	resMgr.CreateAnimation(
		spriteName,
		animName,
		string(bin),
		int64(cfg.FrameFps),
		isAtlas,
	)
}

func buildAnimPayload(cfg *aniConfig, costumes []*costume, isAtlas bool) animPayload {
	if isAtlas {
		return buildAtlasPayload(cfg, costumes)
	}
	return buildNormalPayload(cfg, costumes)
}

func buildNormalPayload(cfg *aniConfig, costumes []*costume) animPayload {
	maxBitmap := 0

	frameCount := cfg.IFrameTo - cfg.IFrameFrom + 1
	frames := make([]any, 0, frameCount)

	for i := cfg.IFrameFrom; i <= cfg.IFrameTo; i++ {
		c := costumes[i]
		b := toBitmapResolution(c.bitmapResolution)

		if b > maxBitmap {
			maxBitmap = b
		}

		path := engine.ToAssetPath(c.path)
		half := mathf.Vec2.Mulf(c.imageSize, 0.5)

		frames = append(frames, frameNormal{
			Path: path,
			Offset: [2]float64{
				c.center.X - half.X,
				-(c.center.Y - half.Y),
			},
			Bitmap: int64(b),
		})
	}

	return animPayload{Frames: frames, MaxBitmap: int64(maxBitmap)}
}

func buildAtlasPayload(cfg *aniConfig, costumes []*costume) animPayload {
	base := engine.ToAssetPath(costumes[0].path)

	step := 1
	if cfg.IFrameTo < cfg.IFrameFrom {
		step = -1
	}

	frameCount := (cfg.IFrameTo-cfg.IFrameFrom)*step + 1
	frames := make([]any, 0, frameCount)
	for i := cfg.IFrameFrom; i != cfg.IFrameTo+step; i += step {
		c := costumes[i]
		frames = append(frames, frameAtlas{
			X:      int64(c.posX),
			Y:      int64(c.posY),
			W:      int64(c.width),
			H:      int64(c.height),
			Offset: [2]float64{0, 0},
		})
	}

	return animPayload{BasePath: base, Frames: frames, MaxBitmap: 1}
}
