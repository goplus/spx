/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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
	"github.com/goplus/spx/internal/engine"
)

type PlayAction int

const (
	PlayRewind PlayAction = iota
	PlayContinue
	PlayPause
	PlayResume
	PlayStop
)

type PlayOptions struct {
	Action PlayAction
	Wait   bool
	Loop   bool
}

type soundMgr struct {
	g      *Game
	audios map[string]Sound
}

func (p *soundMgr) init(g *Game) {
	p.audios = make(map[string]Sound)
	p.g = g
}

func (p *soundMgr) play(media Sound, opts *PlayOptions) (err error) {
	err = p.playSfx(media, opts.Wait, false)
	return
}

func (p *soundMgr) stopAll() {
	engine.SyncAudioStopAll()
}

func (p *soundMgr) playSfx(media Sound, wait, loop bool) (err error) {
	engine.SyncAudioPlaySfx(engine.ToAssetPath(media.Path))
	return
}

func (p *soundMgr) stopMusic(media Sound) {
	engine.SyncAudioPauseMusic()
}

func (p *soundMgr) pauseMusic(media Sound) {
	engine.SyncAudioPauseMusic()
}

func (p *soundMgr) resumeMusic(media Sound) {
	engine.SyncAudioResumeMusic()
}

func (p *soundMgr) getVolume() float64 {
	volume := engine.SyncAudioGetMasterVolume()
	return float64(volume)
}

func (p *soundMgr) setVolume(volume float64) {
	engine.SyncAudioSetMasterVolume(float32(volume))
}

func (p *soundMgr) changeVolume(delta float64) {
	volume := p.getVolume() + delta
	volume = engine.Clamp01d(volume)
	p.setVolume(volume)
}

func (p *soundMgr) getSfxVolume() float64 {
	volume := engine.SyncAudioGetSfxVolume()
	return float64(volume)
}

func (p *soundMgr) setSfxVolume(volume float64) {
	engine.SyncAudioSetSfxVolume(float32(volume))
}

func (p *soundMgr) changeSfxVolume(delta float64) {
	volume := p.getSfxVolume() + delta
	volume = engine.Clamp01d(volume)
	p.setSfxVolume(volume)
}

func (p *soundMgr) getMusicVolume() float64 {
	volume := engine.SyncAudioGetMusicVolume()
	return float64(volume)
}

func (p *soundMgr) setMusicVolume(volume float64) {
	engine.SyncAudioSetMusicVolume(float32(volume))
}

func (p *soundMgr) changeMusicVolume(delta float64) {
	volume := p.getMusicVolume() + delta
	volume = engine.Clamp01d(volume)
	p.setMusicVolume(volume)
}

// -------------------------------------------------------------------------------------
