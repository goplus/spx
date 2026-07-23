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

package runtime

import (
	"sync"
	"time"

	"github.com/goplus/spbase/mathf"
	inputstate "github.com/goplus/spx/v3/internal/input"
)

type SwipeState[T comparable] struct {
	mu         sync.Mutex
	recognizer inputstate.SwipeRecognizer
	target     T
}

type SwipeEvent[T comparable] struct {
	Direction float64
	Velocity  float64
	Distance  float64
	Target    T
}

type SwipeHooks[T comparable] struct {
	Debug          func(SwipeEvent[T])
	DispatchTarget func(direction float64, target T)
	DispatchStage  func(direction float64)
}

func (s *SwipeState[T]) Init() {
	s.InitWithClock(nil)
}

// InitWithClock resets the swipe state with a caller-provided clock.
// A nil clock falls back to the system clock.
func (s *SwipeState[T]) InitWithClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recognizer.InitWithClock(now)
	s.target = zeroValue[T]()
}

func (s *SwipeState[T]) Begin(startPos mathf.Vec2, target T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.target = target
	s.recognizer.StartTracking(startPos)
}

func (s *SwipeState[T]) Finish(point mathf.Vec2, hooks SwipeHooks[T]) {
	s.mu.Lock()
	if !s.recognizer.IsTracking() {
		s.mu.Unlock()
		return
	}
	target := s.target
	s.target = zeroValue[T]()
	result, ok := s.recognizer.Finish(point)
	s.mu.Unlock()
	if !ok {
		return
	}
	dispatchSwipeResult(result, target, hooks)
}

func (s *SwipeState[T]) OnMouseMove(pos mathf.Vec2, hooks SwipeHooks[T]) {
	s.mu.Lock()
	if !s.recognizer.IsTracking() {
		s.mu.Unlock()
		return
	}
	result, ok := s.recognizer.OnMouseMove(pos)
	if !ok {
		if !s.recognizer.IsTracking() {
			s.target = zeroValue[T]()
		}
		s.mu.Unlock()
		return
	}
	target := s.target
	s.target = zeroValue[T]()
	s.mu.Unlock()
	dispatchSwipeResult(result, target, hooks)
}

func dispatchSwipeResult[T comparable](result inputstate.SwipeResult, target T, hooks SwipeHooks[T]) {
	if hooks.Debug != nil {
		hooks.Debug(SwipeEvent[T]{
			Direction: result.Direction,
			Velocity:  result.Velocity,
			Distance:  result.Distance,
			Target:    target,
		})
	}
	if target != zeroValue[T]() {
		if hooks.DispatchTarget != nil {
			hooks.DispatchTarget(result.Direction, target)
		}
		return
	}
	if hooks.DispatchStage != nil {
		hooks.DispatchStage(result.Direction)
	}
}

func zeroValue[T any]() T {
	var zero T
	return zero
}
