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

package spx

import (
	coreevent "github.com/goplus/spx/v3/internal/core/event"
	"github.com/goplus/spx/v3/internal/coroutine"
)

// OnCond handles rising edges without reentry, skipping polls while active.
// Conditions must be fast and non-blocking.
func (p *scriptEventBindings) OnCond(__xgo_autoclosure_condition func() bool, onCondition func()) {
	if __xgo_autoclosure_condition == nil || onCondition == nil {
		return
	}
	running := false
	edge := coreevent.MatchRisingEdge(__xgo_autoclosure_condition)
	p.scriptEventRegistry.manager.AddCondition(coreevent.NewSink(
		p.pthis,
		func() {
			running = true
			defer func() { running = false }()
			onCondition()
		},
		func(data any) bool { return !running && edge(data) },
	))
}

// sampleConditions reads a consistent snapshot before the frame clock advances.
func (p *scriptEventRegistry) sampleConditions() {
	read := func() {
		p.pendingConditions = matchingEventSinks(p.globalSinks(coreevent.BucketCondition), nil)
	}
	if gco == nil {
		read()
	} else {
		gco.RunBetweenScripts(read)
	}
}

// dispatchConditions starts matched handlers without reevaluating conditions.
func (p *scriptEventRegistry) dispatchConditions() {
	sinks := p.pendingConditions
	p.pendingConditions = nil
	if len(sinks) == 0 {
		return
	}
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run: func(_ coroutine.Thread, sink *eventSink) {
			sink.Handler.(func())()
		},
	}
	event.withRegistrationBarrier(func() {
		dispatchMatchedScriptEventBatch(sinks, event)
	})
}
