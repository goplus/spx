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
	"errors"
	"time"
)

const (
	MainExecutionTimedOutMsg = "Main execution timed out. Please check if there is an infinite loop in the code."
	LoopExecutionTimedOutMsg = "For loop execution timed out. Please check if there is an infinite loop in the code."
)

//lint:ignore ST1005 This wraps a user-facing runtime message kept as a complete sentence.
var ErrMainExecutionTimedOut = errors.New(MainExecutionTimedOutMsg)

//lint:ignore ST1005 This wraps a user-facing runtime message kept as a complete sentence.
var ErrLoopExecutionTimedOut = errors.New(LoopExecutionTimedOutMsg)

type ScheduleState struct {
	MainStartedAt   time.Time
	Now             time.Time
	MainExecTimeout time.Duration
}

type SchedulerHooks struct {
	SchedCurrent   func()
	IsSchedTimeout func(float64) bool
	OnSchedTimeout func()
}

func MainExecutionTimedOut(state ScheduleState) bool {
	return !state.MainStartedAt.IsZero() && state.Now.Sub(state.MainStartedAt) >= state.MainExecTimeout
}

func SchedNow(state ScheduleState, hooks SchedulerHooks) error {
	if MainExecutionTimedOut(state) {
		return ErrMainExecutionTimedOut
	}
	if hooks.SchedCurrent != nil {
		hooks.SchedCurrent()
	}
	return nil
}

func Sched(state ScheduleState, schedTimeoutMs float64, hooks SchedulerHooks) error {
	if MainExecutionTimedOut(state) {
		return ErrMainExecutionTimedOut
	}
	if hooks.IsSchedTimeout != nil && hooks.IsSchedTimeout(schedTimeoutMs) {
		if hooks.OnSchedTimeout != nil {
			hooks.OnSchedTimeout()
		}
		return ErrLoopExecutionTimedOut
	}
	return nil
}

func Forever(call func(), yield func()) {
	if call == nil {
		return
	}
	for {
		call()
		yield()
	}
}

func Repeat(loopCount int, call func(), yield func()) {
	if call == nil {
		return
	}
	for range loopCount {
		call()
		yield()
	}
}

func RepeatUntil(condition func() bool, call func(), yield func()) {
	if call == nil || condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		call()
		yield()
	}
}

func WaitUntil(condition func() bool, yield func()) {
	if condition == nil {
		return
	}
	for {
		if condition() {
			return
		}
		yield()
	}
}
