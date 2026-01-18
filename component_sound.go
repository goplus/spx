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
	"github.com/goplus/spx/v2/internal/engine"
)

// ============================================================================
// Sound Component
// ============================================================================
// This component encapsulates all sound-related functionality

type soundComponent struct {
	componentBase

	// Sound object
	soundObj engine.Object

	// Pending audios
	pendingAudios []string
}

// Initialize initializes the sound component
func (sc *soundComponent) initialize(sprite *SpriteImpl) {
	sc.componentBase.initialize(sprite)
	sc.soundObj = sprite.soundObj
	sc.pendingAudios = make([]string, 0)
}

// OnDestroy cleanup when component is destroyed
func (sc *soundComponent) onDestroy() {
	if sc.soundObj != 0 {
		sc.sprite.g.sounds.releaseSound(sc.soundObj)
		sc.soundObj = 0
	}
}

// ============================================================================
// Sound Playback Control
// ============================================================================

func (sc *soundComponent) Play(name SoundName, loop bool) {
	sc.checkSoundObj()
	sc.sprite.g.playSound(sc.sprite.syncSprite, sc.soundObj, name, loop, sc.sprite.g.audioAttenuation, sc.sprite.g.audioMaxDistance)
}

func (sc *soundComponent) PlayAndWait(name SoundName) {
	sc.checkSoundObj()
	sc.sprite.g.playSoundAndWait(sc.sprite.syncSprite, sc.soundObj, name, sc.sprite.g.audioAttenuation, sc.sprite.g.audioMaxDistance)
}

func (sc *soundComponent) PausePlaying(name SoundName) {
	sc.doSoundAction(name, sc.sprite.g.pauseSound)
}

func (sc *soundComponent) ResumePlaying(name SoundName) {
	sc.doSoundAction(name, sc.sprite.g.resumeSound)
}

func (sc *soundComponent) StopPlaying(name SoundName) {
	sc.doSoundAction(name, sc.sprite.g.stopSound)
}

func (sc *soundComponent) doSoundAction(name SoundName, action func(name SoundName)) {
	action(name)
}

// ============================================================================
// Sound Volume Control
// ============================================================================

func (sc *soundComponent) GetVolume() float64 {
	sc.checkSoundObj()
	return sc.sprite.g.sounds.getVolume(sc.soundObj)
}

func (sc *soundComponent) SetVolume(volume float64) {
	sc.checkSoundObj()
	sc.sprite.g.sounds.setVolume(sc.soundObj, volume)
}

func (sc *soundComponent) ChangeVolume(delta float64) {
	sc.checkSoundObj()
	sc.sprite.g.sounds.changeVolume(sc.soundObj, delta)
}

// ============================================================================
// Sound Effects Control
// ============================================================================

func (sc *soundComponent) GetSoundEffect(kind SoundEffectKind) float64 {
	sc.checkSoundObj()
	return sc.sprite.g.sounds.getEffect(sc.soundObj, kind)
}

func (sc *soundComponent) SetSoundEffect(kind SoundEffectKind, value float64) {
	sc.checkSoundObj()
	sc.sprite.g.sounds.setEffect(sc.soundObj, kind, value)
}

func (sc *soundComponent) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	sc.checkSoundObj()
	sc.sprite.g.sounds.changeEffect(sc.soundObj, kind, delta)
}

// ============================================================================
// Internal Audio Management
// ============================================================================

func (sc *soundComponent) playAudio(name SoundName, loop bool) soundId {
	sc.checkSoundObj()
	return sc.sprite.g.playSound(sc.sprite.syncSprite, sc.soundObj, name, loop, sc.sprite.g.audioAttenuation, sc.sprite.g.audioMaxDistance)
}

func (sc *soundComponent) checkSoundObj() {
	if sc.soundObj == 0 {
		sc.soundObj = sc.sprite.g.sounds.allocSound()
	}
}

// cloneFrom creates a new sound component by cloning from source
func (sc *soundComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	newSound := &soundComponent{}
	newSound.initialize(newSprite)
	return newSound
}
