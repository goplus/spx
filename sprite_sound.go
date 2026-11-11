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

import "github.com/goplus/spx/v4/internal/engine"

// -----------------------------------------------------------------------------
// Sound Effects
// -----------------------------------------------------------------------------
type SoundEffectKind int

const (
	SoundPanEffect SoundEffectKind = iota
	SoundPitchEffect
)

// -----------------------------------------------------------------------------
// Playback
// -----------------------------------------------------------------------------
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
	p.g.pauseSound(sound)
}

func (p *SpriteImpl) ResumePlaying(sound SoundName) {
	p.g.resumeSound(sound)
}

func (p *SpriteImpl) StopPlaying(sound SoundName) {
	p.g.stopSound(sound)
}

// -----------------------------------------------------------------------------
// Volume
// -----------------------------------------------------------------------------
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

// -----------------------------------------------------------------------------
// Effect Control
// -----------------------------------------------------------------------------
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

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------
func (p *SpriteImpl) queueAnimationLoopAudio(state *animState) {
	if state == nil {
		return
	}
	if state.OnStartReplayAudioName != "" {
		p.sound().addPendingAudio(state.OnStartReplayAudioName)
	}
	if state.OnPlayReplayAudioName != "" {
		state.OnPlayAudioRestartPending = true
	}
}

func (p *SpriteImpl) flushPendingAudios(buffer []string) []string {
	engine.Lock()
	buffer = p.sound().takePendingAudios(buffer)
	pendingOnPlayAudioStates := p.animation().takePendingOnPlayAudioStates()
	engine.Unlock()

	if p.isDestroyed() || p.runtimeState.syncSprite == nil {
		return buffer[:0]
	}

	for _, audio := range buffer {
		p.sound().play(audio, false)
	}
	for _, state := range pendingOnPlayAudioStates {
		p.animation().restartOnPlayAudio(state)
	}
	return buffer[:0]
}
