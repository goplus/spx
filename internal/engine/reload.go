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
)

var (
	ErrReloadUnavailable = errors.New("game reload is unavailable during a lifecycle transition")
	ErrReloadIncomplete  = errors.New("game reload aborted: cleanup of existing work is incomplete")
)

// Reload prepares, drains, rebuilds, and activates the game.
// ctx is checked between phases and bounds the shutdown barrier and drain waits.
// Its deadline includes preparation, but cannot interrupt callbacks or acquisition
// of the engine lifecycle and frame locks; it is not a return-time guarantee.
// Once rebuilding starts, activation can finish after ctx expires.
// Failure after stopping leaves the game stopped until reset or backend teardown.
func Reload(ctx context.Context, owner any, prepare, rebuild func() error, activate func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bindingMu.Lock()
	if err := ctx.Err(); err != nil {
		bindingMu.Unlock()
		return err
	}
	binding := activeGame.Load()
	if binding == nil || binding.owner != owner || binding.loadPhase() != gameRunning || updateBusy.Load() {
		bindingMu.Unlock()
		return ErrReloadUnavailable
	}
	binding.storePhase(gameReloading)
	bindingMu.Unlock()
	failurePhase := gameRunning
	defer func() { binding.transition(gameReloading, failurePhase) }()

	// New frames fail the phase gate; wait for a frame already past it.
	updateMu.Lock()
	//lint:ignore SA2001 This lock pair is the frame-drain barrier.
	updateMu.Unlock()
	if !binding.isCurrent(gameReloading) {
		return ErrReloadUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if prepare != nil {
		if err := prepare(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var rebuildErr error
	stop := func() bool {
		if !binding.isCurrent(gameReloading) {
			return false
		}
		// Keep the game stopped if native cleanup panics or calls Goexit.
		failurePhase = gameStopped
		return true
	}
	rebuildGame := func() {
		if !binding.isCurrent(gameReloading) {
			rebuildErr = ErrReloadUnavailable
		} else if rebuild != nil {
			rebuildErr = rebuild()
		}
	}
	if gco == nil {
		if !stop() {
			return ErrReloadUnavailable
		}
		rebuildGame()
	} else {
		stopStarted, err := gco.RunAfterStopIf(ctx, stop, rebuildGame)
		if !stopStarted {
			failurePhase = gameRunning
			if err != nil {
				return err
			}
			return ErrReloadUnavailable
		}
		if err != nil {
			return errors.Join(ErrReloadIncomplete, err)
		}
	}
	if rebuildErr != nil {
		return rebuildErr
	}

	// Admission is open again, so activation can create the new game loops.
	bindingMu.Lock()
	defer bindingMu.Unlock()
	if !binding.isCurrent(gameReloading) {
		return ErrReloadUnavailable
	}
	if activate != nil {
		activate()
	}
	binding.storePhase(gameRunning)
	return nil
}
