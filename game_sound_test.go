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
	"math"
	"slices"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
)

type soundControlBackend struct {
	fakeAudioBackend
	actions []string
}

func (b *soundControlBackend) IsPlaying(int64) bool { return true }
func (b *soundControlBackend) Pause(int64)          { b.actions = append(b.actions, "pause") }
func (b *soundControlBackend) Resume(int64)         { b.actions = append(b.actions, "resume") }
func (b *soundControlBackend) Stop(int64)           { b.actions = append(b.actions, "stop") }
func (b *soundControlBackend) StopAll()             { b.actions = append(b.actions, "stop all") }

func TestGameSoundPlaybackControls(t *testing.T) {
	backend := &soundControlBackend{}
	game := &Game{fs: reloadConfigFS{}, sounds: map[string]sound{
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

type fakeAudioBackend struct {
	pan          float64
	pitch        float64
	volume       float64
	createCalls  int
	destroyCalls int
	plays        []fakeAudioPlay
}

type fakeAudioPlay struct {
	owner       internalengine.Object
	attenuation float64
	maxDistance float64
}

func (f *fakeAudioBackend) CreateAudio() internalengine.Object {
	f.createCalls++
	f.volume = 1
	return 77
}

func (f *fakeAudioBackend) DestroyAudio(obj internalengine.Object) {
	f.destroyCalls++
}

func (f *fakeAudioBackend) SetPitch(obj internalengine.Object, pitch float64) {
	f.pitch = pitch
}

func (f *fakeAudioBackend) GetPitch(obj internalengine.Object) float64 {
	return f.pitch
}

func (f *fakeAudioBackend) SetPan(obj internalengine.Object, pan float64) {
	f.pan = pan
}

func (f *fakeAudioBackend) GetPan(obj internalengine.Object) float64 {
	return f.pan
}

func (f *fakeAudioBackend) SetVolume(obj internalengine.Object, volume float64) {
	f.volume = volume
}

func (f *fakeAudioBackend) GetVolume(obj internalengine.Object) float64 {
	return f.volume
}

func (f *fakeAudioBackend) PlayWithAttenuation(obj internalengine.Object, path string, ownerID internalengine.Object, attenuation, maxDistance float64) int64 {
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

func TestGameClearSoundEffects(t *testing.T) {
	for _, test := range []struct {
		name                   string
		soundObj, wantSoundObj internalengine.Object
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
	fs := reloadConfigFS{
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
			fs := reloadConfigFS{}
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
