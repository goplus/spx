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
	"reflect"
	"sync"
	"time"

	spxfs "github.com/goplus/spx/v2/fs"
	_ "github.com/goplus/spx/v2/fs/asset"
	_ "github.com/goplus/spx/v2/fs/zip"

	"github.com/goplus/spx/v2/internal/audio"
	"github.com/goplus/spx/v2/internal/base/collision"
	corestate "github.com/goplus/spx/v2/internal/core/state"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	itime "github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/ui"
)

const (
	XGoPackage = true
)

const (
	Gop_sched = "Sched,SchedNow"
)

var (
	gco      *coroutine.Coroutines
	tySprite = reflect.TypeFor[Sprite]()
)

var runtimeStateMgr corestate.RuntimeManager

type dbgFlags int

const (
	DbgFlagLoad dbgFlags = 1 << iota
	DbgFlagInstr
	DbgFlagEvent
	DbgFlagPerf
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
)

const (
	MOUSE_BUTTON_LEFT   int64 = 1
	MOUSE_BUTTON_RIGHT  int64 = 2
	MOUSE_BUTTON_MIDDLE int64 = 3
)

const (
	eventBufferSize             = 16
	schedTimeoutMs              = 3000
	mainExecTimeoutSec          = 3
	mouseMovementThreshold      = 1.0
	initialSpriteSyncBufferSize = 100
	baseScreenWidth             = 480
	baseScreenHeight            = 360
)

// Game represents the main game instance with all core systems.
type Game struct {
	baseObj
	scriptEventBindings
	fs spxfs.Dir

	lifecycleState   corestate.GameLifecycleState
	displayState     corestate.GameDisplayState
	dialogState      corestate.GameDialogState
	debugState       corestate.GameDebugState
	gameRuntimeState corestate.GameRuntimeState
	pathfindingState corestate.GamePathfindingState
	audioState       corestate.GameAudioState

	Camera Camera
	camera *cameraImpl

	typs   map[string]reflect.Type
	sprs   map[string]Sprite
	sounds map[string]sound

	events chan event

	scriptEvents     scriptEventRegistry
	gamer            Gamer
	bootstrapMu      sync.Mutex
	bootstrapGen     uint64
	bootstrapStarted bool
	pendingBootstrap []func()

	sprCollisionInfos       map[string]*spriteCollisionInfo
	sprCollisionData        []*spriteCollisionData
	isCollisionByPixel      bool
	isAutoSetCollisionLayer bool

	engineMgr engineManagers

	inputMgr   inputManager
	soundMgr   audio.Manager
	shapeMgr   shapeManager
	tilemapMgr gameTilemapMgr

	syncBuffer    *engine.SpriteSyncBuffer
	triggerEvents []engine.TriggerEvent
	spatialHash   *collision.SpatialHash[*SpriteImpl]
}

func activeGame() *Game {
	game, _ := engine.GetGame().(*Game)
	return game
}

func (p *Game) initRuntimeState() {
	runtimeStateMgr.Init(&p.debugState, &p.gameRuntimeState)
	syncCoroutinePerfDebug(p.debugState.DebugPerf)
	p.initEventQueueState()
}

func setDefaultDebugFlags(instr, event, perf bool) {
	runtimeStateMgr.SetDefaultDebugFlags(instr, event, perf)
	syncCoroutinePerfDebug(perf)
}

func (p *Game) setDebugFlags(instr, event, perf bool) {
	runtimeStateMgr.ApplyDebugFlags(&p.debugState, instr, event, perf)
	syncCoroutinePerfDebug(perf)
}

// syncCoroutinePerfDebug keeps the process-wide coroutine scheduler aligned with
// the most recently applied perf-debug setting. The scheduler is global, so this
// flag is intentionally global as well.
func syncCoroutinePerfDebug(enabled bool) {
	gco.SetPerfDebug(enabled)
}

func isDebugInstrEnabled() bool {
	return runtimeStateMgr.DebugInstrEnabled(activeGameDebugState())
}

func isDebugEventEnabled() bool {
	return runtimeStateMgr.DebugEventEnabled(activeGameDebugState())
}

func (p *Game) setPhysicsEnabled(enabled bool) {
	runtimeStateMgr.SetPhysicsEnabled(&p.gameRuntimeState, enabled)
}

func isPhysicsEnabled() bool {
	return runtimeStateMgr.PhysicsEnabled(activeGameRuntimeState())
}

func (p *Game) resetImageSizeCache() {
	runtimeStateMgr.ResetImageSizeCache(&p.gameRuntimeState)
}

func imageSizeCacheRef() *sync.Map {
	return runtimeStateMgr.ImageSizeCacheRef(activeGameRuntimeState())
}

func setSchedInMain(inMain bool) {
	runtimeStateMgr.SetSchedInMain(activeGameRuntimeState(), inMain)
}

func isSchedInMainState() bool {
	return runtimeStateMgr.IsSchedInMain(activeGameRuntimeState())
}

func setMainSchedTime(t time.Time) {
	runtimeStateMgr.SetMainSchedTime(activeGameRuntimeState(), t)
}

func mainSchedTime() time.Time {
	return runtimeStateMgr.MainSchedTime(activeGameRuntimeState())
}

func activeGameDebugState() *corestate.GameDebugState {
	if g := activeGame(); g != nil {
		return &g.debugState
	}
	return nil
}

func activeGameRuntimeState() *corestate.GameRuntimeState {
	return runtimeStateOfGame(activeGame())
}

func runtimeStateOfGame(g *Game) *corestate.GameRuntimeState {
	if g == nil {
		return nil
	}
	return &g.gameRuntimeState
}

func (p *Game) newSpriteAndLoad(name string, tySpr reflect.Type, g reflect.Value) Sprite {
	spr := reflect.New(tySpr).Interface().(Sprite)
	if err := p.loadSprite(spr, name, g); err != nil {
		panic(err)
	}
	return spr
}

func (p *Game) getSpriteProto(tySpr reflect.Type, g reflect.Value) Sprite {
	name := tySpr.Name()
	spr, ok := p.sprs[name]
	if !ok {
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) getSpriteProtoByName(name string, g reflect.Value) Sprite {
	spr, ok := p.sprs[name]
	if !ok {
		tySpr, ok := p.typs[name]
		if !ok {
			spxlog.Panicf("sprite %s is not defined", name)
		}
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) reset() {
	p.lifecycleState.IsRunned.Store(false)

	p.releaseGameAudio()
	p.EraseAll()

	p.scriptEvents.Reset()
	p.shapeMgr.reset()

	p.debugState.DebugPanel = nil
	p.dialogState.AskPanel = nil

	p.Stop(AllOtherScripts)

	p.lifecycleState.StartFlag = sync.Once{}
	p.lifecycleState.RunOnce = sync.Once{}
	p.lifecycleState.OncePathFinder = sync.Once{}
	p.lifecycleState.StartDispatched.Store(false)
	p.sprs = make(map[string]Sprite)

	p.resetBootstrapState()
	p.resetCollisionLayerState()
	p.resetImageSizeCache()
	p.resetEventQueueStats()
	close(p.events)

	itime.OnReload()
}

func (p *Game) initGame(sprites []Sprite) *Game {
	engine.SetGame(p)
	p.initShapeMgr()
	p.initRuntimeState()
	p.scriptEventBindings.init(&p.scriptEvents, p)
	p.engineMgr = engineManagers{}
	engine.SetManagers(&p.engineMgr)
	ui.Init(&p.engineMgr)
	p.sprs = make(map[string]Sprite)
	p.sounds = make(map[string]sound)
	p.typs = make(map[string]reflect.Type)
	p.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

func (p *Game) initShapeMgr() {
	p.shapeMgr.init()
}
