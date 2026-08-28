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

package audio

import (
	"math"

	"github.com/goplus/spx/v3/internal/engine"
)

const scratchPitchStepsPerOctave = 120

type Backend interface {
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
	IsPlaying(aid int64) bool
	StopAll()
}

type Manager struct {
	backend        Backend
	path2ids       map[string][]int64
	obj2ids        map[engine.Object][]int64
	playbacks      map[int64]playbackInfo
	pendingDestroy map[engine.Object]struct{}
}

type playbackInfo struct {
	path     string
	soundObj engine.Object
	loop     bool
}

func (m *Manager) Init(backend Backend) {
	m.backend = backend
	m.path2ids = make(map[string][]int64)
	m.obj2ids = make(map[engine.Object][]int64)
	m.playbacks = make(map[int64]playbackInfo)
	m.pendingDestroy = make(map[engine.Object]struct{})
}

func (m *Manager) AllocSound() engine.Object {
	return m.backend.CreateAudio()
}

func (m *Manager) ReleaseSound(soundObj engine.Object) {
	if soundObj == 0 {
		return
	}
	_, wasPending := m.pendingDestroy[soundObj]
	m.preparePlaybacksForRelease(soundObj)
	if len(m.obj2ids[soundObj]) == 0 {
		if _, pending := m.pendingDestroy[soundObj]; pending {
			delete(m.pendingDestroy, soundObj)
			m.backend.DestroyAudio(soundObj)
		} else if !wasPending {
			m.backend.DestroyAudio(soundObj)
		}
		return
	}
	m.pendingDestroy[soundObj] = struct{}{}
}

func (m *Manager) Pause(path string) {
	ids := m.pruneDeadIDs(path)
	for _, id := range ids {
		m.backend.Pause(id)
	}
}

func (m *Manager) Resume(path string) {
	ids := m.pruneDeadIDs(path)
	for _, id := range ids {
		m.backend.Resume(id)
	}
}

func (m *Manager) Stop(path string) {
	ids := append([]int64(nil), m.path2ids[path]...)
	for _, id := range ids {
		m.backend.Stop(id)
		m.dropPlaybackTracking(id)
	}
	delete(m.path2ids, path)
}

func (m *Manager) StopID(id int64) {
	if id == 0 {
		return
	}
	m.backend.Stop(id)
	m.removeID(id)
}

func (m *Manager) RestartID(id int64) bool {
	if id == 0 || m.backend == nil {
		return false
	}
	if !m.backend.Restart(id) {
		m.removeID(id)
		return false
	}
	return true
}

func (m *Manager) IsPlaying(id int64) bool {
	if id == 0 {
		return false
	}
	return m.backend.IsPlaying(id)
}

func (m *Manager) PruneStoppedIDs(ids []int64) []int64 {
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

func (m *Manager) Play(
	soundObj engine.Object,
	path string,
	isLoop, isWait bool,
	owner engine.Object,
	attenuation, maxDistance float64,
) int64 {
	if attenuation == 0 {
		owner = 0
	}

	// Scratch keeps one player per sound. Replaying the same sound stops its
	// current playback before starting it again instead of mixing both plays.
	m.Stop(path)
	curID := m.backend.PlayWithAttenuation(soundObj, engine.ToAssetPath(path), owner, attenuation, maxDistance)
	if curID == 0 {
		return 0
	}
	m.trackPlayback(curID, path, soundObj, isLoop)
	m.path2ids[path] = []int64{curID}
	if isLoop {
		m.backend.SetLoop(curID, true)
	} else if isWait {
		for m.backend.IsPlaying(curID) {
			engine.WaitNextFrame()
		}
		m.pruneDeadIDs(path)
	}
	return curID
}

func (m *Manager) StopAll() {
	m.path2ids = make(map[string][]int64)
	m.obj2ids = make(map[engine.Object][]int64)
	m.playbacks = make(map[int64]playbackInfo)
	m.backend.StopAll()
	for soundObj := range m.pendingDestroy {
		m.backend.DestroyAudio(soundObj)
	}
	m.pendingDestroy = make(map[engine.Object]struct{})
}

func (m *Manager) Update() {
	if m.backend == nil || len(m.path2ids) == 0 && len(m.pendingDestroy) == 0 {
		return
	}
	for path := range m.path2ids {
		m.pruneDeadIDs(path)
	}
	for soundObj := range m.pendingDestroy {
		if len(m.obj2ids[soundObj]) == 0 {
			delete(m.pendingDestroy, soundObj)
			m.backend.DestroyAudio(soundObj)
		}
	}
}

func (m *Manager) GetPan(soundObj engine.Object) float64 {
	return m.backend.GetPan(soundObj) * 100
}

func (m *Manager) SetPan(soundObj engine.Object, value float64) {
	m.backend.SetPan(soundObj, value/100)
}

func (m *Manager) ChangePan(soundObj engine.Object, delta float64) {
	m.SetPan(soundObj, m.GetPan(soundObj)+delta)
}

func (m *Manager) GetPitch(soundObj engine.Object) float64 {
	return pitchScaleToScratchEffect(m.backend.GetPitch(soundObj))
}

func (m *Manager) SetPitch(soundObj engine.Object, value float64) {
	m.backend.SetPitch(soundObj, scratchPitchEffectToScale(value))
}

func (m *Manager) ChangePitch(soundObj engine.Object, delta float64) {
	m.SetPitch(soundObj, m.GetPitch(soundObj)+delta)
}

func scratchPitchEffectToScale(value float64) float64 {
	return math.Pow(2, value/scratchPitchStepsPerOctave)
}

func pitchScaleToScratchEffect(scale float64) float64 {
	if scale <= 0 {
		return 0
	}
	return scratchPitchStepsPerOctave * math.Log2(scale)
}

func (m *Manager) GetVolume(soundObj engine.Object) float64 {
	return m.backend.GetVolume(soundObj) * 100
}

func (m *Manager) SetVolume(soundObj engine.Object, value float64) {
	val := value / 100
	if val <= 0 {
		val = 0.01
	}
	m.backend.SetVolume(soundObj, val)
}

func (m *Manager) ChangeVolume(soundObj engine.Object, delta float64) {
	m.SetVolume(soundObj, m.GetVolume(soundObj)+delta)
}

func (m *Manager) pruneDeadIDs(path string) []int64 {
	ids := m.path2ids[path]
	if len(ids) == 0 {
		delete(m.path2ids, path)
		return nil
	}

	live := make([]int64, 0, len(ids))
	for _, id := range ids {
		if m.backend.IsPlaying(id) {
			live = append(live, id)
			continue
		}
		m.dropPlaybackTracking(id)
	}
	if len(live) == 0 {
		delete(m.path2ids, path)
		return nil
	}
	m.path2ids[path] = live
	return live
}

func (m *Manager) removeID(target int64) {
	info, ok := m.playbacks[target]
	if ok {
		ids := m.path2ids[info.path]
		for i, id := range ids {
			if id != target {
				continue
			}
			ids = append(ids[:i], ids[i+1:]...)
			if len(ids) == 0 {
				delete(m.path2ids, info.path)
			} else {
				m.path2ids[info.path] = ids
			}
			break
		}
	}
	m.dropPlaybackTracking(target)
}

func (m *Manager) trackPlayback(id int64, path string, soundObj engine.Object, loop bool) {
	if id == 0 {
		return
	}
	m.playbacks[id] = playbackInfo{
		path:     path,
		soundObj: soundObj,
		loop:     loop,
	}
	m.obj2ids[soundObj] = append(m.obj2ids[soundObj], id)
}

func (m *Manager) dropPlaybackTracking(id int64) {
	info, ok := m.playbacks[id]
	if !ok {
		return
	}
	delete(m.playbacks, id)
	m.dropSoundPlaybackID(info.soundObj, id)
	if len(m.obj2ids[info.soundObj]) == 0 {
		if _, pending := m.pendingDestroy[info.soundObj]; pending {
			delete(m.pendingDestroy, info.soundObj)
			m.backend.DestroyAudio(info.soundObj)
		}
	}
}

func (m *Manager) dropSoundPlaybackID(soundObj engine.Object, id int64) {
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

func (m *Manager) preparePlaybacksForRelease(soundObj engine.Object) {
	ids := m.obj2ids[soundObj]
	if len(ids) == 0 {
		delete(m.obj2ids, soundObj)
		return
	}
	for _, id := range append([]int64(nil), ids...) {
		info, ok := m.playbacks[id]
		if !ok {
			m.dropSoundPlaybackID(soundObj, id)
			continue
		}
		if info.loop {
			m.backend.Stop(id)
			m.removeID(id)
			continue
		}
		if m.backend.IsPlaying(id) {
			continue
		}
		m.removeID(id)
	}
}
