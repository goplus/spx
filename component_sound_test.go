package spx

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
)

type cloneSoundSettings struct{ volume, pan, pitch float64 }
type cloneSoundBackend struct {
	animationAudioBackend
	next      engine.Object
	settings  map[engine.Object]cloneSoundSettings
	destroyed []engine.Object
}

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
func (b *cloneSoundBackend) GetPan(id engine.Object) float64    { return b.settings[id].pan }
func (b *cloneSoundBackend) GetPitch(id engine.Object) float64  { return b.settings[id].pitch }
func (b *cloneSoundBackend) DestroyAudio(id engine.Object) {
	b.destroyed = append(b.destroyed, id)
	delete(b.settings, id)
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
	source := &soundComponent{pendingAudios: []string{"do not replay"}}
	target := &SpriteImpl{}
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

type spritePlaybackBackend struct {
	animationAudioBackend
	paused, resumed []int64
}

func (b *spritePlaybackBackend) Pause(id int64)  { b.paused = append(b.paused, id) }
func (b *spritePlaybackBackend) Resume(id int64) { b.resumed = append(b.resumed, id) }

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
