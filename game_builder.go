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

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
)

// gameBuilder provides a fluent interface for initializing and running a game
type gameBuilder struct {
	gamer    Gamer
	resource any
	gameConf []*Config

	fs   spxfs.Dir
	conf Config
	proj projConfig

	game       *Game
	gamerValue reflect.Value
	err        error // stores first error encountered during build process
}

// newGameBuilder creates a new game builder
func newGameBuilder(game Gamer, resource any, gameConf ...*Config) *gameBuilder {
	return &gameBuilder{
		gamer:    game,
		resource: resource,
		gameConf: gameConf,
	}
}

// loadResources loads filesystem and configuration
func (b *gameBuilder) loadResources() *gameBuilder {
	if b.err != nil {
		return b
	}

	switch resfld := b.resource.(type) {
	case string:
		if resfld != "" {
			engine.SetAssetDir(resfld)
		} else {
			engine.SetAssetDir("assets")
		}
	}

	fs, err := resourceDir(b.resource)
	if err != nil {
		b.err = err
		return b
	}
	b.fs = fs

	resMgr.SetDefaultFont("res://engine/fonts/CnFont.ttf")
	engine.RegisterFileSystem(fs)

	if b.gameConf != nil {
		b.conf = *b.gameConf[0]
		err = loadProjConfig(&b.proj, fs, b.conf.Index)
	} else {
		err = loadProjConfig(&b.proj, fs, nil)
		if b.proj.Run != nil {
			b.conf = *b.proj.Run
		}
	}
	if err != nil {
		b.err = err
		return b
	}

	return b
}

// parseFlags parses command line flags and updates configuration
func (b *gameBuilder) parseFlags() *gameBuilder {
	if b.err != nil {
		return b
	}
	parseCommandLineFlags(&b.conf)
	return b
}

// initializeGame initializes the game instance
func (b *gameBuilder) initializeGame() *gameBuilder {
	if b.err != nil {
		return b
	}
	b.gamerValue = reflect.ValueOf(b.gamer).Elem()
	b.game = instance(b.gamerValue)
	return b
}

// setupConfig sets up game configuration and global settings
func (b *gameBuilder) setupConfig() *gameBuilder {
	if b.err != nil {
		return b
	}
	setupGameConfig(b.game, &b.conf, &b.proj)
	return b
}

// setupSystems initializes game subsystems (collision, physics, audio, etc.)
func (b *gameBuilder) setupSystems() *gameBuilder {
	if b.err != nil {
		return b
	}
	setupGameSystems(b.game, &b.proj)
	return b
}

// loadSprites loads all game sprites
func (b *gameBuilder) loadSprites() *gameBuilder {
	if b.err != nil {
		return b
	}
	loadGameSprites(b.game, b.gamerValue, b.fs, &b.proj)
	return b
}

// finalizeLoad completes the loading process
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

// run starts the game loop
func (b *gameBuilder) run() error {
	return b.game.runLoop(&b.conf)
}

// build executes the complete build pipeline and returns the game instance
func (b *gameBuilder) build() (*Game, error) {
	b.loadResources().
		parseFlags().
		initializeGame().
		setupConfig().
		setupSystems().
		loadSprites().
		finalizeLoad()

	return b.game, b.err
}

// buildAndRun executes the complete build pipeline and starts the game
func (b *gameBuilder) buildAndRun() error {
	if _, err := b.build(); err != nil {
		return err
	}
	return b.run()
}
