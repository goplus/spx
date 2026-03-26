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
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Sound Types
// -----------------------------------------------------------------------------
type sound *coreproject.SoundConfig

type soundId = int64

// -----------------------------------------------------------------------------
// Playback
// -----------------------------------------------------------------------------
func (p *Game) Volume() float64 {
	return p.withGameSoundFloat(func(soundObj engine.Object) float64 {
		return p.soundMgr.GetVolume(soundObj)
	})
}

func (p *Game) Play__0(name SoundName, loop bool) {
	p.withGameSound(func(soundObj engine.Object) {
		p.playSound(p.runtimeState.SyncSprite, soundObj, name, loop, 0, defaultAudioMaxDist)
	})
}

func (p *Game) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *Game) PlayAndWait(name SoundName) {
	p.withGameSound(func(soundObj engine.Object) {
		p.playSoundAndWait(p.runtimeState.SyncSprite, soundObj, name, 0, defaultAudioMaxDist)
	})
}

func (p *Game) PausePlaying(name SoundName) {
	p.pauseSound(name)
}

func (p *Game) ResumePlaying(name SoundName) {
	p.resumeSound(name)
}

func (p *Game) StopPlaying(name SoundName) {
	p.stopSound(name)
}

func (p *Game) SetVolume(volume float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.soundMgr.SetVolume(soundObj, volume)
	})
}

func (p *Game) ChangeVolume(delta float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.soundMgr.ChangeVolume(soundObj, delta)
	})
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	return p.withGameSoundFloat(func(soundObj engine.Object) float64 {
		return p.getSoundEffect(soundObj, kind)
	})
}

func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.setSoundEffect(soundObj, kind, value)
	})
}

func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.changeSoundEffect(soundObj, kind, delta)
	})
}

func (p *Game) ClearSoundEffects() {
	panic("todo")
}

func (p *Game) StopAllSounds() {
	p.soundMgr.StopAll()
}

func (p *Game) Loudness() float64 {
	return 0
}

// -----------------------------------------------------------------------------
// Settings
// -----------------------------------------------------------------------------
const (
	defaultAudioMaxDist         = coreproject.DefaultAudioMaxDistance
	invalidSoundId      soundId = 0
)

func (p *Game) applyAudioSettings(settings coreproject.SystemSettings) {
	p.audioState.AudioAttenuation = settings.AudioAttenuation
	p.audioState.AudioMaxDistance = settings.AudioMaxDistance
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------
func (p *Game) loadSound(name SoundName) (media sound, err error) {
	if media, ok := p.sounds[name]; ok {
		return media, nil
	}

	spxlog.Debug("==> LoadSound: %s", name)
	loaded, err := coreproject.LoadSoundConfig(p.fs, name)
	if err != nil {
		spxlog.Error("loadSound failed: %v", err)
		return
	}
	media = &loaded.Config
	p.sounds[name] = media
	return
}

func (p *Game) playSound(sprite *engine.Sprite, audioId engine.Object, name SoundName, isLoop bool, attenuation, maxDistance float64) soundId {
	m, err := p.loadSound(name)
	if err != nil {
		return invalidSoundId
	}
	ownerID := engine.Object(0)
	if sprite != nil {
		ownerID = sprite.Id
	}
	return p.soundMgr.Play(audioId, m.Path, isLoop, false, ownerID, attenuation, maxDistance)
}

func (p *Game) playSoundAndWait(sprite *engine.Sprite, audioId engine.Object, name SoundName, attenuation, maxDistance float64) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	ownerID := engine.Object(0)
	if sprite != nil {
		ownerID = sprite.Id
	}
	p.soundMgr.Play(audioId, m.Path, false, true, ownerID, attenuation, maxDistance)
}

func (p *Game) withSound(name SoundName, action func(m sound)) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	action(m)
}

func (p *Game) pauseSound(name SoundName) {
	p.withSound(name, func(m sound) {
		p.soundMgr.Pause(m.Path)
	})
}

func (p *Game) resumeSound(name SoundName) {
	p.withSound(name, func(m sound) {
		p.soundMgr.Resume(m.Path)
	})
}

func (p *Game) stopSound(name SoundName) {
	p.withSound(name, func(m sound) {
		p.soundMgr.Stop(m.Path)
	})
}

func (p *Game) checkSoundObj() {
	if p.audioState.SoundObj == 0 {
		p.audioState.SoundObj = p.soundMgr.AllocSound()
	}
}

func (p *Game) withGameSound(action func(soundObj engine.Object)) {
	p.checkSoundObj()
	action(p.audioState.SoundObj)
}

func (p *Game) withGameSoundFloat(action func(soundObj engine.Object) float64) float64 {
	p.checkSoundObj()
	return action(p.audioState.SoundObj)
}

func (p *Game) releaseGameAudio() {
	p.soundMgr.StopAll()
	if p.audioState.SoundObj != 0 {
		p.soundMgr.ReleaseSound(p.audioState.SoundObj)
		p.audioState.SoundObj = 0
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
	p.setSoundEffect(soundObj, kind, p.getSoundEffect(soundObj, kind)+delta)
}
