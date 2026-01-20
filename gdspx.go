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
	spxlog "github.com/goplus/spx/v2/internal/log"

	"github.com/goplus/spbase/mathf"
)

// copy these variable to any namespace you want
var (
	audioMgr         enginewrap.AudioMgrImpl
	cameraMgr        enginewrap.CameraMgrImpl
	inputMgr         enginewrap.InputMgrImpl
	physicsMgr       enginewrap.PhysicsMgrImpl
	platformMgr      enginewrap.PlatformMgrImpl
	resMgr           enginewrap.ResMgrImpl
	sceneMgr         enginewrap.SceneMgrImpl
	spriteMgr        enginewrap.SpriteMgrImpl
	uiMgr            enginewrap.UiMgrImpl
	extMgr           enginewrap.ExtMgrImpl
	penMgr           enginewrap.PenMgrImpl
	debugMgr         enginewrap.DebugMgrImpl
	navigationMgr    enginewrap.NavigationMgrImpl
	tilemapMgr       enginewrap.TilemapMgrImpl
	tilemapparserMgr enginewrap.TilemapparserMgrImpl
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
	p.syncUpdateProxy()
}

func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
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
	// Collect sprite IDs that need position sync
	var spriteIDs []int64
	var sprites []*SpriteImpl

	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok && sprite.syncSprite != nil && sprite.PhysicsMode() != NoPhysics {
			spriteIDs = append(spriteIDs, int64(sprite.syncSprite.Id))
			sprites = append(sprites, sprite)
		}
	}

	// Batch get positions in one FFI call
	positions := engine.SyncBatchGetPositions(spriteIDs)

	// Update sprite positions
	for i, sprite := range sprites {
		x := float64(positions[i*2])
		y := float64(positions[i*2+1])
		revertRenderOffset(sprite, &x, &y)
		sprite.SetXYpos(x, y)
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
		sprite.syncSprite = engine.SyncNewSprite(sprite, mathf.NewVec2(sprite.getXY()))
		syncInitSpritePhysicInfo(sprite, sprite.syncSprite)
		sprite.syncSprite.Name = sprite.name
		sprite.syncSprite.SetTypeName(sprite.name)
		sprite.syncSprite.SetVisible(sprite.isVisible)
		sprite.applyGraphicEffects(true)
		sprite.syncSprite.RegisterOnAnimationLooped(sprite.syncOnAnimationLooped)
		sprite.syncSprite.RegisterOnAnimationFinished(sprite.syncOnAnimationFinished)
		// Mark as dirty to ensure initial sync
		sprite.isDirty = true
	}
}

func (sprite *SpriteImpl) syncOnAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" && sprite.syncSprite != nil {
		curAnimName := sprite.syncSprite.GetCurrentAnimName()
		sprite.animation().addDonedAnimation(curAnimName)
	}
}

func (sprite *SpriteImpl) syncOnAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurTweenState()
	if state != nil && state.AudioName != "" {
		sprite.sound().addPendingAudio(state.AudioName)
	}
}

func (p *Game) syncUpdateProxy() {
	items := p.getTempShapes()
	// Clear buffer for reuse (avoids allocation every frame)
	p.syncBuffer.Clear()

	for _, item := range items {
		if sprite, ok := item.(*SpriteImpl); ok {
			if sprite.HasDestroyed {
				continue
			}

			if sprite.isVisible {
				syncCheckUpdateCostume(&sprite.baseObj)
			}

			// Only sync if sprite is dirty (transform or visibility changed)
			if sprite.isDirty {
				// Collect transform data into buffer for batch sync
				x, y := sprite.getXY()
				applyRenderOffset(sprite, &x, &y)
				offsetX, offsetY := getRenderOffset(sprite)
				rot, scale := calcRenderRotation(sprite)

				// Add to reusable buffer
				p.syncBuffer.Add(
					int64(sprite.syncSprite.Id),
					x, y,
					engine.DegToRad(rot),
					scale, 1.0, // scaleX, scaleY
					offsetX, offsetY,
					sprite.isVisible,
				)

				// Clear dirty flag after syncing
				sprite.isDirty = false
			}
		}
	}

	// Collect sprite deletions into the same buffer
	p.spriteMgr.flushDestroy(p.syncBuffer)

	// Serialize and send batch updates (updates + deletes in one call)
	if p.syncBuffer.UpdateCount() > 0 || p.syncBuffer.DeleteCount() > 0 {
		buffer := p.syncBuffer.Serialize()
		engine.SyncBatchUpdateSprites(buffer)
	}
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
	rect := p.getCostumeAtlasRegion()
	isAtlas := p.isCostumeAtlas()
	if isAtlas {
		// if is animating, will not update render texture
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.isAnimating)
		syncOnAtlasChanged(p)
	} else {
		syncSprite.UpdateTexture(path, renderScale, !p.isAnimating)
	}
}

func syncOnAtlasChanged(p *baseObj) {
	key := "atlas_uv_rect2"
	uvRemap := p.getCostumeAtlasUvRemap()
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
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types\n")
		}
	}
}

func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	sprite.physics().syncInitPhysicInfo(syncProxy)
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
