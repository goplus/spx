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
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
)

// engineManagers is an alias for the internal EngineManagers type.
// It groups all engine-facing manager wrappers for a Game instance.
type engineManagers = enginewrap.EngineManagers

func (p *Game) engine() *engineManagers {
	return &p.engineMgr
}

func (p *SpriteImpl) engine() *engineManagers {
	return p.g.engine()
}

func (c *componentBase) engine() *engineManagers {
	return c.sprite.engine()
}

var (
	// cachedBounds stores cached sprite bounds for performance optimization
	cachedBounds map[string]mathf.Rect2
)

// OnEngineStart is called when the engine starts.
// It initializes the game and starts the main game loop.
func (p *Game) OnEngineStart() {
	p.runOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		onStart := func() {
			defer engine.CheckPanic()
			gamer := p.gamer
			if me, ok := gamer.(interface{ MainEntry() }); ok {
				runMain(me.MainEntry)
			}
			if !p.isRunned {
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
	if !p.isRunned {
		return
	}
	p.syncUpdateInput()
	p.syncUpdateLogic()
	p.syncUpdateProxy()
	p.syncEnginePositions()
}

// OnEngineRender is called every frame to render the game.
func (p *Game) OnEngineRender(delta float64) {
	if !p.isRunned {
		return
	}
	p.syncUpdatePhysic()
}

// OnEnginePause is called when the engine is paused or resumed.
func (p *Game) OnEnginePause(isPaused bool) {
	if !p.isRunned {
		return
	}
}
