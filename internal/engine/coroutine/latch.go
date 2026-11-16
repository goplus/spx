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
	manager *Scheduler
	opened  completion
}

// NewLatch creates a closed latch associated with this coroutine scheduler.
func (p *Scheduler) NewLatch() *Latch {
	return &Latch{
		manager: p,
		opened:  newCompletion(),
	}
}

// Done is closed when the latch opens.
func (p *Latch) Done() <-chan struct{} {
	return p.opened.done
}

// Open opens the latch once and resumes all waiters.
func (p *Latch) Open() {
	p.manager.wakeWaiters(p.opened.complete())
}

// Wait yields inside this scheduler and blocks other callers until the latch opens.
func (p *Latch) Wait() {
	p.manager.waitOn(p.manager.Caller(), &p.opened)
}
