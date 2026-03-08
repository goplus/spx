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
	"os"
	"reflect"

	spxfs "github.com/goplus/spx/v2/fs"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// setupGameConfig configures game settings.
func setupGameConfig(g *Game, conf *Config, proj *projConfig) {
	cwd, _ := os.Getwd()
	runtimeCfg := coreproject.ResolveRuntimeConfig(conf, proj, cwd, os.Getenv("SPX_SCREENSHOT_KEY"))

	conf.Title = runtimeCfg.Title
	proj.FullScreen = runtimeCfg.FullScreen
	g.setPhysicsEnabled(runtimeCfg.PhysicsEnabled)
	g.setEventQueuePolicy(parseEventQueuePolicy(runtimeCfg.EventQueuePolicy))
	g.WindowHeight = runtimeCfg.WindowHeight
	g.WindowWidth = runtimeCfg.WindowWidth

	if runtimeCfg.ScreenshotKey != "" {
		if err := os.Setenv("SPX_SCREENSHOT_KEY", runtimeCfg.ScreenshotKey); err != nil {
			engine.Panic(err)
		}
	}
}

// setupGameSystems initializes game subsystems.
func setupGameSystems(g *Game, proj *projConfig) {
	settings := coreproject.ResolveSystemSettings(proj)
	engine.SetLayerSortMode(settings.LayerSortMode)
	g.applyPathFinderSettings(settings)
	g.applyAudioSettings(settings)
	g.applyPhysicsSettings(settings)
}

// loadGameSprites loads all sprites.
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *projConfig) {
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
	p.soundMgr.init(p)
	p.inputMgr.init(p)
	p.events = make(chan event, eventBufferSize)
	p.resetEventQueueStats()
	p.fs = fs
}

func (p *Game) canBindSprite(name string) bool {
	return p.typs[name] != nil
}
