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
	"context"
	"errors"
	"sync"
	"sync/atomic"

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
	activeGame atomic.Pointer[gameBinding]
	bindingMu  sync.Mutex
	logicMu    sync.Mutex
)

var ErrGameAlreadyRunning = errors.New("spx: a game is already running")

type gamePhase uint32

const (
	gameStarting gamePhase = iota
	gameRunning
	gameStopped
	gameClosing
)

type gameBinding struct {
	callbacks            IGame
	owner                any
	phase                atomic.Uint32
	startDone            chan struct{}
	link                 *gde.LinkSession
	resetReleaseDeferred atomic.Bool
}

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

// Main reserves the runtime until backend teardown.
func Main(game IGame, owner any, initialize func()) error {
	binding, err := beginGame(game, owner)
	if err != nil {
		return err
	}
	committed := false
	finishStart := sync.OnceFunc(func() { close(binding.startDone) })
	defer func() {
		finishStart()
		if !committed {
			abortGameStart(binding)
		}
	}()

	if initialize != nil {
		initialize()
	}
	enginewrap.Init(WaitMainThread)
	callbacks := gdx.CoreCallbackInfo{
		OnEngineStart:     onStart,
		OnEngineUpdate:    onUpdate,
		OnEngineDestroy:   onDestroy,
		OnEngineDestroyed: onDestroyed,
		OnEngineReset:     onReset,
		OnEnginePause:     onPause,
		OnMousePressed:    onMousePressed,
		OnMouseReleased:   onMouseReleased,
		OnKeyPressed:      onKeyPressed,
		OnKeyReleased:     onKeyReleased,
	}
	if !activateGame(binding, callbacks) {
		return nil
	}
	binding.link.Run(finishStart)
	committed = true
	return nil
}

func (b *gameBinding) loadPhase() gamePhase {
	return gamePhase(b.phase.Load())
}

func (b *gameBinding) storePhase(phase gamePhase) {
	b.phase.Store(uint32(phase))
}

func onStart() {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	itime.Start(Managers().PlatformMgr.SetTimeScale)
	binding.callbacks.OnEngineStart()
}

func onUpdate(delta float64) {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	profiler.BeginSample()
	defer profiler.EndSample()
	cacheFrameEvents()
	binding.callbacks.OnEngineBeforeUpdate(delta)
	itime.Update(delta, profiler.Calcfps())
	Execute(nil, func(context.Context, any) { updateRuntime(delta) })
	if binding.loadPhase() != gameRunning {
		return
	}
	profiler.MeasureFunctionTime("GameUpdate", func() {
		binding.callbacks.OnEngineUpdate(delta)
	})
	profiler.MeasureFunctionTime("CoroUpdateJobs", gco.Update)
	if binding.loadPhase() != gameRunning {
		return
	}
	// Complete the frame once rendering starts.
	profiler.MeasureFunctionTime("GameRender", func() {
		binding.callbacks.OnEngineRender(delta)
	})
	if err := FlushCaptures(); err != nil {
		Panic(err)
		return
	}
	binding.callbacks.OnEngineFrameEnd()
}

func onDestroy() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil || binding.callbacks == nil || !beginGameClose(binding) {
		return
	}
	waitForGameStart(binding)
	drainCoroutines(binding.callbacks.OnEngineDestroy)
}

// Called after Go cleanup and backend teardown.
func onDestroyed() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil {
		return
	}
	releaseGame(binding)
}

func onPause(paused bool) {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	binding.callbacks.OnEnginePause(paused)
}

func onReset() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil || binding.callbacks == nil {
		return
	}
	directReset := beginGameClose(binding)
	waitForGameStart(binding)
	reset := func() {
		resetCaptures()
		resetInputState()
		resetTriggerEvents()
		binding.callbacks.OnEngineReset()
	}
	switch {
	case directReset:
		// Direct editor reset.
		drainCoroutines(func() {
			defer releaseGame(binding)
			reset()
		})
	case binding.resetReleaseDeferred.Load():
		// Web exit handles draining and release.
		reset()
	}
}

func cacheFrameEvents() {
	cacheTriggerEvents()
	cacheKeyEvents()
	cacheMouseEvents()
}

// Drain and clean up synchronously on the caller thread.
func drainCoroutines(cleanup func()) {
	if gco == nil {
		cleanup()
		return
	}
	gco.RunAfterStop(cleanup)
}

func runningBinding() *gameBinding {
	binding := activeGame.Load()
	if binding == nil || binding.callbacks == nil || binding.loadPhase() != gameRunning {
		return nil
	}
	return binding
}

func acceptsRuntimeWork() bool {
	_, accepted := captureRuntimeWork()
	return accepted
}

func captureRuntimeWork() (*gameBinding, bool) {
	binding := activeGame.Load()
	return binding, binding == nil || binding.loadPhase() == gameRunning
}

func isRuntimeWorkCurrent(binding *gameBinding) bool {
	current := activeGame.Load()
	return current == binding && (binding == nil || binding.loadPhase() == gameRunning)
}

func beginGameClose(binding *gameBinding) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()
	if activeGame.Load() != binding || binding.loadPhase() == gameClosing {
		return false
	}
	binding.storePhase(gameClosing)
	return true
}

func beginDeferredReset() (*gameBinding, bool) {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	binding := activeGame.Load()
	if binding != nil && binding.loadPhase() == gameClosing {
		return binding, false
	}
	if binding == nil {
		binding = &gameBinding{}
	}
	binding.storePhase(gameClosing)
	binding.resetReleaseDeferred.Store(true)
	activeGame.Store(binding)
	return binding, true
}

func beginGame(callbacks IGame, owner any) (*gameBinding, error) {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != nil {
		return nil, ErrGameAlreadyRunning
	}
	binding := &gameBinding{
		callbacks: callbacks,
		owner:     owner,
		startDone: make(chan struct{}),
	}
	activeGame.Store(binding)
	return binding, nil
}

func activateGame(binding *gameBinding, callbacks gdx.CoreCallbackInfo) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if binding.loadPhase() != gameStarting {
		return false
	}
	binding.link = gde.PrepareLink(callbacks)
	binding.storePhase(gameRunning)
	return true
}

func abortGameStart(binding *gameBinding) {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != binding || binding.loadPhase() == gameClosing {
		return
	}
	binding.link.Unlink()
	activeGame.Store(nil)
}

func waitForGameStart(binding *gameBinding) {
	if binding.startDone != nil {
		<-binding.startDone
	}
}

func releaseGame(binding *gameBinding) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != binding {
		return false
	}
	binding.link.Unlink()
	activeGame.Store(nil)
	return true
}
