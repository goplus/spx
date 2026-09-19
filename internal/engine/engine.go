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
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goplus/spx/v3/internal/engine/profiler"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gde "github.com/goplus/spx/v3/internal/gdengine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	itime "github.com/goplus/spx/v3/internal/time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type (
	Object = gdx.Object
	Array  = gdx.Array
)

const Float2IntFactor = gdx.Float2IntFactor

const coroutineShutdownTimeout = 2 * time.Second

var (
	activeGame atomic.Pointer[gameBinding]
	bindingMu  sync.Mutex
	updateMu   sync.Mutex
	updateBusy atomic.Bool
	logicMu    sync.Mutex
)

var ErrGameAlreadyRunning = errors.New("spx: a game is already running")

type gamePhase uint32

const (
	gameStarting gamePhase = iota
	gameRunning
	gameReloading
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
	destroyReady         atomic.Bool
	backendDestroyed     atomic.Bool
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

// Main holds the single runtime from initialization through backend teardown.
func Main(game IGame, owner any, initialize func()) error {
	binding, err := bindGameAtPhase(game, owner, gameStarting)
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
	if !activateGame(binding, func() {
		binding.link = gde.PrepareLink(callbacks)
	}) {
		return nil
	}
	binding.link.Run(finishStart)
	committed = true
	return nil
}

func OnGameStarted() {
	gco.OnInited()
}

func onStart() {
	defer CheckPanic()
	binding := runningBinding()
	if binding == nil {
		return
	}
	ResetInputState()
	resetTriggerEvents()

	itime.Start(Managers().PlatformMgr.SetTimeScale)
	binding.callbacks.OnEngineStart()
}

func onUpdate(delta float64) {
	defer CheckPanic()
	updateMu.Lock()
	defer updateMu.Unlock()
	updateBusy.Store(true)
	defer updateBusy.Store(false)
	binding := runningBinding()
	if binding == nil {
		return
	}
	profiler.BeginSample()
	defer profiler.EndSample()
	cacheTriggerEvents()
	cacheKeyEvents()
	cacheMouseEvents()
	binding.callbacks.OnEngineBeforeUpdate(delta)
	if !binding.isCurrent(gameRunning) {
		return
	}
	itime.Update(delta, profiler.Calcfps())
	profiler.MeasureFunctionTime("GameUpdate", func() {
		binding.callbacks.OnEngineUpdate(delta)
	})
	if !binding.isCurrent(gameRunning) {
		return
	}
	profiler.MeasureFunctionTime("CoroUpdateJobs", gco.Update)
	if !binding.isCurrent(gameRunning) {
		return
	}
	profiler.MeasureFunctionTime("GameRender", func() {
		binding.callbacks.OnEngineRender(delta)
	})
	if !binding.isCurrent(gameRunning) {
		return
	}
	if err := FlushCaptures(); err != nil {
		Panic(err)
		return
	}
	if !binding.isCurrent(gameRunning) {
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
	drainCoroutines(func() {
		defer func() {
			ResetFrameRuntime()
			binding.destroyReady.Store(true)
			releaseDestroyedGame(binding)
		}()
		binding.callbacks.OnEngineDestroy()
	})
}

// onDestroyed releases the binding after backend teardown.
func onDestroyed() {
	defer CheckPanic()
	binding := activeGame.Load()
	if binding == nil {
		return
	}
	binding.backendDestroyed.Store(true)
	releaseDestroyedGame(binding)
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
	ownsRelease := beginGameClose(binding)
	if !ownsRelease && !binding.resetReleaseDeferred.Load() {
		return
	}
	waitForGameStart(binding)
	if !ownsRelease {
		binding.callbacks.OnEngineReset()
		return
	}

	drainCoroutines(func() {
		defer releaseGameAfter(binding, binding.unlink)
		binding.callbacks.OnEngineReset()
	})
}

// drainCoroutines continues asynchronously if scripts exceed the shutdown timeout.
func drainCoroutines(cleanup func()) {
	co := gco
	if co == nil {
		cleanup()
		return
	}
	if co.RunAfterStopAll(coroutineShutdownTimeout, cleanup) {
		return
	}
	spxlog.Warn("Coroutine shutdown timed out; cleanup continues asynchronously.")
	go func() {
		defer CheckPanic()
		co.RunAfterStopAll(0, cleanup)
	}()
}

func (b *gameBinding) loadPhase() gamePhase {
	return gamePhase(b.phase.Load())
}

func (b *gameBinding) storePhase(phase gamePhase) {
	b.phase.Store(uint32(phase))
}

func (b *gameBinding) isCurrent(phase gamePhase) bool {
	return activeGame.Load() == b && b.loadPhase() == phase
}

func (b *gameBinding) transition(from, to gamePhase) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()
	if !b.isCurrent(from) {
		return false
	}
	b.storePhase(to)
	return true
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
	if binding == nil || activeGame.Load() != binding || binding.loadPhase() == gameClosing {
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

func finishDeferredReset(binding *gameBinding) {
	waitForGameStart(binding)
	releaseGameAfter(binding, binding.unlink)
}

func (b *gameBinding) unlink() {
	if b != nil && b.link != nil {
		b.link.Unlink()
	}
}

func bindGameAtPhase(callbacks IGame, owner any, phase gamePhase) (*gameBinding, error) {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != nil {
		return nil, ErrGameAlreadyRunning
	}
	binding := &gameBinding{callbacks: callbacks, owner: owner}
	binding.storePhase(phase)
	if phase == gameStarting {
		binding.startDone = make(chan struct{})
	}
	activeGame.Store(binding)
	return binding, nil
}

func activateGame(binding *gameBinding, prepare func()) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if !binding.isCurrent(gameStarting) {
		return false
	}
	prepare()
	binding.storePhase(gameRunning)
	return true
}

func abortGameStart(binding *gameBinding) {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != binding || binding.loadPhase() == gameClosing {
		return
	}
	binding.unlink()
	activeGame.Store(nil)
}

func waitForGameStart(binding *gameBinding) {
	if binding != nil && binding.startDone != nil {
		<-binding.startDone
	}
}

func releaseDestroyedGame(binding *gameBinding) bool {
	if binding == nil || !binding.destroyReady.Load() || !binding.backendDestroyed.Load() {
		return false
	}
	return releaseGameAfter(binding, binding.unlink)
}

// releaseGameAfter clears the binding after teardown.
func releaseGameAfter(binding *gameBinding, teardown func()) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != binding {
		return false
	}
	if teardown != nil {
		teardown()
	}
	activeGame.Store(nil)
	return true
}
