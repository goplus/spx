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
	"sync/atomic"

	spxfs "github.com/goplus/spx/v4/fs"
	_ "github.com/goplus/spx/v4/fs/asset"
	_ "github.com/goplus/spx/v4/fs/zip"

	"github.com/goplus/spx/v4/internal/audio"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	inputstate "github.com/goplus/spx/v4/internal/input"
	itime "github.com/goplus/spx/v4/internal/time"
	"github.com/goplus/spx/v4/internal/ui"
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

var defaultDebugFlags atomic.Uint32

type dbgFlags int

const (
	DbgFlagLoad dbgFlags = 1 << iota
	DbgFlagInstr
	DbgFlagEvent
	DbgFlagPerf
	DbgFlagAll = DbgFlagLoad | DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
)

const (
	MOUSE_BUTTON_LEFT   int64 = inputstate.MouseButtonLeft
	MOUSE_BUTTON_RIGHT  int64 = inputstate.MouseButtonRight
	MOUSE_BUTTON_MIDDLE int64 = inputstate.MouseButtonMiddle
)

const (
	schedTimeoutMs              = 3000
	mainExecTimeoutSec          = 3
	initialSpriteSyncBufferSize = 100
	initialPenSyncBufferSize    = 256
	baseScreenWidth             = 480
	baseScreenHeight            = 360
)

// Game represents the main game instance with all core systems.
type Game struct {
	baseObj
	scriptEventBindings
	gameBootstrapState
	fs spxfs.Dir

	lifecycleState   gameLifecycleState
	displayState     gameDisplayState
	dialogState      gameDialogState
	debugState       gameDebugState
	pathfindingState gamePathfindingState
	audioState       gameAudioState
	physicsEnabled   bool

	Camera Camera
	camera *cameraImpl

	typs   map[string]reflect.Type
	sprs   map[string]Sprite
	sounds map[string]sound

	scriptEvents            scriptEventRegistry
	gamer                   Gamer
	sprCollisionInfos       map[string]*spriteCollisionInfo
	sprCollisionData        []*spriteCollisionData
	isAutoSetCollisionLayer bool

	inputMgr       inputManager
	inputSessionMu sync.RWMutex
	inputSession   *inputSession
	inputTerminal  InputSessionStatus
	inputClaimed   bool
	soundMgr       audio.Manager
	shapeMgr       shapeManager
	tilemapMgr     gameTilemapMgr

	syncBuffer    *engine.SpriteSyncBuffer
	physicsPull   physicsPullBuffer
	penSyncBuffer *engine.PenSyncBuffer
	triggerEvents []engine.TriggerEvent
}

type gameLifecycleState struct {
	RunOnce         sync.Once
	OncePathFinder  sync.Once
	BootstrapDone   atomic.Bool
	StartDispatched atomic.Bool
	IsRunned        atomic.Bool
}

type gameDisplayState struct {
	WorldWidth   int
	WorldHeight  int
	MinWorldX    int
	MinWorldY    int
	MapMode      int
	WindowWidth  int
	WindowHeight int
	WindowScale  float64
	StretchMode  bool
}

type gameDialogState struct {
	AskPanel  *ui.UiAsk
	AnswerVal string
}

type gameDebugState struct {
	Debug      bool
	DebugPanel *ui.UiDebug
	DebugInstr bool
	DebugEvent bool
}

type gamePathfindingState struct {
	PathCellSizeX int
	PathCellSizeY int
}

type gameAudioState struct {
	AudioAttenuation float64
	AudioMaxDistance float64
	SoundObj         engine.Object
}

// gameBootstrapState owns generation-scoped admission and task execution.
// Game coordinates lifecycle gates under the same mutex; callbacks run unlocked.
type gameBootstrapState struct {
	bootstrapMu      sync.Mutex
	bootstrapGen     uint64
	bootstrapStarted bool
	startScheduled   bool
	pendingBootstrap []func()
}

func (p *Game) setDebugFlags(flags dbgFlags) {
	p.debugState.DebugInstr = flags&DbgFlagInstr != 0
	p.debugState.DebugEvent = flags&DbgFlagEvent != 0
	gco.SetPerfDebug(flags&DbgFlagPerf != 0)
}

func (p *Game) reset() {
	p.resetBootstrap()

	p.releaseGameAudio()
	p.EraseAll()

	p.scriptEvents.manager.Reset()
	p.shapeMgr.reset()

	p.debugState.DebugPanel = nil
	p.dialogState.AskPanel = nil

	p.Stop(AllOtherScripts)

	p.lifecycleState.RunOnce = sync.Once{}
	p.lifecycleState.OncePathFinder = sync.Once{}
	p.sprs = make(map[string]Sprite)

	p.abortInputSession("game reset")
	p.resetCollisionLayerState()
	costumeSizeCache.Clear()
	p.inputMgr.init(p)

	itime.ClearTimers()
}

func (p *Game) initGame(sprites []Sprite) *Game {
	costumeSizeCache.Clear()
	if err := p.attachPreparedInputSession(); err != nil {
		engine.Panic(err)
	}
	p.initShapeMgr()
	p.setDebugFlags(dbgFlags(defaultDebugFlags.Load()))
	p.bindScriptEvents()
	p.sprs = make(map[string]Sprite)
	p.sounds = make(map[string]sound)
	p.typs = make(map[string]reflect.Type)
	p.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	p.penSyncBuffer = engine.NewPenSyncBuffer(initialPenSyncBufferSize)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

func (p *Game) baseGame() *Game {
	return p
}

func (p *Game) initShapeMgr() {
	p.shapeMgr.init()
}

func (p *gameBootstrapState) queueBootstrap(generation uint64, call func()) bool {
	if call == nil {
		return false
	}
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	p.pendingBootstrap = append(p.pendingBootstrap, call)
	return true
}

func (p *gameBootstrapState) bootstrapGeneration() uint64 {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	return p.bootstrapGen
}

// Validate and publish under the reset lock so stale work cannot reopen a gate.
func (p *gameBootstrapState) markBootstrapFlag(generation uint64, flag *atomic.Bool) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return false
	}
	flag.Store(true)
	return true
}

func (p *gameBootstrapState) takeBootstrapTasks(generation uint64) []func() {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen {
		return nil
	}
	tasks := p.pendingBootstrap
	p.pendingBootstrap = nil
	return tasks
}

func (p *gameBootstrapState) runBootstrapTasks(generation uint64) {
	for {
		tasks := p.takeBootstrapTasks(generation)
		if len(tasks) == 0 {
			return
		}
		// Also drain tasks queued by earlier tasks.
		for _, task := range tasks {
			if !p.isCurrentBootstrap(generation) {
				return
			}
			task()
		}
	}
}

func (p *gameBootstrapState) isCurrentBootstrap(generation uint64) bool {
	return generation == p.bootstrapGeneration()
}

func (p *gameBootstrapState) claimBootstrap(generation uint64) bool {
	p.bootstrapMu.Lock()
	defer p.bootstrapMu.Unlock()
	if generation != p.bootstrapGen || p.bootstrapStarted {
		return false
	}
	p.bootstrapStarted = true
	return true
}

func currentGame() *Game {
	game, _ := engine.GetGame().(*Game)
	return game
}

func isDebugInstrEnabled() bool {
	if game := currentGame(); game != nil {
		return game.debugState.DebugInstr
	}
	return dbgFlags(defaultDebugFlags.Load())&DbgFlagInstr != 0
}

func isDebugEventEnabled() bool {
	if game := currentGame(); game != nil {
		return game.debugState.DebugEvent
	}
	return dbgFlags(defaultDebugFlags.Load())&DbgFlagEvent != 0
}
