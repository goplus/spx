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

	internalaudio "github.com/goplus/spx/v2/internal/audio"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
)

func boolPtr(v bool) *bool {
	return &v
}

func lastBoundPlaybackID(state *animState) int64 {
	if len(state.BoundAudioPlaybackIDs) == 0 {
		return 0
	}
	return state.BoundAudioPlaybackIDs[len(state.BoundAudioPlaybackIDs)-1]
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
		donedAnimations: make([]string, 0),
	}
	sprite.components.animation = anim
	sprite.components.sound = &soundComponent{
		componentBase: componentBase{sprite: sprite},
		pendingAudios: make([]string, 0),
	}
	return anim
}

type animationAudioBackend struct {
	nextID  int64
	plays   []animationPlayCall
	loops   []animationLoopCall
	stops   []int64
	playing map[int64]bool
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

func TestPlayAnimAudioStartsAndStopsOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{Play: "walk"},
	}, state)

	playID := lastBoundPlaybackID(state)
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

func TestPlayAnimAudioHonorsOnPlayLoopConfig(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{
			Play: "walk",
			Loop: boolPtr(false),
		},
	}, state)

	if lastBoundPlaybackID(state) == 0 {
		t.Fatal("onPlay did not capture a playback id")
	}
	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 0 {
		t.Fatalf("loop calls = %+v, want none", backend.loops)
	}
	if state.BoundLoopReplayAudio != "walk" {
		t.Fatalf("BoundLoopReplayAudio = %q, want walk", state.BoundLoopReplayAudio)
	}
}

func TestHandleAnimationLoopedReplaysOnStartSoundForCurrentAnimation(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimAudio(&coreproject.AniConfig{
		OnStart: &coreproject.ActionConfig{
			Play: "step",
			Loop: boolPtr(false),
		},
	}, state)

	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.plays) != 2 {
		t.Fatalf("plays = %d, want 2", len(backend.plays))
	}
}

func TestHandleAnimationLoopedReplaysLoopFalseOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{
			Play: "walk",
			Loop: boolPtr(false),
		},
	}, state)

	firstID := lastBoundPlaybackID(state)
	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	if len(backend.plays) != 2 {
		t.Fatalf("plays = %d, want 2", len(backend.plays))
	}
	if len(state.BoundAudioPlaybackIDs) != 2 {
		t.Fatalf("BoundAudioPlaybackIDs = %+v, want 2 tracked playbacks", state.BoundAudioPlaybackIDs)
	}

	secondID := lastBoundPlaybackID(state)
	if secondID == 0 || secondID == firstID {
		t.Fatalf("last bound playback id = %d, want a new playback id after loop", secondID)
	}

	anim.onAnimationDone("walk")

	if len(backend.stops) != 2 || backend.stops[0] != firstID || backend.stops[1] != secondID {
		t.Fatalf("stops = %+v, want [%d %d]", backend.stops, firstID, secondID)
	}
}

func TestHandleAnimationLoopedReplayDoesNotStopPreviousOnPlaySound(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.curAnimState = state
	anim.playAnimAudio(&coreproject.AniConfig{
		OnPlay: &coreproject.ActionConfig{
			Play: "walk",
			Loop: boolPtr(false),
		},
	}, state)

	firstID := lastBoundPlaybackID(state)
	if firstID == 0 {
		t.Fatal("first onPlay playback id = 0, want a tracked playback")
	}

	anim.sprite.handleAnimationLooped()
	anim.sprite.flushPendingAudios(nil)

	secondID := lastBoundPlaybackID(state)
	if secondID == 0 || secondID == firstID {
		t.Fatalf("second onPlay playback id = %d, want a new tracked playback", secondID)
	}
	if len(backend.stops) != 0 {
		t.Fatalf("stops = %+v, want none before animation ends", backend.stops)
	}
	if !backend.playing[firstID] || !backend.playing[secondID] {
		t.Fatalf("playing = %+v, want both playback ids %d and %d to remain active", backend.playing, firstID, secondID)
	}
}

func TestTrackBoundAudioPlaybackPrunesFinishedPlaybackIDs(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.trackBoundAudioPlayback(state, 1)
	anim.trackBoundAudioPlayback(state, 2)
	backend.playing = map[int64]bool{
		1: false,
		2: true,
		3: true,
	}

	anim.trackBoundAudioPlayback(state, 3)

	if len(state.BoundAudioPlaybackIDs) != 2 {
		t.Fatalf("BoundAudioPlaybackIDs = %+v, want [2 3]", state.BoundAudioPlaybackIDs)
	}
	if state.BoundAudioPlaybackIDs[0] != 2 || state.BoundAudioPlaybackIDs[1] != 3 {
		t.Fatalf("BoundAudioPlaybackIDs = %+v, want [2 3]", state.BoundAudioPlaybackIDs)
	}
}

func TestPlayAnimAudioHonorsOnStartLoopConfig(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimAudio(&coreproject.AniConfig{
		OnStart: &coreproject.ActionConfig{
			Play: "step",
			Loop: boolPtr(true),
		},
	}, state)

	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want 1", len(backend.plays))
	}
	if len(backend.loops) != 1 || !backend.loops[0].loop {
		t.Fatalf("loop calls = %+v, want one looping playback", backend.loops)
	}
	if state.LoopReplayAudioName != "" {
		t.Fatalf("LoopReplayAudioName = %q, want empty when onStart uses loop playback", state.LoopReplayAudioName)
	}
}

func TestStopAnimStateLeavesOnStartSoundAlone(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)

	state := &animState{Name: "walk"}
	anim.playAnimAudio(&coreproject.AniConfig{
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
