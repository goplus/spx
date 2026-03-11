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
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"time"

	spxfs "github.com/goplus/spx/v2/fs"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// SetDebug sets debug flags for the game
func SetDebug(flags dbgFlags) {
	spxlog.SetLevel(spxlog.LevelDebug)
	instr := (flags & DbgFlagInstr) != 0
	event := (flags & DbgFlagEvent) != 0
	perf := (flags & DbgFlagPerf) != 0
	setDefaultDebugFlags(instr, event, perf)
	if g := activeGame(); g != nil {
		g.setDebugFlags(instr, event, perf)
	}
}

// XGot_Game_Main is required by XGo compiler as the entry of a .gmx project.
func XGot_Game_Main(game Gamer, sprites ...Sprite) {
	g := game.initGame(sprites)
	g.gamer = game
	engine.Main(game)
}

// XGot_Game_Run runs the game using the builder pattern
func XGot_Game_Run(game Gamer, resource any, gameConf ...*Config) {
	builder := newGameBuilder(game, resource, gameConf...)
	if err := builder.buildAndRun(); err != nil {
		engine.Panic(err)
	}
}

type gameBuilder struct {
	gamer    Gamer
	resource any
	gameConf []*Config

	fs   spxfs.Dir
	conf Config
	proj coreproject.ProjectConfig

	game       *Game
	gamerValue reflect.Value
	err        error
}

func newGameBuilder(game Gamer, resource any, gameConf ...*Config) *gameBuilder {
	return &gameBuilder{
		gamer:    game,
		resource: resource,
		gameConf: gameConf,
	}
}

func (b *gameBuilder) loadResources() *gameBuilder {
	if b.err != nil {
		return b
	}

	var gameConf *Config
	if len(b.gameConf) > 0 {
		gameConf = b.gameConf[0]
	}
	opened, err := coreproject.OpenBuilderResources(b.resource, gameConf)
	if err != nil {
		b.err = err
		return b
	}
	if opened.AssetDir != "" {
		engine.SetAssetDir(opened.AssetDir)
	}
	b.fs = opened.FS
	b.conf = opened.Config
	b.proj = opened.Project

	b.game.engine().ResMgr.SetDefaultFont("res://engine/fonts/CnFont.ttf")
	return b
}

func (b *gameBuilder) parseFlags() *gameBuilder {
	if b.err != nil {
		return b
	}
	parseCommandLineFlags(&b.conf)
	return b
}

func (b *gameBuilder) initializeGame() *gameBuilder {
	if b.err != nil {
		return b
	}
	b.gamerValue = reflect.ValueOf(b.gamer).Elem()
	b.game = instance(b.gamerValue)
	return b
}

func (b *gameBuilder) setupConfig() *gameBuilder {
	if b.err != nil {
		return b
	}
	setupGameConfig(b.game, &b.conf, &b.proj)
	return b
}

func (b *gameBuilder) setupSystems() *gameBuilder {
	if b.err != nil {
		return b
	}
	setupGameSystems(b.game, &b.proj)
	return b
}

func (b *gameBuilder) loadSprites() *gameBuilder {
	if b.err != nil {
		return b
	}
	loadGameSprites(b.game, b.gamerValue, b.fs, &b.proj)
	return b
}

func (b *gameBuilder) finalizeLoad() *gameBuilder {
	if b.err != nil {
		return b
	}
	if err := b.game.endLoad(b.gamerValue, &b.proj); err != nil {
		b.err = err
		return b
	}
	return b
}

func (b *gameBuilder) run() error {
	return b.game.runLoop(&b.conf)
}

func (b *gameBuilder) build() (*Game, error) {
	b.initializeGame().
		loadResources().
		parseFlags().
		setupConfig().
		setupSystems().
		loadSprites().
		finalizeLoad()

	return b.game, b.err
}

func (b *gameBuilder) buildAndRun() error {
	if _, err := b.build(); err != nil {
		return err
	}
	return b.run()
}

// setupGameConfig configures game settings.
func setupGameConfig(g *Game, conf *Config, proj *coreproject.ProjectConfig) {
	cwd, _ := os.Getwd()
	runtimeCfg := coreproject.ResolveRuntimeConfig(conf, proj, cwd, os.Getenv("SPX_SCREENSHOT_KEY"))

	conf.Title = runtimeCfg.Title
	proj.FullScreen = runtimeCfg.FullScreen
	g.setPhysicsEnabled(runtimeCfg.PhysicsEnabled)
	g.setEventQueuePolicy(parseEventQueuePolicy(runtimeCfg.EventQueuePolicy))
	g.displayState.WindowHeight = runtimeCfg.WindowHeight
	g.displayState.WindowWidth = runtimeCfg.WindowWidth

	if runtimeCfg.ScreenshotKey != "" {
		if err := os.Setenv("SPX_SCREENSHOT_KEY", runtimeCfg.ScreenshotKey); err != nil {
			engine.Panic(err)
		}
	}
}

// setupGameSystems initializes game subsystems.
func setupGameSystems(g *Game, proj *coreproject.ProjectConfig) {
	settings := coreproject.ResolveSystemSettings(proj)
	engine.SetLayerSortMode(settings.LayerSortMode)
	g.applyPathFinderSettings(settings)
	g.applyAudioSettings(settings)
	g.applyPhysicsSettings(settings)
}

// loadGameSprites loads all sprites.
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *coreproject.ProjectConfig) {
	spxlog.Debug("==> StartLoad")

	g.startLoad(fs)
	err := coreproject.WalkFields(v, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, v, fieldIndex)
	}, func(name string, val any) error {
		fld, ok := val.(Sprite)
		if !ok || !g.canBindSprite(name) {
			return nil
		}
		return g.loadSprite(fld, name, v)
	})
	if err != nil {
		engine.Panic(err)
	}
	g.tilemapMgr.init(g, fs, proj.TilemapPath)
}

func (p *Game) startLoad(fs spxfs.Dir) {
	p.soundMgr.Init(&p.engine().AudioMgr)
	p.sounds = make(map[string]sound)
	p.inputMgr.init(p)
	p.events = make(chan event, eventBufferSize)
	p.resetEventQueueStats()
	p.fs = fs
}

func (p *Game) canBindSprite(name string) bool {
	return p.typs[name] != nil
}

func parseCommandLineFlags(conf *Config) {
	f := flag.CommandLine
	effects, err := coreproject.ParseCommandLineFlags(f, os.Args[1:], conf)
	if err != nil {
		engine.Panic(err)
	}

	if effects.ShowHelp {
		fmt.Fprintf(os.Stderr, "Usage: %v [-v -f -h]\n", os.Args[0])
		f.PrintDefaults()
		os.Exit(0)
	}

	if effects.Verbose {
		SetDebug(DbgFlagAll)
	}
}

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		panic("type doesn't have field spx.Game")
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val any) {
	return coreproject.FieldPtrOrAlloc(v, i, coreproject.FieldAllocConfig{
		IsPointerSpriteType: func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		ResolveInterfaceSpriteType: func(fieldName string) (reflect.Type, bool) {
			typ, ok := g.typs[fieldName]
			return typ, ok
		},
	})
}

// XGot_Game_Reload reloads the game with new configuration
func XGot_Game_Reload(game Gamer, index any) (err error) {
	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	g.reset()
	engine.ClearAllSprites()

	g.events = make(chan event, eventBufferSize)
	g.resetEventQueueStats()

	err = coreproject.WalkFields(v, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, v, fieldIndex)
	}, func(name string, val any) error {
		fld, ok := val.(Sprite)
		if !ok {
			return nil
		}
		return g.loadSprite(fld, name, v)
	})
	if err != nil {
		engine.Panic(err)
	}
	var proj coreproject.ProjectConfig
	if err = coreproject.LoadConfig(&proj, g.fs, index); err != nil {
		return
	}
	gco.OnRestart()
	err = g.loadIndex(v, &proj)
	gco.OnInited()
	g.initEventLoop()
	return
}

// SchedNow performs immediate scheduling without timeout check
func SchedNow() int {
	err := coreruntime.SchedNow(
		coreruntime.ScheduleState{
			IsSchedInMain:   isSchedInMainState(),
			MainSchedTime:   mainSchedTime(),
			Now:             time.Now(),
			MainExecTimeout: time.Second * mainExecTimeoutSec,
		},
		coreruntime.SchedulerHooks{
			SchedCurrent: func() {
				if me := gco.Current(); me != nil {
					gco.Sched(me)
				}
			},
		},
	)
	if err != nil && !errors.Is(err, coreruntime.ErrLoopExecutionTimedOut) {
		engine.Panic(err.Error())
	}
	return 0
}

// Sched performs scheduling with timeout check
func Sched() int {
	err := coreruntime.Sched(
		coreruntime.ScheduleState{
			IsSchedInMain:   isSchedInMainState(),
			MainSchedTime:   mainSchedTime(),
			Now:             time.Now(),
			MainExecTimeout: time.Second * mainExecTimeoutSec,
		},
		schedTimeoutMs,
		coreruntime.SchedulerHooks{
			IsSchedTimeout: func(ms float64) bool {
				if me := gco.Current(); me != nil {
					return me.IsSchedTimeout(ms)
				}
				return false
			},
			OnSchedTimeout: func() {
				spxlog.Warn("%s\n%s", coreruntime.LoopExecutionTimedOutMsg, debug.GetStackTrace())
				engine.WaitNextFrame()
			},
		},
	)
	if err != nil {
		engine.Panic(err.Error())
	}
	return 0
}

// Forever executes a function indefinitely
func Forever(call func()) {
	coreruntime.Forever(call, func() {
		engine.WaitNextFrame()
	})
}

// Repeat executes a function for a specified number of times
func Repeat(loopCount int, call func()) {
	coreruntime.Repeat(loopCount, call, func() {
		engine.WaitNextFrame()
	})
}

// RepeatUntil executes a function until a condition is met
func RepeatUntil(condition func() bool, call func()) {
	coreruntime.RepeatUntil(condition, call, func() {
		engine.WaitNextFrame()
	})
}

// WaitUntil waits until a condition is met
func WaitUntil(condition func() bool) {
	coreruntime.WaitUntil(condition, func() {
		engine.WaitNextFrame()
	})
}

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}

// runLoop starts the game loop.
func (p *Game) runLoop(cfg *Config) (err error) {
	spxlog.Debug("==> RunLoop")
	if !cfg.DontRunOnUnfocused {
		p.engine().PlatformMgr.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	p.engine().PlatformMgr.SetWindowTitle(cfg.Title)
	p.lifecycleState.IsRunned = true
	return nil
}

// runMain wraps the main execution with scheduler tracking
func runMain(call func()) {
	coreruntime.RunMain(call, time.Now(), setSchedInMain, setMainSchedTime)
}
