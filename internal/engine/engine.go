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

package engine

import (
	"sync"

	"github.com/goplus/spx/v3/internal/engine/profiler"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gde "github.com/goplus/spx/v3/internal/gdengine"
	itime "github.com/goplus/spx/v3/internal/time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type (
	Object = gdx.Object
	Array  = gdx.Array
)

const Float2IntFactor = gdx.Float2IntFactor

var (
	currentGame IGame
	logicMu     sync.Mutex
)

type IGame interface {
	OnEngineStart()
	OnEngineUpdate(delta float64)
	OnEngineBeforeUpdate(delta float64)
	OnEngineRender(delta float64)
	OnEngineFrameEnd()
	OnEngineDestroy()
	OnEngineReset()
	OnEnginePause(isPaused bool)
}

func ConvertToFloat64(value int64) float64 {
	return float64(value) / Float2IntFactor
}

func ConvertToInt64(value float64) int64 {
	return int64(value * Float2IntFactor)
}

func Lock() {
	logicMu.Lock()
}

func Unlock() {
	logicMu.Unlock()
}

func Main(game IGame) {
	enginewrap.Init(WaitMainThread)
	currentGame = game
	gde.Link(gdx.CoreCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
		OnEngineReset:   onReset,
		OnEnginePause:   onPause,
		OnMousePressed:  onMousePressed,
		OnMouseReleased: onMouseReleased,
		OnKeyPressed:    onKeyPressed,
		OnKeyReleased:   onKeyReleased,
	})
}

func OnGameStarted() {
	gco.OnInited()
}

func onStart() {
	defer CheckPanic()
	resetInputState()
	resetTriggerEvents()

	itime.Start(func(scale float64) {
		Managers().PlatformMgr.SetTimeScale(scale)
	})
	currentGame.OnEngineStart()
}

func onUpdate(delta float64) {
	defer CheckPanic()
	profiler.BeginSample()
	defer profiler.EndSample()
	cacheTriggerEvents()
	cacheKeyEvents()
	cacheMouseEvents()
	currentGame.OnEngineBeforeUpdate(delta)
	itime.Update(delta, profiler.Calcfps())
	profiler.MeasureFunctionTime("GameUpdate", func() {
		currentGame.OnEngineUpdate(delta)
	})
	profiler.MeasureFunctionTime("CoroUpdateJobs", func() {
		gco.Update()
	})
	profiler.MeasureFunctionTime("GameRender", func() {
		currentGame.OnEngineRender(delta)
	})
	if err := FlushCaptures(); err != nil {
		Panic(err)
		return
	}
	currentGame.OnEngineFrameEnd()
}

func onDestroy() {
	defer CheckPanic()
	currentGame.OnEngineDestroy()
}

func onPause(paused bool) {
	defer CheckPanic()
	currentGame.OnEnginePause(paused)
}

func onReset() {
	defer CheckPanic()
	defer gde.Unlink()
	currentGame.OnEngineReset()
}
