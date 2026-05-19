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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/base/defaults"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/tools"
)

// sharedAnimationData contains read-only animation data shared across cloned sprites (Flyweight Pattern).
type sharedAnimationData struct {
	animations        map[SpriteAnimationName]*coreproject.AniConfig
	animBindings      map[string]string
	defaultAnimation  SpriteAnimationName
	animationWrappers map[SpriteAnimationName]*animationWrapper
}

type animationComponent struct {
	componentBase

	// Shared animation configuration (read-only, shared across clones)
	shared *sharedAnimationData

	// Animation state (per-instance)
	curAnimState      *animState
	curTweenState     *animState
	defaultAnimActive bool

	// Animation tracking (per-instance)
	donedAnimations []string

	pendingBoundAudioReplays []boundAudioReplay
	drainedBoundAudioReplays []boundAudioReplay
}

type boundAudioReplay struct {
	state *animState
	name  SoundName
}

// initialize initializes the animation component from configuration.
func (a *animationComponent) initialize(sprite *SpriteImpl, spriteCfg *coreproject.SpriteConfig) {
	a.componentBase.initialize(sprite, spriteCfg)
	a.initFromConfig(spriteCfg)
	a.donedAnimations = make([]string, 0)
	a.pendingBoundAudioReplays = make([]boundAudioReplay, 0)
	a.drainedBoundAudioReplays = make([]boundAudioReplay, 0)
}

// initFromConfig initializes animations from sprite configuration.
func (a *animationComponent) initFromConfig(spriteCfg *coreproject.SpriteConfig) {
	a.shared = &sharedAnimationData{
		defaultAnimation:  spriteCfg.DefaultAnimation,
		animations:        make(map[SpriteAnimationName]*coreproject.AniConfig),
		animBindings:      make(map[string]string),
		animationWrappers: make(map[SpriteAnimationName]*animationWrapper),
	}

	anims := spriteCfg.FAnimations
	for key, val := range anims {
		var ani = val
		_, ok := a.shared.animations[key]
		if ok {
			spxlog.Panicf("Animation key [%s] already exists", key)
		}

		defaults.SetDefaultIfZero(&ani.FrameFps, 25)
		defaults.SetDefaultIfZero(&ani.TurnToDuration, 1.0)
		defaults.SetDefaultIfZero(&ani.StepDuration, 0.01)

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
			isCostumeSet: a.sprite.runtimeState.IsCostumeSet,
			engineMgr:    a.engine(),
		}
	}
}

// cloneFrom creates a new animation component by cloning from source.
func (a *animationComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	srcAnim := src.(*animationComponent)
	newAnim := &animationComponent{
		componentBase:            componentBase{sprite: newSprite},
		shared:                   srcAnim.shared,
		donedAnimations:          make([]string, 0),
		pendingBoundAudioReplays: make([]boundAudioReplay, 0),
		drainedBoundAudioReplays: make([]boundAudioReplay, 0),
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

func (a *animationComponent) registerOnAnimationLooped(f func()) {
	if syncSprite := a.syncSprite(); syncSprite != nil {
		syncSprite.RegisterOnAnimationLooped(f)
	}
}

func (a *animationComponent) unRegisterOnAnimationLooped() {
	if syncSprite := a.syncSprite(); syncSprite != nil {
		syncSprite.UnRegisterOnAnimationLooped()
	}
}

func (a *animationComponent) registerOnAnimationFinished(f func()) {
	if syncSprite := a.syncSprite(); syncSprite != nil {
		syncSprite.RegisterOnAnimationFinished(f)
	}
}

func (a *animationComponent) unRegisterOnAnimationFinished() {
	if syncSprite := a.syncSprite(); syncSprite != nil {
		syncSprite.UnRegisterOnAnimationFinished()
	}
}

func (a *animationComponent) Animate(name SpriteAnimationName, loop bool) {
	a.playAnimation(name, loop, false, "Animation: %s")
}

func (a *animationComponent) AnimateAndWait(name SpriteAnimationName) {
	a.playAnimation(name, false, true, "AnimateAndWait: %s")
}

func (a *animationComponent) StopAnimation(name SpriteAnimationName) {
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

func (a *animationComponent) doAnimation(animName SpriteAnimationName, ani *coreproject.AniConfig, loop bool, speed float64, isBlocking bool, playAudio bool) *animState {
	syncSprite := a.syncSpriteForPlayback()
	if syncSprite == nil {
		return nil
	}

	a.stopAnimState(a.curAnimState)
	a.defaultAnimActive = false
	a.curAnimState = &animState{
		AniType: coreproject.AniTypeFrame,
		Name:    animName,
		Speed:   speed,
	}

	info := a.curAnimState
	if playAudio {
		a.playAnimAudio(ani, info)
	}

	a.sprite.baseObj.applyCostumeUpdate()
	a.prepareAnimationPlayback(animName, ani)

	a.engine().SpriteMgr.PlayAnim(syncSprite.GetId(), animName, speed, loop, false)
	if isBlocking {
		a.sprite.runtimeState.IsAnimating = true
		for a.engine().SpriteMgr.IsPlayingAnim(syncSprite.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
		a.sprite.runtimeState.IsAnimating = false
		a.stopAnimState(info)
	}
	return info
}

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
	if a.curTweenState == nil {
		animName = a.shared.defaultAnimation
	} else {
		switch a.curTweenState.AniType {
		case coreproject.AniTypeMove:
			animName = a.sprite.getStateAnimName(StateStep)
		case coreproject.AniTypeTurn:
			animName = a.sprite.getStateAnimName(StateTurn)
		case coreproject.AniTypeGlide:
			animName = a.sprite.getStateAnimName(StateGlide)
		}
		speed = a.curTweenState.Speed
	}

	if animName == "" {
		animName = a.shared.defaultAnimation
	}

	if ani, ok := a.shared.animations[animName]; ok {
		a.prepareAnimationPlayback(animName, ani)
		a.engine().SpriteMgr.PlayAnim(syncSprite.GetId(), animName, speed, true, false)
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
	if a.defaultAnimActive {
		return true
	}
	return a.curAnimState != nil && !a.curAnimState.IsCanceled
}

func (a *animationComponent) playAnimAudio(ani *coreproject.AniConfig, info *animState) {
	if ani.OnStart != nil && ani.OnStart.Play != "" {
		loop := ani.OnStart.GetLoop(false)
		if !loop {
			info.LoopReplayAudioName = ani.OnStart.Play
		}
		a.sprite.playAudio(ani.OnStart.Play, loop)
	}
	if ani.OnPlay != nil && ani.OnPlay.Play != "" {
		loop := ani.OnPlay.GetLoop(true)
		if !loop {
			info.BoundLoopReplayAudio = ani.OnPlay.Play
		}
		a.trackBoundAudioPlayback(info, a.sprite.playAudio(ani.OnPlay.Play, loop))
	}
}

func (a *animationComponent) stopAnimPlaybackAudio(state *animState) {
	if state == nil {
		return
	}
	for _, id := range state.BoundAudioPlaybackIDs {
		a.sprite.stopAudioPlayback(id)
	}
	state.BoundAudioPlaybackIDs = state.BoundAudioPlaybackIDs[:0]
}

func (a *animationComponent) trackBoundAudioPlayback(state *animState, id int64) {
	if state == nil || id == 0 {
		return
	}
	state.BoundAudioPlaybackIDs = a.pruneActiveBoundAudioPlaybackIDs(state.BoundAudioPlaybackIDs)
	state.BoundAudioPlaybackIDs = append(state.BoundAudioPlaybackIDs, id)
}

func (a *animationComponent) pruneActiveBoundAudioPlaybackIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return ids
	}
	live := ids[:0]
	for _, id := range ids {
		if a.sprite != nil && a.sprite.g != nil && a.sprite.g.soundMgr.IsPlaying(id) {
			live = append(live, id)
		}
	}
	return live
}

func (a *animationComponent) addPendingBoundAudioReplay(state *animState, name SoundName) {
	if state == nil || name == "" {
		return
	}
	a.pendingBoundAudioReplays = append(a.pendingBoundAudioReplays, boundAudioReplay{
		state: state,
		name:  name,
	})
}

func (a *animationComponent) takePendingBoundAudioReplays() []boundAudioReplay {
	a.drainedBoundAudioReplays = a.drainedBoundAudioReplays[:0]
	a.drainedBoundAudioReplays = append(a.drainedBoundAudioReplays, a.pendingBoundAudioReplays...)
	a.pendingBoundAudioReplays = a.pendingBoundAudioReplays[:0]
	return a.drainedBoundAudioReplays
}

func (a *animationComponent) adaptAnimBitmapResolution(ani *coreproject.AniConfig) {
	syncSprite := a.syncSprite()
	if syncSprite == nil {
		return
	}
	renderScale := a.sprite.getAnimRenderScale(ani.AdaptAnimBitmapResolution)
	syncSprite.SetRenderScale(engine.UniformVec2(renderScale))
}

func (a *animationComponent) prepareAnimationPlayback(animName SpriteAnimationName, ani *coreproject.AniConfig) {
	a.shared.animationWrappers[animName].ensureRegistered(animName, ani)
	a.adaptAnimBitmapResolution(ani)
}

func (a *animationComponent) onAnimationDone(animName string) {
	if a.syncSpriteForPlayback() == nil {
		return
	}
	if state := a.curAnimState; state != nil && state.Name == animName {
		if !a.stopCurrentAnimState(state) {
			return
		}
		a.playDefaultAnim()
	}
}

func (a *animationComponent) stopCurrentAnimState(state *animState) bool {
	a.stopAnimState(state)
	if a.curAnimState != state {
		return false
	}
	a.curAnimState = nil
	return true
}

func (a *animationComponent) stopAnimState(state *animState) {
	if state == nil {
		return
	}
	a.stopAnimPlaybackAudio(state)
	state.IsCanceled = true
}

func (a *animationComponent) costumeIndex(nameOrIndex any) int {
	switch v := nameOrIndex.(type) {
	case SpriteCostumeName:
		idx := a.sprite.findCostume(v)
		if idx < 0 {
			spxlog.Panicf("FindCostume failed for %s", v)
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
	_, ok := a.shared.animations[animName]
	return ok
}

func (a *animationComponent) getAnimation(animName SpriteAnimationName) (*coreproject.AniConfig, bool) {
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

// tweenParams holds pre-calculated parameters for tween animations.
type tweenParams struct {
	moveDiff  mathf.Vec2
	moveSpeed float64
	moveDir   mathf.Vec2
	turnDiff  float64
}

func (a *animationComponent) doTween(name SpriteAnimationName, ani *coreproject.AniConfig) {
	info, ownedPlayback := a.initTweenState(name, ani)
	if info == nil {
		return
	}

	params, ok := a.prepareTweenParams(ani)
	if !ok {
		return
	}

	a.executeTweenLoop(info, ani, params)
	a.cleanupTween(info, ownedPlayback, name, ani)
}

func (a *animationComponent) initTweenState(name SpriteAnimationName, ani *coreproject.AniConfig) (*animState, *animState) {
	if ani.Duration <= 0 {
		spxlog.Warn("Invalid animation duration: %v", ani.Duration)
		return nil, nil
	}

	info := &animState{
		AniType: ani.AniType,
		Name:    name,
		Speed:   ani.Speed,
	}
	a.stopAnimState(a.curTweenState)
	a.curTweenState = info

	var ownedPlayback *animState
	if a.hasAnim(name) {
		ownedPlayback = a.doAnimation(name, ani, ani.IsLoop, ani.Speed, false, false)
		a.playAnimAudio(ani, info)
	}

	return info, ownedPlayback
}

func (a *animationComponent) prepareTweenParams(ani *coreproject.AniConfig) (*tweenParams, bool) {
	params := &tweenParams{}
	duration := ani.Duration

	switch ani.AniType {
	case coreproject.AniTypeMove, coreproject.AniTypeGlide:
		src, srcOk := tools.GetVec2(ani.From)
		dst, dstOk := tools.GetVec2(ani.To)
		if !srcOk || !dstOk {
			spxlog.Warn("Invalid 'From' or 'To' for move/glide animation: not a *mathf.Vec2")
			return nil, false
		}

		params.moveDiff = dst.Sub(src)
		if ani.AniType == coreproject.AniTypeMove {
			params.moveSpeed = params.moveDiff.Length() / duration
			params.moveDir = params.moveDiff.Normalize()
		}
	case coreproject.AniTypeTurn:
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

func (a *animationComponent) executeTweenLoop(info *animState, ani *coreproject.AniConfig, params *tweenParams) {
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

func (a *animationComponent) applyTweenStep(aniType coreproject.AniType, deltaPercent float64, params *tweenParams) {
	switch aniType {
	case coreproject.AniTypeMove:
		physicsMode := a.sprite.PhysicsMode()
		if isPhysicsEnabled() && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			vel := params.moveDir.Mulf(params.moveSpeed)
			a.sprite.SetVelocity(vel.X, vel.Y)
		} else {
			val := params.moveDiff.Mulf(deltaPercent)
			a.sprite.ChangeXYpos(val.X, val.Y)
		}
	case coreproject.AniTypeGlide:
		val := params.moveDiff.Mulf(deltaPercent)
		a.sprite.ChangeXYpos(val.X, val.Y)
	case coreproject.AniTypeTurn:
		val := params.turnDiff * deltaPercent
		a.sprite.ChangeHeading(val)
	}
}

func (a *animationComponent) cleanupTween(info, ownedPlayback *animState, name SpriteAnimationName, ani *coreproject.AniConfig) {
	if ani.AniType == coreproject.AniTypeMove {
		physicsMode := a.sprite.PhysicsMode()
		if isPhysicsEnabled() && physicsMode != NoPhysics && physicsMode != StaticPhysics {
			a.sprite.SetVelocity(0, 0)
		}
	}

	a.stopAnimState(info)
	if a.curTweenState != info {
		return
	}

	a.curTweenState = nil
	if name == a.shared.defaultAnimation || ani.IsKeepOnStop {
		return
	}
	if ownedPlayback != nil && a.curAnimState == ownedPlayback {
		a.stopCurrentAnimState(ownedPlayback)
	}
	a.playDefaultAnimIfIdle()
}
