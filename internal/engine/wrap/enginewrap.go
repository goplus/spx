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
	. "github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine/platform"
	"github.com/goplus/spx/v4/internal/input"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

// A nil binding is a zero-value facade. A non-nil binding keeps its own backend,
// including after release, when that backend is cleared and calls must fail.
type managerBinding[T comparable] struct {
	backend T
}

func (m *managerBinding[T]) binding() *managerBinding[T] { return m }

// W is the concrete generated wrapper. Other implementations may embed a
// wrapper and override methods, so they must remain valid backend targets.
func resolveManager[W interface{ binding() *managerBinding[T] }, T comparable](binding *managerBinding[T], current T, name string) T {
	var backend, zero T
	if binding == nil {
		if wrapper, ok := any(current).(W); ok {
			binding = wrapper.binding()
		} else {
			backend = current
		}
	}
	if binding != nil {
		backend = binding.backend
	}
	if backend == zero {
		panic("enginewrap: " + name + " manager is not bound")
	}
	return backend
}

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

var mainCallback func(call func())

func Init(call func(f func())) {
	mainCallback = call
}

// CallInMainThread runs call synchronously on the engine main thread.
// Without a dispatcher, only calls already on the engine thread may run.
func CallInMainThread(call func()) {
	if mainCallback == nil {
		if platform.TryCallEngineDirectly(call) {
			return
		}
		panic("enginewrap: Init must be called before using manager methods off the main thread")
	}
	mainCallback(call)
}

// CallInMainThreadValue runs call synchronously on the engine main thread and
// returns its result.
func CallInMainThreadValue[T any](call func() T) (result T) {
	CallInMainThread(func() {
		result = call()
	})
	return
}

func (i *InputMgrImpl) MousePressed() bool {
	return i.GetMouseState(input.MouseButtonLeft) || i.GetMouseState(input.MouseButtonRight)
}

func (*PlatformMgrImpl) SetRunnableOnUnfocused(flag bool) {
	if !flag {
		spxlog.Warn("SetRunnableOnUnfocused(false) is not implemented yet")
	}
}

func (c *CameraMgrImpl) GetPosition() Vec2 {
	return c.GetCameraPosition()
}

func (c *CameraMgrImpl) SetPosition(position Vec2) {
	c.SetCameraPosition(position)
}
