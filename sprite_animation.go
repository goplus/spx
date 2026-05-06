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

	intani "github.com/goplus/spx/v2/internal/animation"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
type animationWrapper struct {
	spriteName   string
	ani          *coreproject.AniConfig
	costumes     []*costume
	isCostumeSet bool
	engineMgr    *engineManagers
	loadOnce     sync.Once
}

func (aw *animationWrapper) ensureRegistered(animName string, callerAni *coreproject.AniConfig) {
	aw.loadOnce.Do(func() {
		payloadJSON, maxBitmap, err := intani.BuildPayloadJSON(
			intani.Config{
				FrameFrom: aw.ani.IFrameFrom,
				FrameTo:   aw.ani.IFrameTo,
			},
			buildAnimationSources(aw.costumes),
			aw.isCostumeSet,
		)
		if err != nil {
			panic(err)
		}
		aw.ani.AdaptAnimBitmapResolution = maxBitmap
		// AdaptAnimBitmapResolution is computed during registration.
		// Propagate it to the caller's config when it differs from the canonical aw.ani.
		if callerAni != nil && callerAni != aw.ani {
			callerAni.AdaptAnimBitmapResolution = aw.ani.AdaptAnimBitmapResolution
		}
		aw.engineMgr.ResMgr.CreateAnimation(
			aw.spriteName,
			animName,
			payloadJSON,
			int64(aw.ani.FrameFps),
			aw.isCostumeSet,
		)
	})
}

func buildAnimationSources(costumes []*costume) []intani.FrameSource {
	frames := make([]intani.FrameSource, 0, len(costumes))
	for _, c := range costumes {
		frames = append(frames, intani.FrameSource{
			Path:             c.path,
			BitmapResolution: c.bitmapResolution,
			Center:           c.center,
			ImageSize:        c.imageSize,
			PosX:             c.posX,
			PosY:             c.posY,
			Width:            c.width,
			Height:           c.height,
		})
	}
	return frames
}

type animState struct {
	AniType              coreproject.AniType
	Name                 string
	IsCanceled           bool
	Speed                float64
	LoopReplayAudioName  string
	BoundAudioPlaybackID int64
}

// -----------------------------------------------------------------------------
// Shared Helpers
// -----------------------------------------------------------------------------
func (p *SpriteImpl) getStateAnimName(stateName string) string {
	return p.animation().getStateAnimName(stateName)
}

func (p *SpriteImpl) hasAnim(animName string) bool {
	return p.animation().hasAnim(animName)
}

func (p *SpriteImpl) getAnimation(animName SpriteAnimationName) (*coreproject.AniConfig, bool) {
	return p.animation().getAnimation(animName)
}

// -----------------------------------------------------------------------------
// State
// -----------------------------------------------------------------------------
func (p *SpriteImpl) onAnimationDone(animName string) {
	p.animation().onAnimationDone(animName)
}

func (p *SpriteImpl) flushCompletedAnimations(buffer []string) []string {
	engine.Lock()
	buffer = p.animation().takeDonedAnimations(buffer)
	engine.Unlock()

	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return buffer[:0]
	}

	for _, animName := range buffer {
		p.onAnimationDone(animName)
	}
	return buffer[:0]
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------
func (p *SpriteImpl) doTween(name SpriteAnimationName, ani *coreproject.AniConfig) {
	p.animation().doTween(name, ani)
}

func (p *SpriteImpl) playDefaultAnim() {
	p.animation().playDefaultAnim()
}

// -----------------------------------------------------------------------------
// Playback
// -----------------------------------------------------------------------------
func (p *SpriteImpl) Animate__0(name SpriteAnimationName) {
	p.AnimateWith(name, false)
}

func (p *SpriteImpl) Animate__1(name SpriteAnimationName, loop bool) {
	p.AnimateWith(name, loop)
}

func (p *SpriteImpl) AnimateWith(name SpriteAnimationName, __xgo_optional_loop bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Animate: %s", name)
	}
	p.animation().Animate(name, __xgo_optional_loop)
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if isDebugInstrEnabled() {
		spxlog.Debug("AnimateAndWait: %s", name)
	}
	p.animation().AnimateAndWait(name)
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	p.animation().StopAnimation(name)
}
