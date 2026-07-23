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
	"testing"

	"github.com/goplus/spbase/mathf"
	internalaudio "github.com/goplus/spx/v3/internal/audio"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
)

func lastOnPlayPlaybackID(state *animState) int64 {
	if state == nil {
		return 0
	}
	return state.OnPlayAudioPlaybackID
}

func newTestAnimationComponent() *animationComponent {
	sprite := &SpriteImpl{
		g:    &Game{},
		name: "TestSprite",
	}
	sprite.runtimeState.SyncSprite = &engine.Sprite{}
	sprite.spriteState.IsVisible = true
	sprite.spriteState.DefaultCostumeIndex = 0
	sprite.costumes = []*costume{newCostumeWithSize(1, 1)}

	anim := &animationComponent{
		componentBase: componentBase{sprite: sprite},
		shared: &sharedAnimationData{
			defaultAnimation:  "idle",
			animations:        map[SpriteAnimationName]*coreproject.AniConfig{},
			animBindings:      map[string]string{},
			animationWrappers: map[SpriteAnimationName]*animationWrapper{},
		},
		activeTweenStates: make([]*animState, 0),
		donedAnimations:   make([]string, 0),
	}
	sprite.components.animation = anim
	sprite.components.sound = &soundComponent{
		componentBase: componentBase{sprite: sprite},
		pendingAudios: make([]string, 0),
	}
	sprite.components.physics = &physicsComponent{
		componentBase: componentBase{sprite: sprite},
		physicsMode:   NoPhysics,
	}
	return anim
}

func initTestMotionComponents(sprite *SpriteImpl, x, y float64) {
	sprite.components.transform = &transformComponent{
		componentBase: componentBase{sprite: sprite},
		x:             x,
		y:             y,
	}
	sprite.components.pen = &penComponent{
		componentBase: componentBase{sprite: sprite},
	}
}

type animationAudioBackend struct {
	nextID    int64
	plays     []animationPlayCall
	loops     []animationLoopCall
	restarts  []int64
	stops     []int64
	playing   map[int64]bool
	onPlay    func(id int64)
	onRestart func(id int64)
}

type animationPlayCall struct {
	path       string
	owner      engine.Object
	attenation float64
	maxDist    float64
}

type animationLoopCall struct {
	id   int64
	loop bool
}

func (f *animationAudioBackend) CreateAudio() engine.Object {
	return 77
}

func (f *animationAudioBackend) DestroyAudio(obj engine.Object) {}

func (f *animationAudioBackend) SetPitch(obj engine.Object, pitch float64) {}

func (f *animationAudioBackend) GetPitch(obj engine.Object) float64 {
	return 0
}

func (f *animationAudioBackend) SetPan(obj engine.Object, pan float64) {}

func (f *animationAudioBackend) GetPan(obj engine.Object) float64 {
	return 0
}

func (f *animationAudioBackend) SetVolume(obj engine.Object, volume float64) {}

func (f *animationAudioBackend) GetVolume(obj engine.Object) float64 {
	return 1
}

func (f *animationAudioBackend) PlayWithAttenuation(obj engine.Object, path string, ownerID engine.Object, attenuation, maxDistance float64) int64 {
	f.nextID++
	if f.playing == nil {
		f.playing = make(map[int64]bool)
	}
	f.playing[f.nextID] = true
	f.plays = append(f.plays, animationPlayCall{
		path:       path,
		owner:      ownerID,
		attenation: attenuation,
		maxDist:    maxDistance,
	})
	if f.onPlay != nil {
		f.onPlay(f.nextID)
	}
	return f.nextID
}

func (f *animationAudioBackend) Pause(aid int64) {}

func (f *animationAudioBackend) Resume(aid int64) {}

func (f *animationAudioBackend) Stop(aid int64) {
	if f.playing != nil {
		f.playing[aid] = false
	}
	f.stops = append(f.stops, aid)
}

func (f *animationAudioBackend) Restart(aid int64) bool {
	if f.playing == nil || !f.playing[aid] {
		return false
	}
	f.restarts = append(f.restarts, aid)
	if f.onRestart != nil {
		f.onRestart(aid)
	}
	return true
}

func (f *animationAudioBackend) SetLoop(aid int64, loop bool) {
	f.loops = append(f.loops, animationLoopCall{id: aid, loop: loop})
}

func (f *animationAudioBackend) IsPlaying(aid int64) bool {
	return f.playing[aid]
}

func (f *animationAudioBackend) StopAll() {}

func initTestAnimationAudio(anim *animationComponent, backend *animationAudioBackend) {
	anim.sprite.g.soundMgr = internalaudio.Manager{}
	anim.sprite.g.soundMgr.Init(backend)
	anim.sprite.g.sounds = map[string]sound{
		"walk": &coreproject.SoundConfig{Path: "sounds/walk.wav"},
		"step": &coreproject.SoundConfig{Path: "sounds/step.wav"},
	}
}

func TestStopCurrentAnimStateDoesNotClearReplacement(t *testing.T) {
	anim := newTestAnimationComponent()
	oldState := &animState{Name: "walk"}
	replacement := &animState{Name: "walk"}
	anim.curAnimState = replacement

	if anim.stopCurrentAnimState(oldState) {
		t.Fatal("stopCurrentAnimState returned true for a stale state")
	}
	if anim.curAnimState != replacement {
		t.Fatal("stopCurrentAnimState cleared the replacement animation state")
	}
	if !oldState.IsCanceled {
		t.Fatal("old animation state was not canceled")
	}
	if replacement.IsCanceled {
		t.Fatal("replacement animation state was canceled")
	}
}

func TestPlayDefaultAnimIfIdleSkipsActiveDefaultAnimation(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.defaultAnimActive = true

	anim.playDefaultAnimIfIdle()

	if !anim.defaultAnimActive {
		t.Fatal("playDefaultAnimIfIdle restarted or cleared the active default animation")
	}
}

func TestCleanupTweenWithoutPlaybackKeepsActiveAnimation(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	activeAnim := &animState{Name: "wave"}
	tween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curAnimState = activeAnim
	anim.curTweenState = tween
	anim.activeTweenStates = []*animState{tween}

	anim.cleanupTween(tween, nil, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.curAnimState != activeAnim {
		t.Fatal("cleanupTween replaced an unrelated active animation")
	}
	if anim.sprite.costumeIndex != 1 {
		t.Fatalf("costumeIndex = %d, want active animation costume 1", anim.sprite.costumeIndex)
	}
}

func TestCleanupTweenWithoutPlaybackRestoresDefaultWhenIdle(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	tween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curTweenState = tween
	anim.activeTweenStates = []*animState{tween}

	anim.cleanupTween(tween, nil, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.sprite.costumeIndex != 0 {
		t.Fatalf("costumeIndex = %d, want default costume 0", anim.sprite.costumeIndex)
	}
}

func TestCleanupTweenRestoresDefaultForOwnedPlayback(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	playback := &animState{Name: StateGlide}
	tween := &animState{
		Name:    StateGlide,
		AniType: coreproject.AniTypeGlide,
	}
	anim.curAnimState = playback
	anim.curTweenState = tween
	anim.activeTweenStates = []*animState{tween}

	anim.cleanupTween(tween, playback, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != nil {
		t.Fatal("cleanupTween did not clear the completed tween state")
	}
	if anim.curAnimState != nil {
		t.Fatal("cleanupTween did not clear its own animation playback state")
	}
	if anim.sprite.costumeIndex != 0 {
		t.Fatalf("costumeIndex = %d, want default costume 0", anim.sprite.costumeIndex)
	}
}

func TestInitTweenStateDoesNotCancelPreviousTween(t *testing.T) {
	anim := newTestAnimationComponent()
	previous := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curTweenState = previous
	anim.activeTweenStates = []*animState{previous}

	next, _ := anim.initTweenState(StateGlide, &coreproject.AniConfig{
		AniType:  coreproject.AniTypeGlide,
		Duration: 1,
	})

	if previous.IsCanceled {
		t.Fatal("initTweenState canceled the previous tween")
	}
	if anim.curTweenState != next {
		t.Fatal("initTweenState did not track the new tween state")
	}
}

func TestApplyTweenStepGlideUsesAbsolutePosition(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.spriteState.IsVisible = false
	initTestMotionComponents(anim.sprite, 100, 20)

	params := &tweenParams{
		moveFrom: mathf.NewVec2(10, 20),
		moveTo:   mathf.NewVec2(100, 20),
	}

	anim.applyTweenStep(coreproject.AniTypeGlide, 0.5, params)

	x, y := anim.sprite.getXY()
	if x != 55 || y != 20 {
		t.Fatalf("glide position = (%v, %v), want (55, 20)", x, y)
	}
}

func TestApplyTweenStepMoveUsesAbsolutePosition(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.spriteState.IsVisible = false
	initTestMotionComponents(anim.sprite, 100, 20)

	params := &tweenParams{
		moveFrom: mathf.NewVec2(10, 20),
		moveTo:   mathf.NewVec2(100, 20),
	}

	anim.applyTweenStep(coreproject.AniTypeMove, 0.5, params)

	x, y := anim.sprite.getXY()
	if x != 55 || y != 20 {
		t.Fatalf("move position = (%v, %v), want (55, 20)", x, y)
	}
}

func TestApplyTweenStepTurnUsesAbsoluteHeading(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.spriteState.IsVisible = false
	initTestMotionComponents(anim.sprite, 0, 0)
	anim.sprite.components.transform.direction = 100

	params := &tweenParams{
		turnFrom: 10,
		turnTo:   100,
	}

	anim.applyTweenStep(coreproject.AniTypeTurn, 0.5, params)

	if heading := anim.sprite.Heading(); heading != 55 {
		t.Fatalf("heading = %v, want 55", heading)
	}
}

func TestCleanupTweenStopsStaleOwnedPlayback(t *testing.T) {
	anim := newTestAnimationComponent()
	anim.sprite.costumes = []*costume{newCostumeWithSize(1, 1), newCostumeWithSize(1, 1)}
	anim.sprite.spriteState.DefaultCostumeIndex = 0
	anim.sprite.costumeIndex = 1

	playback := &animState{Name: StateGlide}
	staleTween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	currentTween := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curAnimState = playback
	anim.curTweenState = currentTween
	anim.activeTweenStates = []*animState{currentTween}

	anim.cleanupTween(staleTween, playback, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curAnimState != nil {
		t.Fatal("cleanupTween did not clear the stale playback state")
	}
	if anim.sprite.costumeIndex != 0 {
		t.Fatalf("costumeIndex = %d, want default costume 0", anim.sprite.costumeIndex)
	}
}

func TestCleanupTweenRestoresPreviousActiveTweenState(t *testing.T) {
	anim := newTestAnimationComponent()
	first, _ := anim.initTweenState(StateGlide, &coreproject.AniConfig{
		AniType:  coreproject.AniTypeGlide,
		Duration: 1,
	})
	second, _ := anim.initTweenState(StateGlide, &coreproject.AniConfig{
		AniType:  coreproject.AniTypeGlide,
		Duration: 1,
	})

	anim.cleanupTween(second, nil, StateGlide, &coreproject.AniConfig{AniType: coreproject.AniTypeGlide})

	if anim.curTweenState != first {
		t.Fatal("cleanupTween did not restore the previous active tween state")
	}
}

func TestPlayAnimAudioStartsAndStopsOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	playID := lastOnPlayPlaybackID(state)
	if playID == 0 {
		t.Fatal("onPlay did not capture a playback id")
	}
	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 1 || backend.loops[0].id != playID || !backend.loops[0].loop {
		t.Fatalf("loop calls = %+v, want one loop call for playback %d", backend.loops, playID)
	}

	anim.onAnimationDone("walk")

	if len(backend.stops) != 1 || backend.stops[0] != playID {
		t.Fatalf("stops = %+v, want [%d]", backend.stops, playID)
	}
	if anim.curAnimState != nil {
		t.Fatal("onAnimationDone did not clear the finished animation state")
	}
}

func TestPlayAnimAudioStartsAnimationBoundOnPlayReplay(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	if lastOnPlayPlaybackID(state) == 0 {
		t.Fatal("onPlay did not capture a playback id")
	}
	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 1 || !backend.loops[0].loop {
		t.Fatalf("loop calls = %+v, want one looping playback", backend.loops)
	}
	if state.OnPlayReplayAudioName != "walk" {
		t.Fatalf("OnPlayReplayAudioName = %q, want walk", state.OnPlayReplayAudioName)
	}
}

func TestHandleAnimationLoopedReplaysOnStartSound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnStart: &coreproject.ActionConfig{Play: "step"},
	}, state)

	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.plays) != 2 {
		t.Fatalf("plays = %d, want 2", len(backend.plays))
	}
}

func TestHandleAnimationLoopedRestartsOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	firstID := lastOnPlayPlaybackID(state)
	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1 initial play plus no extra backend plays on restart", len(backend.plays))
	}
	if len(backend.restarts) != 1 || backend.restarts[0] != firstID {
		t.Fatalf("restart calls after loop = %+v, want [%d]", backend.restarts, firstID)
	}

	secondID := lastOnPlayPlaybackID(state)
	if secondID != firstID {
		t.Fatalf("last onPlay playback id = %d, want original playback id %d to be reused", secondID, firstID)
	}
	if len(backend.loops) != 1 || backend.loops[0].id != firstID || !backend.loops[0].loop {
		t.Fatalf("loop calls = %+v, want one loop playback setup for id %d", backend.loops, firstID)
	}

	anim.onAnimationDone("walk")

	if len(backend.stops) != 1 || backend.stops[0] != firstID {
		t.Fatalf("stops = %+v, want [%d]", backend.stops, firstID)
	}
}

func TestHandleAnimationLoopedCoalescesOnPlayRestartBeforeFlush(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	firstID := lastOnPlayPlaybackID(state)
	anim.sprite.handleAnimationLooped()
	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.restarts) != 1 || backend.restarts[0] != firstID {
		t.Fatalf("restart calls after coalesced loops = %+v, want [%d]", backend.restarts, firstID)
	}
	if lastOnPlayPlaybackID(state) != firstID {
		t.Fatalf("last onPlay playback id = %d, want reused playback id %d", lastOnPlayPlaybackID(state), firstID)
	}
}

func TestHandleAnimationLoopedRestartsOnPlaySoundForAnimationAndTweenStates(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	animationState := &animState{Name: "walk"}
	tweenState := &animState{Name: StateGlide, AniType: coreproject.AniTypeGlide}
	anim.curAnimState = animationState
	anim.curTweenState = tweenState

	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, animationState)
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "step"},
	}, tweenState)

	animID := lastOnPlayPlaybackID(animationState)
	tweenID := lastOnPlayPlaybackID(tweenState)
	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.restarts) != 2 || backend.restarts[0] != animID || backend.restarts[1] != tweenID {
		t.Fatalf("restart calls for animation and tween = %+v, want [%d %d]", backend.restarts, animID, tweenID)
	}
	if lastOnPlayPlaybackID(animationState) != animID {
		t.Fatalf("animation onPlay playback id = %d, want %d", lastOnPlayPlaybackID(animationState), animID)
	}
	if lastOnPlayPlaybackID(tweenState) != tweenID {
		t.Fatalf("tween onPlay playback id = %d, want %d", lastOnPlayPlaybackID(tweenState), tweenID)
	}
}

func TestHandleAnimationLoopedSkipsStopForFinishedOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	firstID := lastOnPlayPlaybackID(state)
	if firstID == 0 {
		t.Fatal("first onPlay playback id = 0, want a tracked playback")
	}
	backend.playing[firstID] = false

	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	secondID := lastOnPlayPlaybackID(state)
	if secondID == 0 || secondID == firstID {
		t.Fatalf("second onPlay playback id = %d, want a new tracked playback", secondID)
	}
	if len(backend.restarts) != 0 {
		t.Fatalf("restart calls = %+v, want none when prior playback is stale", backend.restarts)
	}
	if len(backend.loops) != 2 || backend.loops[0].id != firstID || backend.loops[1].id != secondID || !backend.loops[0].loop || !backend.loops[1].loop {
		t.Fatalf("loop calls = %+v, want loop playback for initial id %d and replacement id %d", backend.loops, firstID, secondID)
	}
	if len(backend.stops) != 0 {
		t.Fatalf("stops = %+v, want none before animation ends when prior playback already finished", backend.stops)
	}
}

func TestRestartOnPlayAudioStopsReplacementWhenStateIsCanceledDuringReplay(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	firstID := lastOnPlayPlaybackID(state)
	backend.playing[firstID] = false
	backend.onPlay = func(id int64) {
		if id == firstID {
			return
		}
		anim.stopAnimState(state)
	}

	anim.restartOnPlayAudio(state)

	if state.OnPlayAudioPlaybackID != 0 {
		t.Fatalf("OnPlayAudioPlaybackID = %d, want 0 after cancellation", state.OnPlayAudioPlaybackID)
	}
	if !state.IsCanceled {
		t.Fatal("state.IsCanceled = false, want true")
	}
	if len(backend.plays) != 2 {
		t.Fatalf("plays = %d, want 2", len(backend.plays))
	}
	replacementID := backend.nextID
	if len(backend.stops) != 1 || backend.stops[0] != replacementID {
		t.Fatalf("stops = %+v, want [%d]", backend.stops, replacementID)
	}
	if backend.playing[replacementID] {
		t.Fatalf("replacement playback %d is still marked playing", replacementID)
	}
}

func TestPlayAnimAudioTracksOnStartReplayName(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnStart: &coreproject.ActionConfig{Play: "step"},
	}, state)

	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 0 {
		t.Fatalf("loop calls = %+v, want none", backend.loops)
	}
	if state.OnStartReplayAudioName != "step" {
		t.Fatalf("OnStartReplayAudioName = %q, want step", state.OnStartReplayAudioName)
	}
}

func TestStopAnimStateLeavesOnStartSoundAlone(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimationAudio(&coreproject.AniConfig{
		OnStart: &coreproject.ActionConfig{Play: "step"},
	}, state)
	anim.stopAnimState(state)

	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 0 {
		t.Fatalf("loop calls = %+v, want none", backend.loops)
	}
	if len(backend.stops) != 0 {
		t.Fatalf("stops = %+v, want none", backend.stops)
	}
}
