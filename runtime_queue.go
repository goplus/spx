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

import coreevent "github.com/goplus/spx/v3/internal/core/event"

func (p *Game) initEventQueueState() {
	p.eventQueueState.EventQueuePolicy = coreevent.DefaultQueuePolicy
	p.eventQueueState.EventQueueStats.Reset()
}

func (p *Game) eventQueueSnapshot() coreevent.QueueSnapshot {
	state := &p.eventQueueState
	return coreevent.Snapshot(state.EventQueuePolicy, &state.EventQueueStats, len(p.events), cap(p.events))
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	state := &p.eventQueueState
	enqueue := coreevent.EnqueueWithPolicy[event]
	// A managed send must release the scheduler slot when the queue is full.
	if state.EventQueuePolicy == coreevent.QueueBlock && gco != nil && gco.IsInCoroutine() {
		enqueue = coreevent.EnqueueWithPolicyNonBlocking[event]
	}
	return enqueue(p.events, ev, state.EventQueuePolicy, &state.EventQueueStats, &state.EventQueueMu)
}
