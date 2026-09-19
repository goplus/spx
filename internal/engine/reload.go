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
	"time"
)

var (
	ErrReloadUnavailable = errors.New("game reload is unavailable during a lifecycle transition")
	ErrReloadTimeout     = errors.New("game reload aborted: existing coroutines did not stop")
)

// Reload prepares a replacement, drains scripts, rebuilds the game, then starts
// its new loops. Only preparation is reversible: once scripts are canceled,
// failure leaves the game stopped until reset or backend teardown.
func Reload(owner any, timeout time.Duration, prepare, rebuild func() error, activate func()) error {
	bindingMu.Lock()
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
	if prepare != nil {
		if err := prepare(); err != nil {
			return err
		}
	}

	var rebuildErr error
	stop := func() bool {
		if !binding.isCurrent(gameReloading) {
			return false
		}
		failurePhase = gameStopped
		return true
	}
	apply := func() {
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
		apply()
	} else {
		selected, completed := gco.RunAfterStopAllIf(timeout, stop, apply)
		if !selected {
			return ErrReloadUnavailable
		}
		if !completed {
			return ErrReloadTimeout
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
