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
	"maps"
	"math"

	"github.com/goplus/spx/v2/internal/base/valueutil"
)

// sharedAnimationData contains read-only animation data shared across cloned sprites (Flyweight Pattern).
type sharedAnimationData struct {
	animations        map[SpriteAnimationName]*aniConfig
	animBindings      map[string]string
	defaultAnimation  SpriteAnimationName
	animationWrappers map[SpriteAnimationName]*animationWrapper
}

type animationComponent struct {
	componentBase

	// Shared animation configuration (read-only, shared across clones)
	shared *sharedAnimationData

	// Animation state (per-instance)
	curAnimState  *animState
	curTweenState *animState

	// Animation tracking (per-instance)
	donedAnimations []string
}

// initialize initializes the animation component from configuration.
func (a *animationComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	a.componentBase.initialize(sprite, spriteCfg)
	a.initFromConfig(spriteCfg)
	a.donedAnimations = make([]string, 0)
}

// initFromConfig initializes animations from sprite configuration.
func (a *animationComponent) initFromConfig(spriteCfg *spriteConfig) {
	a.shared = &sharedAnimationData{
		defaultAnimation:  spriteCfg.DefaultAnimation,
		animations:        make(map[string]*aniConfig),
		animBindings:      make(map[string]string),
		animationWrappers: make(map[SpriteAnimationName]*animationWrapper),
	}

	anims := spriteCfg.FAnimations
	for key, val := range anims {
		var ani = val
		_, ok := a.shared.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}

		valueutil.SetDefaultIfZero(&ani.FrameFps, 25)
		valueutil.SetDefaultIfZero(&ani.TurnToDuration, 1.0)
		valueutil.SetDefaultIfZero(&ani.StepDuration, 0.01)

		ani.IFrameFrom, ani.IFrameTo = a.frameRange(ani.FrameFrom, ani.FrameTo)
		ani.Speed = 1
		ani.Duration = (math.Abs(float64(ani.IFrameFrom-ani.IFrameTo)) + 1) / float64(ani.FrameFps)
		a.shared.animations[key] = ani
	}

	maps.Copy(a.shared.animBindings, spriteCfg.AnimBindings)

	for animName, ani := range a.shared.animations {
		a.shared.animationWrappers[animName] = &animationWrapper{
			spriteName:   a.sprite.name,
			ani:          ani,
			costumes:     a.sprite.costumes,
			isCostumeSet: a.sprite.IsCostumeSet,
			engineMgr:    a.engine(),
		}
	}
}

// cloneFrom creates a new animation component by cloning from source.
func (a *animationComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcAnim := src.(*animationComponent)
	newAnim := &animationComponent{
		componentBase:   componentBase{sprite: newSprite},
		shared:          srcAnim.shared,
		donedAnimations: make([]string, 0),
	}
	return newAnim
}

// onDestroy cleanup when component is destroyed.
func (a *animationComponent) onDestroy() {
	a.stopAnimState(a.curAnimState)
	a.stopAnimState(a.curTweenState)
	a.unRegisterOnAnimationLooped()
	a.unRegisterOnAnimationFinished()
}
