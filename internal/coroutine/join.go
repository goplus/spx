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
// managed by this scheduler and blocks the calling goroutine otherwise.
// A non-nil target must belong to this scheduler, even if already finished.
func (p *Scheduler) Join(target Thread) {
	if target == nil {
		return
	}

	p.join(target, target.done, &target.joinWaiters)
}

// JoinAll validates all targets before waiting for each to finish.
func (p *Scheduler) JoinAll(targets []Thread) {
	for _, target := range targets {
		p.requireOwnThread(target)
	}
	for _, target := range targets {
		p.Join(target)
	}
}

// JoinFirstSlice waits until target first yields or finishes in this scheduler.
func (p *Scheduler) JoinFirstSlice(target Thread) {
	if target == nil {
		return
	}

	p.join(target, target.firstSliceDone, &target.firstSliceWaiters)
}

func (p *Scheduler) join(target Thread, done <-chan struct{}, waiters *waiterSet) {
	p.requireOwnThread(target)
	me := p.Caller()
	if me == target {
		return
	}
	p.waitOn(me, done, waiters)
}

func (p *Scheduler) requireOwnThread(target Thread) {
	if target != nil && target.manager != p {
		panic(ErrForeignThread)
	}
}
