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
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
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
	if media, ok := p.sounds.sounds[name]; ok {
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
	p.sounds.sounds[name] = media
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

func (p *Game) withSound(name SoundName, action func(m sound)) {
	m, err := p.loadSound(name)
	if err != nil {
		return
	}
	action(m)
}

func (p *Game) pauseSound(name SoundName) {
	p.withSound(name, p.sounds.pause)
}

func (p *Game) resumeSound(name SoundName) {
	p.withSound(name, p.sounds.resume)
}

func (p *Game) stopSound(name SoundName) {
	p.withSound(name, p.sounds.stop)
}

func (p *Game) stopSoundInstance(instanceId soundId) {
	p.sounds.stopInstance(instanceId)
}

// ============================================================================
// Sound Control Methods
// ============================================================================

func (p *Game) Volume() float64 {
	p.checkSoundObj()
	return p.sounds.getVolume(p.soundObj)
}

func (p *Game) Play__0(name SoundName, loop bool) {
	p.checkSoundObj()
	p.playSound(p.syncSprite, p.soundObj, name, loop, 0, defaultAudioMaxDist)
}

func (p *Game) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *Game) PlayAndWait(name SoundName) {
	p.checkSoundObj()
	p.playSoundAndWait(p.syncSprite, p.soundObj, name, 0, defaultAudioMaxDist)
}

func (p *Game) PausePlaying(name SoundName) {
	p.checkSoundObj()
	p.pauseSound(name)
}

func (p *Game) ResumePlaying(name SoundName) {
	p.checkSoundObj()
	p.resumeSound(name)
}

func (p *Game) StopPlaying(name SoundName) {
	p.checkSoundObj()
	p.stopSound(name)
}

func (p *Game) SetVolume(volume float64) {
	p.checkSoundObj()
	p.sounds.setVolume(p.soundObj, volume)
}

func (p *Game) ChangeVolume(delta float64) {
	p.checkSoundObj()
	p.sounds.changeVolume(p.soundObj, delta)
}

func (p *Game) GetSoundEffect(kind SoundEffectKind) float64 {
	p.checkSoundObj()
	return p.sounds.getEffect(p.soundObj, kind)
}

func (p *Game) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.checkSoundObj()
	p.sounds.setEffect(p.soundObj, kind, value)
}

func (p *Game) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.checkSoundObj()
	p.sounds.changeEffect(p.soundObj, kind, delta)
}

func (p *Game) checkSoundObj() {
	if p.soundObj == 0 {
		p.soundObj = p.sounds.allocSound()
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

// ============================================================================
// Sound Resource Management
// ============================================================================

// releaseGameAudio releases the game's audio resources
func (p *Game) releaseGameAudio() {
	p.sounds.stopAll()
	if p.soundObj != 0 {
		p.sounds.releaseSound(p.soundObj)
		p.soundObj = 0
	}
}
