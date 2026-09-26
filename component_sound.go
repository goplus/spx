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

import engine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"

// ============================================================================
// Sound Component
// ============================================================================
// This component encapsulates all sound-related functionality.

type soundComponent struct {
	sprite *SpriteImpl

	// Sound object.
	soundObj engine.Object

	// Pending audio names.
	pendingAudios []string
}

// ============================================================================
// Lifecycle
// ============================================================================

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

// onDestroy cleans up when the component is destroyed.
func (s *soundComponent) onDestroy() {
	if s.soundObj != invalidSoundObject {
		s.sprite.g.soundMgr.ReleaseSound(s.soundObj)
		s.soundObj = invalidSoundObject
	}
}

// ============================================================================
// Sound Playback
// ============================================================================

func (s *soundComponent) play(name SoundName, loop bool) int64 {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	return s.sprite.g.playSound(s.sprite.runtimeState.SyncSprite, s.soundObj, name, loop, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

func (s *soundComponent) playAndWait(name SoundName) {
	s.sprite.g.ensureSoundObject(&s.soundObj)
	s.sprite.g.playSoundAndWait(s.sprite.runtimeState.SyncSprite, s.soundObj, name, s.sprite.g.audioState.AudioAttenuation, s.sprite.g.audioState.AudioMaxDistance)
}

// ============================================================================
// Volume
// ============================================================================

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

// ============================================================================
// Sound Effects
// ============================================================================

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

// ============================================================================
// Internal Playback Management
// ============================================================================

func (s *soundComponent) restartOrPlayLoopedAudio(name SoundName, id int64) int64 {
	if name == "" {
		return 0
	}
	if id != 0 && s.sprite.g.soundMgr.RestartID(id) {
		return id
	}
	return s.play(name, true)
}

func (s *soundComponent) addPendingAudio(audioName string) {
	s.pendingAudios = append(s.pendingAudios, audioName)
}

func (s *soundComponent) takePendingAudios(buffer []string) []string {
	buffer = append(buffer, s.pendingAudios...)
	s.pendingAudios = s.pendingAudios[:0]
	return buffer
}
