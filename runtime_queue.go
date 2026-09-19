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

// -----------------------------------------------------------------------------
// Queue State
// -----------------------------------------------------------------------------
type eventQueuePolicy = coreevent.QueuePolicy

const defaultEventQueuePolicy = coreevent.DefaultQueuePolicy

type eventQueueSnapshot = coreevent.QueueSnapshot

func (p *Game) initEventQueueState() {
	p.eventQueueState.EventQueuePolicy = defaultEventQueuePolicy
	p.eventQueueState.EventQueueStats.Reset()
}

func (p *Game) resetEventQueueStats() {
	p.eventQueueState.EventQueueStats.Reset()
}

func (p *Game) setEventQueuePolicy(policy eventQueuePolicy) {
	p.eventQueueState.EventQueuePolicy = policy
}

func (p *Game) eventQueueSnapshot() eventQueueSnapshot {
	queueLen, queueCap := 0, 0
	if p.events != nil {
		queueLen = len(p.events)
		queueCap = cap(p.events)
	}
	return coreevent.Snapshot(p.eventQueueState.EventQueuePolicy, &p.eventQueueState.EventQueueStats, queueLen, queueCap)
}

func (p *Game) queueEventWithPolicy(ev event) bool {
	policy := p.eventQueueState.EventQueuePolicy
	// A managed send must release the scheduler slot when the queue is full.
	if policy == coreevent.QueueBlock && gco != nil && gco.IsInCoroutine() {
		return coreevent.EnqueueWithPolicyNonBlocking(
			p.events,
			ev,
			policy,
			&p.eventQueueState.EventQueueStats,
			&p.eventQueueState.EventQueueMu,
		)
	}
	return coreevent.EnqueueWithPolicy(
		p.events,
		ev,
		policy,
		&p.eventQueueState.EventQueueStats,
		&p.eventQueueState.EventQueueMu,
	)
}

// -----------------------------------------------------------------------------
// Queue Control
// -----------------------------------------------------------------------------
func parseEventQueuePolicy(policy string) eventQueuePolicy {
	return coreevent.ParsePolicy(policy)
}
