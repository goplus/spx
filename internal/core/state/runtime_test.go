package state

import (
	"testing"
	"time"
)

func TestRuntimeManagerInitUsesDefaults(t *testing.T) {
	var mgr RuntimeManager
	mgr.SetDefaultDebugFlags(true, true, false)
	mgr.SetPhysicsEnabled(nil, true)

	var debug GameDebugState
	var runtime GameRuntimeState
	mgr.Init(&debug, &runtime)

	if !debug.DebugInstr || !debug.DebugEvent || debug.DebugPerf {
		t.Fatalf("unexpected debug defaults: %+v", debug)
	}
	if !runtime.EnabledPhysics {
		t.Fatal("expected physics default to be applied")
	}
}

func TestRuntimeManagerPrefersActiveState(t *testing.T) {
	var mgr RuntimeManager
	mgr.SetDefaultDebugFlags(false, false, false)
	mgr.SetPhysicsEnabled(nil, false)

	debug := &GameDebugState{DebugInstr: true, DebugEvent: true}
	runtime := &GameRuntimeState{EnabledPhysics: true}

	if !mgr.DebugInstrEnabled(debug) || !mgr.DebugEventEnabled(debug) {
		t.Fatal("expected active debug state to override defaults")
	}
	if !mgr.PhysicsEnabled(runtime) {
		t.Fatal("expected active runtime state to override defaults")
	}
}

func TestRuntimeManagerFallbackSchedulerState(t *testing.T) {
	var mgr RuntimeManager
	at := time.Unix(123, 0)

	mgr.SetSchedInMain(nil, true)
	mgr.SetMainSchedTime(nil, at)

	if !mgr.IsSchedInMain(nil) {
		t.Fatal("expected fallback sched flag to be true")
	}
	if !mgr.MainSchedTime(nil).Equal(at) {
		t.Fatal("expected fallback sched time to match")
	}
}

func TestRuntimeManagerImageSizeCacheSwitchesByState(t *testing.T) {
	var mgr RuntimeManager

	fallbackCache := mgr.ImageSizeCacheRef(nil)
	fallbackCache.Store("fallback", 1)

	runtime := &GameRuntimeState{}
	mgr.ResetImageSizeCache(runtime)
	runtimeCache := mgr.ImageSizeCacheRef(runtime)

	if runtimeCache == fallbackCache {
		t.Fatal("expected runtime cache to differ from fallback cache")
	}
	if _, ok := runtimeCache.Load("fallback"); ok {
		t.Fatal("runtime cache should not contain fallback entries")
	}
}

func TestRuntimeManagerSetPhysicsEnabledDoesNotMutateDefaultWhenRuntimeProvided(t *testing.T) {
	var mgr RuntimeManager
	mgr.SetPhysicsEnabled(nil, true)

	runtime := &GameRuntimeState{}
	mgr.SetPhysicsEnabled(runtime, false)

	if runtime.EnabledPhysics {
		t.Fatal("expected runtime-specific physics flag to be updated")
	}
	if !mgr.PhysicsEnabled(nil) {
		t.Fatal("expected default physics flag to remain unchanged")
	}
}
