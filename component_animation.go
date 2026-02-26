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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

// ============================================================================
// Animation Component
// ============================================================================
// This component encapsulates all animation-related functionality.

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
	// Always initialize from config
	a.initFromConfig(spriteCfg)
	a.donedAnimations = make([]string, 0)
}

// initFromConfig initializes animations from sprite configuration.
func (a *animationComponent) initFromConfig(spriteCfg *spriteConfig) {
	// Create shared animation data
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

		// Set default values
		setDefaultIfZero(&ani.FrameFps, 25)
		setDefaultIfZero(&ani.TurnToDuration, 1.0)
		setDefaultIfZero(&ani.StepDuration, 0.01)

		// Calculate frame ranges and duration
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
			isCostumeSet: a.sprite.isCostumeSet,
		}
	}
}

// cloneFrom creates a new animation component by cloning from source.
func (a *animationComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcAnim := src.(*animationComponent)
	newAnim := &animationComponent{
		componentBase:   componentBase{sprite: newSprite},
		shared:          srcAnim.shared, // Share all animation data including wrappers (Flyweight Pattern)
		curAnimState:    nil,
		curTweenState:   nil,
		donedAnimations: make([]string, 0),
	}
	// Wrappers are shared via sync.Once - register only happens once for all clones
	return newAnim
}

// onDestroy cleanup when component is destroyed.
func (a *animationComponent) onDestroy() {
	a.stopAnimState(a.curAnimState)
	a.stopAnimState(a.curTweenState)
	a.unRegisterOnAnimationLooped()
	a.unRegisterOnAnimationFinished()
}

// ============================================================================
// Animation Playback Methods
// ============================================================================

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

// ============================================================================
// Core Animation Implementation
// ============================================================================

func (a *animationComponent) playAnimation(name SpriteAnimationName, loop, blocking bool, debugMsg string) {
	if debugInstr {
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
		AniType:    aniTypeFrame,
		IsCanceled: false,
		Name:       animName,
		Speed:      speed,
	}

	info := a.curAnimState
	if playAudio {
		a.playAnimAudio(ani, info)
	}

	syncCheckUpdateCostume(&a.sprite.baseObj)
	a.prepareAnimationPlayback(animName, ani)

	spriteMgr.PlayAnim(a.sprite.syncSprite.GetId(), animName, speed, loop, false)
	if isBlocking {
		a.sprite.isAnimating = true
		for spriteMgr.IsPlayingAnim(a.sprite.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		a.sprite.isAnimating = false
		a.stopAnimState(info)
	}
}

// tweenParams holds pre-calculated parameters for tween animations
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
		AniType:    ani.AniType,
		Name:       name,
		Speed:      ani.Speed,
		IsCanceled: false,
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

// prepareTweenParams pre-calculates animation parameters based on animation type
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
		if enabledPhysics && physicsMode != NoPhysics && physicsMode != StaticPhysics {
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
		if enabledPhysics && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			a.sprite.SetVelocity(0, 0)
		}
	}

	a.stopAnimState(info)
	a.curTweenState = nil
	if name != a.shared.defaultAnimation && !ani.IsKeepOnStop {
		a.playDefaultAnim()
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
		spriteMgr.PlayAnim(a.sprite.syncSprite.GetId(), animName, speed, true, false)
	} else {
		a.sprite.goSetCostume(a.sprite.defaultCostumeIndex)
	}
}

// ============================================================================
// Animation State Management
// ============================================================================

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

func (a *animationComponent) playAnimAudio(ani *aniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		info.AudioName = ani.OnStart.Play
		info.AudioId = a.sprite.playAudio(info.AudioName, false)
	}
}

// ============================================================================
// Animation Utility Methods
// ============================================================================

func (a *animationComponent) adaptAnimBitmapResolution(ani *aniConfig) {
	renderScale := a.sprite.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	a.sprite.syncSprite.SetRenderScale(mathf.NewVec2(renderScale, renderScale))
}

func (a *animationComponent) prepareAnimationPlayback(animName SpriteAnimationName, ani *aniConfig) {
	a.shared.animationWrappers[animName].ensureRegistered(animName)
	a.adaptAnimBitmapResolution(ani)
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
	if _, ok := a.shared.animations[animName]; ok {
		return true
	}

	return false
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

// ============================================================================
// Animation Event Methods
// ============================================================================

func (a *animationComponent) registerOnAnimationLooped(f func()) {
	a.sprite.syncSprite.RegisterOnAnimationLooped(f)
}

func (a *animationComponent) unRegisterOnAnimationLooped() {
	a.sprite.syncSprite.UnRegisterOnAnimationLooped()
}

func (a *animationComponent) registerOnAnimationFinished(f func()) {
	a.sprite.syncSprite.RegisterOnAnimationFinished(f)
}

func (a *animationComponent) unRegisterOnAnimationFinished() {
	a.sprite.syncSprite.UnRegisterOnAnimationFinished()
}
