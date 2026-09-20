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

	me := p.callerThread()
	if me == nil {
		p.waitOutsideScript(target.done)
		return
	}
	if me == target {
		return
	}
	p.waitOn(me, &target.joinWaiters)
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

	me := p.callerThread()
	if me == nil {
		p.waitOutsideScript(target.yieldedOrDone)
		return
	}
	if me == target {
		return
	}
	p.waitOn(me, &target.yieldWaiters)
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
