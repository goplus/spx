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

	corestate "github.com/goplus/spx/v2/internal/core/state"
	"github.com/goplus/spx/v2/internal/engine"
)

var runtimeStateMgr corestate.RuntimeManager

func activeGame() *Game {
	game, _ := engine.GetGame().(*Game)
	return game
}

func (p *Game) initRuntimeState() {
	runtimeStateMgr.Init(&p.GameDebugState, &p.GameRuntimeState)
	p.initEventQueueState()
}

func setDefaultDebugFlags(instr, event, perf bool) {
	runtimeStateMgr.SetDefaultDebugFlags(instr, event, perf)
}

func (p *Game) setDebugFlags(instr, event, perf bool) {
	runtimeStateMgr.ApplyDebugFlags(&p.GameDebugState, instr, event, perf)
}

func isDebugInstrEnabled() bool {
	return runtimeStateMgr.DebugInstrEnabled(activeGameDebugState())
}

func isDebugEventEnabled() bool {
	return runtimeStateMgr.DebugEventEnabled(activeGameDebugState())
}

func isDebugPerfEnabled() bool {
	return runtimeStateMgr.DebugPerfEnabled(activeGameDebugState())
}

func setPhysicsEnabled(enabled bool) {
	runtimeStateMgr.SetPhysicsEnabled(activeGameRuntimeState(), enabled)
}

func (p *Game) setPhysicsEnabled(enabled bool) {
	runtimeStateMgr.SetPhysicsEnabled(&p.GameRuntimeState, enabled)
}

func isPhysicsEnabled() bool {
	return runtimeStateMgr.PhysicsEnabled(activeGameRuntimeState())
}

func resetImageSizeCache(g *Game) {
	runtimeStateMgr.ResetImageSizeCache(gameRuntimeState(g))
}

func imageSizeCacheRef() *sync.Map {
	return runtimeStateMgr.ImageSizeCacheRef(activeGameRuntimeState())
}

func setSchedInMain(inMain bool) {
	runtimeStateMgr.SetSchedInMain(activeGameRuntimeState(), inMain)
}

func isSchedInMainState() bool {
	return runtimeStateMgr.IsSchedInMain(activeGameRuntimeState())
}

func setMainSchedTime(t time.Time) {
	runtimeStateMgr.SetMainSchedTime(activeGameRuntimeState(), t)
}

func mainSchedTime() time.Time {
	return runtimeStateMgr.MainSchedTime(activeGameRuntimeState())
}

func activeGameDebugState() *corestate.GameDebugState {
	if g := activeGame(); g != nil {
		return &g.GameDebugState
	}
	return nil
}

func activeGameRuntimeState() *corestate.GameRuntimeState {
	return gameRuntimeState(activeGame())
}

func gameRuntimeState(g *Game) *corestate.GameRuntimeState {
	if g == nil {
		return nil
	}
	return &g.GameRuntimeState
}
