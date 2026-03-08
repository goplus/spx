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
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/coroutine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// handleEvent dispatches events to their respective handlers.
func (p *Game) handleEvent(ev event) {
	switch e := ev.(type) {
	case *eventLeftButtonUp:
		p.doWhenLeftButtonUp(e)
	case *eventLeftButtonDown:
		p.doWhenLeftButtonDown(e)
	case *eventKeyDown:
		// Note: key-up callbacks are not part of the current event sink API.
		p.sinkMgr.doWhenKeyPressed(e.Key)
	case *eventStart:
		p.sinkMgr.doWhenAwake(nil)
		p.sinkMgr.doWhenStart()
	case *eventTimer:
		p.sinkMgr.doWhenTimer(e.Time)
	}
}

func (p *Game) fireEvent(ev event) {
	if p.queueEventWithPolicy(ev) {
		return
	}
	if isDebugInstrEnabled() {
		spxlog.Warn("Event buffer is full (policy=%s). Drop event: %v", p.EventQueuePolicy, ev)
	}
}

func (p *Game) eventLoop(me coroutine.Thread) int {
	return coreruntime.RunEventLoop(me, p.events, p.handleEvent)
}

func (p *Game) initEventLoop() {
	coreruntime.InitLoops(gco.Create, p.eventLoop, p.inputEventLoop, p.logicLoop)
}
