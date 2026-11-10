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

package engine

import gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"

type timerCall struct {
	remaining float64
	sprite    gdx.ISpriter
	callback  func()
}

func delayCall(delay float64, callback func()) {
	delaySpriteCall(delay, nil, callback)
}

// Registration runs in the caller's serialized runtime context. The frame
// dispatcher decides whether the callback runs inline or in a managed script.
func delaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	if sprite != nil {
		sprite = resolveRuntimeSprite(sprite)
		if sprite == nil {
			return
		}
	}

	state.jobs.timers = append(state.jobs.timers, &timerCall{
		remaining: delay,
		sprite:    sprite,
		callback:  callback,
	})
}

func (p *frameJobs) updateTimers(delta float64, emit func(gdx.ISpriter, func())) {
	current := p.timers
	p.timers, p.timerScratch = p.timerScratch[:0], nil
	defer func() {
		clear(current)
		if cap(current) > cap(p.timerScratch) {
			p.timerScratch = current[:0]
		}
	}()
	ready := 0
	for _, call := range current {
		if call.sprite != nil && !runtimeSpriteAlive(call.sprite) {
			continue
		}
		call.remaining -= delta
		if call.remaining > 0 {
			p.timers = append(p.timers, call)
		} else if call.remaining <= 0 {
			current[ready] = call
			ready++
		}
	}
	for _, call := range current[:ready] {
		emit(call.sprite, call.callback)
	}
}
