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

package coroutine

import "sync"

// HandlerPolicy determines how a registration handles an overlapping invocation.
type HandlerPolicy uint8

const (
	// RestartExisting cancels the prior invocation when a new one is registered.
	RestartExisting HandlerPolicy = iota
	// IgnoreWhileRunning cancels new invocations until the current one finishes.
	IgnoreWhileRunning
)

// HandlerState owns execution admission for a single registration.
// It must not be copied after its first use.
type HandlerState struct {
	mu     sync.Mutex
	active Thread
	policy HandlerPolicy
}

// NewHandlerState creates unstarted execution state for a handler policy.
func NewHandlerState(policy HandlerPolicy) HandlerState {
	return HandlerState{policy: policy}
}

// Start admits or cancels a registered invocation and returns its cleanup.
// Cleanup from an older invocation cannot release a newer invocation's state.
func (p *HandlerState) Start(thread Thread) func() {
	p.mu.Lock()
	previous := p.active
	if p.policy == IgnoreWhileRunning && previous != nil && !previous.Stopped() {
		p.mu.Unlock()
		if thread != nil {
			stopThreadIfRunning(thread)
		}
		return nil
	}
	p.active = thread
	p.mu.Unlock()

	if previous != nil && previous != thread {
		stopThreadIfRunning(previous)
	}
	return func() {
		p.mu.Lock()
		if p.active == thread {
			p.active = nil
		}
		p.mu.Unlock()
	}
}
