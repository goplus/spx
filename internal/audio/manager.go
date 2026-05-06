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

import "github.com/goplus/spx/v2/internal/engine"

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
	SetLoop(aid int64, loop bool)
	IsPlaying(aid int64) bool
	StopAll()
}

type Manager struct {
	backend  Backend
	path2ids map[string][]int64
}

func (m *Manager) Init(backend Backend) {
	m.backend = backend
	m.path2ids = make(map[string][]int64)
}

func (m *Manager) AllocSound() engine.Object {
	return m.backend.CreateAudio()
}

func (m *Manager) ReleaseSound(soundObj engine.Object) {
	if soundObj == 0 {
		return
	}
	m.backend.DestroyAudio(soundObj)
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
	for _, id := range m.path2ids[path] {
		m.backend.Stop(id)
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

	ids := m.pruneDeadIDs(path)
	curID := m.backend.PlayWithAttenuation(soundObj, engine.ToAssetPath(path), owner, attenuation, maxDistance)
	ids = append(ids, curID)
	m.path2ids[path] = ids
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
	m.backend.StopAll()
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
	return m.backend.GetPitch(soundObj) * 100
}

func (m *Manager) SetPitch(soundObj engine.Object, value float64) {
	m.backend.SetPitch(soundObj, value/100)
}

func (m *Manager) ChangePitch(soundObj engine.Object, delta float64) {
	m.SetPitch(soundObj, m.GetPitch(soundObj)+delta)
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
		}
	}
	if len(live) == 0 {
		delete(m.path2ids, path)
		return nil
	}
	m.path2ids[path] = live
	return live
}

func (m *Manager) removeID(target int64) {
	for path, ids := range m.path2ids {
		live := ids[:0]
		for _, id := range ids {
			if id == target {
				continue
			}
			if m.backend.IsPlaying(id) {
				live = append(live, id)
			}
		}
		if len(live) == 0 {
			delete(m.path2ids, path)
			continue
		}
		m.path2ids[path] = live
	}
}
