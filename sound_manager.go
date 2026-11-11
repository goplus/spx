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
	"math"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
)

const pitchStepsPerOctave = 120

type soundBackend interface {
	CreateAudio() engine.Object
	DestroyAudio(obj engine.Object)
	SetPitch(obj engine.Object, pitch float64)
	GetPitch(obj engine.Object) float64
	SetPan(obj engine.Object, pan float64)
	GetPan(obj engine.Object) float64
	SetVolume(obj engine.Object, volume float64)
	GetVolume(obj engine.Object) float64
	PlayWithAttenuation(obj engine.Object, path string, ownerID engine.Object, attenuation, maxDistance float64) int64
	Pause(aid int64)
	Resume(aid int64)
	Stop(aid int64)
	Restart(aid int64) bool
	SetLoop(aid int64, loop bool)
	// IsPlaying reports unfinished playback, including paused playback.
	IsPlaying(aid int64) bool
	StopAll()
}

type soundManager struct {
	backend        soundBackend
	path2id        map[string]int64
	obj2ids        map[engine.Object][]int64
	playbacks      map[int64]soundPlaybackInfo
	pendingDestroy map[engine.Object]struct{}
}

type soundPlaybackInfo struct {
	path     string
	soundObj engine.Object
	loop     bool
}

func (m *soundManager) Init(backend soundBackend) {
	m.backend = backend
	m.path2id = make(map[string]int64)
	m.obj2ids = make(map[engine.Object][]int64)
	m.playbacks = make(map[int64]soundPlaybackInfo)
	m.pendingDestroy = make(map[engine.Object]struct{})
}

func (m *soundManager) AllocSound() engine.Object {
	return m.backend.CreateAudio()
}

// CloneSoundEffects allocates independent effects without copying volume or
// playback. An unallocated source stays unallocated.
func (m *soundManager) CloneSoundEffects(source engine.Object) engine.Object {
	if source == invalidSoundObject {
		return invalidSoundObject
	}
	target := m.AllocSound()
	m.backend.SetPan(target, m.backend.GetPan(source))
	m.backend.SetPitch(target, m.backend.GetPitch(source))
	return target
}

func (m *soundManager) ReleaseSound(soundObj engine.Object) {
	if soundObj == invalidSoundObject {
		return
	}
	m.pendingDestroy[soundObj] = struct{}{}
	m.preparePlaybacksForRelease(soundObj)
	m.destroyReleasedSoundIfIdle(soundObj)
}

func (m *soundManager) Pause(path string) {
	if id := m.pruneDeadID(path); id != invalidPlaybackID {
		m.backend.Pause(id)
	}
}

func (m *soundManager) Resume(path string) {
	if id := m.pruneDeadID(path); id != invalidPlaybackID {
		m.backend.Resume(id)
	}
}

func (m *soundManager) Stop(path string) {
	m.StopID(m.path2id[path])
	delete(m.path2id, path)
}

func (m *soundManager) StopID(id int64) {
	if id == invalidPlaybackID {
		return
	}
	m.backend.Stop(id)
	m.removeID(id)
}

func (m *soundManager) RestartID(id int64) bool {
	if id == invalidPlaybackID || m.backend == nil {
		return false
	}
	if !m.backend.Restart(id) {
		m.removeID(id)
		return false
	}
	return true
}

func (m *soundManager) IsPlaying(id int64) bool {
	if id == invalidPlaybackID {
		return false
	}
	return m.backend.IsPlaying(id)
}

func (m *soundManager) PruneStoppedIDs(ids []int64) []int64 {
	if len(ids) == 0 || m.backend == nil {
		return ids
	}
	live := ids[:0]
	for _, id := range ids {
		if m.backend.IsPlaying(id) {
			live = append(live, id)
			continue
		}
		m.removeID(id)
	}
	return live
}

func (m *soundManager) Play(
	soundObj engine.Object,
	path string,
	isLoop, isWait bool,
	owner engine.Object,
	attenuation, maxDistance float64,
) int64 {
	if attenuation == 0 {
		owner = invalidSoundObject
	}

	// Scratch keeps one player per sound. Replaying the same sound stops its
	// current playback before starting it again instead of mixing both plays.
	m.Stop(path)
	curID := m.backend.PlayWithAttenuation(soundObj, engine.ToAssetPath(path), owner, attenuation, maxDistance)
	if curID == invalidPlaybackID {
		return invalidPlaybackID
	}
	m.trackPlayback(curID, path, soundObj, isLoop)
	m.path2id[path] = curID
	if isLoop {
		m.backend.SetLoop(curID, true)
	} else if isWait {
		for m.backend.IsPlaying(curID) {
			engine.WaitNextFrame()
		}
		m.pruneDeadID(path)
	}
	return curID
}

func (m *soundManager) StopAll() {
	clear(m.path2id)
	clear(m.obj2ids)
	clear(m.playbacks)
	if m.backend != nil {
		m.backend.StopAll()
	}
	for soundObj := range m.pendingDestroy {
		m.destroyReleasedSoundIfIdle(soundObj)
	}
}

func (m *soundManager) Update() {
	if m.backend == nil {
		return
	}
	for path := range m.path2id {
		m.pruneDeadID(path)
	}
}

func (m *soundManager) GetPan(soundObj engine.Object) float64 {
	return m.backend.GetPan(soundObj) * 100
}

func (m *soundManager) SetPan(soundObj engine.Object, value float64) {
	m.backend.SetPan(soundObj, clampSoundControl(value, -100, 100)/100)
}

func (m *soundManager) GetPitch(soundObj engine.Object) float64 {
	scale := m.backend.GetPitch(soundObj)
	if scale <= 0 {
		return 0
	}
	return pitchStepsPerOctave * math.Log2(scale)
}

func (m *soundManager) SetPitch(soundObj engine.Object, value float64) {
	value = clampSoundControl(value, -360, 360)
	m.backend.SetPitch(soundObj, math.Pow(2, value/pitchStepsPerOctave))
}

func (m *soundManager) GetVolume(soundObj engine.Object) float64 {
	return m.backend.GetVolume(soundObj) * 100
}

func (m *soundManager) SetVolume(soundObj engine.Object, value float64) {
	m.backend.SetVolume(soundObj, clampSoundControl(value, 0, 100)/100)
}

func (m *soundManager) ChangeVolume(soundObj engine.Object, delta float64) {
	if math.IsNaN(delta) {
		delta = 0
	}
	m.SetVolume(soundObj, m.GetVolume(soundObj)+delta)
}

func (m *soundManager) destroyReleasedSoundIfIdle(soundObj engine.Object) {
	if _, pending := m.pendingDestroy[soundObj]; !pending || len(m.obj2ids[soundObj]) != 0 {
		return
	}
	delete(m.pendingDestroy, soundObj)
	m.backend.DestroyAudio(soundObj)
}

func (m *soundManager) pruneDeadID(path string) int64 {
	id := m.path2id[path]
	if id != invalidPlaybackID && m.backend.IsPlaying(id) {
		return id
	}
	m.removeID(id)
	delete(m.path2id, path)
	return invalidPlaybackID
}

func (m *soundManager) trackPlayback(id int64, path string, soundObj engine.Object, loop bool) {
	if id == invalidPlaybackID {
		return
	}
	m.playbacks[id] = soundPlaybackInfo{
		path:     path,
		soundObj: soundObj,
		loop:     loop,
	}
	m.obj2ids[soundObj] = append(m.obj2ids[soundObj], id)
}

func (m *soundManager) removeID(id int64) {
	info, ok := m.playbacks[id]
	if !ok {
		return
	}
	if m.path2id[info.path] == id {
		delete(m.path2id, info.path)
	}
	delete(m.playbacks, id)
	m.dropSoundPlaybackID(info.soundObj, id)
	m.destroyReleasedSoundIfIdle(info.soundObj)
}

func (m *soundManager) dropSoundPlaybackID(soundObj engine.Object, id int64) {
	ids := m.obj2ids[soundObj]
	for i, playbackID := range ids {
		if playbackID != id {
			continue
		}
		ids = append(ids[:i], ids[i+1:]...)
		if len(ids) == 0 {
			delete(m.obj2ids, soundObj)
		} else {
			m.obj2ids[soundObj] = ids
		}
		break
	}
}

func (m *soundManager) preparePlaybacksForRelease(soundObj engine.Object) {
	for _, id := range append([]int64(nil), m.obj2ids[soundObj]...) {
		info, ok := m.playbacks[id]
		if !ok {
			m.dropSoundPlaybackID(soundObj, id)
			continue
		}
		if info.loop {
			m.backend.Stop(id)
		} else if m.backend.IsPlaying(id) {
			continue
		}
		m.removeID(id)
	}
}

func clampSoundControl(value, low, high float64) float64 {
	if math.IsNaN(value) {
		value = 0
	}
	return mathf.Clamp(value, low, high)
}
