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
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func (a *animationComponent) Animate(name SpriteAnimationName, loop bool) {
	a.playAnimation(name, loop, false, "==> Animation %s")
}

func (a *animationComponent) AnimateAndWait(name SpriteAnimationName) {
	a.playAnimation(name, false, true, "==> AnimateAndWait %s")
}

func (a *animationComponent) StopAnimation(name SpriteAnimationName) {
	if name == "" || !a.hasAnim(name) {
		return
	}
	if a.curAnimState == nil || a.curAnimState.Name != name {
		return
	}

	a.sprite.syncSprite.PauseAnim()
	a.playDefaultAnim()
}

func (a *animationComponent) playAnimation(name SpriteAnimationName, loop, blocking bool, debugMsg string) {
	if isDebugInstrEnabled() {
		spxlog.Debug(debugMsg, name)
	}

	ani, ok := a.shared.animations[name]
	if !ok {
		spxlog.Warn("Animation not found: %s", name)
		return
	}

	a.doAnimation(name, ani, loop, 1, blocking, true)
}

func (a *animationComponent) doAnimation(animName SpriteAnimationName, ani *aniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) {
	a.stopAnimState(a.curAnimState)
	a.curAnimState = &animState{
		AniType: aniTypeFrame,
		Name:    animName,
		Speed:   speed,
	}

	info := a.curAnimState
	if playAudio {
		a.playAnimAudio(ani, info)
	}

	syncCheckUpdateCostume(&a.sprite.baseObj)
	a.prepareAnimationPlayback(animName, ani)

	a.engine().SpriteMgr.PlayAnim(a.sprite.syncSprite.GetId(), animName, speed, loop, false)
	if isBlocking {
		a.sprite.isAnimating = true
		for a.engine().SpriteMgr.IsPlayingAnim(a.sprite.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		a.sprite.isAnimating = false
		a.stopAnimState(info)
	}
}

func (a *animationComponent) playDefaultAnim() {
	animName := ""
	if !a.sprite.isVisible || a.sprite.isDying {
		return
	}

	speed := 1.0
	if a.curTweenState == nil {
		animName = a.shared.defaultAnimation
	} else {
		switch a.curTweenState.AniType {
		case aniTypeMove:
			animName = a.sprite.getStateAnimName(StateStep)
		case aniTypeTurn:
			animName = a.sprite.getStateAnimName(StateTurn)
		case aniTypeGlide:
			animName = a.sprite.getStateAnimName(StateGlide)
		}
		speed = a.curTweenState.Speed
	}

	if animName == "" {
		animName = a.shared.defaultAnimation
	}

	if _, ok := a.shared.animations[animName]; ok {
		a.prepareAnimationPlayback(animName, a.shared.animations[animName])
		a.engine().SpriteMgr.PlayAnim(a.sprite.syncSprite.GetId(), animName, speed, true, false)
	} else {
		a.sprite.goSetCostume(a.sprite.defaultCostumeIndex)
	}
}

func (a *animationComponent) playAnimAudio(ani *aniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		info.AudioName = ani.OnStart.Play
		info.AudioId = a.sprite.playAudio(info.AudioName, false)
	}
}

func (a *animationComponent) adaptAnimBitmapResolution(ani *aniConfig) {
	renderScale := a.sprite.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	a.sprite.syncSprite.SetRenderScale(mathf.NewVec2(renderScale, renderScale))
}

func (a *animationComponent) prepareAnimationPlayback(animName SpriteAnimationName, ani *aniConfig) {
	a.shared.animationWrappers[animName].ensureRegistered(animName)
	a.adaptAnimBitmapResolution(ani)
}
