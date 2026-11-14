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
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/clock"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
)

func SetDebug(flags dbgFlags) {
	spxlog.SetLevel(spxlog.LevelDebug)
	flags &= DbgFlagInstr | DbgFlagEvent | DbgFlagPerf
	defaultDebugFlags.Store(uint32(flags))
	if g := currentGame(); g != nil {
		g.setDebugFlags(flags)
		return
	}
	scheduler.SetPerfDebug(flags&DbgFlagPerf != 0)
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

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// DeltaTime returns the time elapsed since the previous frame.
func DeltaTime() Seconds {
	return clock.DeltaTime()
}

// TimeSinceLevelLoad returns the time elapsed since the current level started.
func TimeSinceLevelLoad() Seconds {
	return clock.TimeSinceLevelLoad()
}

// Exit__0 exits the program with exit code 0.
func Exit__0() {
	engine.RequestExit(0)
}

// Exit__1 exits the program with the specified exit code.
func Exit__1(code int) {
	engine.RequestExit(int64(code))
}

func init() {
	scheduler = coroutine.New(engine.OnPanic)
	engine.SetScheduler(scheduler)
}

// capture runs fn, if present, and queues a screenshot on success.
// Frame and input tick are captured after fn returns.
func capture(name string, fn func() error) {
	if fn != nil {
		if err := fn(); err != nil {
			engine.Panic(err)
			return
		}
	}

	if err := engine.EnqueueCapture(name, captureInputTick()); err != nil {
		engine.Panic(err)
	}
}
