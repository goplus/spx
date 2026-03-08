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
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

// tweenParams holds pre-calculated parameters for tween animations.
type tweenParams struct {
	moveDiff  mathf.Vec2
	moveSpeed float64
	moveDir   mathf.Vec2
	turnDiff  float64
}

func (a *animationComponent) doTween(name SpriteAnimationName, ani *aniConfig) {
	info := a.initTweenState(name, ani)
	if info == nil {
		return
	}

	params, ok := a.prepareTweenParams(ani)
	if !ok {
		return
	}

	a.executeTweenLoop(info, ani, params)
	a.cleanupTween(info, name, ani)
}

func (a *animationComponent) initTweenState(name SpriteAnimationName, ani *aniConfig) *animState {
	info := &animState{
		AniType: ani.AniType,
		Name:    name,
		Speed:   ani.Speed,
	}
	a.stopAnimState(a.curTweenState)
	a.curTweenState = info

	if a.hasAnim(name) {
		a.doAnimation(name, ani, ani.IsLoop, ani.Speed, false, false)
		a.playAnimAudio(ani, info)
	}

	if ani.Duration <= 0 {
		spxlog.Warn("Invalid animation duration: %v", ani.Duration)
		return nil
	}

	return info
}

func (a *animationComponent) prepareTweenParams(ani *aniConfig) (*tweenParams, bool) {
	params := &tweenParams{}
	duration := ani.Duration

	switch ani.AniType {
	case aniTypeMove, aniTypeGlide:
		src, srcOk := tools.GetVec2(ani.From)
		dst, dstOk := tools.GetVec2(ani.To)
		if !srcOk || !dstOk {
			spxlog.Warn("Invalid 'From' or 'To' for move/glide animation: not a *mathf.Vec2")
			return nil, false
		}

		params.moveDiff = dst.Sub(src)
		if ani.AniType == aniTypeMove {
			params.moveSpeed = params.moveDiff.Length() / duration
			params.moveDir = params.moveDiff.Normalize()
		}
	case aniTypeTurn:
		src, srcOk := tools.GetFloat(ani.From)
		dst, dstOk := tools.GetFloat(ani.To)
		if !srcOk || !dstOk {
			spxlog.Warn("Invalid 'From' or 'To' for turn animation: not a float")
			return nil, false
		}

		params.turnDiff = dst - src
	}

	return params, true
}

func (a *animationComponent) executeTweenLoop(info *animState, ani *aniConfig, params *tweenParams) {
	timer := 0.0
	prePercent := 0.0
	duration := ani.Duration

	for timer < duration {
		if info.IsCanceled {
			return
		}

		timer += time.DeltaTime()
		percent := mathf.Clamp01f(timer / duration)
		deltaPercent := percent - prePercent
		prePercent = percent

		a.applyTweenStep(ani.AniType, deltaPercent, params)
		engine.WaitNextFrame()
	}
}

func (a *animationComponent) applyTweenStep(aniType aniTypeEnum, deltaPercent float64, params *tweenParams) {
	switch aniType {
	case aniTypeMove:
		physicsMode := a.sprite.PhysicsMode()
		if isPhysicsEnabled() && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			vel := params.moveDir.Mulf(params.moveSpeed)
			a.sprite.SetVelocity(vel.X, vel.Y)
		} else {
			val := params.moveDiff.Mulf(deltaPercent)
			a.sprite.ChangeXYpos(val.X, val.Y)
		}
	case aniTypeGlide:
		val := params.moveDiff.Mulf(deltaPercent)
		a.sprite.ChangeXYpos(val.X, val.Y)
	case aniTypeTurn:
		val := params.turnDiff * deltaPercent
		a.sprite.ChangeHeading(val)
	}
}

func (a *animationComponent) cleanupTween(info *animState, name SpriteAnimationName, ani *aniConfig) {
	if ani.AniType == aniTypeMove {
		physicsMode := a.sprite.PhysicsMode()
		if isPhysicsEnabled() && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			a.sprite.SetVelocity(0, 0)
		}
	}

	a.stopAnimState(info)
	a.curTweenState = nil
	if name != a.shared.defaultAnimation && !ani.IsKeepOnStop {
		a.playDefaultAnim()
	}
}
