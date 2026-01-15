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
	"sync"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

// ======================== Animation Component ========================
// This file contains animation-related functionality for sprites,
// including animation playback, tweening, and animation state management.

// -----------------------------------------------------------------------------
// Animation Wrapper and State
// -----------------------------------------------------------------------------

type animationWrapper struct {
	spr      *SpriteImpl
	ani      *aniConfig
	loaded   bool
	loadOnce sync.Once
}

func (aw *animationWrapper) ensureRegistered(pName string) {
	aw.loadOnce.Do(func() {
		createAnimation(aw.spr.name, pName, aw.ani, aw.spr.costumes, aw.spr.isCostumeSet)
		aw.loaded = true
	})

	aw.spr.adaptAnimBitmapResolution(aw.ani)
}

type animState struct {
	AniType    aniTypeEnum
	Name       string
	IsCanceled bool
	Speed      float64
	AudioName  string
	AudioId    soundId
}

// -----------------------------------------------------------------------------
// Animation Utility Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) adaptAnimBitmapResolution(ani *aniConfig) {
	renderScale := p.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	p.syncSprite.SetRenderScale(mathf.NewVec2(renderScale, renderScale))
}

func (p *SpriteImpl) getFromAnToForAni(anitype aniTypeEnum, from any, to any) (any, any) {
	if anitype == aniTypeFrame {
		return p.getFromAnToForAniFrames(from, to)
	}
	return from, to
}

func (p *SpriteImpl) getFromAnToForAniFrames(from any, to any) (float64, float64) {
	fromval := 0.0
	toval := 0.0
	switch v := from.(type) {
	case SpriteCostumeName:
		fromval = float64(p.findCostume(v))
		if fromval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		fromval, _ = tools.GetFloat(from)
	}

	switch v := to.(type) {
	case SpriteCostumeName:
		toval = float64(p.findCostume(v))
		if toval < 0 {
			log.Panicf("findCostume %s failed", v)
		}
	default:
		toval, _ = tools.GetFloat(to)
	}

	return fromval, toval
}

func (p *SpriteImpl) getStateAnimName(stateName string) string {
	if bindingName, ok := p.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}

func (p *SpriteImpl) hasAnim(animName string) bool {
	if _, ok := p.animations[animName]; ok {
		return true
	}
	return false
}

// getDefaultTweenConfig returns default animation config for Spine mode
// when the animation exists in Spine but has no frame animation config
func (p *SpriteImpl) getDefaultTweenConfig() *aniConfig {
	stepDuration := 0.01
	turnToDuration := 1.0

	// Use values from Spine config if available
	if p.spineConfig != nil {
		if p.spineConfig.StepDuration > 0 {
			stepDuration = p.spineConfig.StepDuration
		}
		if p.spineConfig.TurnToDuration > 0 {
			turnToDuration = p.spineConfig.TurnToDuration
		}
	}

	return &aniConfig{
		FrameFps:       25,
		StepDuration:   stepDuration,
		TurnToDuration: turnToDuration,
		Speed:          1,
	}
}

// -----------------------------------------------------------------------------
// Animation State Management
// -----------------------------------------------------------------------------

func (p *SpriteImpl) onAnimationDone(animName string) {
	if p.curAnimState != nil && p.curAnimState.Name == animName {
		p.playDefaultAnim()
	}
}

func (p *SpriteImpl) stopAnimState(state *animState) {
	if state == nil {
		return
	}
	state.IsCanceled = true
	// don't need to stop audio when anim is canceled
	//p.stopAnimAudio(state)
}

func (p *SpriteImpl) stopAnimAudio(state *animState) {
	if state != nil {
		if state.AudioName != "" {
			p.g.stopSoundInstance(state.AudioId)
		}
	}
}

func (p *SpriteImpl) playAnimAudio(ani *aniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		info.AudioName = ani.OnStart.Play
		info.AudioId = p.playAudio(info.AudioName, false)
	}
}

// -----------------------------------------------------------------------------
// Core Animation Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) doAnimation(animName SpriteAnimationName, ani *aniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) {
	p.stopAnimState(p.curAnimState)
	p.curAnimState = &animState{
		AniType:    aniTypeFrame,
		IsCanceled: false,
		Name:       animName,
		Speed:      speed,
	}
	info := p.curAnimState
	if playAudio {
		p.playAnimAudio(ani, info)
	}

	// Get the actual animation name to play
	actualAnimName := p.getSpineAnimName(animName)

	// Spine mode doesn't need to register frame animation resources
	if !p.isSpineMode() {
		syncCheckUpdateCostume(&p.baseObj)
		p.animationWrappers[animName].ensureRegistered(animName)
	}

	spriteMgr.PlayAnim(p.syncSprite.GetId(), actualAnimName, speed, loop, false)
	if isBlocking {
		p.isAnimating = true
		for spriteMgr.IsPlayingAnim(p.syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		p.isAnimating = false
		p.stopAnimState(info)
	}
}

func (p *SpriteImpl) doTween(name SpriteAnimationName, ani *aniConfig) {
	info := &animState{
		AniType:    ani.AniType,
		Name:       name,
		Speed:      ani.Speed,
		IsCanceled: false,
	}
	p.stopAnimState(p.curTweenState)
	p.curTweenState = info
	animName := info.Name
	// Use hasAnimation to support both Spine and frame animations
	if p.hasAnimation(animName) {
		p.doAnimation(animName, ani, ani.IsLoop, ani.Speed, false, false)
		p.playAnimAudio(ani, info)
	}
	duration := ani.Duration
	timer := 0.0
	prePercent := 0.0
	for timer < duration {
		if info.IsCanceled {
			return
		}
		timer += time.DeltaTime()
		percent := mathf.Clamp01f(timer / duration)
		deltaPercent := percent - prePercent
		prePercent = percent
		switch ani.AniType {
		case aniTypeMove:
			src, _ := tools.GetVec2(ani.From)
			dst, _ := tools.GetVec2(ani.To)
			diff := dst.Sub(src)
			if enabledPhysics && p.physicsMode != NoPhysics && p.physicsMode != StaticPhysics {
				speed := diff.Length() / duration
				dir := diff.Normalize()
				vel := dir.Mulf(speed)
				p.SetVelocity(vel.X, vel.Y)
			} else {
				val := diff.Mulf(deltaPercent)
				p.ChangeXYpos(val.X, val.Y)
			}
		case aniTypeGlide:
			src, _ := tools.GetVec2(ani.From)
			dst, _ := tools.GetVec2(ani.To)
			diff := dst.Sub(src)
			val := diff.Mulf(deltaPercent)
			p.ChangeXYpos(val.X, val.Y)
		case aniTypeTurn:
			src, _ := tools.GetFloat(ani.From)
			dst, _ := tools.GetFloat(ani.To)
			diff := dst - src
			val := diff * deltaPercent
			p.ChangeHeading(val)
		}
		engine.WaitNextFrame()
	}
	switch ani.AniType {
	case aniTypeMove:
		if enabledPhysics && p.physicsMode != NoPhysics && p.physicsMode != StaticPhysics {
			p.SetVelocity(0, 0)
		}
	}
	p.stopAnimState(info)
	p.curTweenState = nil
	if animName != p.defaultAnimation && !ani.IsKeepOnStop {
		p.playDefaultAnim()
	}
}

func (p *SpriteImpl) playDefaultAnim() {
	animName := ""
	if !p.isVisible || p.isDying {
		return
	}
	speed := 1.0
	if p.curTweenState == nil {
		animName = p.defaultAnimation
	} else {
		switch p.curTweenState.AniType {
		case aniTypeMove:
			animName = p.getStateAnimName(StateStep)
		case aniTypeTurn:
			animName = p.getStateAnimName(StateTurn)
		case aniTypeGlide:
			animName = p.getStateAnimName(StateGlide)
		}
		speed = p.curTweenState.Speed
	}

	if animName == "" {
		animName = p.defaultAnimation
	}

	// Check if animation exists
	if !p.hasAnimation(animName) {
		p.goSetCostume(p.defaultCostumeIndex)
		return
	}

	// Get the actual animation name to play
	actualAnimName := p.getSpineAnimName(animName)

	// Spine mode doesn't need to register frame animation resources
	if !p.isSpineMode() {
		p.animationWrappers[animName].ensureRegistered(animName)
	}

	spriteMgr.PlayAnim(p.syncSprite.GetId(), actualAnimName, speed, true, false)
}

// -----------------------------------------------------------------------------
// Public Animation API
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Animate__0(name SpriteAnimationName) {
	p.Animate__1(name, false)
}

func (p *SpriteImpl) Animate__1(name SpriteAnimationName, loop bool) {
	if debugInstr {
		spxlog.Debug("==> Animation %s", name)
	}

	// Unified check if animation exists (supports both Spine and frame animations)
	if !p.hasAnimation(name) {
		spxlog.Debug("Animation not found:", name)
		return
	}

	// Get animation config (may be nil for pure Spine animations)
	ani := p.animations[name]
	if ani == nil {
		ani = &aniConfig{}
	}
	p.doAnimation(name, ani, loop, 1, false, true)
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("==> AnimateAndWait %s", name)
	}

	// Unified check if animation exists (supports both Spine and frame animations)
	if !p.hasAnimation(name) {
		spxlog.Debug("Animation not found:", name)
		return
	}

	// Get animation config (may be nil for pure Spine animations)
	ani := p.animations[name]
	if ani == nil {
		ani = &aniConfig{}
	}
	p.doAnimation(name, ani, false, 1, true, true)
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	// Use hasAnimation to support both Spine and frame animations
	if name == "" || !p.hasAnimation(name) {
		return
	}
	if p.curAnimState == nil || p.curAnimState.Name != name {
		return
	}
	p.syncSprite.PauseAnim()
	p.playDefaultAnim()
}
