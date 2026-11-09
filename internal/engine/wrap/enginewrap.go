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

package enginewrap

//lint:file-ignore ST1001 Engine manager extension code intentionally dot-imports engine API types.

import (
	"slices"

	. "github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine/managerstate"
	"github.com/goplus/spx/v4/internal/engine/platform"
	"github.com/goplus/spx/v4/internal/input"
	spxlog "github.com/goplus/spx/v4/internal/log"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

// A nil binding is a zero-value facade. A non-nil binding keeps its own backend,
// including after release, when that backend is cleared and calls must fail.
type managerBinding[T comparable] struct {
	backend T
	name    string
}

var mainCallback func(call func())

func Init(call func(f func())) {
	managerstate.Mu.Lock()
	mainCallback = call
	managerstate.Mu.Unlock()
}

// BindManagers installs main-thread wrappers as one registry update.
// Cleanup invalidates the set without clearing replacements or waiting for
// calls that already acquired a backend.
func BindManagers(managers []gdx.IManager) (cleanup func()) {
	managerstate.Mu.Lock()
	defer managerstate.Mu.Unlock()

	var releases []func()
	releaseLocked := func() {
		for _, release := range slices.Backward(releases) {
			release()
		}
		releases = nil
	}
	defer func() {
		if cleanup == nil {
			releaseLocked()
		}
	}()
	for _, manager := range managers {
		releases = append(releases, bindManager(manager))
	}
	return func() {
		managerstate.Mu.Lock()
		defer managerstate.Mu.Unlock()
		releaseLocked()
	}
}

// CallInMainThread runs call synchronously on the engine main thread.
// Without a dispatcher, only calls already on the engine thread may run.
func CallInMainThread(call func()) {
	managerstate.Mu.RLock()
	callback := mainCallback
	managerstate.Mu.RUnlock()
	if callback == nil {
		if platform.TryCallEngineDirectly(call) {
			return
		}
		panic("enginewrap: Init must be called before using manager methods off the main thread")
	}
	callback(call)
}

// CallInMainThreadValue runs call synchronously on the engine main thread and
// returns its result.
func CallInMainThreadValue[T any](call func() T) (result T) {
	CallInMainThread(func() {
		result = call()
	})
	return
}

func (c *CameraMgrImpl) GetPosition() Vec2 {
	return c.GetCameraPosition()
}

func (c *CameraMgrImpl) SetPosition(position Vec2) {
	c.SetCameraPosition(position)
}

func (i *InputMgrImpl) MousePressed() bool {
	binding := i.resolve()
	return CallInMainThreadValue(func() bool {
		target := binding.target()
		return target.GetMouseState(input.MouseButtonLeft) || target.GetMouseState(input.MouseButtonRight)
	})
}

func (*PlatformMgrImpl) SetRunnableOnUnfocused(flag bool) {
	if !flag {
		spxlog.Warn("SetRunnableOnUnfocused(false) is not implemented yet")
	}
}

func (m *managerBinding[T]) binding() *managerBinding[T] { return m }

// target rechecks the captured binding after dispatch. A queued facade call
// must never resolve to a replacement session's backend.
func (m *managerBinding[T]) target() T {
	managerstate.Mu.RLock()
	defer managerstate.Mu.RUnlock()
	var zero T
	if m.backend == zero {
		panic("enginewrap: " + m.name + " manager is not bound")
	}
	return m.backend
}

// bind and its cleanup require managerstate.Mu's write lock.
func (m *managerBinding[T]) bind(slot *T, wrapper T) func() {
	*slot = wrapper
	return func() {
		var zero T
		if *slot == wrapper {
			*slot = zero
		}
		m.backend = zero
	}
}

// W is the concrete generated wrapper. Other implementations may embed a
// wrapper and override methods, so they must remain valid backend targets.
func resolveManager[W interface{ binding() *managerBinding[T] }, T comparable](binding *managerBinding[T], slot *T, name string) *managerBinding[T] {
	managerstate.Mu.RLock()
	defer managerstate.Mu.RUnlock()
	if binding == nil {
		current := *slot
		if wrapper, ok := any(current).(W); ok {
			binding = wrapper.binding()
		} else {
			binding = &managerBinding[T]{backend: current, name: name}
		}
	}
	var zero T
	if binding == nil || binding.backend == zero {
		panic("enginewrap: " + name + " manager is not bound")
	}
	return binding
}
