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
	"time"

	"github.com/goplus/spbase/mathf"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Engine Callbacks
// -----------------------------------------------------------------------------
func (p *Game) OnEngineStart() {
	p.lifecycleState.RunOnce.Do(func() {
		cachedBounds = make(map[string]mathf.Rect2)
		onStart := func() {
			defer engine.CheckPanic()
			gamer := p.gamer
			if me, ok := gamer.(interface{ MainEntry() }); ok {
				runMain(me.MainEntry)
			}
			if !p.lifecycleState.IsRunned {
				builder := newGameBuilder(gamer, "assets")
				if err := builder.buildAndRun(); err != nil {
					engine.Panic(err)
				}
			}
			engine.OnGameStarted()
		}
		go onStart()
	})
}

func (p *Game) OnEngineDestroy() {
}

func (p *Game) OnEngineReset() {
	p.reset()
}

func (p *Game) OnEngineUpdate(delta float64) {
	if !p.lifecycleState.IsRunned {
		return
	}
	p.dispatchStartEventIfNeeded()
	p.updateSpriteProxies()
	p.pullPhysicsPositions()
}

func (p *Game) OnEngineRender(delta float64) {
	if !p.lifecycleState.IsRunned {
		return
	}
	p.processPhysicsTriggers()
}

func (p *Game) OnEnginePause(isPaused bool) {
	if !p.lifecycleState.IsRunned {
		return
	}
}

// -----------------------------------------------------------------------------
// Loop Setup
// -----------------------------------------------------------------------------
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

func runMain(call func()) {
	coreruntime.RunMain(call, time.Now(), setSchedInMain, setMainSchedTime)
}
