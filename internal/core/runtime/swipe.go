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

func (s *SwipeState[T]) Finish(point mathf.Vec2) (inputstate.SwipeResult, T, bool) {
	s.mu.Lock()
	if !s.recognizer.IsTracking() {
		s.mu.Unlock()
		return inputstate.SwipeResult{}, zeroValue[T](), false
	}
	target := s.target
	s.target = zeroValue[T]()
	result, ok := s.recognizer.Finish(point)
	s.mu.Unlock()
	return result, target, ok
}

func (s *SwipeState[T]) Expire() {
	s.mu.Lock()
	if !s.recognizer.IsTracking() {
		s.mu.Unlock()
		return
	}
	s.recognizer.Expire()
	if !s.recognizer.IsTracking() {
		s.target = zeroValue[T]()
	}
	s.mu.Unlock()
}

func zeroValue[T any]() T {
	var zero T
	return zero
}
