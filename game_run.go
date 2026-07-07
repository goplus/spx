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

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
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
	g.reset()
	generation := g.currentBootstrapGeneration()
	engine.ClearAllSprites()

	g.events = make(chan event, eventBufferSize)
	g.resetEventQueueStats()

	err = coreproject.WalkFields(v, func(fieldIndex int) (string, any) {
		return getFieldPtrOrAlloc(g, v, fieldIndex)
	}, func(name string, val any) error {
		fld, ok := val.(Sprite)
		if !ok {
			return nil
		}
		return g.loadSprite(fld, name, v)
	})
	if err != nil {
		engine.Panic(err)
		return
	}
	var proj coreproject.ProjectConfig
	if err = coreproject.LoadConfig(&proj, g.fs, index); err != nil {
		return
	}
	g.applyStoredRuntimeConfig(&proj)
	setupGameSystems(g, &proj)
	gco.OnRestart()
	err = g.loadIndex(v, &proj, generation)
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
	err := coreruntime.SchedNow(
		coreruntime.ScheduleState{
			IsSchedInMain:   isSchedInMainState(),
			MainSchedTime:   mainSchedTime(),
			Now:             time.Now(),
			MainExecTimeout: time.Second * mainExecTimeoutSec,
		},
		coreruntime.SchedulerHooks{
			SchedCurrent: func() {
				if me := gco.Current(); me != nil {
					gco.Sched(me)
				}
			},
		},
	)
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
				if me := gco.Current(); me != nil {
					return me.IsSchedTimeout(ms)
				}
				return false
			},
			OnSchedTimeout: func() {
				spxlog.Warn("%s\n%s", coreruntime.LoopExecutionTimedOutMsg, debug.GetStackTrace())
				engine.WaitNextFrame()
			},
		},
	)
	if err != nil {
		engine.Panic(err.Error())
	}
	return 0
}

func Forever(call func()) {
	coreruntime.Forever(call, func() {
		waitNextFrameForControlFlow()
	})
}

func Repeat(loopCount int, call func()) {
	coreruntime.Repeat(loopCount, call, func() {
		waitNextFrameForControlFlow()
	})
}

func RepeatUntil(condition func() bool, call func()) {
	coreruntime.RepeatUntil(condition, call, func() {
		waitNextFrameForControlFlow()
	})
}

func WaitUntil(condition func() bool) {
	coreruntime.WaitUntil(condition, func() {
		waitNextFrameForControlFlow()
	})
}

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}
