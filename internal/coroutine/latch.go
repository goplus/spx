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

// Latch is a scheduler-aware, idempotent one-shot signal.
// Wait atomically publishes blocking and waiter registration.
type Latch struct {
	manager *Coroutines

	mu      sync.Mutex
	opened  bool
	done    chan struct{}
	waiters map[Thread]struct{}
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

// Open opens the latch and resumes every registered waiter. Repeated calls are
// safe and have no effect.
func (p *Latch) Open() {
	p.mu.Lock()
	if p.opened {
		p.mu.Unlock()
		return
	}
	p.opened = true
	close(p.done)
	waiters := copyThreadSet(p.waiters)
	p.waiters = nil
	p.mu.Unlock()

	for _, waiter := range waiters {
		p.manager.markRunnableAndResume(waiter)
	}
}

// Wait suspends the current managed coroutine until the latch opens. Calls
// outside this latch's coroutine manager block the calling goroutine.
func (p *Latch) Wait() {
	manager := p.manager
	me := manager.currentCoroutineThread()
	if me == nil {
		<-p.done
		return
	}

	manager.schedulerMu.Lock()
	manager.setThreadStateLocked(me, threadBlocked)
	p.mu.Lock()
	registered := !p.opened
	if registered {
		if p.waiters == nil {
			p.waiters = make(map[Thread]struct{})
		}
		p.waiters[me] = struct{}{}
	}
	p.mu.Unlock()
	if !registered {
		manager.setThreadStateLocked(me, threadRunnable)
	}
	manager.schedulerCond.Signal()
	manager.schedulerMu.Unlock()

	if registered {
		defer p.removeWaiter(me)
		manager.Yield(me)
	}
}

func (p *Latch) removeWaiter(waiter Thread) {
	p.mu.Lock()
	delete(p.waiters, waiter)
	if len(p.waiters) == 0 {
		p.waiters = nil
	}
	p.mu.Unlock()
}
