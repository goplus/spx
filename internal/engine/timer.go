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

import gdx "github.com/goplus/spx/v2/pkg/spx/pkg/engine"

type delaySpriteCallInfo struct {
	timer    float64
	objectID Object
	callback func()
}

var (
	delaySpriteCalls     = make([]*delaySpriteCallInfo, 0)
	tempDelaySpriteCalls = make([]*delaySpriteCallInfo, 0)
)

func updateTimers(delta float64) {
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
	count := len(delaySpriteCalls)
	tempDelaySpriteCalls = append(tempDelaySpriteCalls, delaySpriteCalls...)
	delaySpriteCalls = delaySpriteCalls[:0]
	for i := range count {
		tempDelaySpriteCalls[i].timer -= delta
		if tempDelaySpriteCalls[i].timer > 0 {
			delaySpriteCalls = append(delaySpriteCalls, tempDelaySpriteCalls[i])
		}
	}
	for i := range count {
		if tempDelaySpriteCalls[i].timer <= 0 {
			id := tempDelaySpriteCalls[i].objectID
			if id == 0 || isNodeExist(id) {
				tempDelaySpriteCalls[i].callback()
			}
		}
	}
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
}

func delayCall(delay float64, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{timer: delay, callback: callback})
}

func delaySpriteCall(delay float64, sprite gdx.ISpriter, callback func()) {
	delaySpriteCalls = append(delaySpriteCalls, &delaySpriteCallInfo{
		timer:    delay,
		objectID: sprite.GetId(),
		callback: callback,
	})
}
