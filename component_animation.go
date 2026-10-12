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
	"maps"
	"slices"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
	"github.com/goplus/spx/v3/internal/time"
)

// ============================================================================
// Animation Component
// ============================================================================
// This component manages sprite frame playback and tween-driven motion state.

// sharedAnimationData stores read-only animation data shared across cloned sprites.
type sharedAnimationData struct {
	animations       map[SpriteAnimationName]*animationEntry
	animBindings     map[string]string
	defaultAnimation SpriteAnimationName
}

type animationComponent struct {
	sprite *SpriteImpl

	// Shared animation configuration (read-only, shared across clones).
	shared *sharedAnimationData

	// Animation state (per-instance).
	curAnimState      *animState
	activeTweenStates []*animState
	velocityOwner     *animState
	defaultAnimActive bool

	// Animation tracking (per-instance).
	doneAnimations []*animState
}

// tweenParams holds typed endpoints and derived velocity.
type tweenParams struct {
	aniType      coreproject.AniType
	duration     float64
	speed        float64
	moveFrom     mathf.Vec2
	moveTo       mathf.Vec2
	moveVelocity mathf.Vec2
	turnFrom     float64
	turnTo       float64
}

// ============================================================================
// Lifecycle
// ============================================================================

// initialize initializes the animation component from prepared configuration.
func (a *animationComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	a.sprite = sprite
	a.shared = &sharedAnimationData{
		defaultAnimation: spriteCfg.DefaultAnimation,
		animations:       make(map[SpriteAnimationName]*animationEntry, len(spriteCfg.FAnimations)),
		animBindings:     maps.Clone(spriteCfg.AnimBindings),
	}

	for name, ani := range spriteCfg.FAnimations {
		a.shared.animations[name] = &animationEntry{
			name:         name,
			config:       ani,
			spriteName:   a.sprite.name,
			costumes:     a.sprite.costumes,
			isCostumeSet: a.sprite.runtimeState.IsCostumeSet,
		}
	}
}

// cloneFor creates a new animation component for newSprite.
func (a *animationComponent) cloneFor(newSprite *SpriteImpl) *animationComponent {
	return &animationComponent{
		sprite: newSprite,
		shared: a.shared,
	}
}

// onDestroy cleans up when the component is destroyed.
func (a *animationComponent) onDestroy() {
	a.stopAnimState(a.curAnimState)
	for _, state := range a.activeTweenStates {
		a.stopAnimState(state)
	}
	a.activeTweenStates = nil
	if syncSprite := a.syncSprite(); syncSprite != nil {
		syncSprite.UnRegisterOnAnimationLooped()
		syncSprite.UnRegisterOnAnimationFinished()
	}
}

// ============================================================================
// Animation Control
// ============================================================================

func (a *animationComponent) stopAnimation(name SpriteAnimationName) {
	if name == "" || !a.hasAnim(name) {
		return
	}
	state := a.curAnimState
	if state == nil || state.Name != name {
		return
	}
	syncSprite := a.syncSpriteForPlayback()
	if syncSprite == nil {
		return
	}

	if !a.stopCurrentAnimState(state) {
		return
	}
	syncSprite.PauseAnim()
	a.playDefaultAnim()
}

// ============================================================================
// Animation Playback
// ============================================================================

func (a *animationComponent) playAnimation(name SpriteAnimationName, loop, blocking bool) {
	entry, ok := a.shared.animations[name]
	if !ok {
		spxlog.Warn("Animation not found: %s", name)
		return
	}

	a.doAnimation(entry, loop, 1, blocking, true)
}

func (a *animationComponent) doAnimation(entry *animationEntry, loop bool, speed float64, isBlocking bool, playAudio bool) *animState {
	syncSprite := a.syncSpriteForPlayback()
	if syncSprite == nil {
		return nil
	}

	a.stopAnimState(a.curAnimState)
	a.defaultAnimActive = false
	a.curAnimState = &animState{
		AniType: coreproject.AniTypeFrame,
		Name:    entry.name,
		Speed:   speed,
	}

	info := a.curAnimState
	completed := false
	playbackStarted := false
	defer func() {
		if completed && !isBlocking {
			return
		}
		current := a.curAnimState == info
		audioID := a.cancelAnimState(info)
		if !completed && current {
			a.curAnimState = nil
		}
		gco.CleanupMainThread(func() {
			a.stopAnimationAudio(audioID)
			if !completed && playbackStarted && current && a.syncSpriteForPlayback() != nil {
				syncSprite.PauseAnim()
			}
		})
	}()

	// Capture resources before cancellation can discard an engine-call result.
	engine.WaitMainThread(func() {
		if playAudio {
			a.playAnimationAudio(entry.config, info)
		}
		a.sprite.baseObj.applyCostumeUpdate()
		a.prepareAnimationPlayback(entry, syncSprite)
		playbackStarted = true
		engine.Managers().SpriteMgr.PlayAnim(syncSprite.GetId(), entry.name, speed, loop, false)
	})
	if isBlocking {
		a.sprite.runtimeState.IsAnimating = true
		for engine.Managers().SpriteMgr.IsPlayingAnim(syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
	}
	completed = true
	return info
}

func (a *animationComponent) prepareAnimationPlayback(entry *animationEntry, syncSprite *engine.Sprite) {
	bitmapResolution := entry.ensureRegistered()
	renderScale := a.sprite.getAnimRenderScale(bitmapResolution)
	syncSprite.SetRenderScale(engine.UniformVec2(renderScale))
}

// ============================================================================
// Default Animation
// ============================================================================

func (a *animationComponent) playDefaultAnim() {
	animName := ""
	syncSprite := a.syncSpriteForPlayback()
	if syncSprite == nil {
		return
	}
	if !a.sprite.spriteState.IsVisible || a.sprite.spriteState.IsDying {
		return
	}

	speed := 1.0
	if tweenState := a.getCurTweenState(); tweenState != nil {
		animName = a.getTweenAnimName(tweenState.AniType)
		speed = tweenState.Speed
	}

	if animName == "" {
		animName = a.shared.defaultAnimation
	}

	if entry, ok := a.shared.animations[animName]; ok {
		a.prepareAnimationPlayback(entry, syncSprite)
		engine.Managers().SpriteMgr.PlayAnim(syncSprite.GetId(), entry.name, speed, true, false)
		a.defaultAnimActive = true
	} else {
		a.defaultAnimActive = false
		a.sprite.goSetCostume(a.sprite.spriteState.DefaultCostumeIndex)
	}
}

func (a *animationComponent) playDefaultAnimIfIdle() {
	if a.hasActiveAnimationPlayback() {
		return
	}
	a.playDefaultAnim()
}

func (a *animationComponent) hasActiveAnimationPlayback() bool {
	return a.defaultAnimActive || (a.curAnimState != nil && !a.curAnimState.IsCanceled)
}

// ============================================================================
// Animation Events
// ============================================================================

func (a *animationComponent) onAnimationDone(state *animState) {
	if a.syncSpriteForPlayback() == nil {
		return
	}
	if state != nil && a.curAnimState == state {
		if !a.stopCurrentAnimState(state) {
			return
		}
		a.playDefaultAnim()
	}
}

// ============================================================================
// Animation Completion
// ============================================================================

func (a *animationComponent) addDoneAnimation(state *animState) {
	a.doneAnimations = append(a.doneAnimations, state)
}

func (a *animationComponent) takeDoneAnimations(buffer []*animState) []*animState {
	buffer = append(buffer, a.doneAnimations...)
	clear(a.doneAnimations)
	a.doneAnimations = a.doneAnimations[:0]
	return buffer
}

// ============================================================================
// Animation State
// ============================================================================

func (a *animationComponent) stopCurrentAnimState(state *animState) bool {
	current := a.curAnimState == state
	id := a.cancelAnimState(state)
	if current {
		a.curAnimState = nil
	}
	a.stopAnimationAudio(id)
	return current
}

func (a *animationComponent) stopAnimState(state *animState) {
	a.stopAnimationAudio(a.cancelAnimState(state))
}

// cancelAnimState detaches local state before any engine cleanup can fail.
func (a *animationComponent) cancelAnimState(state *animState) int64 {
	if state == nil {
		return 0
	}
	engine.Lock()
	state.IsCanceled = true
	state.OnPlayAudioRestartPending = false
	id := state.OnPlayAudioPlaybackID
	state.OnPlayAudioPlaybackID = 0
	if a.curAnimState == state {
		a.sprite.runtimeState.IsAnimating = false
	}
	engine.Unlock()
	return id
}

// ============================================================================
// Animation Lookup
// ============================================================================

func (a *animationComponent) hasAnim(animName string) bool {
	_, ok := a.shared.animations[animName]
	return ok
}

func (a *animationComponent) getAnimation(animName SpriteAnimationName) (*coreproject.AniConfig, bool) {
	entry, ok := a.shared.animations[animName]
	if !ok {
		return nil, false
	}
	return entry.config, true
}

func (a *animationComponent) getStateAnimName(stateName string) string {
	if bindingName, ok := a.shared.animBindings[stateName]; ok {
		return bindingName
	}
	return stateName
}

func (a *animationComponent) getTweenAnimName(aniType coreproject.AniType) string {
	switch aniType {
	case coreproject.AniTypeMove:
		return a.getStateAnimName(StateStep)
	case coreproject.AniTypeTurn:
		return a.getStateAnimName(StateTurn)
	case coreproject.AniTypeGlide:
		return a.getStateAnimName(StateGlide)
	}
	return ""
}

// ============================================================================
// Tween State
// ============================================================================

func (a *animationComponent) getCurTweenState() *animState {
	if len(a.activeTweenStates) == 0 {
		return nil
	}
	return a.activeTweenStates[len(a.activeTweenStates)-1]
}

func (a *animationComponent) unregisterTweenState(state *animState) bool {
	i := slices.Index(a.activeTweenStates, state)
	if i < 0 {
		return false
	}
	a.activeTweenStates = slices.Delete(a.activeTweenStates, i, i+1)
	return true
}

// ============================================================================
// Tween Execution
// ============================================================================

func (a *animationComponent) doTween(name SpriteAnimationName, base *coreproject.AniConfig, params tweenParams) {
	params, ok := prepareTweenParams(params)
	if !ok {
		return
	}

	info := &animState{AniType: params.aniType, Name: name, Speed: params.speed}
	a.activeTweenStates = append(a.activeTweenStates, info)
	var ownedPlayback *animState
	completed := false
	defer func() { a.cleanupTween(info, ownedPlayback, base, !completed) }()
	if entry, ok := a.shared.animations[name]; ok {
		ownedPlayback = a.doAnimation(entry, true, params.speed, false, false)
		if base != nil {
			engine.WaitMainThread(func() { a.playAnimationAudio(base, info) })
		}
	}
	for elapsed := 0.0; elapsed < params.duration && !info.IsCanceled; {
		elapsed += time.DeltaTime()
		a.applyTweenStep(info, mathf.Clamp01f(elapsed/params.duration), &params)
		engine.WaitNextFrame()
	}
	completed = true
}

func prepareTweenParams(params tweenParams) (tweenParams, bool) {
	duration := params.duration
	if duration <= 0 {
		spxlog.Warn("Invalid animation duration: %v", duration)
		return tweenParams{}, false
	}

	if params.aniType == coreproject.AniTypeMove {
		params.moveVelocity = params.moveTo.Sub(params.moveFrom).Mulf(1 / duration)
	}

	return params, true
}

func (a *animationComponent) applyTweenStep(info *animState, percent float64, params *tweenParams) {
	switch params.aniType {
	case coreproject.AniTypeMove:
		physicsMode := a.sprite.PhysicsMode()
		if a.sprite.g.physicsEnabled && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			a.velocityOwner = info
			a.sprite.SetVelocity(params.moveVelocity.X, params.moveVelocity.Y)
			return
		}
		fallthrough
	case coreproject.AniTypeGlide:
		pos := params.moveFrom.Lerp(params.moveTo, percent)
		a.sprite.SetXYpos(pos.X, pos.Y)
	case coreproject.AniTypeTurn:
		a.sprite.SetHeading(mathf.Lerpf(params.turnFrom, params.turnTo, percent))
	}
}

func (a *animationComponent) cleanupTween(info, ownedPlayback *animState, base *coreproject.AniConfig, interrupted bool) {
	registered := a.unregisterTweenState(info)
	stopVelocity := a.velocityOwner == info
	if stopVelocity {
		a.velocityOwner = nil
	}
	audioID := a.cancelAnimState(info)
	stoppedOwnedPlayback := ownedPlayback != nil && a.curAnimState == ownedPlayback
	if stoppedOwnedPlayback {
		a.stopCurrentAnimState(ownedPlayback)
	}
	restoreDefault := stoppedOwnedPlayback
	if registered {
		restoreDefault = info.Name != a.shared.defaultAnimation && (base == nil || !base.IsKeepOnStop)
	}
	gco.CleanupMainThread(func() {
		a.stopAnimationAudio(audioID)
		syncSprite := a.syncSpriteForPlayback()
		if syncSprite == nil {
			return
		}
		if interrupted && stoppedOwnedPlayback {
			syncSprite.PauseAnim()
		}
		if stopVelocity {
			a.sprite.SetVelocity(0, 0)
		}
		if restoreDefault {
			a.playDefaultAnimIfIdle()
		}
	})
}

// ============================================================================
// Runtime Access
// ============================================================================

func (a *animationComponent) syncSprite() *engine.Sprite {
	if a.sprite == nil {
		return nil
	}
	return a.sprite.runtimeState.SyncSprite
}

func (a *animationComponent) syncSpriteForPlayback() *engine.Sprite {
	if a.sprite == nil || a.sprite.isDestroyed() {
		return nil
	}
	return a.sprite.runtimeState.SyncSprite
}
