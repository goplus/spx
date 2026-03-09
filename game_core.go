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
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/goplus/spx/v2/internal/audiorecord"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/timer"
	"github.com/goplus/spx/v2/internal/ui"

	spxfs "github.com/goplus/spx/v2/fs"
	_ "github.com/goplus/spx/v2/fs/asset"
	_ "github.com/goplus/spx/v2/fs/zip"
)

const (
	GopPackage = true
	Gop_sched  = "Sched,SchedNow"
)

var (
	gco      *coroutine.Coroutines
	tySprite = reflect.TypeOf((*Sprite)(nil)).Elem()
)

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

type Shape any
type threadObj = coroutine.ThreadObj

// Game represents the main game instance with all core systems
type Game struct {
	baseObj
	eventSinks
	fs spxfs.Dir

	Camera Camera
	camera *cameraImpl

	typs map[string]reflect.Type
	sprs map[string]Sprite

	events    chan event
	aurec     *audiorecord.Recorder
	startFlag sync.Once
	runOnce   sync.Once

	worldWidth  int
	worldHeight int
	minWorldX   int
	minWorldY   int
	mapMode     int

	windowWidth  int
	windowHeight int

	sinkMgr  eventSinkMgr
	isLoaded bool
	isRunned bool
	gamer    Gamer

	windowScale float64
	stretchMode bool

	askPanel  *ui.UiAsk
	answerVal string

	oncePathFinder sync.Once
	pathCellSizeX  int
	pathCellSizeY  int

	debug      bool
	debugPanel *ui.UiDebug

	debugInstr bool
	debugEvent bool
	debugPerf  bool

	enabledPhysics   bool
	isSchedInMain    bool
	mainSchedTime    time.Time
	imageSizeCache   sync.Map
	eventQueuePolicy eventQueuePolicy
	eventQueueStats  eventQueueStats

	sprCollisionInfos       map[string]*spriteCollisionInfo
	isCollisionByPixel      bool
	isAutoSetCollisionLayer bool

	audioAttenuation float64
	audioMaxDistance float64
	soundObj         engine.Object

	engineMgr engineManagers

	inputMgr   inputManager
	soundMgr   soundMgr
	spriteMgr  spriteManager
	tilemapMgr gameTilemapMgr

	syncBuffer  *engine.SpriteSyncBuffer
	spatialHash *SpatialHash
}

type Gamer interface {
	engine.IGame
	initGame(sprites []Sprite) *Game
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
			log.Panicf("sprite %s is not defined\n", name)
		}
		spr = p.newSpriteAndLoad(name, tySpr, g)
	}
	return spr
}

func (p *Game) reset() {
	p.isRunned = false

	p.releaseGameAudio()
	p.EraseAll()

	p.sinkMgr.reset()
	p.spriteMgr.reset()

	p.debugPanel = nil
	p.askPanel = nil
	p.isLoaded = false

	p.startFlag = sync.Once{}
	p.runOnce = sync.Once{}
	p.oncePathFinder = sync.Once{}
	p.sprs = make(map[string]Sprite)

	resetImageSizeCache(p)
	p.resetEventQueueStats()
	close(p.events)

	timer.OnReload()
	p.Stop(AllOtherScripts)
}

func (p *Game) initGame(sprites []Sprite) *Game {
	engine.SetGame(p)
	p.initSpriteMgr()
	p.initRuntimeState()
	p.eventSinks.init(&p.sinkMgr, p)
	p.engineMgr = engineManagers{}
	ui.Init(&p.engineMgr)
	p.sprs = make(map[string]Sprite)
	p.typs = make(map[string]reflect.Type)
	p.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	for _, spr := range sprites {
		tySpr := reflect.TypeOf(spr).Elem()
		p.typs[tySpr.Name()] = tySpr
	}
	return p
}

func (p *Game) initSpriteMgr() {
	p.spriteMgr.init()
}
