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
	"github.com/realdream-ai/mathf"
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
	Music  bool
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
	if opts.Music {
		err = p.playBgm(media, opts.Action)
	} else {
		err = p.playSfx(media)
	}
	return
}

func (p *soundMgr) stopAll() {
	audioMgr.StopAll()
}

func (p *soundMgr) playBgm(media Sound, action PlayAction) (err error) {
	switch action {
	case PlayRewind:
		p.playMusic(media)
	case PlayContinue:
		p.resumeMusic(media)
	case PlayPause:
		p.pauseMusic(media)
	case PlayResume:
		p.resumeMusic(media)
	case PlayStop:
		p.stopMusic(media)
	}
	return
}

func (p *soundMgr) playSfx(media Sound) (err error) {
	audioMgr.PlaySfx(engine.ToAssetPath(media.Path))
	return
}

func (p *soundMgr) playMusic(media Sound) (err error) {
	audioMgr.PlayMusic(engine.ToAssetPath(media.Path))
	return
}

func (p *soundMgr) stopMusic(media Sound) {
	audioMgr.PauseMusic()
}

func (p *soundMgr) pauseMusic(media Sound) {
	audioMgr.PauseMusic()
}

func (p *soundMgr) resumeMusic(media Sound) {
	audioMgr.ResumeMusic()
}

func (p *soundMgr) getVolume() float64 {
	return audioMgr.GetMasterVolume()
}

func (p *soundMgr) setVolume(volume float64) {
	audioMgr.SetMasterVolume(volume)
}

func (p *soundMgr) changeVolume(delta float64) {
	volume := p.getVolume() + delta
	volume = mathf.Clamp01f(volume)
	p.setVolume(volume)
}

func (p *soundMgr) getSfxVolume() float64 {
	return audioMgr.GetSfxVolume()
}

func (p *soundMgr) setSfxVolume(volume float64) {
	audioMgr.SetSfxVolume(volume)
}

func (p *soundMgr) changeSfxVolume(delta float64) {
	volume := p.getSfxVolume() + delta
	volume = mathf.Clamp01f(volume)
	p.setSfxVolume(volume)
}

func (p *soundMgr) getMusicVolume() float64 {
	return audioMgr.GetMusicVolume()
}

func (p *soundMgr) setMusicVolume(volume float64) {
	audioMgr.SetMusicVolume(volume)
}

func (p *soundMgr) changeMusicVolume(delta float64) {
	volume := p.getMusicVolume() + delta
	volume = mathf.Clamp01f(volume)
	p.setMusicVolume(volume)
}

// -------------------------------------------------------------------------------------
