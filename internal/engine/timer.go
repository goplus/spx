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

type delaySpriteCallInfo struct {
	timer    float64
	sprite   gdx.ISpriter
	callback func()
}

var (
	delaySpriteCalls     = make([]*delaySpriteCallInfo, 0)
	tempDelaySpriteCalls = make([]*delaySpriteCallInfo, 0)
)

func updateTimers(delta float64, emit func(gdx.ISpriter, func())) {
	delaySpriteCalls, tempDelaySpriteCalls = tempDelaySpriteCalls[:0], delaySpriteCalls
	for _, info := range tempDelaySpriteCalls {
		if info.sprite != nil && !runtimeSpriteAlive(info.sprite) {
			continue
		}
		info.timer -= delta
		if info.timer > 0 {
			delaySpriteCalls = append(delaySpriteCalls, info)
		}
	}
	for _, info := range tempDelaySpriteCalls {
		if info.timer <= 0 {
			emit(info.sprite, info.callback)
		}
	}
	clear(tempDelaySpriteCalls)
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
}

func delayCall(delay float64, callback func()) {
	delaySpriteCall(delay, nil, callback)
}

func delaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	if sprite != nil {
		sprite = resolveRuntimeSprite(sprite)
		if sprite == nil {
			return
		}
	}

	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{
		timer:    delay,
		sprite:   sprite,
		callback: callback,
	})
}
