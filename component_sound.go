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
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

// ============================================================================
// Sound Component
// ============================================================================
// This component encapsulates all sound-related functionality.

type soundComponent struct {
	componentBase

	// Sound object
	soundObj engine.Object

	// Pending audios
	pendingAudios []string
}

// initialize initializes the sound component from config.
func (s *soundComponent) initialize(sprite *SpriteImpl, spriteCfg *spriteConfig) {
	s.componentBase.initialize(sprite, spriteCfg)
	// Always initialize with default sound values
	s.soundObj = 0
	s.pendingAudios = make([]string, 0)
}

// cloneFrom creates a new sound component by cloning from source.
func (s *soundComponent) cloneFrom(src component, newSprite *SpriteImpl) component {
	// srcSound := src.(*soundComponent) // Not used since we don't clone sound state
	return &soundComponent{
		componentBase: componentBase{sprite: newSprite},
		soundObj:      0, // Don't share sound object, will be allocated if needed
		pendingAudios: make([]string, 0),
	}
}

// OnDestroy cleanup when component is destroyed.
func (s *soundComponent) onDestroy() {
	if s.soundObj != 0 {
		s.sprite.g.soundMgr.ReleaseSound(s.soundObj)
		s.soundObj = 0
	}
}

// ============================================================================
// Sound Playback Control
// ============================================================================

func (s *soundComponent) Play(name SoundName, loop bool) {
	s.checkSoundObj()
	s.sprite.g.playSound(s.sprite.runtimeState.SyncSprite, s.soundObj, name, loop, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) PlayAndWait(name SoundName) {
	s.checkSoundObj()
	s.sprite.g.playSoundAndWait(s.sprite.runtimeState.SyncSprite, s.soundObj, name, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) PausePlaying(name SoundName) {
	s.sprite.g.pauseSound(name)
}

func (s *soundComponent) ResumePlaying(name SoundName) {
	s.sprite.g.resumeSound(name)
}

func (s *soundComponent) StopPlaying(name SoundName) {
	s.sprite.g.stopSound(name)
}

// ============================================================================
// Sound Volume Control
// ============================================================================

func (s *soundComponent) GetVolume() float64 {
	s.checkSoundObj()
	return s.sprite.g.soundMgr.GetVolume(s.soundObj)
}

func (s *soundComponent) SetVolume(volume float64) {
	s.checkSoundObj()
	s.sprite.g.soundMgr.SetVolume(s.soundObj, volume)
}

func (s *soundComponent) ChangeVolume(delta float64) {
	s.checkSoundObj()
	s.sprite.g.soundMgr.ChangeVolume(s.soundObj, delta)
}

// ============================================================================
// Sound Effects Control
// ============================================================================

func (s *soundComponent) GetSoundEffect(kind SoundEffectKind) float64 {
	s.checkSoundObj()
	return s.sprite.g.getSoundEffect(s.soundObj, kind)
}

func (s *soundComponent) SetSoundEffect(kind SoundEffectKind, value float64) {
	s.checkSoundObj()
	s.sprite.g.setSoundEffect(s.soundObj, kind, value)
}

func (s *soundComponent) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	s.checkSoundObj()
	s.sprite.g.changeSoundEffect(s.soundObj, kind, delta)
}

// ============================================================================
// Internal Audio Management
// ============================================================================

func (s *soundComponent) playAudio(name SoundName, loop bool) soundId {
	s.checkSoundObj()
	return s.sprite.g.playSound(s.sprite.runtimeState.SyncSprite, s.soundObj, name, loop, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) checkSoundObj() {
	if s.soundObj == 0 {
		s.soundObj = s.sprite.g.soundMgr.AllocSound()
	}
}

func (s *soundComponent) addPendingAudio(audioName string) {
	s.pendingAudios = append(s.pendingAudios, audioName)
}

func (s *soundComponent) takePendingAudios(buffer []string) []string {
	buffer = append(buffer, s.pendingAudios...)
	s.pendingAudios = s.pendingAudios[:0]
	return buffer
}
