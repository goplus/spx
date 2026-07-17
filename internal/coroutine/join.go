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

// Join waits until target has finished. It yields when called from a coroutine
// managed by this manager and blocks the calling goroutine otherwise.
func (p *Coroutines) Join(target Thread) {
	if target == nil {
		return
	}

	me := p.currentCoroutineThread()
	if me == nil {
		<-target.done
		return
	}
	if me == target {
		return
	}
	p.registerWaiterAndYield(me, func() bool {
		return target.addJoinWaiter(me)
	})
}

// JoinAll waits for each distinct, non-nil target to finish.
func (p *Coroutines) JoinAll(targets []Thread) {
	joinUnique(targets, p.Join)
}

// JoinYieldedOrDone waits until target first yields or finishes.
func (p *Coroutines) JoinYieldedOrDone(target Thread) {
	if target == nil {
		return
	}

	me := p.currentCoroutineThread()
	if me == nil {
		<-target.yieldedOrDone
		return
	}
	if me == target {
		return
	}
	p.registerWaiterAndYield(me, func() bool {
		return target.addYieldWaiter(me)
	})
}

// JoinYieldedOrDoneAll waits until every distinct, non-nil target has first
// yielded or finished.
func (p *Coroutines) JoinYieldedOrDoneAll(targets []Thread) {
	joinUnique(targets, p.JoinYieldedOrDone)
}

func joinUnique(targets []Thread, join func(Thread)) {
	if len(targets) == 0 {
		return
	}
	if len(targets) == 1 {
		join(targets[0])
		return
	}

	seen := make(map[Thread]struct{}, len(targets))
	for _, target := range targets {
		if target == nil {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		join(target)
	}
}

func (p *Coroutines) registerWaiterAndYield(me Thread, register func() bool) {
	// Publish waiter registration and the blocked state atomically.
	p.schedulerMu.Lock()
	p.setThreadStateLocked(me, threadBlocked)
	registered := register()
	if !registered {
		p.setThreadStateLocked(me, threadRunnable)
	}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()

	if registered {
		p.Yield(me)
	}
}

func (th *threadImpl) finishYieldWaiters() (waiters []Thread) {
	th.yieldedOrDoneOnce.Do(func() {
		th.waitersMu.Lock()
		if len(th.yieldWaiters) != 0 {
			waiters = copyThreadSet(th.yieldWaiters)
			th.yieldWaiters = nil
		}
		close(th.yieldedOrDone)
		th.waitersMu.Unlock()
	})
	return waiters
}

func (th *threadImpl) addYieldWaiter(waiter Thread) bool {
	if waiter == nil {
		return false
	}

	th.waitersMu.Lock()
	defer th.waitersMu.Unlock()
	select {
	case <-th.yieldedOrDone:
		return false
	default:
	}
	if th.yieldWaiters == nil {
		th.yieldWaiters = make(map[Thread]struct{})
	}
	th.yieldWaiters[waiter] = struct{}{}
	return true
}

func (th *threadImpl) addJoinWaiter(waiter Thread) bool {
	if waiter == nil {
		return false
	}

	th.waitersMu.Lock()
	defer th.waitersMu.Unlock()
	if th.joinDone {
		return false
	}
	if th.joinWaiters == nil {
		th.joinWaiters = make(map[Thread]struct{})
	}
	th.joinWaiters[waiter] = struct{}{}
	return true
}

func (th *threadImpl) finishJoinWaiters() []Thread {
	th.waitersMu.Lock()
	waiters := copyThreadSet(th.joinWaiters)
	th.joinWaiters = nil
	th.joinDone = true
	th.waitersMu.Unlock()
	return waiters
}

func copyThreadSet(set map[Thread]struct{}) []Thread {
	threads := make([]Thread, 0, len(set))
	for th := range set {
		threads = append(threads, th)
	}
	return threads
}
