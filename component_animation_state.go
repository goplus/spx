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

package spx

import (
	"log"

	"github.com/goplus/spx/v2/internal/tools"
)

func (a *animationComponent) onAnimationDone(animName string) {
	if a.curAnimState != nil && a.curAnimState.Name == animName {
		a.playDefaultAnim()
	}
}

func (a *animationComponent) stopAnimState(state *animState) {
	if state == nil {
		return
	}
	state.IsCanceled = true
}

func (a *animationComponent) costumeIndex(nameOrIndex any) int {
	switch v := nameOrIndex.(type) {
	case SpriteCostumeName:
		idx := a.sprite.findCostume(v)
		if idx < 0 {
			log.Panicf("findCostume %s failed", v)
		}
		return idx
	default:
		val, _ := tools.GetFloat(nameOrIndex)
		return int(val)
	}
}

func (a *animationComponent) frameRange(from, to any) (int, int) {
	return a.costumeIndex(from), a.costumeIndex(to)
}

func (a *animationComponent) hasAnim(animName string) bool {
	_, ok := a.shared.animations[animName]
	return ok
}

func (a *animationComponent) getAnimation(animName SpriteAnimationName) (*aniConfig, bool) {
	ani, ok := a.shared.animations[animName]
	return ani, ok
}

func (a *animationComponent) getStateAnimName(stateName string) string {
	if bindingName, ok := a.shared.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}

func (a *animationComponent) addDonedAnimation(animName string) {
	a.donedAnimations = append(a.donedAnimations, animName)
}

func (a *animationComponent) takeDonedAnimations(buffer []string) []string {
	buffer = append(buffer, a.donedAnimations...)
	a.donedAnimations = a.donedAnimations[:0]
	return buffer
}

func (a *animationComponent) getCurAnimState() *animState {
	return a.curAnimState
}

func (a *animationComponent) getCurTweenState() *animState {
	return a.curTweenState
}
