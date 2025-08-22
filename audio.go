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

type PlayAction int

const (
	PlayRewind PlayAction = iota
	PlayContinue
	PlayPause
	PlayResume
	PlayStop
)

type soundMgr struct {
	g        *Game
	audios   map[string]sound
	path2ids map[string][]int64
}

func (p *soundMgr) init(g *Game) {
	p.audios = make(map[string]sound)
	p.path2ids = make(map[string][]int64)
	p.g = g
}

func (p *soundMgr) allocAudio() engine.Object {
	return audioMgr.CreateAudio()
}

func (p *soundMgr) releaseAudio(audioId engine.Object) {
	if audioId == 0 {
		return
	}
	audioMgr.DestroyAudio(audioId)
}
func (p *soundMgr) pause(audioId engine.Object, media sound) {
	for _, id := range p.path2ids[media.Path] {
		audioMgr.Pause(id)
	}
}
func (p *soundMgr) resume(audioId engine.Object, media sound) {
	for _, id := range p.path2ids[media.Path] {
		audioMgr.Resume(id)
	}
}
func (p *soundMgr) stop(audioId engine.Object, media sound) {
	for _, id := range p.path2ids[media.Path] {
		audioMgr.Stop(id)
	}
	delete(p.path2ids, media.Path)
}

func (p *soundMgr) play(audioId engine.Object, media sound, isLoop, isWait bool) {
	var curId int64 = 0
	curId = audioMgr.Play(audioId, engine.ToAssetPath(media.Path))
	p.path2ids[media.Path] = append(p.path2ids[media.Path], curId)
	if isLoop {
		for _, id := range p.path2ids[media.Path] {
			audioMgr.SetLoop(id, true)
		}
	} else {
		if isWait {
			for {
				if !audioMgr.IsPlaying(curId) {
					break
				}
				engine.WaitNextFrame()
			}
		}
	}
}

func (p *soundMgr) stopAll() {
	p.path2ids = make(map[string][]int64)
	audioMgr.StopAll()
}

func (p *soundMgr) getEffect(audioId engine.Object, kind SoundEffectKind) float64 {
	switch kind {
	case SoundPanEffect:
		return audioMgr.GetPan(audioId) * 100
	case SoundPitchEffect:
		return audioMgr.GetPitch(audioId) * 100
	default:
		panic("GetSoundEffect: invalid kind")
	}
}

func (p *soundMgr) setEffect(audioId engine.Object, kind SoundEffectKind, value float64) {
	val := value / 100
	switch kind {
	case SoundPanEffect:
		audioMgr.SetPan(audioId, val)
	case SoundPitchEffect:
		audioMgr.SetPitch(audioId, val)
	default:
		panic("SetSoundEffect: invalid kind")
	}
}
func (p *soundMgr) changeEffect(audioId engine.Object, kind SoundEffectKind, delta float64) {
	val := (p.getEffect(audioId, kind) + delta)
	p.setEffect(audioId, kind, val)
}

func (p *soundMgr) getVolume(audioId engine.Object) float64 {
	return audioMgr.GetVolume(audioId) * 100
}

func (p *soundMgr) setVolume(audioId engine.Object, value float64) {
	val := value / 100
	if val <= 0 {
		val = 0.01
	}
	audioMgr.SetVolume(audioId, val)
}

func (p *soundMgr) changeVolume(audioId engine.Object, delta float64) {
	value := p.getVolume(audioId) + delta
	p.setVolume(audioId, value)
}

// -------------------------------------------------------------------------------------
