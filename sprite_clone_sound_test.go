package spx

import (
	"fmt"
	"reflect"
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
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
	cloned := source.cloneFrom(source, target).(*soundComponent)
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
	applySprite(reflect.ValueOf(&instance).Elem(), template, coreproject.StageShape{})
	game.addShape(&instance.SpriteImpl)
	instance.SetSoundEffect(SoundPanEffect, 20)
	child := createRuntimeClone(&instance.SpriteImpl)
	if child.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("clone inherited template effects instead of its original instance")
	}
}
