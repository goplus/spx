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
	"reflect"
	"time"

	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/debug"
	"github.com/goplus/spx/v3/internal/engine"
	engineplatform "github.com/goplus/spx/v3/internal/engine/platform"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

const reloadDrainTimeout = 2 * time.Second

var (
	errReloadInactiveGame = errors.New("game reload requires the active game")
	errReloadWrongThread  = errors.New("game reload requires the engine main thread")
)

// -----------------------------------------------------------------------------
// Entry Points
// -----------------------------------------------------------------------------

func SetDebug(flags dbgFlags) {
	spxlog.SetLevel(spxlog.LevelDebug)
	flags &= DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
	defaultDebugFlags.Store(uint32(flags))
	if g := currentGame(); g != nil {
		g.setDebugFlags(flags)
		return
	}
	gco.SetPerfDebug(flags&DbgFlagPerf != 0)
}

// XGot_Game_Main is required by XGo compiler as the entry of a .gmx project.
func XGot_Game_Main(game Gamer, sprites ...Sprite) {
	g := game.baseGame()
	err := engine.Main(game, g, func() {
		g.initGame(sprites)
		g.gamer = game
	})
	if err != nil {
		panic(err)
	}
}

// XGot_Game_Reload reloads the game with new configuration.
func XGot_Game_Reload(game Gamer, index any) (err error) {
	if gco.IsInCoroutine() {
		return errors.New("game reload cannot be called from an active coroutine")
	}
	g := game.baseGame()
	if currentGame() != g {
		return errReloadInactiveGame
	}
	if !engineplatform.TryCallEngineDirectly(func() {
		err = reloadGame(game, g, index)
	}) {
		return errReloadWrongThread
	}
	return err
}

// -----------------------------------------------------------------------------
// Scheduling
// -----------------------------------------------------------------------------
func SchedNow() int {
	if thread := currentScriptThread(); thread != nil && !handleMainExecutionTimeout(thread) {
		gco.Sched(thread)
	}
	return 0
}

func Sched() int {
	thread := currentScriptThread()
	if thread == nil || handleMainExecutionTimeout(thread) || !thread.IsSchedTimeout(schedTimeoutMs) {
		return 0
	}
	spxlog.Warn("%s\n%s", coreruntime.LoopExecutionTimedOutMsg, debug.GetStackTrace())
	engine.WaitNextFrame()
	engine.Panic(coreruntime.LoopExecutionTimedOutMsg)
	return 0
}

// Current identifies the execution-lock owner, not necessarily the caller.
func currentScriptThread() coroutine.Thread {
	if gco == nil || !gco.IsInCoroutine() {
		return nil
	}
	return gco.Current()
}

func Forever(call func()) {
	coreruntime.Forever(call, engine.NewControlFlowWaiter())
}

func Repeat(loopCount int, call func()) {
	coreruntime.Repeat(loopCount, call, engine.NewControlFlowWaiter())
}

// The __xgo_autoclosure_ prefix preserves XGo's command-style condition syntax.
func RepeatUntil(__xgo_autoclosure_condition func() bool, call func()) {
	coreruntime.RepeatUntil(__xgo_autoclosure_condition, call, engine.NewControlFlowWaiter())
}

func WaitUntil(__xgo_autoclosure_condition func() bool) {
	coreruntime.WaitUntil(__xgo_autoclosure_condition, engine.NewControlFlowWaiter())
}

func reloadGame(game Gamer, g *Game, index any) error {
	v := reflect.ValueOf(game).Elem()
	var plan *reloadPlan
	var generation uint64
	err := engine.Reload(g, reloadDrainTimeout, func() error {
		var err error
		plan, err = prepareReload(g, v, index)
		return err
	}, func() error {
		g.reset()
		generation = g.bootstrapGeneration()
		if err := g.attachPreparedInputSession(); err != nil {
			return err
		}

		g.events = make(chan event, eventBufferSize)

		proj := &plan.project
		g.applyStoredRuntimeConfig(proj)
		setupGameSystems(g, proj)
		if err := plan.loadSprites(g, v); err != nil {
			return err
		}
		g.tilemapMgr.replaceMap(plan.tilemap)
		g.loadStage(v, proj, generation, plan.spriteLoader(g), plan.stage)
		return nil
	}, func() {
		g.initEventLoop()
		g.lifecycleState.IsRunned.Store(true)
	})
	if err != nil {
		return err
	}
	g.startBootstrap(generation)
	return nil
}

func handleMainExecutionTimeout(thread coroutine.Thread) bool {
	if !coreruntime.MainExecutionTimedOut(thread.MainStartedAt(), time.Now(), mainExecTimeoutSec*time.Second) {
		return false
	}
	spxlog.Warn("%s\n%s", coreruntime.MainExecutionTimedOutMsg, debug.GetStackTrace())
	// Warn once, then schedule Main as a regular coroutine.
	thread.DisableMainTimeout()
	gco.Sched(thread)
	return true
}

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}
