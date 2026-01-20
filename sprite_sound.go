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
// Internal Audio Management
// -----------------------------------------------------------------------------

func (p *SpriteImpl) playAudio(name SoundName, loop bool) soundId {
	return p.components.Sound().playAudio(name, loop)
}

func (p *SpriteImpl) checkSoundObj() {
	p.components.Sound().checkSoundObj()
}

// -----------------------------------------------------------------------------
// Sound Playback Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Play__0(name SoundName, loop bool) {
	p.components.Sound().Play(name, loop)
}

func (p *SpriteImpl) Play__1(name SoundName) {
	p.Play__0(name, false)
}

func (p *SpriteImpl) PlayAndWait(name SoundName) {
	p.components.Sound().PlayAndWait(name)
}

func (p *SpriteImpl) PausePlaying(name SoundName) {
	p.components.Sound().PausePlaying(name)
}

func (p *SpriteImpl) ResumePlaying(name SoundName) {
	p.components.Sound().ResumePlaying(name)
}

func (p *SpriteImpl) StopPlaying(name SoundName) {
	p.components.Sound().StopPlaying(name)
}

// -----------------------------------------------------------------------------
// Sound Volume Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) Volume() float64 {
	return p.components.Sound().GetVolume()
}

func (p *SpriteImpl) SetVolume(volume float64) {
	p.components.Sound().SetVolume(volume)
}

func (p *SpriteImpl) ChangeVolume(delta float64) {
	p.components.Sound().ChangeVolume(delta)
}

// -----------------------------------------------------------------------------
// Sound Effects Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) GetSoundEffect(kind SoundEffectKind) float64 {
	return p.components.Sound().GetSoundEffect(kind)
}

func (p *SpriteImpl) SetSoundEffect(kind SoundEffectKind, value float64) {
	p.components.Sound().SetSoundEffect(kind, value)
}

func (p *SpriteImpl) ChangeSoundEffect(kind SoundEffectKind, delta float64) {
	p.components.Sound().ChangeSoundEffect(kind, delta)
}
