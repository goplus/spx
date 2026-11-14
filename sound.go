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

	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

type SoundEffectKind int

const (
	SoundPanEffect SoundEffectKind = iota
	SoundPitchEffect
)

type sound *coreproject.SoundConfig

type gameAudioState struct {
	AudioAttenuation float64
	AudioMaxDistance float64
	SoundObj         engine.Object
}

type soundComponent struct {
	sprite   *SpriteImpl
	soundObj engine.Object
}

const (
	defaultAudioMaxDist               = coreproject.DefaultAudioMaxDistance
	invalidSoundObject  engine.Object = 0
	invalidPlaybackID   int64         = 0
)

func (p *Game) Play__0(sound SoundName) {
	p.Play__1(sound, false)
}

func (p *Game) Play__1(sound SoundName, loop bool) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.playSound(nil, p.audioState.SoundObj, sound, loop, 0, defaultAudioMaxDist)
}

func (p *Game) PlayAndWait(sound SoundName) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.playSoundAndWait(nil, p.audioState.SoundObj, sound, 0, defaultAudioMaxDist)
}

func (p *Game) PausePlaying(sound SoundName) {
	p.withSound(sound, p.soundMgr.Pause)
}

func (p *Game) ResumePlaying(sound SoundName) {
	p.withSound(sound, p.soundMgr.Resume)
}

func (p *Game) StopPlaying(sound SoundName) {
	p.withSound(sound, p.soundMgr.Stop)
}

func (p *Game) Volume() float64 {
	p.ensureSoundObject(&p.audioState.SoundObj)
	return p.soundMgr.GetVolume(p.audioState.SoundObj)
}

func (p *Game) SetVolume(volume float64) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.soundMgr.SetVolume(p.audioState.SoundObj, volume)
	engine.RequestRedraw()
}

func (p *Game) ChangeVolume(dVolume float64) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.soundMgr.ChangeVolume(p.audioState.SoundObj, dVolume)
	engine.RequestRedraw()
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	p.ensureSoundObject(&p.audioState.SoundObj)
	return p.getSoundEffect(p.audioState.SoundObj, kind)
}

func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.setSoundEffect(p.audioState.SoundObj, kind, value)
	engine.RequestRedraw()
}

func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.changeSoundEffect(p.audioState.SoundObj, kind, delta)
	engine.RequestRedraw()
}

func (p *Game) ClearSoundEffects() {
	p.ensureSoundObject(&p.audioState.SoundObj)
	p.clearSoundEffects(p.audioState.SoundObj)
}

func (p *Game) StopAllSounds() {
	p.soundMgr.StopAll()
}

func (p *Game) Loudness() float64 {
	return 0
}

func (p *Game) applyAudioSettings(settings coreproject.SystemSettings) {
	p.audioState.AudioAttenuation = settings.AudioAttenuation
	p.audioState.AudioMaxDistance = settings.AudioMaxDistance
}

func (p *Game) loadSound(name SoundName) (sound, error) {
	if media, ok := p.sounds[name]; ok {
		return media, nil
	}

	spxlog.Debug("LoadSound: %s", name)
	config, err := coreproject.LoadSoundConfig(p.fs, name)
	if err != nil {
		spxlog.Error("LoadSound failed: %v", err)
		return nil, err
	}
	media := &config
	p.sounds[name] = media
	return media, nil
}

func (p *Game) playSound(sprite *engine.Sprite, soundObj engine.Object, name SoundName, isLoop bool, attenuation, maxDistance float64) int64 {
	return p.playSoundInternal(sprite, soundObj, name, isLoop, false, attenuation, maxDistance)
}

func (p *Game) playSoundAndWait(sprite *engine.Sprite, soundObj engine.Object, name SoundName, attenuation, maxDistance float64) int64 {
	return p.playSoundInternal(sprite, soundObj, name, false, true, attenuation, maxDistance)
}

func (p *Game) playSoundInternal(sprite *engine.Sprite, soundObj engine.Object, name SoundName, isLoop, wait bool, attenuation, maxDistance float64) int64 {
	m, err := p.loadSound(name)
	if err != nil {
		return 0
	}
	ownerID := invalidSoundObject
	if sprite != nil {
		ownerID = sprite.Id
	}
	return p.soundMgr.Play(soundObj, m.Path, isLoop, wait, ownerID, attenuation, maxDistance)
}

func (p *Game) withSound(name SoundName, action func(path string)) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	action(m.Path)
}

func (p *Game) ensureSoundObject(soundObj *engine.Object) {
	if *soundObj == invalidSoundObject {
		*soundObj = p.soundMgr.AllocSound()
	}
}

func (p *Game) releaseGameAudio() {
	p.soundMgr.StopAll()
	if p.audioState.SoundObj != invalidSoundObject {
		p.soundMgr.ReleaseSound(p.audioState.SoundObj)
		p.audioState.SoundObj = invalidSoundObject
	}
}

func (p *Game) getSoundEffect(soundObj engine.Object, kind SoundEffectKind) float64 {
	switch kind {
	case SoundPanEffect:
		return p.soundMgr.GetPan(soundObj)
	case SoundPitchEffect:
		return p.soundMgr.GetPitch(soundObj)
	default:
		panic("GetSoundEffect: invalid kind")
	}
}

func (p *Game) setSoundEffect(soundObj engine.Object, kind SoundEffectKind, value float64) {
	switch kind {
	case SoundPanEffect:
		p.soundMgr.SetPan(soundObj, value)
	case SoundPitchEffect:
		p.soundMgr.SetPitch(soundObj, value)
	default:
		panic("SetSoundEffect: invalid kind")
	}
}

func (p *Game) changeSoundEffect(soundObj engine.Object, kind SoundEffectKind, delta float64) {
	if math.IsNaN(delta) {
		delta = 0
	}
	p.setSoundEffect(soundObj, kind, p.getSoundEffect(soundObj, kind)+delta)
}

func (p *Game) clearSoundEffects(soundObj engine.Object) {
	if soundObj == invalidSoundObject {
		return
	}
	p.soundMgr.SetPan(soundObj, 0)
	p.soundMgr.SetPitch(soundObj, 0)
}

func (p *SpriteImpl) Play__0(sound SoundName) {
	p.Play__1(sound, false)
}

func (p *SpriteImpl) Play__1(sound SoundName, loop bool) {
	p.sound().play(sound, loop)
}

func (p *SpriteImpl) PlayWith(sound SoundName, __xgo_optional_loop bool) {
	p.sound().play(sound, __xgo_optional_loop)
}

func (p *SpriteImpl) PlayAndWait(sound SoundName) {
	p.sound().playAndWait(sound)
}

func (p *SpriteImpl) PausePlaying(sound SoundName) {
	p.g.PausePlaying(sound)
}

func (p *SpriteImpl) ResumePlaying(sound SoundName) {
	p.g.ResumePlaying(sound)
}

func (p *SpriteImpl) StopPlaying(sound SoundName) {
	p.g.StopPlaying(sound)
}

func (p *SpriteImpl) Volume() float64 {
	return p.sound().getVolume()
}

func (p *SpriteImpl) SetVolume(volume float64) {
	p.sound().setVolume(volume)
	engine.RequestRedraw()
}

func (p *SpriteImpl) ChangeVolume(dVolume float64) {
	p.sound().changeVolume(dVolume)
	engine.RequestRedraw()
}

func (p *SpriteImpl) GetSoundEffect(kind SoundEffectKind) float64 {
	return p.sound().getSoundEffect(kind)
}

func (p *SpriteImpl) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.sound().setSoundEffect(kind, value)
	engine.RequestRedraw()
}

func (p *SpriteImpl) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.sound().changeSoundEffect(kind, delta)
	engine.RequestRedraw()
}

func (p *SpriteImpl) ClearSoundEffects() {
	p.sound().clearSoundEffects()
}

// cloneFor creates a new sound component for newSprite without sharing playback state.
func (s *soundComponent) cloneFor(newSprite *SpriteImpl) *soundComponent {
	cloned := &soundComponent{
		sprite: newSprite,
	}
	if s.soundObj != invalidSoundObject {
		cloned.soundObj = newSprite.g.soundMgr.CloneSoundEffects(s.soundObj)
	}
	return cloned
}

func (s *soundComponent) onDestroy() {
	if s.soundObj != invalidSoundObject {
		s.sprite.g.soundMgr.ReleaseSound(s.soundObj)
		s.soundObj = invalidSoundObject
	}
}

func (s *soundComponent) play(name SoundName, loop bool) int64 {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	return s.sprite.g.playSound(s.sprite.runtimeState.proxy, s.soundObj, name, loop, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) playAndWait(name SoundName) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.playSoundAndWait(s.sprite.runtimeState.proxy, s.soundObj, name, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) getVolume() float64 {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	return s.sprite.g.soundMgr.GetVolume(s.soundObj)
}

func (s *soundComponent) setVolume(volume float64) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.soundMgr.SetVolume(s.soundObj, volume)
}

func (s *soundComponent) changeVolume(delta float64) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.soundMgr.ChangeVolume(s.soundObj, delta)
}

func (s *soundComponent) getSoundEffect(kind SoundEffectKind) float64 {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	return s.sprite.g.getSoundEffect(s.soundObj, kind)
}

func (s *soundComponent) setSoundEffect(kind SoundEffectKind, value float64) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.setSoundEffect(s.soundObj, kind, value)
}

func (s *soundComponent) changeSoundEffect(kind SoundEffectKind, delta float64) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.changeSoundEffect(s.soundObj, kind, delta)
}

func (s *soundComponent) clearSoundEffects() {
	s.sprite.g.clearSoundEffects(s.soundObj)
}
