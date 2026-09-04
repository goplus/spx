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
	"runtime"
	"time"

	coreruntime "github.com/goplus/spx/v3/internal/core/runtime"
	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/debug"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// -----------------------------------------------------------------------------
// Entry Points
// -----------------------------------------------------------------------------
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

// XGot_Game_Reload reloads the game with new configuration.
func XGot_Game_Reload(game Gamer, index any) (err error) {
	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	if gco.IsInCoroutine() {
		return errors.New("game reload cannot be called from an active coroutine")
	}
	plan, err := prepareReload(g, v, index)
	if err != nil {
		return err
	}
	if !gco.RunAfterAbortAll(2*time.Second, g.reset) {
		return errors.New("game reload aborted: existing coroutines did not stop")
	}
	generation := g.currentBootstrapGeneration()
	if err = g.attachPreparedInputSession(); err != nil {
		return err
	}
	engine.ClearAllSprites()

	g.events = make(chan event, eventBufferSize)
	g.resetEventQueueStats()

	proj := &plan.project
	g.applyStoredRuntimeConfig(proj)
	setupGameSystems(g, proj)
	err = plan.loadSprites(g, v)
	if err != nil {
		engine.Panic(err)
		return
	}
	gco.OnRestart()
	err = g.loadIndexWithSpriteLoader(v, proj, generation, plan.spriteLoader(g))
	if err != nil {
		return
	}
	g.initEventLoop()
	gco.OnInited()
	g.lifecycleState.IsRunned.Store(true)
	g.startBootstrapPhaseFor(generation)
	return
}

// -----------------------------------------------------------------------------
// Scheduling
// -----------------------------------------------------------------------------
func SchedNow() int {
	now := time.Now()
	err := coreruntime.SchedNow(
		coreruntime.ScheduleState{
			IsSchedInMain:   isSchedInMainState(),
			MainSchedTime:   mainSchedTime(),
			Now:             now,
			MainExecTimeout: time.Second * mainExecTimeoutSec,
		},
		coreruntime.SchedulerHooks{
			SchedCurrent: func() {
				if gco.IsInCoroutine() {
					if me := gco.Current(); me != nil {
						gco.Sched(me)
					}
				}
			},
		},
	)
	if handleMainExecutionTimeout(err) {
		return 0
	}
	if err != nil && !errors.Is(err, coreruntime.ErrLoopExecutionTimedOut) {
		engine.Panic(err.Error())
	}
	return 0
}

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
				if gco.IsInCoroutine() {
					if me := gco.Current(); me != nil {
						return me.IsSchedTimeout(ms)
					}
				}
				return false
			},
			OnSchedTimeout: func() {
				spxlog.Warn("%s\n%s", coreruntime.LoopExecutionTimedOutMsg, debug.GetStackTrace())
				engine.WaitNextFrame()
			},
		},
	)
	if handleMainExecutionTimeout(err) {
		return 0
	}
	if err != nil {
		engine.Panic(err.Error())
	}
	return 0
}

func handleMainExecutionTimeout(err error) bool {
	if !errors.Is(err, coreruntime.ErrMainExecutionTimedOut) {
		return false
	}
	spxlog.Warn("%s\n%s", coreruntime.MainExecutionTimedOutMsg, debug.GetStackTrace())
	// Demote long-running top-level Main code to regular coroutine scheduling
	// after the first timeout so it keeps running without repeated panics.
	setSchedInMain(false)
	setMainSchedTime(time.Time{})
	if engine.IsInCoroutine() {
		me := gco.Current()
		if me != nil {
			gco.Sched(me)
		}
	} else {
		runtime.Gosched()
	}
	return true
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

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}
