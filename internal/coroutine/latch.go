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

// Latch is a scheduler-aware, idempotent one-shot signal.
// Wait atomically publishes blocking and waiter registration.
type Latch struct {
	manager *Coroutines

	done    chan struct{}
	waiters waiterSet
}

// NewLatch creates a closed latch associated with this coroutine manager.
func (p *Coroutines) NewLatch() *Latch {
	return &Latch{
		manager: p,
		done:    make(chan struct{}),
	}
}

// Done is closed when the latch opens.
func (p *Latch) Done() <-chan struct{} {
	return p.done
}

// Open opens the latch once and resumes all waiters.
func (p *Latch) Open() {
	p.manager.wakeWaiters(p.waiters.close(p.done))
}

// Wait yields inside this manager and blocks other callers until the latch opens.
func (p *Latch) Wait() {
	manager := p.manager
	me := manager.callerThread()
	if me == nil {
		manager.waitOutsideScript(p.done)
		return
	}

	manager.waitOn(me, &p.waiters)
}
