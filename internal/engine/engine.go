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
	"fmt"
	"sync"
	stime "time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine/platform"
	"github.com/goplus/spx/v3/internal/engine/profiler"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gde "github.com/goplus/spx/v3/internal/gdengine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

// Shared engine managers.
var (
	platformMgr enginewrap.PlatformMgrImpl
	resMgr      enginewrap.ResMgrImpl
	extMgr      enginewrap.ExtMgrImpl
)

type Object = gdx.Object
type Array = gdx.Array

type layerSortMode int

const (
	layerSortModeNone layerSortMode = iota
	layerSortModeVertical
)

type LayerSortInfo struct {
	X      float64
	Y      float64
	Sprite *Sprite
}

var curLayerSortMode layerSortMode

// SetLayerSortMode sets sprite layer sorting.
// Supported modes:
//   - "" or "none": disable sorting (default)
//   - "vertical": sort by Y, then X, both descending
//
// When enabled, manual layer changes are disabled.
func SetLayerSortMode(s string) error {
	switch s {
	case "", "none":
		curLayerSortMode = layerSortModeNone
	case "vertical":
		curLayerSortMode = layerSortModeVertical
	default:
		return fmt.Errorf("unknown layer sort mode: %s", s)
	}

	extMgr.SetLayerSorterMode(int64(curLayerSortMode))
	return nil
}

func HasLayerSortMethod() bool {
	return curLayerSortMode != layerSortModeNone
}

const Float2IntFactor = gdx.Float2IntFactor

func ConvertToFloat64(val int64) float64 {
	return float64(val) / Float2IntFactor
}
func ConvertToInt64(val float64) int64 {
	return int64(val * Float2IntFactor)
}

type TriggerEvent struct {
	Src *Sprite
	Dst *Sprite
}

var (
	game              IGame
	triggerEventsTemp []TriggerEvent
	triggerEvents     []TriggerEvent
	triggerMutex      sync.Mutex

	logicMutex sync.Mutex
)

func Lock() {
	logicMutex.Lock()
}

func Unlock() {
	logicMutex.Unlock()
}

type IGame interface {
	OnEngineStart()
	OnEngineUpdate(delta float64)
	OnEngineRender(delta float64)
	OnEngineFrameEnd()
	OnEngineDestroy()
	OnEngineReset()
	OnEnginePause(isPaused bool)
}

func Main(g IGame) {
	enginewrap.Init(WaitMainThread)
	game = g
	gde.Link(gdx.CoreCallbackInfo{
		OnEngineStart:   onStart,
		OnEngineUpdate:  onUpdate,
		OnEngineDestroy: onDestroy,
		OnEngineReset:   onReset,
		OnEnginePause:   onPaused,
		OnMousePressed:  onMousePressed,
		OnMouseReleased: onMouseReleased,
		OnKeyPressed:    onKeyPressed,
		OnKeyReleased:   onKeyReleased,
	})
}

func OnGameStarted() {
	gco.OnInited()
}

// Engine callbacks.
func onStart() {
	defer CheckPanic()
	resetInputState()
	triggerEventsTemp = make([]TriggerEvent, 0)
	triggerEvents = make([]TriggerEvent, 0)

	time.Start(func(scale float64) {
		platformMgr.SetTimeScale(scale)
	})
	game.OnEngineStart()
}

func onUpdate(delta float64) {
	defer CheckPanic()
	profiler.BeginSample()
	defer profiler.EndSample()
	updateTime(delta)
	cacheTriggerEvents()
	cacheKeyEvents()
	cacheMouseEvents()
	profiler.MeasureFunctionTime("GameUpdate", func() {
		game.OnEngineUpdate(delta)
	})
	profiler.MeasureFunctionTime("CoroUpdateJobs", func() {
		gco.Update()
	})
	profiler.MeasureFunctionTime("GameRender", func() {
		game.OnEngineRender(delta)
	})
	if err := FlushCaptures(); err != nil {
		Panic(err)
		return
	}
	game.OnEngineFrameEnd()
}

func onDestroy() {
	defer CheckPanic()
	game.OnEngineDestroy()
}

func onPaused(isPaused bool) {
	defer CheckPanic()
	game.OnEnginePause(isPaused)
}

func onReset() {
	defer CheckPanic()
	defer gde.Unlink()
	game.OnEngineReset()
}

func updateTime(delta float64) {
	time.Update(delta, profiler.Calcfps())
}

func cacheTriggerEvents() {
	triggerMutex.Lock()
	triggerEvents = append(triggerEvents, triggerEventsTemp...)
	triggerMutex.Unlock()
	triggerEventsTemp = triggerEventsTemp[:0]
}

func GetTriggerEvents(lst []TriggerEvent) []TriggerEvent {
	triggerMutex.Lock()
	lst = append(lst, triggerEvents...)
	triggerEvents = triggerEvents[:0]
	triggerMutex.Unlock()
	return lst
}

// DeferPanic recovers a panic, reports it, and optionally exits.
func DeferPanic(name, stack string, exitOnPanic bool) {
	if e := recover(); e != nil {
		handlePanic(name, stack, e, exitOnPanic)
	}
}

// CheckPanic is a shorthand panic handler for engine callbacks.
func CheckPanic() {
	if e := recover(); e != nil {
		handlePanic("", "", e, true)
	}
}

// OnPanic reports a panic with an optional name and stack.
func OnPanic(name, stack string) {
	handlePanic(name, stack, nil, true)
}

// handlePanic reports a panic and optionally exits.
func handlePanic(name, stack string, err any, exitOnPanic bool) {
	var msg string
	if err != nil {
		msg = fmt.Sprintf("panic: %v", err)
	}
	if name != "" {
		if msg != "" {
			msg = name + ": " + msg
		} else {
			msg = name
		}
	}
	if stack != "" {
		msg += "\nstack:\n" + stack
	}

	if msg != "" {
		spxlog.Error("%s", msg)
	}

	extMgr.OnRuntimePanic(msg)

	if exitOnPanic {
		RequestExit(1)
	}
}

// Panic reports a panic message through the engine.
func Panic(args ...any) {
	msg := fmt.Sprint(args...)
	OnPanic(msg, "")
}

// Panicf reports a formatted panic message through the engine.
func Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	OnPanic(msg, "")
}

// abortCoroutinesAndReset aborts coroutines and resets the engine.
// Used on web, where the process cannot exit.
func abortCoroutinesAndReset(exitCode int64) {
	co := gco
	// Panic handling has already dropped the caller identity but keeps its
	// thread registered until the handler returns, so the drain must never run
	// inline with the caller.
	go requestResetAfterCoroutinesStop(co, 2*stime.Second, func() {
		extMgr.RequestReset(exitCode)
	})
	if co.IsInCoroutine() {
		co.Abort()
	}
}

func requestResetAfterCoroutinesStop(co *coroutine.Coroutines, timeout stime.Duration, requestReset func()) bool {
	completed := co.RunAfterAbortAll(timeout, func() {
		co.WaitMainThread(requestReset)
	})
	if !completed {
		spxlog.Error("Coroutine shutdown timed out; engine reset was not requested.")
		return false
	}
	spxlog.Debug("Coroutine shutdown completed. Engine reset requested.")
	return true
}

func RequestExit(exitCode int64) {
	if platform.IsWeb() {
		// Web resets instead of exiting.
		abortCoroutinesAndReset(exitCode)
		return
	}
	extMgr.RequestExit(exitCode)
}
