package state

import (
	"sync"
	"time"
)

type RuntimeManager struct {
	defaultDebugInstr bool
	defaultDebugEvent bool
	defaultDebugPerf  bool

	defaultPhysicsEnabled bool

	fallbackSchedInMain bool
	fallbackMainSchedAt time.Time

	fallbackImageSizeCache sync.Map
}

func (m *RuntimeManager) Reset() {
	m.defaultDebugInstr = false
	m.defaultDebugEvent = false
	m.defaultDebugPerf = false
	m.defaultPhysicsEnabled = false
	m.fallbackSchedInMain = false
	m.fallbackMainSchedAt = time.Time{}
	m.fallbackImageSizeCache = sync.Map{}
}

func (m *RuntimeManager) Init(debug *GameDebugState, runtime *GameRuntimeState) {
	if debug != nil {
		debug.DebugInstr = m.defaultDebugInstr
		debug.DebugEvent = m.defaultDebugEvent
		debug.DebugPerf = m.defaultDebugPerf
	}
	if runtime != nil {
		runtime.EnabledPhysics = m.defaultPhysicsEnabled
	}
}

func (m *RuntimeManager) SetDefaultDebugFlags(instr, event, perf bool) {
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
	return m.defaultDebugInstr
}

func (m *RuntimeManager) DebugEventEnabled(debug *GameDebugState) bool {
	if debug != nil {
		return debug.DebugEvent
	}
	return m.defaultDebugEvent
}

func (m *RuntimeManager) DebugPerfEnabled(debug *GameDebugState) bool {
	if debug != nil {
		return debug.DebugPerf
	}
	return m.defaultDebugPerf
}

func (m *RuntimeManager) SetPhysicsEnabled(runtime *GameRuntimeState, enabled bool) {
	m.defaultPhysicsEnabled = enabled
	if runtime != nil {
		runtime.EnabledPhysics = enabled
	}
}

func (m *RuntimeManager) PhysicsEnabled(runtime *GameRuntimeState) bool {
	if runtime != nil {
		return runtime.EnabledPhysics
	}
	return m.defaultPhysicsEnabled
}

func (m *RuntimeManager) ResetImageSizeCache(runtime *GameRuntimeState) {
	if runtime != nil {
		runtime.ImageSizeCache = sync.Map{}
		return
	}
	m.fallbackImageSizeCache = sync.Map{}
}

func (m *RuntimeManager) ImageSizeCacheRef(runtime *GameRuntimeState) *sync.Map {
	if runtime != nil {
		return &runtime.ImageSizeCache
	}
	return &m.fallbackImageSizeCache
}

func (m *RuntimeManager) SetSchedInMain(runtime *GameRuntimeState, inMain bool) {
	if runtime != nil {
		runtime.IsSchedInMain = inMain
		return
	}
	m.fallbackSchedInMain = inMain
}

func (m *RuntimeManager) IsSchedInMain(runtime *GameRuntimeState) bool {
	if runtime != nil {
		return runtime.IsSchedInMain
	}
	return m.fallbackSchedInMain
}

func (m *RuntimeManager) SetMainSchedTime(runtime *GameRuntimeState, t time.Time) {
	if runtime != nil {
		runtime.MainSchedTime = t
		return
	}
	m.fallbackMainSchedAt = t
}

func (m *RuntimeManager) MainSchedTime(runtime *GameRuntimeState) time.Time {
	if runtime != nil {
		return runtime.MainSchedTime
	}
	return m.fallbackMainSchedAt
}
