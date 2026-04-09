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

package state

import (
	"sync"
	"time"
)

type RuntimeManager struct {
	mu sync.RWMutex

	defaultDebugInstr bool
	defaultDebugEvent bool
	defaultDebugPerf  bool

	defaultPhysicsEnabled bool

	fallbackSchedInMain bool
	fallbackMainSchedAt time.Time

	fallbackImageSizeCache *sync.Map
}

func (m *RuntimeManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultDebugInstr = false
	m.defaultDebugEvent = false
	m.defaultDebugPerf = false
	m.defaultPhysicsEnabled = false
	m.fallbackSchedInMain = false
	m.fallbackMainSchedAt = time.Time{}
	m.fallbackImageSizeCache = &sync.Map{}
}

func (m *RuntimeManager) Init(debug *GameDebugState, runtime *GameRuntimeState) {
	m.mu.RLock()
	debugInstr := m.defaultDebugInstr
	debugEvent := m.defaultDebugEvent
	debugPerf := m.defaultDebugPerf
	physicsEnabled := m.defaultPhysicsEnabled
	m.mu.RUnlock()

	if debug != nil {
		debug.DebugInstr = debugInstr
		debug.DebugEvent = debugEvent
		debug.DebugPerf = debugPerf
	}
	if runtime != nil {
		runtime.EnabledPhysics = physicsEnabled
	}
}

func (m *RuntimeManager) SetDefaultDebugFlags(instr, event, perf bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultDebugInstr = instr
	m.defaultDebugEvent = event
	m.defaultDebugPerf = perf
}

func (m *RuntimeManager) ApplyDebugFlags(debug *GameDebugState, instr, event, perf bool) {
	if debug == nil {
		return
	}
	debug.DebugInstr = instr
	debug.DebugEvent = event
	debug.DebugPerf = perf
}

func (m *RuntimeManager) DebugInstrEnabled(debug *GameDebugState) bool {
	if debug != nil {
		return debug.DebugInstr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultDebugInstr
}

func (m *RuntimeManager) DebugEventEnabled(debug *GameDebugState) bool {
	if debug != nil {
		return debug.DebugEvent
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultDebugEvent
}

func (m *RuntimeManager) DebugPerfEnabled(debug *GameDebugState) bool {
	if debug != nil {
		return debug.DebugPerf
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultDebugPerf
}

func (m *RuntimeManager) SetPhysicsEnabled(runtime *GameRuntimeState, enabled bool) {
	if runtime != nil {
		runtime.EnabledPhysics = enabled
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultPhysicsEnabled = enabled
}

func (m *RuntimeManager) PhysicsEnabled(runtime *GameRuntimeState) bool {
	if runtime != nil {
		return runtime.EnabledPhysics
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultPhysicsEnabled
}

func (m *RuntimeManager) ResetImageSizeCache(runtime *GameRuntimeState) {
	if runtime != nil {
		runtime.ImageSizeCache = sync.Map{}
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackImageSizeCache = &sync.Map{}
}

func (m *RuntimeManager) ImageSizeCacheRef(runtime *GameRuntimeState) *sync.Map {
	if runtime != nil {
		return &runtime.ImageSizeCache
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fallbackImageSizeCache == nil {
		m.fallbackImageSizeCache = &sync.Map{}
	}
	return m.fallbackImageSizeCache
}

func (m *RuntimeManager) SetSchedInMain(runtime *GameRuntimeState, inMain bool) {
	if runtime != nil {
		runtime.IsSchedInMain = inMain
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackSchedInMain = inMain
}

func (m *RuntimeManager) IsSchedInMain(runtime *GameRuntimeState) bool {
	if runtime != nil {
		return runtime.IsSchedInMain
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fallbackSchedInMain
}

func (m *RuntimeManager) SetMainSchedTime(runtime *GameRuntimeState, t time.Time) {
	if runtime != nil {
		runtime.MainSchedTime = t
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallbackMainSchedAt = t
}

func (m *RuntimeManager) MainSchedTime(runtime *GameRuntimeState) time.Time {
	if runtime != nil {
		return runtime.MainSchedTime
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fallbackMainSchedAt
}
