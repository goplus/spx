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
	internalengine "github.com/goplus/spx/v2/internal/engine"
)

type fakeAudioBackend struct {
	pan         float64
	pitch       float64
	createCalls int
}

type targetPropertyActor struct {
	Score int
	Power float64
}

func (a *targetPropertyActor) Health() float64 {
	return a.Power
}

type targetPropertyGame struct {
	Game
	StageScore int
	Actor      targetPropertyActor
}

func (f *fakeAudioBackend) CreateAudio() internalengine.Object {
	f.createCalls++
	return 77
}

func (f *fakeAudioBackend) DestroyAudio(obj internalengine.Object) {}

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

func (f *fakeAudioBackend) SetVolume(obj internalengine.Object, volume float64) {}

func (f *fakeAudioBackend) GetVolume(obj internalengine.Object) float64 {
	return 1
}

func (f *fakeAudioBackend) PlayWithAttenuation(obj internalengine.Object, path string, ownerID internalengine.Object, attenuation, maxDistance float64) int64 {
	return 1
}

func (f *fakeAudioBackend) Pause(aid int64)              {}
func (f *fakeAudioBackend) Resume(aid int64)             {}
func (f *fakeAudioBackend) Stop(aid int64)               {}
func (f *fakeAudioBackend) SetLoop(aid int64, loop bool) {}
func (f *fakeAudioBackend) IsPlaying(aid int64) bool     { return false }
func (f *fakeAudioBackend) StopAll()                     {}

func TestGameClearSoundEffectsResetsPanAndPitch(t *testing.T) {
	backend := &fakeAudioBackend{pan: 0.4, pitch: 1.25}
	var g Game
	g.soundMgr = internalaudio.Manager{}
	g.soundMgr.Init(backend)
	g.audioState.SoundObj = 9

	g.ClearSoundEffects()

	if backend.pan != 0 {
		t.Fatalf("pan = %v, want 0", backend.pan)
	}
	if backend.pitch != 0 {
		t.Fatalf("pitch = %v, want 0", backend.pitch)
	}
}

func TestGameClearSoundEffectsAllocatesWhenUnused(t *testing.T) {
	backend := &fakeAudioBackend{pan: 0.4, pitch: 1.25}
	var g Game
	g.soundMgr = internalaudio.Manager{}
	g.soundMgr.Init(backend)

	g.ClearSoundEffects()

	if backend.createCalls != 1 {
		t.Fatalf("CreateAudio calls = %d, want 1", backend.createCalls)
	}
	if g.audioState.SoundObj != 77 {
		t.Fatalf("SoundObj = %d, want 77", g.audioState.SoundObj)
	}
	if backend.pan != 0 {
		t.Fatalf("pan = %v, want 0", backend.pan)
	}
	if backend.pitch != 0 {
		t.Fatalf("pitch = %v, want 0", backend.pitch)
	}
}

func TestGameUsernameReturnsEmptyString(t *testing.T) {
	var g Game
	if got := g.Username(); got != "" {
		t.Fatalf("Username = %q, want empty string", got)
	}
}

func TestGameGetTargetPropertyStageField(t *testing.T) {
	g := &targetPropertyGame{StageScore: 23}
	g.gamer = g

	got := g.GetTargetProperty("", "StageScore")
	if got.Int() != 23 {
		t.Fatalf("StageScore = %d, want 23", got.Int())
	}
}

func TestGameGetTargetPropertyTargetField(t *testing.T) {
	g := &targetPropertyGame{Actor: targetPropertyActor{Score: 9}}
	g.gamer = g

	got := g.GetTargetProperty("Actor", "Score")
	if got.Int() != 9 {
		t.Fatalf("Actor.Score = %d, want 9", got.Int())
	}
}

func TestGameGetTargetPropertyTargetMethod(t *testing.T) {
	g := &targetPropertyGame{Actor: targetPropertyActor{Power: 3.5}}
	g.gamer = g

	got := g.GetTargetProperty("Actor", "Health")
	if got.Float() != 3.5 {
		t.Fatalf("Actor.Health = %v, want 3.5", got.Float())
	}
}

func TestSpriteGetTargetPropertyExplicitTarget(t *testing.T) {
	g := &targetPropertyGame{Actor: targetPropertyActor{Score: 9}}
	g.gamer = g

	sp := &SpriteImpl{g: &g.Game, name: "Actor"}
	got := sp.GetTargetProperty("Actor", "Score")
	if got.Int() != 9 {
		t.Fatalf("explicit target Score = %d, want 9", got.Int())
	}
}

func TestSpriteGetTargetPropertyEmptyTargetUsesStage(t *testing.T) {
	g := &targetPropertyGame{StageScore: 23, Actor: targetPropertyActor{Score: 9}}
	g.gamer = g

	sp := &SpriteImpl{g: &g.Game, name: "Actor"}
	got := sp.GetTargetProperty("", "StageScore")
	if got.Int() != 23 {
		t.Fatalf("empty target StageScore = %d, want 23", got.Int())
	}
}
