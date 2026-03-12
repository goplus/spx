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
	coreevent "github.com/goplus/spx/v2/internal/core/event"
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

type threadObj = coroutine.ThreadObj
type eventSink = coreevent.Sink

type scriptEventBindings struct {
	*scriptEventRegistry
	pthis threadObj
}

type scriptEventRegistry struct {
	coreevent.Manager
}

type eventQueuePolicy = coreevent.QueuePolicy

const defaultEventQueuePolicy = coreevent.DefaultQueuePolicy

type eventQueueSnapshot = coreevent.QueueSnapshot

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

var (
	// cachedBounds stores cached sprite bounds for performance optimization.
	cachedBounds map[string]mathf.Rect2
)

func (p *Game) engine() *engineManagers {
	return &p.engineMgr
}

func (p *SpriteImpl) engine() *engineManagers {
	return p.g.engine()
}

func (c *componentBase) engine() *engineManagers {
	return c.sprite.engine()
}

// -----------------------------------------------------------------------------
// Script Event Bindings
// -----------------------------------------------------------------------------

func (p *scriptEventBindings) init(mgr *scriptEventRegistry, this threadObj) {
	p.scriptEventRegistry = mgr
	p.pthis = this
}

func (p *scriptEventBindings) initFrom(src *scriptEventBindings, this threadObj) {
	p.scriptEventRegistry = src.scriptEventRegistry
	p.pthis = this
}

func (p *scriptEventBindings) doDeleteClone() {
	p.scriptEventRegistry.DeleteOwner(p.pthis)
}

func (p *scriptEventBindings) doWhenSwipe(direction Direction, target threadObj) {
	p.scriptEventRegistry.doWhenSwipe(direction, target)
}

func (p *scriptEventBindings) onAwake(onAwake func()) {
	pthis := p.pthis
	p.scriptEventRegistry.AddAwake(coreevent.NewSink(p.pthis, onAwake, coreevent.MatchOwnerOrNil(pthis)))
}

func nameOf(this any) string {
	if spr, ok := this.(*SpriteImpl); ok {
		return spr.name
	}
	if _, ok := this.(*Game); ok {
		return "Game"
	}
	engine.Panic("scriptEventBindings: unexpected this object")
	return ""
}

func isGame(obj threadObj) bool {
	_, ok := obj.(*Game)
	return ok
}

func isSprite(obj threadObj) bool {
	_, ok := obj.(*SpriteImpl)
	return ok
}

// Registration helpers exposed to Game and SpriteImpl.

func (p *scriptEventBindings) OnStart(onStart func()) {
	sink := coreevent.NewSink(p.pthis, onStart)
	if p.scriptEventRegistry.TryAddStart(sink) {
		return
	}
	if sprite, ok := sink.Owner.(*SpriteImpl); ok && sprite.spriteState.Cloned {
		return
	}
	spxlog.Warn("event: ignoring late OnStart registration for %s", nameOf(sink.Owner))
}

func (p *scriptEventBindings) OnClick(onClick func()) {
	pthis := p.pthis
	p.scriptEventRegistry.AddClick(coreevent.NewSink(pthis, onClick, coreevent.MatchOwner(pthis)))
}

func (p *scriptEventBindings) OnAnyKey(onKey func(key Key)) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(p.pthis, onKey))
}

func (p *scriptEventBindings) OnTimer(time float64, call func()) {
	timer.RegisterTimer(time)
	p.scriptEventRegistry.AddTimer(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(call, coreevent.If1(isDebugEventEnabled, func(float64) {
			spxlog.Debug("==> onTimer: %s", nameOf(p.pthis))
		})),
		coreevent.MatchApproxFloat(time, 0.001),
	))
}

func (p *scriptEventBindings) OnKey__0(key Key, onKey func()) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onKey, coreevent.If1(isDebugEventEnabled, func(Key) {
			spxlog.Debug("==> onKey: %v, %s", key, nameOf(p.pthis))
		})),
		coreevent.MatchValue(key),
	))
}

func (p *scriptEventBindings) OnSwipe__0(direction Direction, onSwipe func()) {
	p.scriptEventRegistry.AddSwipe(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onSwipe, coreevent.If1(isDebugEventEnabled, func(Direction) {
			spxlog.Debug("==> onSwipe: %v, %s", direction, nameOf(p.pthis))
		})),
		coreevent.MatchValue(direction),
	))
}

func (p *scriptEventBindings) OnKey__1(keys []Key, onKey func(Key)) {
	p.scriptEventRegistry.AddKeyPressed(coreevent.NewSink(
		p.pthis,
		coreevent.Tap1(onKey, coreevent.If1(isDebugEventEnabled, func(key Key) {
			spxlog.Debug("==> onKey: %v, %s", keys, nameOf(p.pthis))
		})),
		coreevent.MatchAnyOf(keys),
	))
}

func (p *scriptEventBindings) OnKey__2(keys []Key, onKey func()) {
	p.OnKey__1(keys, coreevent.Ignore1[Key](onKey))
}

func (p *scriptEventBindings) OnMsg__0(onMsg func(msg MsgName, data any)) {
	p.scriptEventRegistry.AddIReceive(coreevent.NewSink(p.pthis, onMsg))
}

func (p *scriptEventBindings) OnMsg__1(msg MsgName, onMsg func()) {
	p.scriptEventRegistry.AddIReceive(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid2(onMsg, coreevent.If2(isDebugEventEnabled, func(msg string, data any) {
			spxlog.Debug("==> onMsg: %s, %s", msg, nameOf(p.pthis))
		})),
		coreevent.MatchValue(msg),
	))
}

func (p *scriptEventBindings) OnBackdrop__0(onBackdrop func(name BackdropName)) {
	p.scriptEventRegistry.AddBackdropChanged(coreevent.NewSink(p.pthis, onBackdrop))
}

func (p *scriptEventBindings) OnBackdrop__1(name BackdropName, onBackdrop func()) {
	p.scriptEventRegistry.AddBackdropChanged(coreevent.NewSink(
		p.pthis,
		coreevent.TapVoid1(onBackdrop, coreevent.If1(isDebugEventEnabled, func(name BackdropName) {
			spxlog.Debug("==> onBackdrop: %s, %s", name, nameOf(p.pthis))
		})),
		coreevent.MatchValue(name),
	))
}

func (p *scriptEventBindings) Stop(kind StopKind) {
	current := gco.Current()
	filter, abort := coreevent.ResolveStop(
		kind,
		p.pthis,
		func(obj any) bool { return isSprite(obj) },
		func(obj any) bool { return isGame(obj) },
	)
	if filter != nil {
		gco.StopIf(func(th coroutine.Thread) bool {
			return filter(th.Obj, th == current)
		})
	}
	if abort {
		gco.Abort()
	}
}

// -----------------------------------------------------------------------------
// Event Queue State
// -----------------------------------------------------------------------------

func parseEventQueuePolicy(policy string) eventQueuePolicy {
	return coreevent.ParsePolicy(policy)
}

func (p *Game) initEventQueueState() {
	p.gameRuntimeState.EventQueuePolicy = defaultEventQueuePolicy
	p.gameRuntimeState.EventQueueStats.Reset()
}

func (p *Game) resetEventQueueStats() {
	p.gameRuntimeState.EventQueueStats.Reset()
}

func (p *Game) setEventQueuePolicy(policy eventQueuePolicy) {
	p.gameRuntimeState.EventQueuePolicy = policy
}

func (p *Game) eventQueueSnapshot() eventQueueSnapshot {
	queueLen, queueCap := 0, 0
	if p.events != nil {
		queueLen = len(p.events)
		queueCap = cap(p.events)
	}
	return coreevent.Snapshot(p.gameRuntimeState.EventQueuePolicy, &p.gameRuntimeState.EventQueueStats, queueLen, queueCap)
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	return coreevent.EnqueueWithPolicy(p.events, ev, p.gameRuntimeState.EventQueuePolicy, &p.gameRuntimeState.EventQueueStats, &p.gameRuntimeState.EventQueueMu)
}

// -----------------------------------------------------------------------------
// Engine Lifecycle
// -----------------------------------------------------------------------------

// OnEngineStart is called when the engine starts.
// It initializes the game and starts the main game loop.
func (p *Game) OnEngineStart() {
	p.lifecycleState.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		onStart := func() {
			defer engine.CheckPanic()
			gamer := p.gamer
			if me, ok := gamer.(interface{ MainEntry() }); ok {
				runMain(me.MainEntry)
			}
			if !p.lifecycleState.IsRunned {
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
	if !p.lifecycleState.IsRunned {
		return
	}
	p.updateInputState()
	p.dispatchStartEventIfNeeded()
	p.updateSpriteProxies()
	p.pullPhysicsPositions()
}

// OnEngineRender is called every frame to render the game.
func (p *Game) OnEngineRender(delta float64) {
	if !p.lifecycleState.IsRunned {
		return
	}
	p.processPhysicsTriggers()
}

// OnEnginePause is called when the engine is paused or resumed.
func (p *Game) OnEnginePause(isPaused bool) {
	if !p.lifecycleState.IsRunned {
		return
	}
}

// -----------------------------------------------------------------------------
// Event Dispatch
// -----------------------------------------------------------------------------

// handleEvent dispatches events to their respective handlers.
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.scriptEvents.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.scriptEvents.doWhenAwake(nil)
		p.scriptEvents.doWhenStart()
	case *eventTimer:
		p.scriptEvents.doWhenTimer(e.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	if p.queueEventWithPolicy(ev) {
		return
	}
	if isDebugInstrEnabled() {
		spxlog.Warn("Event buffer is full (policy=%s). Drop event: %v", p.gameRuntimeState.EventQueuePolicy, ev)
	}
}

func (p *scriptEventRegistry) doWhenStart() {
	sinks := p.SnapshotStartOnce()
	if len(sinks) == 0 {
		return
	}
	coreevent.DispatchAsync(sinks, false, nil, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onStart: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenAwake(this threadObj) {
	sinks := p.SnapshotAwake()
	coreevent.DispatchSync(sinks, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onAwake: %s", nameOf(ev.Owner))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenTimer(time float64) {
	sinks := p.SnapshotTimer()
	coreevent.DispatchAsync(sinks, false, time, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(float64))(time)
	})
}

func (p *scriptEventRegistry) doWhenKeyPressed(key Key) {
	sinks := p.SnapshotKeyPressed()
	coreevent.DispatchAsync(sinks, false, key, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(Key))(key)
	})
}

func (p *scriptEventRegistry) doWhenSwipe(direction Direction, this threadObj) {
	sinks := p.SnapshotSwipe()
	coreevent.DispatchAsync(sinks, false, direction, eventDispatchHooks(), func(ev *eventSink) {
		if ev.Owner == this {
			ev.Handler.(func(Direction))(direction)
		}
	})
}

func (p *scriptEventRegistry) doWhenClick(this threadObj) {
	sinks := p.SnapshotClick()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onClick: %s", nameOf(this))
		})()
		ev.Handler.(func())()
	})
}

func (p *scriptEventRegistry) doWhenTouchStart(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchStart()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchStart: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenTouching(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouching()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onTouching: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenTouchEnd(this threadObj, obj *SpriteImpl) {
	sinks := p.SnapshotTouchEnd()
	coreevent.DispatchAsync(sinks, false, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("===> onTouchEnd: %s, %s", nameOf(this), obj.name)
		})()
		ev.Handler.(func(Sprite))(obj.sprite)
	})
}

func (p *scriptEventRegistry) doWhenCloned(this threadObj, data any) {
	sinks := p.SnapshotCloned()
	coreevent.DispatchAsync(sinks, true, this, eventDispatchHooks(), func(ev *eventSink) {
		coreevent.If0(isDebugEventEnabled, func() {
			spxlog.Debug("==> onCloned: %s", nameOf(this))
		})()
		ev.Handler.(func(any))(data)
	})
}

func (p *scriptEventRegistry) doWhenIReceive(msg string, data any, wait bool) {
	sinks := p.SnapshotIReceive()
	coreevent.Dispatch(sinks, wait, msg, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(string, any))(msg, data)
	})
}

func (p *scriptEventRegistry) doWhenBackdropChanged(name BackdropName, wait bool) {
	sinks := p.SnapshotBackdropChanged()
	coreevent.Dispatch(sinks, wait, name, eventDispatchHooks(), func(ev *eventSink) {
		ev.Handler.(func(BackdropName))(name)
	})
}

func eventDispatchHooks() coreevent.DispatchHooks {
	return coreevent.DispatchHooks{
		Spawn: func(start bool, owner any, call func()) {
			gco.CreateAndStart(start, owner, func(coroutine.Thread) int {
				call()
				return 0
			})
		},
		Wait: engine.WaitToDo,
	}
}

// -----------------------------------------------------------------------------
// Runtime Loops
// -----------------------------------------------------------------------------

func (p *Game) initEventLoop() {
	coreruntime.InitLoops(gco.Create, p.eventLoop, p.inputEventLoop, p.logicLoop)
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	return coreruntime.RunEventLoop(me, p.events, p.handleEvent)
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
			p.scriptEvents.doWhenClick(p)
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

// -----------------------------------------------------------------------------
// Frame Synchronization
// -----------------------------------------------------------------------------

// updateInputState refreshes input state from the engine.
func (p *Game) updateInputState() {
	coreruntime.SyncMousePos(engine.MainThreadGetMousePos(), p.inputMgr.setMousePos)
}

// dispatchStartEventIfNeeded fires the start event once after the game begins running.
func (p *Game) dispatchStartEventIfNeeded() error {
	coreruntime.SyncOnce(&p.lifecycleState.StartFlag, func() {
		p.fireEvent(&eventStart{})
	})
	return nil
}

// updateSpriteProxies activates pending shapes, batches dirty sprite proxy changes,
// processes pending destroys, flushes the batch to the engine, and updates camera state.
func (p *Game) updateSpriteProxies() {
	p.camera.onUpdate()
	p.spriteMgr.flushActivate()
	p.syncBuffer.Clear()
	p.spriteMgr.collectProxyUpdates(p.syncBuffer)
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

// pullPhysicsPositions retrieves sprite positions from the physics engine in batch.
func (p *Game) pullPhysicsPositions() error {
	coreruntime.SyncBatchPositions(
		p.getTempShapes(),
		func(item Shape) bool {
			sprite, ok := item.(*SpriteImpl)
			return ok && sprite.shouldPullPhysicsPosition()
		},
		func(item Shape) int64 {
			return int64(item.(*SpriteImpl).runtimeState.SyncSprite.Id)
		},
		engine.SyncBatchGetPositions,
		func(item Shape, x, y float64) {
			item.(*SpriteImpl).applyPhysicsPosition(x, y)
		},
	)
	return nil
}

// processPhysicsTriggers consumes trigger events and fires collision callbacks.
func (p *Game) processPhysicsTriggers() {
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
			srcSprite.spriteState.HasOnTouchStart = true
			srcSprite.fireTouchStart(dstSprite)
		},
		func() {
			spxlog.Info("Physics error: unexpected trigger pair - invalid sprite types")
		},
	)
}

func isSpriteTouchable(sprite *SpriteImpl) bool {
	return sprite.spriteState.IsVisible && !sprite.spriteState.IsDying
}

// -----------------------------------------------------------------------------
// Costume and Proxy Synchronization
// -----------------------------------------------------------------------------

// scheduleCostumeUpdate schedules a costume update on the main thread.
func (p *baseObj) scheduleCostumeUpdate() {
	engine.WaitMainThread(func() {
		p.applyCostumeUpdate()
	})
}

// applyCostumeUpdate pushes pending costume and layer changes to the proxy.
func (p *baseObj) applyCostumeUpdate() {
	syncSprite := p.runtimeState.SyncSprite
	if p.runtimeState.IsLayerDirty {
		if !engine.HasLayerSortMethod() {
			syncSprite.SetZIndex(int64(p.runtimeState.Layer))
		}
		p.runtimeState.IsLayerDirty = false
	}
	if !p.runtimeState.IsCostumeDirty {
		return
	}
	p.runtimeState.IsCostumeDirty = false
	path := p.getCostumePath()
	renderScale := p.getCostumeRenderScale()
	if p.isCostumeAtlas() {
		rect := p.getCostumeAtlasRegion()
		syncSprite.UpdateTextureAtlas(path, rect, renderScale, !p.runtimeState.IsAnimating)
		p.applyAtlasUVRemap()
		return
	}
	syncSprite.UpdateTexture(path, renderScale, !p.runtimeState.IsAnimating)
}

func (p *baseObj) applyAtlasUVRemap() {
	uvRemap := p.getCostumeAtlasUvRemap()
	val := mathf.NewVec4(uvRemap.Position.X, uvRemap.Position.Y, uvRemap.Size.X, uvRemap.Size.Y)
	p.setMaterialParamsVec4("atlas_uv_rect2", val, true)
}

// Sprite proxy lifecycle hooks.

// ensureProxyInitialized initializes the sprite's engine proxy if it hasn't been created yet.
func (sprite *SpriteImpl) ensureProxyInitialized() {
	if sprite.runtimeState.SyncSprite != nil || sprite.isDestroyed() {
		return
	}
	sprite.runtimeState.SyncSprite = engine.MainThreadNewSprite(sprite, mathf.NewVec2(sprite.getXYWithRenderOffset()))
	sprite.applyPhysicsProxyConfig()
	sprite.runtimeState.SyncSprite.SetVisible(sprite.spriteState.IsVisible)
	sprite.runtimeState.SyncSprite.Name = sprite.name
	sprite.runtimeState.SyncSprite.SetTypeName(sprite.name)
	sprite.applyGraphicEffects(true)
	sprite.animation().registerOnAnimationLooped(sprite.handleAnimationLooped)
	sprite.animation().registerOnAnimationFinished(sprite.handleAnimationFinished)
	sprite.spriteState.IsDirty = true
}

// handleAnimationFinished records completed animation events from the proxy.
func (sprite *SpriteImpl) handleAnimationFinished() {
	engine.Lock()
	defer engine.Unlock()
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	state := sprite.animation().getCurAnimState()
	if state != nil && state.Name != "" {
		sprite.animation().addDonedAnimation(sprite.runtimeState.SyncSprite.GetCurrentAnimName())
	}
}

// handleAnimationLooped records looped animation audio playback requests.
func (sprite *SpriteImpl) handleAnimationLooped() {
	engine.Lock()
	defer engine.Unlock()
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	state := sprite.animation().getCurTweenState()
	if state != nil && state.AudioName != "" {
		sprite.sound().addPendingAudio(state.AudioName)
	}
}

func (sprite *SpriteImpl) applyPhysicsProxyConfig() {
	sprite.physics().applyPhysicsProxyConfig(sprite.runtimeState.SyncSprite)
}

func (sprite *SpriteImpl) shouldPullPhysicsPosition() bool {
	return sprite.runtimeState.SyncSprite != nil && sprite.PhysicsMode() != NoPhysics
}

func (sprite *SpriteImpl) applyPhysicsPosition(x, y float64) {
	revertRenderOffset(sprite, &x, &y)
	sprite.transform().setXY(x, y)
}

func (sprite *SpriteImpl) collectProxyUpdate(buffer *engine.SpriteSyncBuffer) {
	if sprite.isDestroyed() || sprite.runtimeState.SyncSprite == nil {
		return
	}
	if sprite.spriteState.IsVisible {
		sprite.baseObj.applyCostumeUpdate()
	}
	if !sprite.spriteState.IsDirty {
		return
	}
	sprite.appendTransformUpdate(buffer)
	sprite.spriteState.IsDirty = false
}

func (sprite *SpriteImpl) appendTransformUpdate(buffer *engine.SpriteSyncBuffer) {
	x, y := sprite.getXY()
	offsetX, offsetY := getRenderOffset(sprite)
	rot, scaleX, scaleY := getRenderRotationAndScale(sprite)
	buffer.Add(
		int64(sprite.runtimeState.SyncSprite.Id),
		x+offsetX, y+offsetY,
		engine.DegToRad(rot),
		scaleX, scaleY,
		offsetX, offsetY,
		sprite.spriteState.IsVisible,
	)
}
