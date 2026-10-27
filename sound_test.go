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
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
)

type soundControlBackend struct {
	fakeAudioBackend
	actions []string
}

type fakeAudioBackend struct {
	pan          float64
	pitch        float64
	volume       float64
	createCalls  int
	destroyCalls int
	plays        []fakeAudioPlay
}

type fakeAudioPlay struct {
	owner       engine.Object
	attenuation float64
	maxDistance float64
}

type cloneSoundSettings struct{ volume, pan, pitch float64 }

type cloneSoundBackend struct {
	animationAudioBackend
	next      engine.Object
	settings  map[engine.Object]cloneSoundSettings
	destroyed []engine.Object
}

type spritePlaybackBackend struct {
	animationAudioBackend
	paused, resumed []int64
}

func TestGameSoundPlaybackControls(t *testing.T) {
	backend := &soundControlBackend{}
	game := &Game{fs: gameTestFS{}, sounds: map[string]sound{
		"Jump": &coreproject.SoundConfig{Path: "sounds/jump.wav"},
	}}
	game.soundMgr.Init(backend)
	game.Play__0("Jump")
	game.PausePlaying("Jump")
	game.ResumePlaying("Jump")
	game.StopPlaying("Jump")
	game.StopAllSounds()
	want := []string{"pause", "resume", "stop", "stop all"}
	if !slices.Equal(backend.actions, want) || len(backend.plays) != 1 {
		t.Fatalf("playback = %d plays, actions %v; want 1 play, %v", len(backend.plays), backend.actions, want)
	}
	game.Play__0("Missing")
	game.PausePlaying("Missing")
	game.ResumePlaying("Missing")
	game.StopPlaying("Missing")
	if !slices.Equal(backend.actions, want) || len(backend.plays) != 1 {
		t.Fatal("missing sound reached the audio backend")
	}
}

func TestGamePlayAndWaitReturnsForFinishedSound(t *testing.T) {
	backend := &fakeAudioBackend{}
	game := &Game{sounds: map[string]sound{
		"Jump": &coreproject.SoundConfig{Path: "sounds/jump.wav"},
	}}
	game.soundMgr.Init(backend)
	game.PlayAndWait("Jump")
	if len(backend.plays) != 1 || backend.createCalls != 1 {
		t.Fatalf("PlayAndWait: plays = %d, allocations = %d; want 1 each", len(backend.plays), backend.createCalls)
	}
}

func TestGameClearSoundEffects(t *testing.T) {
	for _, test := range []struct {
		name                   string
		soundObj, wantSoundObj engine.Object
		wantCreateCalls        int
	}{
		{name: "existing handle", soundObj: 9, wantSoundObj: 9},
		{name: "unused handle", wantSoundObj: 77, wantCreateCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &fakeAudioBackend{pan: 0.4, pitch: 1.25}
			var g Game
			g.soundMgr.Init(backend)
			g.audioState.SoundObj = test.soundObj

			g.ClearSoundEffects()

			if backend.createCalls != test.wantCreateCalls {
				t.Fatalf("CreateAudio calls = %d, want %d", backend.createCalls, test.wantCreateCalls)
			}
			if got := g.audioState.SoundObj; got != test.wantSoundObj {
				t.Fatalf("SoundObj = %d, want %d", got, test.wantSoundObj)
			}
			if backend.pan != 0 {
				t.Fatalf("pan = %v, want 0", backend.pan)
			}
			if backend.pitch != 1 {
				t.Fatalf("pitch = %v, want 1", backend.pitch)
			}
		})
	}
}

func TestGameSoundHandleIsLazyReusedAndReleased(t *testing.T) {
	backend := &fakeAudioBackend{}
	var g Game
	g.soundMgr.Init(backend)

	g.SetVolume(60)
	g.ChangeVolume(5)
	g.SetSoundEffect(SoundPanEffect, -25)
	if got := g.Volume(); got != 65 {
		t.Fatalf("Volume() = %v, want 65", got)
	}
	if got := g.GetSoundEffect(SoundPanEffect); got != -25 {
		t.Fatalf("GetSoundEffect(Pan) = %v, want -25", got)
	}
	if backend.createCalls != 1 {
		t.Fatalf("CreateAudio calls = %d, want one lazy allocation", backend.createCalls)
	}
	if got := g.audioState.SoundObj; got != 77 {
		t.Fatalf("SoundObj = %d, want 77", got)
	}

	g.releaseGameAudio()
	if g.audioState.SoundObj != 0 {
		t.Fatalf("SoundObj = %d after release, want 0", g.audioState.SoundObj)
	}
	if backend.destroyCalls != 1 {
		t.Fatalf("DestroyAudio calls = %d, want one release", backend.destroyCalls)
	}
	if got := g.Volume(); got != 100 || backend.createCalls != 2 {
		t.Fatalf("Volume() after release = %v with %d allocations, want 100 and 2", got, backend.createCalls)
	}
}

func TestSoundControlChanges(t *testing.T) {
	type soundControls interface {
		Volume() float64
		SetVolume(float64)
		ChangeVolume(float64)
		GetSoundEffect(SoundEffectKind) float64
		SetSoundEffect(SoundEffectKind, float64)
		ChangeSoundEffect(SoundEffectKind, float64)
	}
	for _, target := range []string{"game", "sprite"} {
		t.Run(target, func(t *testing.T) {
			var g Game
			g.soundMgr.Init(&fakeAudioBackend{})
			var controls soundControls = &g
			if target == "sprite" {
				sprite := &SpriteImpl{g: &g}
				sprite.components.sound = &soundComponent{sprite: sprite}
				controls = sprite
			}
			for _, control := range []struct {
				name        string
				set, change func(float64)
				get         func() float64
				low, high   float64
			}{
				{"volume", controls.SetVolume, controls.ChangeVolume, controls.Volume, 0, 100},
				{"pan",
					func(v float64) { controls.SetSoundEffect(SoundPanEffect, v) },
					func(v float64) { controls.ChangeSoundEffect(SoundPanEffect, v) },
					func() float64 { return controls.GetSoundEffect(SoundPanEffect) }, -100, 100},
				{"pitch",
					func(v float64) { controls.SetSoundEffect(SoundPitchEffect, v) },
					func(v float64) { controls.ChangeSoundEffect(SoundPitchEffect, v) },
					func() float64 { return controls.GetSoundEffect(SoundPitchEffect) }, -360, 360},
			} {
				t.Run(control.name, func(t *testing.T) {
					control.set(control.high - 10)
					for _, step := range []struct {
						name        string
						delta, want float64
					}{
						{"NaN preserves value", math.NaN(), control.high - 10},
						{"upper bound", 30, control.high},
						{"decrease after upper bound", -10, control.high - 10},
						{"lower bound", -1000, control.low},
						{"increase after lower bound", 10, control.low + 10},
					} {
						control.change(step.delta)
						if got := control.get(); math.IsNaN(got) || math.Abs(got-step.want) >= 1e-9 {
							t.Fatalf("%s: got %g, want %g", step.name, got, step.want)
						}
					}
				})
			}
		})
	}
}

func TestGamePlayUsesStageAudioParameters(t *testing.T) {
	backend := &fakeAudioBackend{}
	var g Game
	g.soundMgr.Init(backend)
	g.sounds = map[string]sound{
		"stage": &coreproject.SoundConfig{Path: "sounds/stage.wav"},
	}

	g.Play__1("stage", true)
	if len(backend.plays) != 1 {
		t.Fatalf("plays = %d, want one stage playback", len(backend.plays))
	}
	call := backend.plays[0]
	if call.owner != 0 || call.attenuation != 0 || call.maxDistance != defaultAudioMaxDist {
		t.Fatalf("stage playback parameters = %+v, want owner 0, attenuation 0, max distance %v", call, defaultAudioMaxDist)
	}
	if g.audioState.SoundObj != 77 || backend.createCalls != 1 {
		t.Fatalf("game sound object = %d with %d allocations, want 77 and 1", g.audioState.SoundObj, backend.createCalls)
	}
}

func TestLoadSoundCachesIndependentConfigs(t *testing.T) {
	fs := gameTestFS{
		"sounds/Jump/index.json": `{"path":"jump.wav","rate":1,"sampleCount":10}`,
		"sounds/Land/index.json": `{"path":"land.wav","rate":2,"sampleCount":20}`,
	}
	game := &Game{fs: fs, sounds: make(map[string]sound)}
	jump, err := game.loadSound("Jump")
	if err != nil {
		t.Fatal(err)
	}
	land, err := game.loadSound("Land")
	if err != nil {
		t.Fatal(err)
	}
	if jump == land {
		t.Fatal("different sounds share a config pointer")
	}
	if jump.Path != "sounds/Jump/jump.wav" || jump.Rate != 1 || jump.SampleCount != 10 {
		t.Fatalf("jump config changed after loading land: %+v", jump)
	}
	if land.Path != "sounds/Land/land.wav" || land.Rate != 2 || land.SampleCount != 20 {
		t.Fatalf("land config = %+v", land)
	}
	jump.Rate = 3
	if land.Rate != 2 {
		t.Fatal("changing jump affected land")
	}
	delete(fs, "sounds/Jump/index.json")
	cached, err := game.loadSound("Jump")
	if err != nil {
		t.Fatal(err)
	}
	if cached != jump || cached.Rate != 3 {
		t.Fatalf("cached config = %+v, want original pointer %+v", cached, jump)
	}
}

func TestLoadSoundDoesNotCacheErrors(t *testing.T) {
	for _, tt := range []struct{ name, content string }{
		{"missing", ""},
		{"malformed", `{`},
		{"trailing value", `{"path":"jump.wav"} {}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fs := gameTestFS{}
			if tt.content != "" {
				fs["sounds/Jump/index.json"] = tt.content
			}
			game := &Game{fs: fs, sounds: make(map[string]sound)}
			if media, err := game.loadSound("Jump"); err == nil || media != nil {
				t.Fatalf("failed load = %+v, %v; want nil config and an error", media, err)
			}
			if _, ok := game.sounds["Jump"]; ok {
				t.Fatal("failed load was cached")
			}
			fs["sounds/Jump/index.json"] = `{"path":"jump.wav"}`
			media, err := game.loadSound("Jump")
			if err != nil {
				t.Fatal(err)
			}
			if media.Path != "sounds/Jump/jump.wav" || game.sounds["Jump"] != media {
				t.Fatalf("retry did not cache the normalized config: %+v", media)
			}
		})
	}
}

func TestGameSoundUnsupportedControls(t *testing.T) {
	var game Game
	game.soundMgr.Init(&fakeAudioBackend{})
	if game.Loudness() != 0 {
		t.Fatal("unsupported microphone loudness must be zero")
	}
	for _, test := range []struct {
		name string
		call func()
		want string
	}{
		{"get", func() { game.GetSoundEffect(SoundEffectKind(-1)) }, "GetSoundEffect: invalid kind"},
		{"set", func() { game.SetSoundEffect(SoundEffectKind(-1), 1) }, "SetSoundEffect: invalid kind"},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != test.want {
					t.Fatalf("panic=%v, want %q", got, test.want)
				}
			}()
			test.call()
		})
	}
}

func TestSpriteClearSoundEffects(t *testing.T) {
	type testGame struct{ Game }
	type testSprite struct {
		SpriteImpl
		*testGame
	}
	backend := &cloneSoundBackend{}
	game := &testGame{}
	game.soundMgr.Init(backend)
	game.SetVolume(60)
	game.SetSoundEffect(SoundPanEffect, 20)
	game.SetSoundEffect(SoundPitchEffect, 120)
	stageSettings := backend.settings[game.audioState.SoundObj]

	other := &SpriteImpl{g: &game.Game}
	other.components.sound = &soundComponent{sprite: other}
	other.SetVolume(40)
	other.SetSoundEffect(SoundPanEffect, -40)
	other.SetSoundEffect(SoundPitchEffect, -120)
	otherSettings := backend.settings[other.sound().soundObj]

	// Generated sprites embed both SpriteImpl and a game that embeds Game.
	sprite := &testSprite{testGame: game}
	sprite.g = &game.Game
	sprite.components.sound = &soundComponent{sprite: &sprite.SpriteImpl}
	allocated := backend.next
	sprite.ClearSoundEffects()
	if sprite.sound().soundObj != 0 || backend.next != allocated {
		t.Fatal("clearing unused effects allocated an audio object")
	}

	sprite.SetVolume(25)
	sprite.SetSoundEffect(SoundPanEffect, 80)
	sprite.SetSoundEffect(SoundPitchEffect, 240)
	sprite.ClearSoundEffects()
	for _, tt := range []struct {
		name     string
		soundObj engine.Object
		want     cloneSoundSettings
	}{
		{"sprite", sprite.sound().soundObj, cloneSoundSettings{volume: 0.25, pitch: 1}},
		{"stage", game.audioState.SoundObj, stageSettings},
		{"other sprite", other.sound().soundObj, otherSettings},
	} {
		if got := backend.settings[tt.soundObj]; got != tt.want {
			t.Fatalf("%s sound settings = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestCloneInheritsIndependentSoundEffects(t *testing.T) {
	game := setupCloneLimitGame(t)
	backend := &cloneSoundBackend{}
	game.soundMgr.Init(backend)
	source := newCloneLimitSprite(game, "source")
	source.SetVolume(25)
	source.SetSoundEffect(SoundPanEffect, -40)
	source.SetSoundEffect(SoundPitchEffect, 120)
	source.onClone = func(clone *cloneLimitSprite) {
		if clone.Volume() != 100 || clone.GetSoundEffect(SoundPanEffect) != -40 || clone.GetSoundEffect(SoundPitchEffect) != 120 {
			t.Error("clone handler did not inherit effects with default volume")
		}
	}
	var clone *SpriteImpl
	doClone(source, nil, func(s *SpriteImpl) { clone = s })
	if clone.sound().soundObj == source.sound().soundObj {
		t.Fatal("clone shares its audio object")
	}
	expected := backend.settings[source.sound().soundObj]
	expected.volume = 1
	if backend.settings[clone.sound().soundObj] != expected {
		t.Fatal("clone did not copy exact backend control values")
	}
	clone.SetVolume(80)
	clone.SetSoundEffect(SoundPanEffect, 20)
	if source.Volume() != 25 || source.GetSoundEffect(SoundPanEffect) != -40 {
		t.Fatal("clone changed source sound state")
	}
	clone.Destroy()
	if len(backend.destroyed) != 1 || backend.destroyed[0] == source.sound().soundObj {
		t.Fatal("clone teardown destroyed the wrong audio object")
	}
	if len(backend.plays) != 0 {
		t.Fatal("cloning started playback")
	}
}

func TestCloneWithoutSoundKeepsAudioUnallocated(t *testing.T) {
	game := setupCloneLimitGame(t)
	backend := &cloneSoundBackend{}
	game.soundMgr.Init(backend)
	source := newCloneLimitSprite(game, "source")
	doClone(source, nil, nil)
	if backend.next != 0 {
		t.Fatal("unused sound allocated during cloning")
	}
}

func TestCloneSoundComponentWithoutGame(t *testing.T) {
	sourceSprite, target := &SpriteImpl{}, &SpriteImpl{}
	source := &soundComponent{sprite: sourceSprite, pendingAudios: []string{"do not replay"}}
	cloned := source.cloneFor(target)
	if cloned.sprite != target || cloned.soundObj != 0 || len(cloned.pendingAudios) != 0 {
		t.Fatal("unallocated sound clone inherited runtime resources")
	}
}

func TestCloneSoundEffectsComeFromOriginalSprite(t *testing.T) {
	for _, initiallyAllocated := range []bool{false, true} {
		t.Run(fmt.Sprint(initiallyAllocated), func(t *testing.T) {
			game := setupCloneLimitGame(t)
			backend := &cloneSoundBackend{}
			game.soundMgr.Init(backend)
			original := newCloneLimitSprite(game, "source")
			if initiallyAllocated {
				original.SetSoundEffect(SoundPanEffect, -40)
			}
			parent := createRuntimeClone(&original.SpriteImpl)
			parent.SetSoundEffect(SoundPanEffect, 80)
			parent.SetSoundEffect(SoundPitchEffect, -120)
			parent.SetVolume(25)
			if !initiallyAllocated {
				child := createRuntimeClone(parent)
				if child.sound().soundObj != 0 {
					t.Fatal("inherited parent's effects instead of unused original state")
				}
			}
			original.SetSoundEffect(SoundPanEffect, 20)
			original.SetSoundEffect(SoundPitchEffect, 120)
			child := createRuntimeClone(parent)
			if child.Volume() != 100 || child.GetSoundEffect(SoundPanEffect) != 20 || child.GetSoundEffect(SoundPitchEffect) != 120 {
				t.Fatal("clone of clone did not use current original effects and default volume")
			}
			child.SetSoundEffect(SoundPitchEffect, 240)
			if original.GetSoundEffect(SoundPitchEffect) != 120 || parent.GetSoundEffect(SoundPitchEffect) != -120 {
				t.Fatal("descendant shares effects with an ancestor")
			}
		})
	}
}

func TestCloneSoundEffectsFromStageInstance(t *testing.T) {
	game := setupCloneLimitGame(t)
	backend := &cloneSoundBackend{}
	game.soundMgr.Init(backend)
	template := newCloneLimitSprite(game, "template")
	template.SetSoundEffect(SoundPanEffect, -40)
	var instance cloneLimitSprite
	instantiateStageSprite(reflect.ValueOf(&instance).Elem(), template, spriteProperties{})
	game.shapeMgr.add(&instance.SpriteImpl)
	instance.SetSoundEffect(SoundPanEffect, 20)
	child := createRuntimeClone(&instance.SpriteImpl)
	if child.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("clone inherited template effects instead of its original instance")
	}
}

func TestSoundPlayAndComponentPlayUseSamePlaybackParameters(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)
	anim.sprite.g.audioState.AudioAttenuation = 0.35
	anim.sprite.g.audioState.AudioMaxDistance = 640
	anim.sprite.runtimeState.SyncSprite.Id = 42

	anim.sprite.Play__1("walk", true)
	playID := anim.sprite.sound().play("walk", true)
	if playID == 0 {
		t.Fatal("play returned no playback id")
	}
	if backend.createCalls != 1 || anim.sprite.components.sound.soundObj != 77 {
		t.Fatalf("sprite sound allocations = %d, object = %d; want 1 and 77", backend.createCalls, anim.sprite.components.sound.soundObj)
	}
	if anim.sprite.g.audioState.SoundObj != 0 {
		t.Fatalf("sprite playback initialized game sound object %d", anim.sprite.g.audioState.SoundObj)
	}
	if len(backend.plays) != 2 {
		t.Fatalf("plays = %d, want 2", len(backend.plays))
	}
	first, second := backend.plays[0], backend.plays[1]
	if first != second {
		t.Fatalf("play parameters differ: first=%+v second=%+v", first, second)
	}
	if first.soundObj != 77 || first.owner != 42 || first.attenation != 0.35 || first.maxDist != 640 {
		t.Fatalf("sprite playback parameters = %+v", first)
	}
	if len(backend.loops) != 2 || !backend.loops[0].loop || !backend.loops[1].loop {
		t.Fatalf("loop parameters = %+v, want two looping plays", backend.loops)
	}
}

func TestSpritePlayAndWaitResumesAfterPlaybackFinishes(t *testing.T) {
	co := setupRuntimeScheduler(t)
	anim := newTestAnimationComponent()
	backend := &animationAudioBackend{}
	initTestAnimationAudio(anim, backend)
	sprite := anim.sprite
	sprite.runtimeState.SyncSprite.Id = 42
	sprite.g.audioState.AudioAttenuation = 0.35
	sprite.g.audioState.AudioMaxDistance = 640
	finished := false
	thread := co.Create(sprite, func(coroutine.Thread) {
		sprite.PlayAndWait("walk")
		finished = true
	})
	co.JoinFirstSlice(thread)
	if finished || len(backend.plays) != 1 {
		t.Fatalf("blocking playback finished = %v, plays = %d; want false and 1", finished, len(backend.plays))
	}
	play := backend.plays[0]
	if play.soundObj != 77 || play.owner != 42 || play.attenation != 0.35 || play.maxDist != 640 || len(backend.loops) != 0 {
		t.Fatalf("blocking playback parameters = %+v, loops = %v", play, backend.loops)
	}
	itime.Update(0, 0)
	co.Update()
	if finished {
		t.Fatal("PlayAndWait returned while the backend still reported active playback")
	}
	backend.playing[backend.nextID] = false
	itime.Update(0, 0)
	co.Update()
	if !finished {
		t.Fatal("PlayAndWait did not resume after playback completed")
	}
}

func TestSpritePlaybackControlsResolveSoundNames(t *testing.T) {
	anim := newTestAnimationComponent()
	backend := &spritePlaybackBackend{}
	initTestAnimationAudio(anim, &backend.animationAudioBackend)
	anim.sprite.g.soundMgr.Init(backend)
	sprite := anim.sprite
	sprite.Play__0("walk")
	firstID := backend.nextID
	sprite.PlayWith("step", true)
	secondID := backend.nextID
	sprite.PausePlaying("walk")
	sprite.ResumePlaying("walk")
	sprite.StopPlaying("walk")
	if !slices.Equal(backend.paused, []int64{firstID}) || !slices.Equal(backend.resumed, []int64{firstID}) || !slices.Equal(backend.stops, []int64{firstID}) {
		t.Fatalf("sound controls addressed wrong playback: pause=%v resume=%v stop=%v", backend.paused, backend.resumed, backend.stops)
	}
	if backend.playing[firstID] || !backend.playing[secondID] {
		t.Fatalf("stopping walk changed unrelated sound: playing=%v", backend.playing)
	}
	if len(backend.loops) != 1 || backend.loops[0] != (animationLoopCall{id: secondID, loop: true}) {
		t.Fatalf("loop settings = %v, want only the second playback looping", backend.loops)
	}
}

func (b *soundControlBackend) IsPlaying(int64) bool { return true }

func (b *soundControlBackend) Pause(int64) { b.actions = append(b.actions, "pause") }

func (b *soundControlBackend) Resume(int64) { b.actions = append(b.actions, "resume") }

func (b *soundControlBackend) Stop(int64) { b.actions = append(b.actions, "stop") }

func (b *soundControlBackend) StopAll() { b.actions = append(b.actions, "stop all") }

func (f *fakeAudioBackend) CreateAudio() engine.Object {
	f.createCalls++
	f.volume = 1
	return 77
}

func (f *fakeAudioBackend) DestroyAudio(obj engine.Object) {
	f.destroyCalls++
}

func (f *fakeAudioBackend) SetPitch(obj engine.Object, pitch float64) {
	f.pitch = pitch
}

func (f *fakeAudioBackend) GetPitch(obj engine.Object) float64 {
	return f.pitch
}

func (f *fakeAudioBackend) SetPan(obj engine.Object, pan float64) {
	f.pan = pan
}

func (f *fakeAudioBackend) GetPan(obj engine.Object) float64 {
	return f.pan
}

func (f *fakeAudioBackend) SetVolume(obj engine.Object, volume float64) {
	f.volume = volume
}

func (f *fakeAudioBackend) GetVolume(obj engine.Object) float64 {
	return f.volume
}

func (f *fakeAudioBackend) PlayWithAttenuation(obj engine.Object, path string, ownerID engine.Object, attenuation, maxDistance float64) int64 {
	f.plays = append(f.plays, fakeAudioPlay{owner: ownerID, attenuation: attenuation, maxDistance: maxDistance})
	return 1
}

func (f *fakeAudioBackend) Pause(aid int64) {}

func (f *fakeAudioBackend) Resume(aid int64) {}

func (f *fakeAudioBackend) Stop(aid int64) {}

func (f *fakeAudioBackend) Restart(aid int64) bool { return false }

func (f *fakeAudioBackend) SetLoop(aid int64, loop bool) {}

func (f *fakeAudioBackend) IsPlaying(aid int64) bool { return false }

func (f *fakeAudioBackend) StopAll() {}

func (b *cloneSoundBackend) CreateAudio() engine.Object {
	b.next++
	if b.settings == nil {
		b.settings = make(map[engine.Object]cloneSoundSettings)
	}
	b.settings[b.next] = cloneSoundSettings{volume: 1, pitch: 1}
	return b.next
}

func (b *cloneSoundBackend) SetVolume(id engine.Object, v float64) {
	s := b.settings[id]
	s.volume = v
	b.settings[id] = s
}

func (b *cloneSoundBackend) SetPan(id engine.Object, v float64) {
	s := b.settings[id]
	s.pan = v
	b.settings[id] = s
}

func (b *cloneSoundBackend) SetPitch(id engine.Object, v float64) {
	s := b.settings[id]
	s.pitch = v
	b.settings[id] = s
}

func (b *cloneSoundBackend) GetVolume(id engine.Object) float64 { return b.settings[id].volume }

func (b *cloneSoundBackend) GetPan(id engine.Object) float64 { return b.settings[id].pan }

func (b *cloneSoundBackend) GetPitch(id engine.Object) float64 { return b.settings[id].pitch }

func (b *cloneSoundBackend) DestroyAudio(id engine.Object) {
	b.destroyed = append(b.destroyed, id)
	delete(b.settings, id)
}

func (b *spritePlaybackBackend) Pause(id int64) { b.paused = append(b.paused, id) }

func (b *spritePlaybackBackend) Resume(id int64) { b.resumed = append(b.resumed, id) }
