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
	spriteName   string
	ani          *aniConfig
	costumes     []*costume
	isCostumeSet bool
	loadOnce     sync.Once
}

func (aw *animationWrapper) ensureRegistered(animName string) {
	aw.loadOnce.Do(func() {
		createAnimation(aw.spriteName, animName, aw.ani, aw.costumes, aw.isCostumeSet)
	})
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
	p.animation().adaptAnimBitmapResolution(ani)
}

func (p *SpriteImpl) getStateAnimName(stateName string) string {
	return p.animation().getStateAnimName(stateName)
}

func (p *SpriteImpl) hasAnim(animName string) bool {
	return p.animation().hasAnim(animName)
}

func (p *SpriteImpl) getAnimation(animName SpriteAnimationName) (*aniConfig, bool) {
	return p.animation().getAnimation(animName)
}

// -----------------------------------------------------------------------------
// Animation State Management
// -----------------------------------------------------------------------------

func (p *SpriteImpl) onAnimationDone(animName string) {
	p.animation().onAnimationDone(animName)
}

// -----------------------------------------------------------------------------
// Core Animation Functions
// -----------------------------------------------------------------------------

func (p *SpriteImpl) doTween(name SpriteAnimationName, ani *aniConfig) {
	p.animation().doTween(name, ani)
}

func (p *SpriteImpl) playDefaultAnim() {
	p.animation().playDefaultAnim()
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
	p.animation().Animate(name, loop)
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if debugInstr {
		spxlog.Debug("==> AnimateAndWait %s", name)
	}
	p.animation().AnimateAndWait(name)
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	p.animation().StopAnimation(name)
}
