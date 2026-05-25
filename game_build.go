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
	"flag"
	"fmt"
	"os"
	"reflect"

	spxfs "github.com/goplus/spx/v2/fs"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

type gameBuilder struct {
	gamer    Gamer
	resource any
	gameConf []*Config

	fs   spxfs.Dir
	conf Config
	proj coreproject.ProjectConfig

	game       *Game
	gamerValue reflect.Value
	generation uint64
	err        error
}

// -----------------------------------------------------------------------------
// Builder
// -----------------------------------------------------------------------------
func newGameBuilder(game Gamer, resource any, generation uint64, gameConf ...*Config) *gameBuilder {
	return &gameBuilder{
		gamer:      game,
		resource:   resource,
		gameConf:   gameConf,
		generation: generation,
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

	resMgr := b.game.engine().ResMgr
	display := coreproject.ResolveDisplaySettings(&b.proj)
	coreproject.RegisterDisplayFonts(display, resMgr.SetDefaultFont, resMgr.RegisterSvgFontFace)
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
	if err := b.game.endLoad(b.gamerValue, &b.proj, b.generation); err != nil {
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

// -----------------------------------------------------------------------------
// Setup
// -----------------------------------------------------------------------------
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

func setupGameSystems(g *Game, proj *coreproject.ProjectConfig) {
	settings := coreproject.ResolveSystemSettings(proj)
	if settings.AutoSetCollisionLayer == isPhysicsEnabled() {
		engine.Panic("invalid configuration: autoSetCollisionLayer and physics enabled state must not be the same")
	}
	engine.SetLayerSortMode(settings.LayerSortMode)
	g.applyPathFinderSettings(settings)
	g.applyAudioSettings(settings)
	g.applyPhysicsSettings(settings)
}

// -----------------------------------------------------------------------------
// Loading
// -----------------------------------------------------------------------------
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *coreproject.ProjectConfig) {
	spxlog.Debug("StartLoad")

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
