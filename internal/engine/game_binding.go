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

	gde "github.com/goplus/spx/v4/internal/gdengine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

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

var (
	activeGame atomic.Pointer[gameBinding]
	bindingMu  sync.Mutex
)

var ErrGameAlreadyRunning = errors.New("spx: a game is already running")

// SetGame installs a non-runtime owner binding and cannot replace Main.
func SetGame(game any) {
	bindingMu.Lock()
	defer bindingMu.Unlock()
	if binding := activeGame.Load(); binding != nil && binding.callbacks != nil {
		panic("engine: SetGame cannot replace an active runtime")
	}
	if game == nil {
		activeGame.Store(nil)
		return
	}
	binding := &gameBinding{owner: game}
	binding.storePhase(gameRunning)
	activeGame.Store(binding)
}

func GetGame() any {
	if binding := activeGame.Load(); binding != nil {
		return binding.owner
	}
	return nil
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
	if !beginGameClose(binding) {
		return
	}
	drainAndReleaseGame(scheduler, binding, nil)
}

func waitForGameStart(binding *gameBinding) {
	if binding.startDone != nil {
		<-binding.startDone
	}
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

// releaseGame follows script drain and game cleanup, and releases session state
// before another game can bind. It never destroys objects through the backend.
func releaseGame(binding *gameBinding) bool {
	bindingMu.Lock()
	defer bindingMu.Unlock()

	if activeGame.Load() != binding {
		return false
	}
	binding.link.Unlink()
	resetRuntimeState()
	resetFrameState()
	activeGame.Store(nil)
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

func (b *gameBinding) loadPhase() gamePhase {
	return gamePhase(b.phase.Load())
}

func (b *gameBinding) storePhase(phase gamePhase) {
	b.phase.Store(uint32(phase))
}
