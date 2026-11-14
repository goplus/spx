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
	"sync/atomic"
	"time"

	spxfs "github.com/goplus/spx/v4/fs"
	_ "github.com/goplus/spx/v4/fs/asset"
	_ "github.com/goplus/spx/v4/fs/zip"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/ui"
	spxapi "github.com/goplus/spx/v4/pkg/spx"
)

const (
	XGoPackage = true
)

const (
	Gop_sched = "Sched,SchedNow"
)

var (
	scheduler           *coroutine.Scheduler
	spriteInterfaceType = reflect.TypeFor[Sprite]()
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
	schedTimeoutMs              = 3000
	mainExecTimeoutSec          = 3
	initialSpriteSyncBufferSize = 100
	initialPenSyncBufferSize    = 256
	baseScreenWidth             = 480
	baseScreenHeight            = 360
)

// Game represents the main game instance with all core systems.
type Game struct {
	visualObject
	scriptEventBindings
	fs spxfs.Dir

	lifecycle        gameLifecycle
	displayState     gameDisplayState
	dialogState      gameDialogState
	debugState       gameDebugState
	pathfindingState gamePathfindingState
	audioState       gameAudioState
	physicsEnabled   bool
	touchByShape     bool

	Camera Camera
	camera *cameraImpl

	spriteTypes      map[string]reflect.Type
	spritePrototypes map[string]Sprite
	sounds           map[string]sound

	scriptEvents    scriptEventRegistry
	gamer           Gamer
	touchFilters    map[string]*touchFilter
	touchEntries    []touchEntry
	autoTouchLayers bool

	inputMgr   inputManager
	inputSlot  inputSessionSlot
	soundMgr   soundManager
	scene      sceneObjects
	tilemapMgr tilemapManager

	syncBuffer    *engine.SpriteSyncBuffer
	physicsPull   physicsPullBuffer
	penSyncBuffer *engine.PenSyncBuffer
	triggerEvents []engine.TriggerEvent
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

func (p *Game) Username() string {
	return spxapi.Username()
}

func (p *Game) WaitNextFrame() Seconds {
	return engine.WaitNextFrame()
}

func (p *Game) Wait(seconds Seconds) {
	engine.Wait(seconds)
}

func (p *Game) Timer() Seconds {
	return clock.Timer()
}

func (p *Game) ResetTimer() {
	clock.ResetTimer()
}

func (p *Game) Now() time.Time {
	return time.Now()
}

func (p *Game) Dayssince2000() float64 {
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	return time.Since(base).Hours() / 24
}

// Dialog.
func (p *Game) Ask(question any) {
	msgStr := dialogText(question)
	p.ask(true, msgStr, func(answer string) {})
}

func (p *Game) Answer() string {
	return p.dialogState.AnswerVal
}

func (p *Game) initGame(sprites []Sprite) *Game {
	costumeSizeCache.Clear()
	if err := p.attachPreparedInputSession(); err != nil {
		engine.Panic(err)
	}
	p.scene.init()
	p.setDebugFlags(dbgFlags(defaultDebugFlags.Load()))
	p.bindScriptEvents()
	p.spritePrototypes = make(map[string]Sprite)
	p.sounds = make(map[string]sound)
	p.spriteTypes = make(map[string]reflect.Type)
	p.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	p.penSyncBuffer = engine.NewPenSyncBuffer(initialPenSyncBufferSize)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.spriteTypes[tySpr.Name()] = tySpr
	}
	return p
}

func (p *Game) setDebugFlags(flags dbgFlags) {
	p.debugState.DebugInstr = flags&DbgFlagInstr != 0
	p.debugState.DebugEvent = flags&DbgFlagEvent != 0
	scheduler.SetPerfDebug(flags&DbgFlagPerf != 0)
}

func (p *Game) baseGame() *Game {
	return p
}

// showQuestionInPanel controls whether the question appears in the bottom Ask panel.
func (p *Game) ask(showQuestionInPanel bool, question string, callback func(string)) {
	if p.dialogState.AskPanel == nil {
		p.dialogState.AskPanel = ui.NewUiAsk()
		p.scene.add(p.dialogState.AskPanel)
	}
	hasAnswer := false
	p.dialogState.AskPanel.Show(showQuestionInPanel, question, func(msg string) {
		p.dialogState.AnswerVal = msg
		callback(msg)
		hasAnswer = true
	})
	for !hasAnswer {
		p.dialogState.AskPanel.Update()
		engine.WaitNextFrame()
	}
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
