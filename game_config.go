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
	"path/filepath"
	"reflect"

	spxfs "github.com/goplus/spx/v2/fs"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// setupGameConfig configures game settings.
func setupGameConfig(g *Game, conf *Config, proj *projConfig) {
	if conf.Title == "" {
		dir, _ := os.Getwd()
		conf.Title = filepath.Base(dir) + " (by XGo Builder)"
	}

	proj.FullScreen = proj.FullScreen || conf.FullScreen
	g.setPhysicsEnabled(proj.Physics)
	g.setEventQueuePolicy(parseEventQueuePolicy(conf.EventQueuePolicy))

	g.windowHeight = conf.Height
	g.windowWidth = conf.Width

	key := conf.ScreenshotKey
	if key == "" {
		key = os.Getenv("SPX_SCREENSHOT_KEY")
	}
	if key != "" {
		if err := os.Setenv("SPX_SCREENSHOT_KEY", key); err != nil {
			engine.Panic(err)
		}
	}
}

// setupGameSystems initializes game subsystems.
func setupGameSystems(g *Game, proj *projConfig) {
	engine.SetLayerSortMode(proj.LayerSortMode)
	g.setupPathFinderConfig(proj)
	g.setupAudioConfig(proj)
	g.setupPhysicsConfig(proj)
}

// loadGameSprites loads all sprites.
func loadGameSprites(g *Game, v reflect.Value, fs spxfs.Dir, proj *projConfig) {
	spxlog.Debug("==> StartLoad")

	g.startLoad(fs)
	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		if fld, ok := val.(Sprite); ok && g.canBindSprite(name) {
			if err := g.loadSprite(fld, name, v); err != nil {
				engine.Panic(err)
			}
		}
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
