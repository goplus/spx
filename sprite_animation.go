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
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	"github.com/goplus/spx/v4/internal/core/clock"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

// sharedAnimationData stores read-only animation data shared across cloned sprites.
type sharedAnimationData struct {
	animations       map[SpriteAnimationName]*animationEntry
	animBindings     map[string]string
	defaultAnimation SpriteAnimationName
}

type animationPlayback struct {
	AniType                   coreproject.AniType
	Name                      string
	IsCanceled                bool
	Speed                     float64
	onStartSound              string
	onPlaySound               string
	OnPlayAudioRestartPending bool
	OnPlayAudioPlaybackID     int64
}

type animationComponent struct {
	sprite *SpriteImpl

	// Shared animation configuration (read-only, shared across clones).
	shared *sharedAnimationData

	// Animation state (per-instance).
	curAnimState      *animationPlayback
	activeTweenStates []*animationPlayback
	velocityOwner     *animationPlayback
	defaultAnimActive bool

	// Animation tracking (per-instance).
	doneAnimations []*animationPlayback
	pendingAudios  []string
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

// Types.
type animationEntry struct {
	name                  SpriteAnimationName
	config                *coreproject.AniConfig
	spriteName            string
	costumes              []*costume
	isCostumeSet          bool
	loadOnce              sync.Once
	adaptBitmapResolution int
}

type animationNormalFrame struct {
	Path   string     `json:"path"`
	Offset [2]float64 `json:"offset"`
	Bitmap int64      `json:"bitmap"`
}

type animationAtlasFrame struct {
	X      int64      `json:"x"`
	Y      int64      `json:"y"`
	W      int64      `json:"w"`
	H      int64      `json:"h"`
	Offset [2]float64 `json:"offset"`
}

type animationPayload struct {
	BasePath  string `json:"base_path,omitempty"`
	Frames    []any  `json:"frames"`
	MaxBitmap int64  `json:"max_bitmap"`
}

// Public API.
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
	p.animation().playAnimation(name, __xgo_optional_loop, false)
}

func (p *SpriteImpl) AnimateAndWait(name SpriteAnimationName) {
	if isDebugInstrEnabled() {
		spxlog.Debug("AnimateAndWait: %s", name)
	}
	p.animation().playAnimation(name, false, true)
}

func (p *SpriteImpl) StopAnimation(name SpriteAnimationName) {
	p.animation().stopAnimation(name)
}

// Internals.
func (p *SpriteImpl) queueAnimationLoopAudio(state *animationPlayback) {
	if state == nil {
		return
	}
	if state.onStartSound != "" {
		p.animation().addPendingAudio(state.onStartSound)
	}
	if state.onPlaySound != "" {
		state.OnPlayAudioRestartPending = true
	}
}

func (p *SpriteImpl) flushPendingAudios(buffer []string) []string {
	engine.Lock()
	buffer = p.animation().takePendingAudios(buffer)
	pendingOnPlayAudioStates := p.animation().takePendingOnPlayAudioStates()
	engine.Unlock()

	if p.isDestroyed() || p.runtimeState.proxy == nil {
		return buffer[:0]
	}

	for _, audio := range buffer {
		p.sound().play(audio, false)
	}
	for _, state := range pendingOnPlayAudioStates {
		p.animation().restartOnPlayAudio(state)
	}
	return buffer[:0]
}

// Internal State.
func (p *SpriteImpl) flushCompletedAnimations(buffer []*animationPlayback) []*animationPlayback {
	engine.Lock()
	buffer = p.animation().takeDoneAnimations(buffer)
	engine.Unlock()
	defer clear(buffer)

	for _, state := range buffer {
		p.animation().onAnimationDone(state)
	}
	return buffer[:0]
}

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
			isCostumeSet: a.sprite.runtimeState.isCostumeSet,
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

func (a *animationComponent) onDestroy() {
	a.stopAnimState(a.curAnimState)
	for _, state := range a.activeTweenStates {
		a.stopAnimState(state)
	}
	a.activeTweenStates = nil
	if proxy := a.proxy(); proxy != nil {
		proxy.UnRegisterOnAnimationLooped()
		proxy.UnRegisterOnAnimationFinished()
	}
}

func (a *animationComponent) stopAnimation(name SpriteAnimationName) {
	if name == "" || !a.hasAnim(name) {
		return
	}
	state := a.curAnimState
	if state == nil || state.Name != name {
		return
	}
	proxy := a.playbackProxy()
	if proxy == nil {
		return
	}

	if !a.stopCurrentAnimState(state) {
		return
	}
	proxy.PauseAnim()
	a.playDefaultAnim()
}

func (a *animationComponent) playAnimation(name SpriteAnimationName, loop, blocking bool) {
	entry, ok := a.shared.animations[name]
	if !ok {
		spxlog.Warn("Animation not found: %s", name)
		return
	}

	a.doAnimation(entry, loop, 1, blocking, true)
}

func (a *animationComponent) doAnimation(entry *animationEntry, loop bool, speed float64, isBlocking bool, playAudio bool) *animationPlayback {
	proxy := a.playbackProxy()
	if proxy == nil {
		return nil
	}

	a.stopAnimState(a.curAnimState)
	a.defaultAnimActive = false
	a.curAnimState = &animationPlayback{
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
		scheduler.CleanupMainThread(func() {
			a.stopAnimationAudio(audioID)
			if !completed && playbackStarted && current && a.playbackProxy() != nil {
				proxy.PauseAnim()
			}
		})
	}()

	// Capture resources before cancellation can discard an engine-call result.
	engine.WaitMainThread(func() {
		if playAudio {
			a.playAnimationAudio(entry.config, info)
		}
		a.sprite.applyCostumeUpdate()
		a.prepareAnimationPlayback(entry, proxy)
		playbackStarted = true
		engine.Managers().SpriteMgr.PlayAnim(proxy.GetId(), entry.name, speed, loop, false)
	})
	if isBlocking {
		a.sprite.runtimeState.isAnimating = true
		for engine.Managers().SpriteMgr.IsPlayingAnim(proxy.GetId()) {
			if info.IsCanceled {
				break
			}
			engine.WaitNextFrame()
		}
	}
	completed = true
	return info
}

func (a *animationComponent) prepareAnimationPlayback(entry *animationEntry, proxy *engine.Sprite) {
	bitmapResolution := entry.ensureRegistered()
	renderScale := a.sprite.renderScale(bitmapResolution)
	proxy.SetRenderScale(engine.UniformVec2(renderScale))
}

func (a *animationComponent) playDefaultAnim() {
	animName := ""
	proxy := a.playbackProxy()
	if proxy == nil {
		return
	}
	if !a.sprite.spriteState.isVisible || a.sprite.spriteState.isDying {
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
		a.prepareAnimationPlayback(entry, proxy)
		engine.Managers().SpriteMgr.PlayAnim(proxy.GetId(), entry.name, speed, true, false)
		a.defaultAnimActive = true
	} else {
		a.defaultAnimActive = false
		a.sprite.goSetCostume(a.sprite.spriteState.defaultCostumeIndex)
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

func (a *animationComponent) onAnimationDone(state *animationPlayback) {
	if a.playbackProxy() == nil {
		return
	}
	if state != nil && a.curAnimState == state {
		if !a.stopCurrentAnimState(state) {
			return
		}
		a.playDefaultAnim()
	}
}

func (a *animationComponent) addDoneAnimation(state *animationPlayback) {
	a.doneAnimations = append(a.doneAnimations, state)
}

func (a *animationComponent) takeDoneAnimations(buffer []*animationPlayback) []*animationPlayback {
	buffer = append(buffer, a.doneAnimations...)
	clear(a.doneAnimations)
	a.doneAnimations = a.doneAnimations[:0]
	return buffer
}

func (a *animationComponent) stopCurrentAnimState(state *animationPlayback) bool {
	current := a.curAnimState == state
	id := a.cancelAnimState(state)
	if current {
		a.curAnimState = nil
	}
	a.stopAnimationAudio(id)
	return current
}

func (a *animationComponent) stopAnimState(state *animationPlayback) {
	a.stopAnimationAudio(a.cancelAnimState(state))
}

// cancelAnimState detaches local state before any engine cleanup can fail.
func (a *animationComponent) cancelAnimState(state *animationPlayback) int64 {
	if state == nil {
		return 0
	}
	engine.Lock()
	state.IsCanceled = true
	state.OnPlayAudioRestartPending = false
	id := state.OnPlayAudioPlaybackID
	state.OnPlayAudioPlaybackID = 0
	if a.curAnimState == state {
		a.sprite.runtimeState.isAnimating = false
	}
	engine.Unlock()
	return id
}

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

func (a *animationComponent) getCurTweenState() *animationPlayback {
	if len(a.activeTweenStates) == 0 {
		return nil
	}
	return a.activeTweenStates[len(a.activeTweenStates)-1]
}

func (a *animationComponent) unregisterTweenState(state *animationPlayback) bool {
	i := slices.Index(a.activeTweenStates, state)
	if i < 0 {
		return false
	}
	a.activeTweenStates = slices.Delete(a.activeTweenStates, i, i+1)
	return true
}

func (a *animationComponent) doTween(name SpriteAnimationName, base *coreproject.AniConfig, params tweenParams) {
	params, ok := prepareTweenParams(params)
	if !ok {
		return
	}

	info := &animationPlayback{AniType: params.aniType, Name: name, Speed: params.speed}
	a.activeTweenStates = append(a.activeTweenStates, info)
	var ownedPlayback *animationPlayback
	completed := false
	defer func() { a.cleanupTween(info, ownedPlayback, base, !completed) }()
	if entry, ok := a.shared.animations[name]; ok {
		ownedPlayback = a.doAnimation(entry, true, params.speed, false, false)
		if base != nil {
			engine.WaitMainThread(func() { a.playAnimationAudio(base, info) })
		}
	}
	for elapsed := 0.0; elapsed < params.duration && !info.IsCanceled; {
		elapsed += clock.DeltaTime()
		a.applyTweenStep(info, mathf.Clamp01f(elapsed/params.duration), &params)
		engine.WaitNextFrame()
	}
	completed = true
}

func (a *animationComponent) applyTweenStep(info *animationPlayback, percent float64, params *tweenParams) {
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

func (a *animationComponent) cleanupTween(info, ownedPlayback *animationPlayback, base *coreproject.AniConfig, interrupted bool) {
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
	scheduler.CleanupMainThread(func() {
		a.stopAnimationAudio(audioID)
		proxy := a.playbackProxy()
		if proxy == nil {
			return
		}
		if interrupted && stoppedOwnedPlayback {
			proxy.PauseAnim()
		}
		if stopVelocity {
			a.sprite.SetVelocity(0, 0)
		}
		if restoreDefault {
			a.playDefaultAnimIfIdle()
		}
	})
}

func (a *animationComponent) playAnimationAudio(ani *coreproject.AniConfig, state *animationPlayback) {
	if state == nil {
		return
	}
	if action := ani.OnStart; action != nil && action.Play != "" {
		state.onStartSound = action.Play
		a.sprite.sound().play(action.Play, false)
	}
	if action := ani.OnPlay; action != nil && action.Play != "" {
		state.onPlaySound = action.Play
		a.restartOnPlayAudio(state)
	}
}

func (a *animationComponent) restartOnPlayAudio(state *animationPlayback) {
	if state == nil {
		return
	}

	engine.Lock()
	name := state.onPlaySound
	nextID := state.OnPlayAudioPlaybackID
	canceled := state.IsCanceled
	engine.Unlock()

	if canceled || name == "" {
		return
	}

	if !a.sprite.g.soundMgr.RestartID(nextID) {
		nextID = a.sprite.sound().play(name, true)
	}
	if nextID == 0 {
		return
	}

	engine.Lock()
	canceled = state.IsCanceled
	if !canceled {
		state.OnPlayAudioPlaybackID = nextID
	}
	engine.Unlock()

	if canceled {
		a.sprite.g.soundMgr.StopID(nextID)
	}
}

func (a *animationComponent) stopAnimationAudio(id int64) {
	if id == 0 || a.sprite == nil || a.sprite.g == nil {
		return
	}
	scheduler.CleanupMainThread(func() {
		if !a.sprite.g.soundMgr.IsPlaying(id) {
			a.sprite.g.soundMgr.PruneStoppedIDs([]int64{id})
			return
		}
		a.sprite.g.soundMgr.StopID(id)
	})
}

func (a *animationComponent) takePendingOnPlayAudioStates() (states [2]*animationPlayback) {
	states = [2]*animationPlayback{a.curAnimState, a.getCurTweenState()}
	for i, state := range states {
		if state == nil || !state.OnPlayAudioRestartPending {
			states[i] = nil
			continue
		}
		state.OnPlayAudioRestartPending = false
		if state.IsCanceled || state.onPlaySound == "" {
			states[i] = nil
		}
	}
	return
}

func (s *animationComponent) addPendingAudio(audioName string) {
	s.pendingAudios = append(s.pendingAudios, audioName)
}

func (s *animationComponent) takePendingAudios(buffer []string) []string {
	buffer = append(buffer, s.pendingAudios...)
	s.pendingAudios = s.pendingAudios[:0]
	return buffer
}

func (a *animationComponent) proxy() *engine.Sprite {
	if a.sprite == nil {
		return nil
	}
	return a.sprite.runtimeState.proxy
}

func (a *animationComponent) playbackProxy() *engine.Sprite {
	if a.sprite == nil || a.sprite.isDestroyed() {
		return nil
	}
	return a.sprite.runtimeState.proxy
}

// Animation Data.
func (e *animationEntry) ensureRegistered() int {
	e.loadOnce.Do(func() {
		payloadJSON, maxBitmap, err := buildAnimationPayloadJSON(
			e.costumes, e.config.IFrameFrom, e.config.IFrameTo,
			e.isCostumeSet,
		)
		if err != nil {
			panic(err)
		}
		e.adaptBitmapResolution = maxBitmap
		engine.Managers().ResMgr.CreateAnimation(
			e.spriteName,
			e.name,
			payloadJSON,
			int64(e.config.FrameFps),
			e.isCostumeSet,
		)
	})
	return e.adaptBitmapResolution
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

func buildAnimationPayloadJSON(costumes []*costume, from, to int, isAtlas bool) (string, int, error) {
	if len(costumes) == 0 {
		return "", 0, fmt.Errorf("createAnimation: no costumes configured")
	}
	if from < 0 || from >= len(costumes) ||
		to < 0 || to >= len(costumes) {
		return "", 0, fmt.Errorf(
			"createAnimation: frame index out of bounds (from=%d, to=%d, costumes=%d)",
			from, to, len(costumes),
		)
	}

	payload := buildAnimationPayload(costumes, from, to, isAtlas)
	bin, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("createAnimation: failed to marshal animation payload: %w", err)
	}
	return string(bin), int(payload.MaxBitmap), nil
}

func buildAnimationPayload(costumes []*costume, from, to int, isAtlas bool) animationPayload {
	result := animationPayload{}
	if isAtlas {
		result.BasePath = engine.ToAssetPath(costumes[0].path)
		result.MaxBitmap = 1
	}
	step := 1
	if to < from {
		step = -1
	}
	frameCount := (to-from)*step + 1
	result.Frames = make([]any, 0, frameCount)
	for i := from; i != to+step; i += step {
		c := costumes[i]
		frame := c.frame
		if isAtlas {
			result.Frames = append(result.Frames, animationAtlasFrame{
				X: int64(frame.PosX),
				Y: int64(frame.PosY),
				W: int64(frame.Width),
				H: int64(frame.Height),
			})
			continue
		}
		b := int64(normalizeBitmapResolution(frame.BitmapResolution))
		result.MaxBitmap = max(result.MaxBitmap, b)
		half := frame.ImageSize.Mulf(0.5)
		result.Frames = append(result.Frames, animationNormalFrame{
			Path: engine.ToAssetPath(c.path),
			Offset: [2]float64{
				frame.Center.X - half.X,
				-(frame.Center.Y - half.Y),
			},
			Bitmap: b,
		})
	}
	return result
}
