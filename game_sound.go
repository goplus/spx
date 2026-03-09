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
	"github.com/goplus/spx/v2/internal/audiorecord"
	"github.com/goplus/spx/v2/internal/base/valueutil"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Public types
// -----------------------------------------------------------------------------

type sound *soundConfig

type SoundName = string

// -----------------------------------------------------------------------------
// Public sound control methods
// -----------------------------------------------------------------------------

func (p *Game) Volume() float64 {
	return p.withGameSoundFloat(func(soundObj engine.Object) float64 {
		return p.soundMgr.getVolume(soundObj)
	})
}

func (p *Game) Play__0(name SoundName, loop bool) {
	p.withGameSound(func(soundObj engine.Object) {
		p.playSound(p.syncSprite, soundObj, name, loop, 0, defaultAudioMaxDist)
	})
}

func (p *Game) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *Game) PlayAndWait(name SoundName) {
	p.withGameSound(func(soundObj engine.Object) {
		p.playSoundAndWait(p.syncSprite, soundObj, name, 0, defaultAudioMaxDist)
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
		p.soundMgr.setVolume(soundObj, volume)
	})
}

func (p *Game) ChangeVolume(delta float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.soundMgr.changeVolume(soundObj, delta)
	})
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	return p.withGameSoundFloat(func(soundObj engine.Object) float64 {
		return p.soundMgr.getEffect(soundObj, kind)
	})
}

func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.soundMgr.setEffect(soundObj, kind, value)
	})
}

func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.withGameSound(func(soundObj engine.Object) {
		p.soundMgr.changeEffect(soundObj, kind, delta)
	})
}

func (p *Game) ClearSoundEffects() {
	panic("todo")
}

func (p *Game) StopAllSounds() {
	p.soundMgr.stopAll()
}

func (p *Game) Loudness() float64 {
	if p.aurec == nil {
		p.aurec = audiorecord.Open(gco)
	}
	return p.aurec.Loudness() * 100
}

// -----------------------------------------------------------------------------
// Private audio configuration
// -----------------------------------------------------------------------------

const (
	defaultAudioMaxDist = 2000 // default maximum audio distance
)

func (p *Game) setupAudioConfig(proj *projConfig) {
	p.audioAttenuation = valueutil.OrDefault(proj.AudioAttenuation, 0)
	p.audioMaxDistance = valueutil.OrDefault(proj.AudioMaxDistance, defaultAudioMaxDist)
}

// -----------------------------------------------------------------------------
// Private sound loading and playback
// -----------------------------------------------------------------------------

func (p *Game) loadSound(name SoundName) (media sound, err error) {
	if media, ok := p.soundMgr.sounds[name]; ok {
		return media, nil
	}

	spxlog.Debug("==> LoadSound: %s", name)
	prefix := "sounds/" + name
	media = new(soundConfig)
	if err = loadJson(media, p.fs, prefix+"/index.json"); err != nil {
		spxlog.Error("loadSound failed: %v", err)
		return
	}
	media.Path = prefix + "/" + media.Path
	p.soundMgr.sounds[name] = media
	return
}

func (p *Game) playSound(sprite *engine.Sprite, audioId engine.Object, name SoundName, isLoop bool, attenuation, maxDistance float64) soundId {
	m, err := p.loadSound(name)
	if err != nil {
		return invalidSoundId
	}
	return p.soundMgr.play(audioId, m, isLoop, false, sprite.Id, attenuation, maxDistance)
}

func (p *Game) playSoundAndWait(sprite *engine.Sprite, audioId engine.Object, name SoundName, attenuation, maxDistance float64) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	p.soundMgr.play(audioId, m, false, true, sprite.Id, attenuation, maxDistance)
}

func (p *Game) withSound(name SoundName, action func(m sound)) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	action(m)
}

func (p *Game) pauseSound(name SoundName) {
	p.withSound(name, p.soundMgr.pause)
}

func (p *Game) resumeSound(name SoundName) {
	p.withSound(name, p.soundMgr.resume)
}

func (p *Game) stopSound(name SoundName) {
	p.withSound(name, p.soundMgr.stop)
}

func (p *Game) checkSoundObj() {
	if p.soundObj == 0 {
		p.soundObj = p.soundMgr.allocSound()
	}
}

func (p *Game) withGameSound(action func(soundObj engine.Object)) {
	p.checkSoundObj()
	action(p.soundObj)
}

func (p *Game) withGameSoundFloat(action func(soundObj engine.Object) float64) float64 {
	p.checkSoundObj()
	return action(p.soundObj)
}

func (p *Game) releaseGameAudio() {
	p.soundMgr.stopAll()
	if p.soundObj != 0 {
		p.soundMgr.releaseSound(p.soundObj)
		p.soundObj = 0
	}
}
