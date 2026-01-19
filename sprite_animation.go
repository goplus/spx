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
	"sync"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

// ======================== Animation Component ========================
// This file contains animation-related functionality for sprites,
// including animation playback, tweening, and animation state management.
// All methods proxy to the animation component implementation.

// -----------------------------------------------------------------------------
// Animation Wrapper and State (Shared Types)
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
// Animation Utility Functions (Delegated or Shared)
// -----------------------------------------------------------------------------

func (p *SpriteImpl) adaptAnimBitmapResolution(ani *aniConfig) {
	p.components.Animation().adaptAnimBitmapResolution(ani)
}

func (p *SpriteImpl) getFromAnToForAni(anitype aniTypeEnum, from any, to any) (any, any) {
	return p.components.Animation().getFromAnToForAni(anitype, from, to)
}

func (p *SpriteImpl) getFromAnToForAniFrames(from any, to any) (float64, float64) {
	return p.components.Animation().getFromAnToForAniFrames(from, to)
}

func (p *SpriteImpl) getStateAnimName(stateName string) string {
	return p.components.Animation().getStateAnimName(stateName)
}

func (p *SpriteImpl) hasAnim(animName string) bool {
	return p.components.Animation().hasAnim(animName)
}

// -----------------------------------------------------------------------------
// Animation State Management
// -----------------------------------------------------------------------------

func (p *SpriteImpl) onAnimationDone(animName string) {
	p.components.Animation().onAnimationDone(animName)
}

func (p *SpriteImpl) stopAnimState(state *animState) {
	p.components.Animation().stopAnimState(state)
}

func (p *SpriteImpl) stopAnimAudio(state *animState) {
	p.components.Animation().stopAnimAudio(state)
}

func (p *SpriteImpl) playAnimAudio(ani *aniConfig, info *animState) {
	p.components.Animation().playAnimAudio(ani, info)
}

// -----------------------------------------------------------------------------
// Core Animation Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) doAnimation(animName SpriteAnimationName, ani *aniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) {
	p.components.Animation().doAnimation(animName, ani, loop, speed, isBlocking, playAudio)
}

func (p *SpriteImpl) doTween(name SpriteAnimationName, ani *aniConfig) {
	p.components.Animation().doTween(name, ani)
}

func (p *SpriteImpl) playDefaultAnim() {
	p.components.Animation().playDefaultAnim()
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
	p.components.Animation().Animate(name, loop)
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("==> AnimateAndWait %s", name)
	}
	p.components.Animation().AnimateAndWait(name)
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	p.components.Animation().StopAnimation(name)
}
