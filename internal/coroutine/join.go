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

// JoinAll waits for each target to finish.
func (p *Coroutines) JoinAll(targets []Thread) {
	for _, target := range targets {
		p.Join(target)
	}
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

// JoinYieldedOrDoneAll waits until every target has first yielded or finished.
func (p *Coroutines) JoinYieldedOrDoneAll(targets []Thread) {
	for _, target := range targets {
		p.JoinYieldedOrDone(target)
	}
}
