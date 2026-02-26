package spx

import (
	"sync"
	"testing"
	"time"

	"github.com/goplus/spx/v2/internal/engine"
)

func resetStateForTest() {
	engine.SetGame(nil)
	setDefaultDebugFlags(false, false, false)
	defaultPhysicsEnabled = false
	fallbackSchedInMain = false
	fallbackMainSchedAt = time.Time{}
	fallbackImageSizeCache = sync.Map{}
}

func TestDebugFlagsApplyToActiveGame(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	g := &Game{}
	engine.SetGame(g)
	g.initRuntimeState()

	SetDebug(DbgFlagInstr | DbgFlagEvent)

	if !g.debugInstr || !g.debugEvent || g.debugPerf {
		t.Fatalf("unexpected game debug flags: instr=%v event=%v perf=%v", g.debugInstr, g.debugEvent, g.debugPerf)
	}
	if !isDebugInstrEnabled() || !isDebugEventEnabled() || isDebugPerfEnabled() {
		t.Fatal("helper getters did not reflect active game debug flags")
	}
}

func TestPhysicsFlagDefaultsAndActiveGame(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	setPhysicsEnabled(true)
	if !isPhysicsEnabled() {
		t.Fatal("expected default physics flag to be true")
	}

	g := &Game{}
	engine.SetGame(g)
	g.initRuntimeState()

	if !isPhysicsEnabled() || !g.enabledPhysics {
		t.Fatal("expected active game to inherit default physics flag")
	}

	g.setPhysicsEnabled(false)
	if isPhysicsEnabled() {
		t.Fatal("expected active game physics flag to be false")
	}
}

func TestImageSizeCacheSwitchesWithActiveGame(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	fallbackCache := imageSizeCacheRef()
	fallbackCache.Store("fallback-key", 1)

	g := &Game{}
	engine.SetGame(g)
	resetImageSizeCache(g)

	gameCache := imageSizeCacheRef()
	if gameCache == fallbackCache {
		t.Fatal("expected game cache to be different from fallback cache")
	}
	if _, ok := gameCache.Load("fallback-key"); ok {
		t.Fatal("game cache should not contain fallback cache entries")
	}
}

func TestSchedStatePrefersActiveGame(t *testing.T) {
	resetStateForTest()
	t.Cleanup(resetStateForTest)

	setSchedInMain(true)
	setMainSchedTime(time.Unix(100, 0))
	if !isSchedInMainState() || !mainSchedTime().Equal(time.Unix(100, 0)) {
		t.Fatal("fallback scheduler state not applied")
	}

	g := &Game{}
	engine.SetGame(g)
	setSchedInMain(true)
	setMainSchedTime(time.Unix(200, 0))

	if !g.isSchedInMain || !g.mainSchedTime.Equal(time.Unix(200, 0)) {
		t.Fatal("active game scheduler state not applied")
	}
}
