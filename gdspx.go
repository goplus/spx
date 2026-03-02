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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -------------------------------------------------------------------------------------
// Engine managers
// -------------------------------------------------------------------------------------

// engineManagers is an alias for the internal EngineManagers type.
// It groups all engine-facing manager wrappers for a Game instance.
type engineManagers = enginewrap.EngineManagers

func (p *Game) engine() *engineManagers {
	return &p.engineMgr
}

func (p *SpriteImpl) engine() *engineManagers {
	return p.g.engine()
}

func (c *componentBase) engine() *engineManagers {
	return c.sprite.engine()
}

// -------------------------------------------------------------------------------------
// Global variables
// -------------------------------------------------------------------------------------

var (
	// cachedBounds stores cached sprite bounds for performance optimization
	cachedBounds map[string]mathf.Rect2
)

// -------------------------------------------------------------------------------------
// Engine lifecycle callbacks
// -------------------------------------------------------------------------------------

// OnEngineStart is called when the engine starts.
// It initializes the game and starts the main game loop.
func (p *Game) OnEngineStart() {
	cachedBounds = make(map[string]mathf.Rect2)
	onStart := func() {
		defer engine.CheckPanic()
		gamer := p.gamer
		if me, ok := gamer.(interface{ MainEntry() }); ok {
			runMain(me.MainEntry)
		}
		if !p.isRunned {
			XGot_Game_Run(gamer, "assets")
		}
		engine.OnGameStarted()
	}
	go onStart()
}

// OnEngineDestroy is called when the engine is destroyed.
func (p *Game) OnEngineDestroy() {

}

// OnEngineReset is called when the engine needs to reset.
func (p *Game) OnEngineReset() {
	p.reset()
}

// OnEngineUpdate is called every frame to update game logic.
// All updates are performed on the main thread.
func (p *Game) OnEngineUpdate(delta float64) {
	if !p.isRunned {
		return
	}
	// All these functions are called in main thread
	p.syncUpdateInput()
	p.syncUpdateLogic()
	p.syncUpdateProxy()
	p.syncEnginePositions()
}

// OnEngineRender is called every frame to render the game.
func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
	p.syncUpdatePhysic()
}

// OnEnginePause is called when the engine is paused or resumed.
func (p *Game) OnEnginePause(isPaused bool) {
	if !p.isRunned {
		return
	}
}

// -------------------------------------------------------------------------------------
// Game synchronization and update methods
// -------------------------------------------------------------------------------------

// syncUpdateLogic updates game logic and fires start events.
func (p *Game) syncUpdateLogic() error {
	p.startFlag.Do(func() {
		p.fireEvent(&eventStart{})
	})
	return nil
}

// syncEnginePositions synchronizes sprite positions from the physics engine.
// This is done in batch for performance optimization.
func (p *Game) syncEnginePositions() error {
	items := p.getTempShapes()
	// Collect sprite IDs that need position sync
	spriteIDs := make([]int64, 0, len(items))
	sprites := make([]*SpriteImpl, 0, len(items))
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
		sprite.transform().setXY(x, y)
	}
	return nil
}

// syncUpdateInput updates input state from the engine.
func (p *Game) syncUpdateInput() {
	p.mousePos = engine.SyncGetMousePos()
}

// syncUpdateProxy updates all sprite proxies and synchronizes them with the engine.
func (p *Game) syncUpdateProxy() {
	p.camera.onUpdate()
	p.spriteMgr.flushActivate()
	p.syncBuffer.Clear()
	items := p.getTempShapes()
	for _, item := range items {
		p.processSpriteUpdate(item)
	}
	p.spriteMgr.flushDestroy(p.syncBuffer)
	p.flushSyncBuffer()
	p.camera.setDirtyFlag(false)
}

// processSpriteUpdate processes a single sprite and adds it to the sync buffer if needed.
func (p *Game) processSpriteUpdate(item any) {
	sprite, ok := item.(*SpriteImpl)
	if !ok || sprite.HasDestroyed || sprite.syncSprite == nil {
		return
	}
	if sprite.isVisible {
		syncCheckUpdateCostume(&sprite.baseObj)
	}
	if sprite.isDirty {
		p.syncSpriteTransform(sprite)
		sprite.isDirty = false
	}
}

// syncSpriteTransform collects sprite transform data and adds it to the sync buffer.
func (p *Game) syncSpriteTransform(sprite *SpriteImpl) {
	x, y := sprite.getXY()
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scaleX, scaleY := getRenderRotationAndScale(sprite)
	p.syncBuffer.Add(
		int64(sprite.syncSprite.Id),
		x+offsetX, y+offsetY,
		engine.DegToRad(rot),
		scaleX, scaleY,
		offsetX, offsetY,
		sprite.isVisible,
	)
}

// flushSyncBuffer sends batched updates to the engine if there are any changes.
func (p *Game) flushSyncBuffer() {
	if p.syncBuffer.UpdateCount() > 0 || p.syncBuffer.DeleteCount() > 0 {
		buffer := p.syncBuffer.Serialize()
		engine.SyncBatchUpdateSprites(buffer)
	}
}

// syncUpdatePhysic processes physics trigger events and fires collision callbacks.
func (p *Game) syncUpdatePhysic() {
	triggers := make([]engine.TriggerEvent, 0)
	triggers = engine.GetTriggerEvents(triggers)

	for _, pair := range triggers {
		p.processTriggerPair(pair)
	}
}

// processTriggerPair processes a single physics trigger pair.
func (p *Game) processTriggerPair(pair engine.TriggerEvent) {
	src := pair.Src.Target
	dst := pair.Dst.Target
	srcSprite, ok1 := src.(*SpriteImpl)
	dstSprite, ok2 := dst.(*SpriteImpl)
	// Validate both targets are sprites
	if !ok1 || !ok2 {
		spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		return
	}
	// Check if both sprites are touchable
	if !isSpriteTouchable(srcSprite) || !isSpriteTouchable(dstSprite) {
		return
	}
	// Fire touch event
	srcSprite.hasOnTouchStart = true
	srcSprite.fireTouchStart(dstSprite)
}

// isSpriteTouchable checks if a sprite can participate in touch events.
func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.isVisible && !sprite.isDying
}

// -------------------------------------------------------------------------------------
// Sprite synchronization methods
// -------------------------------------------------------------------------------------

// syncCheckInitProxy initializes the sprite's engine proxy if it hasn't been created yet.
func (sprite *SpriteImpl) syncCheckInitProxy() {
	if sprite.syncSprite == nil && !sprite.HasDestroyed {
		sprite.syncSprite = engine.SyncNewSprite(sprite, mathf.NewVec2(sprite.getXYWithRenderOffset()))
		syncInitSpritePhysicInfo(sprite, sprite.syncSprite)
		sprite.syncSprite.SetVisible(sprite.isVisible)
		sprite.syncSprite.Name = sprite.name
		sprite.syncSprite.SetTypeName(sprite.name)
		sprite.applyGraphicEffects(true)
		sprite.animation().registerOnAnimationLooped(sprite.syncOnAnimationLooped)
		sprite.animation().registerOnAnimationFinished(sprite.syncOnAnimationFinished)
		sprite.isDirty = true
	}
}

// syncOnAnimationFinished is called when an animation finishes.
func (sprite *SpriteImpl) syncOnAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" && sprite.syncSprite != nil {
		curAnimName := sprite.syncSprite.GetCurrentAnimName()
		sprite.animation().addDonedAnimation(curAnimName)
	}
}

// syncOnAnimationLooped is called when an animation loops.
func (sprite *SpriteImpl) syncOnAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurTweenState()
	if state != nil && state.AudioName != "" {
		sprite.sound().addPendingAudio(state.AudioName)
	}
}

// syncInitSpritePhysicInfo initializes physics information for a sprite.
func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	sprite.physics().syncInitPhysicInfo(syncProxy)
}

// -------------------------------------------------------------------------------------
// Costume update methods
// -------------------------------------------------------------------------------------

// checkUpdateCostume schedules a costume update on the main thread.
func checkUpdateCostume(p *baseObj) {
	engine.WaitMainThread(func() {
		syncCheckUpdateCostume(p)
	})
}

// syncCheckUpdateCostume updates sprite costume and layer if they are dirty.
func syncCheckUpdateCostume(p *baseObj) {
	syncSprite := p.syncSprite
	// Update layer if dirty
	if p.isLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.layer))
		}
		p.isLayerDirty = false
	}
	// Update costume if dirty
	if !p.isCostumeDirty {
		return
	}
	p.isCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	isAtlas := p.isCostumeAtlas()
	if isAtlas {
		rect := p.getCostumeAtlasRegion()
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.isAnimating)
		syncOnAtlasChanged(p)
		return
	}
	syncSprite.UpdateTexture(path, renderScale, !p.isAnimating)
}

// syncOnAtlasChanged updates UV remap parameters when atlas texture changes.
func syncOnAtlasChanged(p *baseObj) {
	key := "atlas_uv_rect2"
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4(key, val, true)
}

// -------------------------------------------------------------------------------------
// Animation creation and payload building
// -------------------------------------------------------------------------------------

// createAnimation creates an animation from configuration and costume data.
func createAnimation(
	engineMgr *engineManagers,
	spriteName string,
	animName string,
	cfg *aniConfig,
	costumes []*costume,
	isAtlas bool,
) {
	// Validate frame indices
	if cfg.IFrameFrom < 0 || cfg.IFrameFrom >= len(costumes) ||
		cfg.IFrameTo < 0 || cfg.IFrameTo >= len(costumes) {
		panic(fmt.Sprintf(
			"createAnimation: frame index out of bounds (from=%d, to=%d, costumes=%d)",
			cfg.IFrameFrom, cfg.IFrameTo, len(costumes),
		))
	}
	// Build animation payload
	payload := buildAnimPayload(cfg, costumes, isAtlas)
	cfg.AdaptAnimBitmapResolution = int(payload.MaxBitmap)
	// Serialize payload
	bin, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("createAnimation: failed to marshal animation payload: %v", err))
	}
	// Create animation in resource manager
	engineMgr.ResMgr.CreateAnimation(
		spriteName,
		animName,
		string(bin),
		int64(cfg.FrameFps),
		isAtlas,
	)
}

// buildAnimPayload builds the appropriate animation payload based on atlas type.
func buildAnimPayload(cfg *aniConfig, costumes []*costume, isAtlas bool) animPayload {
	if isAtlas {
		return buildAtlasPayload(cfg, costumes)
	}
	return buildNormalPayload(cfg, costumes)
}

// buildNormalPayload builds animation payload for non-atlas animations.
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
	return animPayload{
		Frames:    frames,
		MaxBitmap: int64(maxBitmap),
	}
}

// buildAtlasPayload builds animation payload for atlas-based animations.
func buildAtlasPayload(cfg *aniConfig, costumes []*costume) animPayload {
	base := engine.ToAssetPath(costumes[0].path)
	// Determine step direction for frame iteration
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
	return animPayload{
		BasePath:  base,
		Frames:    frames,
		MaxBitmap: 1,
	}
}
