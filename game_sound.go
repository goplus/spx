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
	"log"

	"github.com/goplus/spx/v2/internal/audiorecord"
	"github.com/goplus/spx/v2/internal/engine"
)

// ============================================================================
// Sound Types
// ============================================================================

type sound *soundConfig

type SoundName = string

// ============================================================================
// Sound Loading
// ============================================================================

func (p *Game) loadSound(name SoundName) (media sound, err error) {
	if media, ok := p.sounds.audios[name]; ok {
		return media, nil
	}

	if debugLoad {
		log.Println("==> LoadSound", name)
	}
	prefix := "sounds/" + name
	media = new(soundConfig)
	if err = loadJson(media, p.fs, prefix+"/index.json"); err != nil {
		println("loadSound failed:", err.Error())
		return
	}
	media.Path = prefix + "/" + media.Path
	p.sounds.audios[name] = media
	return
}

// ============================================================================
// Sound Playback
// ============================================================================

func (p *Game) playSound(sprite *engine.Sprite, audioId engine.Object, name SoundName, isLoop bool, attenuation, maxDistance float64) soundId {
	m, err := p.loadSound(name)
	if err != nil {
		return invalidSoundId
	}
	return p.sounds.play(audioId, m, isLoop, false, sprite.Id, attenuation, maxDistance)
}

func (p *Game) playSoundAndWait(sprite *engine.Sprite, audioId engine.Object, name SoundName, attenuation, maxDistance float64) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	p.sounds.play(audioId, m, false, true, sprite.Id, attenuation, maxDistance)
}

func (p *Game) pauseSound(audioId engine.Object, name SoundName) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	p.sounds.pause(audioId, m)
}

func (p *Game) resumeSound(audioId engine.Object, name SoundName) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	p.sounds.resume(audioId, m)
}

func (p *Game) stopSound(audioId engine.Object, name SoundName) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	p.sounds.stop(audioId, m)
}

func (p *Game) stopSoundInstance(instanceId soundId) {
	p.sounds.stopInstance(instanceId)
}

// ============================================================================
// Sound Control Methods
// ============================================================================

func (p *Game) Volume() float64 {
	p.checkAudioId()
	return p.sounds.getVolume(p.audioId)
}

func (p *Game) Play__0(name SoundName, loop bool) {
	p.checkAudioId()
	p.playSound(p.syncSprite, p.audioId, name, loop, 0, defaultAudioMaxDist)
}

func (p *Game) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *Game) PlayAndWait(name SoundName) {
	p.checkAudioId()
	p.playSoundAndWait(p.syncSprite, p.audioId, name, 0, defaultAudioMaxDist)
}

func (p *Game) PausePlaying(name SoundName) {
	p.checkAudioId()
	p.pauseSound(p.audioId, name)
}

func (p *Game) ResumePlaying(name SoundName) {
	p.checkAudioId()
	p.resumeSound(p.audioId, name)
}

func (p *Game) StopPlaying(name SoundName) {
	p.checkAudioId()
	p.stopSound(p.audioId, name)
}

func (p *Game) SetVolume(volume float64) {
	p.checkAudioId()
	p.sounds.setVolume(p.audioId, volume)
}

func (p *Game) ChangeVolume(delta float64) {
	p.checkAudioId()
	p.sounds.changeVolume(p.audioId, delta)
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	p.checkAudioId()
	return p.sounds.getEffect(p.audioId, kind)
}

func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.checkAudioId()
	p.sounds.setEffect(p.audioId, kind, value)
}

func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.checkAudioId()
	p.sounds.changeEffect(p.audioId, kind, delta)
}

func (p *Game) checkAudioId() {
	if p.audioId == 0 {
		p.audioId = p.sounds.allocAudio()
	}
}

func (p *Game) ClearSoundEffects() {
	panic("todo")
}

func (p *Game) StopAllSounds() {
	p.sounds.stopAll()
}

func (p *Game) Loudness() float64 {
	if p.aurec == nil {
		p.aurec = audiorecord.Open(gco)
	}
	return p.aurec.Loudness() * 100
}
