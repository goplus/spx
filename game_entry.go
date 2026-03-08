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
	"time"

	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// SetDebug sets debug flags for the game
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

// XGot_Game_Run runs the game using the builder pattern
func XGot_Game_Run(game Gamer, resource any, gameConf ...*Config) {
	builder := newGameBuilder(game, resource, gameConf...)
	if err := builder.buildAndRun(); err != nil {
		engine.Panic(err)
	}
}

// XGot_Game_Reload reloads the game with new configuration
func XGot_Game_Reload(game Gamer, index any) (err error) {
	v := reflect.ValueOf(game).Elem()
	g := instance(v)
	g.reset()
	engine.ClearAllSprites()

	g.events = make(chan event, eventBufferSize)
	g.resetEventQueueStats()

	for i, n := 0, v.NumField(); i < n; i++ {
		name, val := getFieldPtrOrAlloc(g, v, i)
		if fld, ok := val.(Sprite); ok {
			if err := g.loadSprite(fld, name, v); err != nil {
				engine.Panic(err)
			}
		}
	}
	var proj projConfig
	if err = loadProjConfig(&proj, g.fs, index); err != nil {
		return
	}
	gco.OnRestart()
	err = g.loadIndex(v, &proj)
	gco.OnInited()
	g.initEventLoop()
	return
}

// SchedNow performs immediate scheduling without timeout check
func SchedNow() int {
	if isSchedInMainState() {
		if time.Since(mainSchedTime()) >= time.Second*mainExecTimeoutSec {
			engine.Panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	}
	if me := gco.Current(); me != nil {
		gco.Sched(me)
	}
	return 0
}

// Sched performs scheduling with timeout check
func Sched() int {
	if isSchedInMainState() {
		if time.Since(mainSchedTime()) >= time.Second*mainExecTimeoutSec {
			engine.Panic("Main execution timed out. Please check if there is an infinite loop in the code.")
		}
	} else if me := gco.Current(); me != nil {
		if me.IsSchedTimeout(schedTimeoutMs) {
			spxlog.Warn("For loop execution timed out. Please check if there is an infinite loop in the code.\n%s", debug.GetStackTrace())
			engine.WaitNextFrame()
		}
	}
	return 0
}

// Forever executes a function indefinitely
func Forever(call func()) {
	if call == nil {
		return
	}
	for {
		call()
		engine.WaitNextFrame()
	}
}

// Repeat executes a function for a specified number of times
func Repeat(loopCount int, call func()) {
	if call == nil {
		return
	}
	for range loopCount {
		call()
		engine.WaitNextFrame()
	}
}

// RepeatUntil executes a function until a condition is met
func RepeatUntil(condition func() bool, call func()) {
	if call == nil || condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		call()
		engine.WaitNextFrame()
	}
}

// WaitUntil waits until a condition is met
func WaitUntil(condition func() bool) {
	if condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		engine.WaitNextFrame()
	}
}

func init() {
	gco = coroutine.New(engine.OnPanic)
	engine.SetCoroutines(gco)
}

// runLoop starts the game loop.
func (p *Game) runLoop(cfg *Config) (err error) {
	spxlog.Debug("==> RunLoop")
	if !cfg.DontRunOnUnfocused {
		p.engine().PlatformMgr.SetRunnableOnUnfocused(true)
	}
	p.initEventLoop()
	p.engine().PlatformMgr.SetWindowTitle(cfg.Title)
	p.isRunned = true
	return nil
}

// runMain wraps the main execution with scheduler tracking
func runMain(call func()) {
	setSchedInMain(true)
	setMainSchedTime(time.Now())
	defer setSchedInMain(false)
	call()
}
