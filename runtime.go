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
	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/timer"
)

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

var (
	// cachedBounds stores cached sprite bounds for performance optimization
	cachedBounds map[string]mathf.Rect2
)

// OnEngineStart is called when the engine starts.
// It initializes the game and starts the main game loop.
func (p *Game) OnEngineStart() {
	p.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		onStart := func() {
			defer engine.CheckPanic()
			gamer := p.gamer
			if me, ok := gamer.(interface{ MainEntry() }); ok {
				runMain(me.MainEntry)
			}
			if !p.IsRunned {
				XGot_Game_Run(gamer, "assets")
			}
			engine.OnGameStarted()
		}
		go onStart()
	})
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
	if !p.IsRunned {
		return
	}
	p.syncUpdateInput()
	p.syncUpdateLogic()
	p.syncUpdateProxy()
	p.syncEnginePositions()
}

// OnEngineRender is called every frame to render the game.
func (p *Game) OnEngineRender(delta float64) {
	if !p.IsRunned {
		return
	}
	p.syncUpdatePhysic()
}

// OnEnginePause is called when the engine is paused or resumed.
func (p *Game) OnEnginePause(isPaused bool) {
	if !p.IsRunned {
		return
	}
}

type event any

type eventStart struct{}

type eventKeyDown struct {
	Key Key
}

type eventLeftButtonDown struct {
	Pos mathf.Vec2
}

type eventLeftButtonUp struct {
	Pos mathf.Vec2
}

type eventTimer struct {
	Time float64
}

// handleEvent dispatches events to their respective handlers.
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.sinkMgr.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.sinkMgr.doWhenAwake(nil)
		p.sinkMgr.doWhenStart()
	case *eventTimer:
		p.sinkMgr.doWhenTimer(e.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	if p.queueEventWithPolicy(ev) {
		return
	}
	if isDebugInstrEnabled() {
		spxlog.Warn("Event buffer is full (policy=%s). Drop event: %v", p.EventQueuePolicy, ev)
	}
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	return coreruntime.RunEventLoop(me, p.events, p.handleEvent)
}

func (p *Game) initEventLoop() {
	coreruntime.InitLoops(gco.Create, p.eventLoop, p.inputEventLoop, p.logicLoop)
}

func (p *Game) inputEventLoop(me coroutine.Thread) int {
	return coreruntime.RunInputLoop(me, coreruntime.InputLoopConfig{
		CurrentMousePos: func() mathf.Vec2 {
			curMousePos := p.engine().InputMgr.GetGlobalMousePos()
			return mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}
		},
		IsLeftButtonPressed: func() bool {
			return p.engine().InputMgr.GetMouseState(MOUSE_BUTTON_LEFT)
		},
		FireLeftButtonDown: func(point mathf.Vec2) {
			p.fireEvent(&eventLeftButtonDown{Pos: point})
		},
		FireLeftButtonUp: func(point mathf.Vec2) {
			p.fireEvent(&eventLeftButtonUp{Pos: point})
		},
		SetMousePos:  p.inputMgr.setMousePos,
		OnMouseMove:  p.inputMgr.onMouseMove,
		GetKeyEvents: engine.GetKeyEvents,
		OnKeyPressed: func(keyID int64) {
			p.fireEvent(&eventKeyDown{Key: Key(keyID)})
		},
		MouseMovementThreshold: mouseMovementThreshold,
	})
}

const (
	clickTimerGlobal = -1 // Global click cooldown
	clickTimerStage  = 0  // Stage click cooldown
)

type clicker interface {
	threadObj
	doWhenClick(this threadObj)
	getProxy() *engine.Sprite
	Visible() bool
}

func (p *Game) findClickTarget(point mathf.Vec2) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
	return coreruntime.FindClickTarget(p.getTempShapes(), func(item Shape) (coreruntime.ClickSelection[clicker, *SpriteImpl], bool) {
		o, ok := item.(clicker)
		if !ok {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		syncSprite := o.getProxy()
		if syncSprite == nil || !o.Visible() {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		if !p.engine().SpriteMgr.CheckCollisionWithPoint(syncSprite.GetId(), point, true) {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{}, false
		}
		if sprite, ok := o.(*SpriteImpl); ok {
			return coreruntime.ClickSelection[clicker, *SpriteImpl]{Target: o, SwipeTarget: sprite}, true
		}
		return coreruntime.ClickSelection[clicker, *SpriteImpl]{Target: o}, true
	})
}

func (p *Game) doWhenLeftButtonUp(ev *eventLeftButtonUp) {
	p.inputMgr.finishSwipeTracking(ev.Pos)
}

func (p *Game) doWhenLeftButtonDown(ev *eventLeftButtonDown) {
	coreruntime.HandleLeftButtonDown(ev.Pos, coreruntime.ClickDownHooks[clicker, *SpriteImpl, int64]{
		FindTarget: p.findClickTarget,
		BeginSwipe: p.inputMgr.beginSwipeTracking,
		CanTrigger: func(id int64) bool {
			return p.inputMgr.canTriggerClickEvent(id)
		},
		GlobalID: clickTimerGlobal,
		StageID:  clickTimerStage,
		TargetID: func(target clicker) (int64, bool) {
			syncSprite := target.getProxy()
			if syncSprite == nil {
				return 0, false
			}
			return syncSprite.GetId(), true
		},
		DispatchTarget: func(target clicker) {
			target.doWhenClick(target)
		},
		DispatchStage: func() {
			p.sinkMgr.doWhenClick(p)
		},
	})
}

func (p *Game) logicLoop(me coroutine.Thread) int {
	return coreruntime.RunLogicLoop(me, coreruntime.LogicLoopConfig[Shape]{
		Items: p.getTempShapes,
		FlushPendingAudio: func(item Shape, tempAudios []string) []string {
			sprite, ok := item.(*SpriteImpl)
			if !ok {
				return tempAudios
			}
			return sprite.flushPendingAudios(tempAudios)
		},
		FlushCompletedAnimations: func(item Shape, tempAnimations []string) []string {
			sprite, ok := item.(*SpriteImpl)
			if !ok {
				return tempAnimations
			}
			return sprite.flushCompletedAnimations(tempAnimations)
		},
		NextTimer: timer.NextTimer,
		FireTimer: func(targetTimer float64) {
			p.fireEvent(&eventTimer{Time: targetTimer})
		},
		ShowDebugPanel: p.showDebugPanel,
	})
}

// syncUpdatePhysic processes physics trigger events and fires collision callbacks.
func (p *Game) syncUpdatePhysic() {
	triggers := make([]engine.TriggerEvent, 0)
	triggers = engine.GetTriggerEvents(triggers)
	coreruntime.ProcessTriggerPairs(
		triggers,
		func(target any) (*SpriteImpl, bool) {
			sprite, ok := target.(*SpriteImpl)
			return sprite, ok
		},
		isSpriteTouchable,
		func(srcSprite, dstSprite *SpriteImpl) {
			srcSprite.HasOnTouchStart = true
			srcSprite.fireTouchStart(dstSprite)
		},
		func() {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		},
	)
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.IsVisible && !sprite.IsDying
}

// syncUpdateLogic updates game logic and fires start events.
func (p *Game) syncUpdateLogic() error {
	coreruntime.SyncOnce(&p.StartFlag, func() {
		p.fireEvent(&eventStart{})
	})
	return nil
}

// syncEnginePositions synchronizes sprite positions from the physics engine.
// This is done in batch for performance optimization.
func (p *Game) syncEnginePositions() error {
	coreruntime.SyncBatchPositions(
		p.getTempShapes(),
		func(item Shape) bool {
			sprite, ok := item.(*SpriteImpl)
			return ok && sprite.shouldSyncPhysicsPosition()
		},
		func(item Shape) int64 {
			return int64(item.(*SpriteImpl).SyncSprite.Id)
		},
		engine.SyncBatchGetPositions,
		func(item Shape, x, y float64) {
			item.(*SpriteImpl).syncPhysicsPosition(x, y)
		},
	)
	return nil
}

// syncUpdateInput updates input state from the engine.
func (p *Game) syncUpdateInput() {
	coreruntime.SyncMousePos(engine.MainThreadGetMousePos(), p.inputMgr.setMousePos)
}

// syncUpdateProxy updates all sprite proxies and synchronizes them with the engine.
func (p *Game) syncUpdateProxy() {
	p.camera.onUpdate()
	p.spriteMgr.flushActivate()
	p.syncBuffer.Clear()
	p.spriteMgr.syncProxyStates(p.syncBuffer)
	p.spriteMgr.flushDestroy(p.syncBuffer)
	p.flushSyncBuffer()
	p.camera.setDirtyFlag(false)
}

// flushSyncBuffer sends batched updates to the engine if there are any changes.
func (p *Game) flushSyncBuffer() {
	coreruntime.FlushSerializedBuffer(
		p.syncBuffer.UpdateCount(),
		p.syncBuffer.DeleteCount(),
		p.syncBuffer.Serialize,
		engine.SyncBatchUpdateSprites,
	)
}

// checkUpdateCostume schedules a costume update on the main thread.
func checkUpdateCostume(p *baseObj) {
	engine.WaitMainThread(func() {
		syncCheckUpdateCostume(p)
	})
}

// syncCheckUpdateCostume updates sprite costume and layer if they are dirty.
func syncCheckUpdateCostume(p *baseObj) {
	syncSprite := p.SyncSprite
	if p.IsLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.Layer))
		}
		p.IsLayerDirty = false
	}
	if !p.IsCostumeDirty {
		return
	}
	p.IsCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	if p.isCostumeAtlas() {
		rect := p.getCostumeAtlasRegion()
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.IsAnimating)
		syncOnAtlasChanged(p)
		return
	}
	syncSprite.UpdateTexture(path, renderScale, !p.IsAnimating)
}

func syncOnAtlasChanged(p *baseObj) {
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4("atlas_uv_rect2", val, true)
}

// syncCheckInitProxy initializes the sprite's engine proxy if it hasn't been created yet.
func (sprite *SpriteImpl) syncCheckInitProxy() {
	if sprite.SyncSprite != nil || sprite.HasDestroyed {
		return
	}
	sprite.SyncSprite = engine.MainThreadNewSprite(sprite, mathf.NewVec2(sprite.getXYWithRenderOffset()))
	syncInitSpritePhysicInfo(sprite, sprite.SyncSprite)
	sprite.SyncSprite.SetVisible(sprite.IsVisible)
	sprite.SyncSprite.Name = sprite.name
	sprite.SyncSprite.SetTypeName(sprite.name)
	sprite.applyGraphicEffects(true)
	sprite.animation().registerOnAnimationLooped(sprite.syncOnAnimationLooped)
	sprite.animation().registerOnAnimationFinished(sprite.syncOnAnimationFinished)
	sprite.IsDirty = true
}

// syncOnAnimationFinished is called when an animation finishes.
func (sprite *SpriteImpl) syncOnAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" && sprite.SyncSprite != nil {
		sprite.animation().addDonedAnimation(sprite.SyncSprite.GetCurrentAnimName())
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

func syncInitSpritePhysicInfo(sprite *SpriteImpl, syncProxy *engine.Sprite) {
	sprite.physics().syncInitPhysicInfo(syncProxy)
}

func (sprite *SpriteImpl) shouldSyncPhysicsPosition() bool {
	return sprite.SyncSprite != nil && sprite.PhysicsMode() != NoPhysics
}

func (sprite *SpriteImpl) syncPhysicsPosition(x, y float64) {
	revertRenderOffset(sprite, &x, &y)
	sprite.transform().setXY(x, y)
}

func (sprite *SpriteImpl) syncProxyState(buffer *engine.SpriteSyncBuffer) {
	if sprite.HasDestroyed || sprite.SyncSprite == nil {
		return
	}
	if sprite.IsVisible {
		syncCheckUpdateCostume(&sprite.baseObj)
	}
	if !sprite.IsDirty {
		return
	}
	sprite.appendSyncTransform(buffer)
	sprite.IsDirty = false
}

func (sprite *SpriteImpl) appendSyncTransform(buffer *engine.SpriteSyncBuffer) {
	x, y := sprite.getXY()
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scaleX, scaleY := getRenderRotationAndScale(sprite)
	buffer.Add(
		int64(sprite.SyncSprite.Id),
		x+offsetX, y+offsetY,
		engine.DegToRad(rot),
		scaleX, scaleY,
		offsetX, offsetY,
		sprite.IsVisible,
	)
}
