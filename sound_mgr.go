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

type soundId = int64

const invalidSoundId = 0

type soundMgr struct {
	g        *Game
	sounds   map[string]sound
	path2ids map[string][]int64
}

func (p *soundMgr) rt() *runtimeManagers {
	return p.g.rt()
}

func (p *soundMgr) init(g *Game) {
	p.sounds = make(map[string]sound)
	p.path2ids = make(map[string][]int64)
	p.g = g
}

func (p *soundMgr) allocSound() engine.Object {
	return p.rt().audioMgr.CreateAudio()
}

func (p *soundMgr) releaseSound(soundObj engine.Object) {
	if soundObj == 0 {
		return
	}
	p.rt().audioMgr.DestroyAudio(soundObj)
}

func (p *soundMgr) pause(media sound) {
	for _, id := range p.path2ids[media.Path] {
		p.rt().audioMgr.Pause(id)
	}
}

func (p *soundMgr) resume(media sound) {
	for _, id := range p.path2ids[media.Path] {
		p.rt().audioMgr.Resume(id)
	}
}

func (p *soundMgr) stop(media sound) {
	for _, id := range p.path2ids[media.Path] {
		p.rt().audioMgr.Stop(id)
	}
	delete(p.path2ids, media.Path)
}

func (p *soundMgr) stopInstance(soundId soundId) {
	p.rt().audioMgr.Stop(soundId)
}

func (p *soundMgr) play(soundObj engine.Object, media sound, isLoop, isWait bool, owner engine.Object, attenuation, maxDistance float64) soundId {
	var curId soundId = 0
	// Avoid attaching the sound directly to the sprite if it is not using a sound attenuation mode
	if attenuation == 0 {
		owner = 0
	}
	curId = p.rt().audioMgr.PlayWithAttenuation(soundObj, engine.ToAssetPath(media.Path), owner, attenuation, maxDistance)
	p.path2ids[media.Path] = append(p.path2ids[media.Path], curId)
	if isLoop {
		for _, id := range p.path2ids[media.Path] {
			p.rt().audioMgr.SetLoop(id, true)
		}
	} else {
		if isWait {
			for {
				if !p.rt().audioMgr.IsPlaying(curId) {
					break
				}
				engine.WaitNextFrame()
			}
		}
	}
	return curId
}

func (p *soundMgr) stopAll() {
	p.path2ids = make(map[string][]int64)
	p.rt().audioMgr.StopAll()
}

func (p *soundMgr) getEffect(soundObj engine.Object, kind SoundEffectKind) float64 {
	switch kind {
	case SoundPanEffect:
		return p.rt().audioMgr.GetPan(soundObj) * 100
	case SoundPitchEffect:
		return p.rt().audioMgr.GetPitch(soundObj) * 100
	default:
		panic("GetSoundEffect: invalid kind")
	}
}

func (p *soundMgr) setEffect(soundObj engine.Object, kind SoundEffectKind, value float64) {
	val := value / 100
	switch kind {
	case SoundPanEffect:
		p.rt().audioMgr.SetPan(soundObj, val)
	case SoundPitchEffect:
		p.rt().audioMgr.SetPitch(soundObj, val)
	default:
		panic("SetSoundEffect: invalid kind")
	}
}

func (p *soundMgr) changeEffect(soundObj engine.Object, kind SoundEffectKind, delta float64) {
	val := (p.getEffect(soundObj, kind) + delta)
	p.setEffect(soundObj, kind, val)
}

func (p *soundMgr) getVolume(soundObj engine.Object) float64 {
	return p.rt().audioMgr.GetVolume(soundObj) * 100
}

func (p *soundMgr) setVolume(soundObj engine.Object, value float64) {
	val := value / 100
	if val <= 0 {
		val = 0.01
	}
	p.rt().audioMgr.SetVolume(soundObj, val)
}

func (p *soundMgr) changeVolume(soundObj engine.Object, delta float64) {
	value := p.getVolume(soundObj) + delta
	p.setVolume(soundObj, value)
}
