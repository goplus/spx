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
	"sync"
	"time"

	"github.com/goplus/spx/v2/internal/engine"
)

var (
	defaultDebugInstr bool
	defaultDebugEvent bool
	defaultDebugPerf  bool

	defaultPhysicsEnabled bool

	fallbackSchedInMain bool
	fallbackMainSchedAt time.Time

	fallbackImageSizeCache sync.Map
)

func activeGame() *Game {
	game, _ := engine.GetGame().(*Game)
	return game
}

func (p *Game) initRuntimeState() {
	p.debugInstr = defaultDebugInstr
	p.debugEvent = defaultDebugEvent
	p.debugPerf = defaultDebugPerf
	p.enabledPhysics = defaultPhysicsEnabled
	p.initEventQueueState()
}

func setDefaultDebugFlags(instr, event, perf bool) {
	defaultDebugInstr = instr
	defaultDebugEvent = event
	defaultDebugPerf = perf
}

func (p *Game) setDebugFlags(instr, event, perf bool) {
	p.debugInstr = instr
	p.debugEvent = event
	p.debugPerf = perf
}

func isDebugInstrEnabled() bool {
	if g := activeGame(); g != nil {
		return g.debugInstr
	}
	return defaultDebugInstr
}

func isDebugEventEnabled() bool {
	if g := activeGame(); g != nil {
		return g.debugEvent
	}
	return defaultDebugEvent
}

func isDebugPerfEnabled() bool {
	if g := activeGame(); g != nil {
		return g.debugPerf
	}
	return defaultDebugPerf
}

func setPhysicsEnabled(enabled bool) {
	defaultPhysicsEnabled = enabled
	if g := activeGame(); g != nil {
		g.enabledPhysics = enabled
	}
}

func (p *Game) setPhysicsEnabled(enabled bool) {
	p.enabledPhysics = enabled
	defaultPhysicsEnabled = enabled
}

func isPhysicsEnabled() bool {
	if g := activeGame(); g != nil {
		return g.enabledPhysics
	}
	return defaultPhysicsEnabled
}

func resetImageSizeCache(g *Game) {
	if g != nil {
		g.imageSizeCache = sync.Map{}
		return
	}
	fallbackImageSizeCache = sync.Map{}
}

func imageSizeCacheRef() *sync.Map {
	if g := activeGame(); g != nil {
		return &g.imageSizeCache
	}
	return &fallbackImageSizeCache
}

func setSchedInMain(inMain bool) {
	if g := activeGame(); g != nil {
		g.isSchedInMain = inMain
		return
	}
	fallbackSchedInMain = inMain
}

func isSchedInMainState() bool {
	if g := activeGame(); g != nil {
		return g.isSchedInMain
	}
	return fallbackSchedInMain
}

func setMainSchedTime(t time.Time) {
	if g := activeGame(); g != nil {
		g.mainSchedTime = t
		return
	}
	fallbackMainSchedAt = t
}

func mainSchedTime() time.Time {
	if g := activeGame(); g != nil {
		return g.mainSchedTime
	}
	return fallbackMainSchedAt
}
