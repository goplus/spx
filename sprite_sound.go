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

import "github.com/goplus/spx/v2/internal/engine"

// ======================== Sound Component ========================
// This file contains sound-related functionality for sprites,
// including sound playback, volume control, and sound effects.
// All methods proxy to the sound component implementation.

// -----------------------------------------------------------------------------
// Sound Effect Types
// -----------------------------------------------------------------------------

type SoundEffectKind int

const (
	SoundPanEffect SoundEffectKind = iota
	SoundPitchEffect
)

// -----------------------------------------------------------------------------
// Sound Playback Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Play__0(name SoundName, loop bool) {
	p.sound().Play(name, loop)
}

func (p *SpriteImpl) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *SpriteImpl) PlayAndWait(name SoundName) {
	p.sound().PlayAndWait(name)
}

func (p *SpriteImpl) PausePlaying(name SoundName) {
	p.sound().PausePlaying(name)
}

func (p *SpriteImpl) ResumePlaying(name SoundName) {
	p.sound().ResumePlaying(name)
}

func (p *SpriteImpl) StopPlaying(name SoundName) {
	p.sound().StopPlaying(name)
}

// -----------------------------------------------------------------------------
// Sound Volume Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Volume() float64 {
	return p.sound().GetVolume()
}

func (p *SpriteImpl) SetVolume(volume float64) {
	p.sound().SetVolume(volume)
}

func (p *SpriteImpl) ChangeVolume(delta float64) {
	p.sound().ChangeVolume(delta)
}

// -----------------------------------------------------------------------------
// Sound Effects Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) GetSoundEffect(kind SoundEffectKind) float64 {
	return p.sound().GetSoundEffect(kind)
}

func (p *SpriteImpl) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.sound().SetSoundEffect(kind, value)
}

func (p *SpriteImpl) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.sound().ChangeSoundEffect(kind, delta)
}

// -----------------------------------------------------------------------------
// Internal Audio Management
// -----------------------------------------------------------------------------

func (p *SpriteImpl) playAudio(name SoundName, loop bool) soundId {
	return p.sound().playAudio(name, loop)
}

func (p *SpriteImpl) flushPendingAudios(buffer []string) []string {
	engine.Lock()
	buffer = p.sound().takePendingAudios(buffer)
	engine.Unlock()

	if p.isDestroyed() || p.runtimeState.SyncSprite == nil {
		return buffer[:0]
	}

	for _, audio := range buffer {
		p.playAudio(audio, false)
	}
	return buffer[:0]
}
