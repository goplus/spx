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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

// ============================================================================
// Animation Component
// ============================================================================
// This component encapsulates all animation-related functionality

type animationComponent struct {
	componentBase

	// Animation configuration
	animations        map[SpriteAnimationName]*aniConfig
	animBindings      map[string]string
	defaultAnimation  SpriteAnimationName
	animationWrappers map[SpriteAnimationName]*animationWrapper

	// Animation state
	curAnimState  *animState
	curTweenState *animState

	// Animation tracking
	donedAnimations []string
}

// initialize initializes the animation component from config
func (ac *animationComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	ac.componentBase.initialize(sprite, spriteCfg)
	// Always initialize from config
	ac.initFromConfig(spriteCfg)
	ac.donedAnimations = make([]string, 0)
}

// initFromConfig initializes animations from sprite configuration
func (ac *animationComponent) initFromConfig(spriteCfg *spriteConfig) {
	ac.defaultAnimation = spriteCfg.DefaultAnimation
	ac.animations = make(map[string]*aniConfig)
	anims := spriteCfg.FAnimations

	for key, val := range anims {
		var ani = val
		_, ok := ac.animations[key]
		if ok {
			log.Panicf("animation key [%s] is exist", key)
		}

		// Set default values
		if ani.FrameFps == 0 {
			ani.FrameFps = 25
		}
		if ani.TurnToDuration == 0 {
			ani.TurnToDuration = 1
		}
		if ani.StepDuration == 0 {
			ani.StepDuration = 0.01
		}

		// Calculate frame ranges and duration
		from, to := ac.getFromAnToForAniFrames(ani.FrameFrom, ani.FrameTo)
		ani.IFrameFrom, ani.IFrameTo = int(from), int(to)
		ani.Speed = 1
		ani.Duration = (math.Abs(float64(ani.IFrameFrom-ani.IFrameTo)) + 1) / float64(ani.FrameFps)
		ac.animations[key] = ani
	}

	// Lazy register animations to engine
	ac.animationWrappers = make(map[SpriteAnimationName]*animationWrapper)
	for animName, ani := range ac.animations {
		ac.animationWrappers[animName] = &animationWrapper{spr: ac.sprite, ani: ani}
	}
}

// cloneFrom creates a new animation component by cloning from source
func (ac *animationComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcAnim := src.(*animationComponent)
	newAnim := &animationComponent{
		componentBase:     componentBase{sprite: newSprite},
		animations:        srcAnim.animations,
		animBindings:      srcAnim.animBindings,
		defaultAnimation:  srcAnim.defaultAnimation,
		animationWrappers: make(map[SpriteAnimationName]*animationWrapper),
		curAnimState:      nil,
		curTweenState:     nil,
		donedAnimations:   make([]string, 0),
	}
	// Recreate animation wrappers for the new sprite
	for name, ani := range newAnim.animations {
		newAnim.animationWrappers[name] = &animationWrapper{
			spr: newSprite,
			ani: ani,
		}
	}
	return newAnim
}

// onDestroy cleanup when component is destroyed
func (ac *animationComponent) onDestroy() {
	ac.stopAnimState(ac.curAnimState)
	ac.stopAnimState(ac.curTweenState)
}

// ============================================================================
// Animation Playback Methods
// ============================================================================

func (ac *animationComponent) Animate(name SpriteAnimationName, loop bool) {
	if debugInstr {
		spxlog.Debug("==> Animation %s", name)
	}
	if ani, ok := ac.animations[name]; ok {
		ac.doAnimation(name, ani, loop, 1, false, true)
	} else {
		spxlog.Debug("Animation not found: %s", name)
	}
}

func (ac *animationComponent) AnimateAndWait(name SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("==> AnimateAndWait %s", name)
	}
	if ani, ok := ac.animations[name]; ok {
		ac.doAnimation(name, ani, false, 1, true, true)
	} else {
		spxlog.Debug("Animation not found: %s", name)
	}
}

func (ac *animationComponent) StopAnimation(name SpriteAnimationName) {
	if name == "" || !ac.hasAnim(name) {
		return
	}
	if ac.curAnimState == nil || ac.curAnimState.Name != name {
		return
	}
	ac.sprite.syncSprite.PauseAnim()
	ac.playDefaultAnim()
}

// ============================================================================
// Core Animation Implementation
// ============================================================================

func (ac *animationComponent) doAnimation(animName SpriteAnimationName, ani *aniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) {
	ac.stopAnimState(ac.curAnimState)
	ac.curAnimState = &animState{
		AniType:    aniTypeFrame,
		IsCanceled: false,
		Name:       animName,
		Speed:      speed,
	}
	info := ac.curAnimState
	if playAudio {
		ac.playAnimAudio(ani, info)
	}

	syncCheckUpdateCostume(&ac.sprite.baseObj)
	ac.animationWrappers[animName].ensureRegistered(animName)

	spriteMgr.PlayAnim(ac.sprite.syncSprite.GetId(), animName, speed, loop, false)
	if isBlocking {
		ac.sprite.isAnimating = true
		for spriteMgr.IsPlayingAnim(ac.sprite.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		ac.sprite.isAnimating = false
		ac.stopAnimState(info)
	}
}

// tweenParams holds pre-calculated parameters for tween animations
type tweenParams struct {
	moveDiff  mathf.Vec2
	moveSpeed float64
	moveDir   mathf.Vec2
	turnDiff  float64
}

func (ac *animationComponent) doTween(name SpriteAnimationName, ani *aniConfig) {
	info := ac.initTweenState(name, ani)
	if info == nil {
		return
	}

	params, ok := ac.prepareTweenParams(ani)
	if !ok {
		return
	}

	ac.executeTweenLoop(info, ani, params)
	ac.cleanupTween(info, name, ani)
}

// initTweenState initializes the tween animation state
func (ac *animationComponent) initTweenState(name SpriteAnimationName, ani *aniConfig) *animState {
	info := &animState{
		AniType:    ani.AniType,
		Name:       name,
		Speed:      ani.Speed,
		IsCanceled: false,
	}
	ac.stopAnimState(ac.curTweenState)
	ac.curTweenState = info

	if ac.hasAnim(name) {
		ac.doAnimation(name, ani, ani.IsLoop, ani.Speed, false, false)
		ac.playAnimAudio(ani, info)
	}

	// Validate duration to prevent division by zero
	if ani.Duration <= 0 {
		spxlog.Warn("Invalid animation duration: %v", ani.Duration)
		return nil
	}

	return info
}

// prepareTweenParams pre-calculates animation parameters based on animation type
func (ac *animationComponent) prepareTweenParams(ani *aniConfig) (*tweenParams, bool) {
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

// executeTweenLoop runs the main animation loop
func (ac *animationComponent) executeTweenLoop(info *animState, ani *aniConfig, params *tweenParams) {
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

		ac.applyTweenStep(ani.AniType, deltaPercent, params)
		engine.WaitNextFrame()
	}
}

// applyTweenStep applies a single animation step
func (ac *animationComponent) applyTweenStep(aniType aniTypeEnum, deltaPercent float64, params *tweenParams) {
	switch aniType {
	case aniTypeMove:
		physicsMode := ac.sprite.PhysicsMode()
		if enabledPhysics && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			vel := params.moveDir.Mulf(params.moveSpeed)
			ac.sprite.SetVelocity(vel.X, vel.Y)
		} else {
			val := params.moveDiff.Mulf(deltaPercent)
			ac.sprite.ChangeXYpos(val.X, val.Y)
		}
	case aniTypeGlide:
		val := params.moveDiff.Mulf(deltaPercent)
		ac.sprite.ChangeXYpos(val.X, val.Y)
	case aniTypeTurn:
		val := params.turnDiff * deltaPercent
		ac.sprite.ChangeHeading(val)
	}
}

// cleanupTween performs cleanup after tween animation completes
func (ac *animationComponent) cleanupTween(info *animState, name SpriteAnimationName, ani *aniConfig) {
	if ani.AniType == aniTypeMove {
		physicsMode := ac.sprite.PhysicsMode()
		if enabledPhysics && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			ac.sprite.SetVelocity(0, 0)
		}
	}

	ac.stopAnimState(info)
	ac.curTweenState = nil

	if name != ac.defaultAnimation && !ani.IsKeepOnStop {
		ac.playDefaultAnim()
	}
}

func (ac *animationComponent) playDefaultAnim() {
	animName := ""
	if !ac.sprite.isVisible || ac.sprite.isDying {
		return
	}
	speed := 1.0
	if ac.curTweenState == nil {
		animName = ac.defaultAnimation
	} else {
		switch ac.curTweenState.AniType {
		case aniTypeMove:
			animName = ac.sprite.getStateAnimName(StateStep)
		case aniTypeTurn:
			animName = ac.sprite.getStateAnimName(StateTurn)
		case aniTypeGlide:
			animName = ac.sprite.getStateAnimName(StateGlide)
		}
		speed = ac.curTweenState.Speed
	}

	if animName == "" {
		animName = ac.defaultAnimation
	}

	if _, ok := ac.animations[animName]; ok {
		ac.animationWrappers[animName].ensureRegistered(animName)
		spriteMgr.PlayAnim(ac.sprite.syncSprite.GetId(), animName, speed, true, false)
	} else {
		ac.sprite.goSetCostume(ac.sprite.defaultCostumeIndex)
	}
}

// ============================================================================
// Animation State Management
// ============================================================================

func (ac *animationComponent) onAnimationDone(animName string) {
	if ac.curAnimState != nil && ac.curAnimState.Name == animName {
		ac.playDefaultAnim()
	}
}

func (ac *animationComponent) stopAnimState(state *animState) {
	if state == nil {
		return
	}
	state.IsCanceled = true
}

func (ac *animationComponent) stopAnimAudio(state *animState) {
	if state != nil {
		if state.AudioName != "" {
			ac.sprite.g.stopSoundInstance(state.AudioId)
		}
	}
}

func (ac *animationComponent) playAnimAudio(ani *aniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		info.AudioName = ani.OnStart.Play
		info.AudioId = ac.sprite.playAudio(info.AudioName, false)
	}
}

// ============================================================================
// Animation Utility Methods
// ============================================================================

func (ac *animationComponent) adaptAnimBitmapResolution(ani *aniConfig) {
	renderScale := ac.sprite.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	ac.sprite.syncSprite.SetRenderScale(mathf.NewVec2(renderScale, renderScale))
}

func (ac *animationComponent) getFromAnToForAni(anitype aniTypeEnum, from any, to any) (any, any) {
	if anitype == aniTypeFrame {
		return ac.getFromAnToForAniFrames(from, to)
	}
	return from, to
}

func (ac *animationComponent) getFromAnToForAniFrames(from any, to any) (float64, float64) {
	fromval := 0.0
	toval := 0.0
	switch v := from.(type) {
	case SpriteCostumeName:
		fromval = float64(ac.sprite.findCostume(v))
		if fromval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		fromval, _ = tools.GetFloat(from)
	}

	switch v := to.(type) {
	case SpriteCostumeName:
		toval = float64(ac.sprite.findCostume(v))
		if toval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		toval, _ = tools.GetFloat(to)
	}

	return fromval, toval
}

func (ac *animationComponent) hasAnim(animName string) bool {
	if _, ok := ac.animations[animName]; ok {
		return true
	}
	return false
}

func (ac *animationComponent) getAnimation(animName SpriteAnimationName) (*aniConfig, bool) {
	ani, ok := ac.animations[animName]
	return ani, ok
}

func (ac *animationComponent) getStateAnimName(stateName string) string {
	if bindingName, ok := ac.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}
