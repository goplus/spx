package impl

import (
	"reflect"
	"testing"

	enginewrap "github.com/goplus/spx/v4/internal/engine/wrap"
	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestCreateManagersReturnsIndependentOrderedSlices(t *testing.T) {
	want := []engine.IManager{
		&audioMgr{}, &cameraMgr{}, &debugMgr{}, &extMgr{}, &inputMgr{},
		&navigationMgr{}, &penMgr{}, &physicsMgr{}, &platformMgr{}, &resMgr{},
		&sceneMgr{}, &spriteMgr{}, &tilemapMgr{}, &tilemapparserMgr{}, &uiMgr{},
	}
	first := CreateManagers()
	if len(first) != len(want) {
		t.Fatalf("manager count = %d, want %d", len(first), len(want))
	}
	for i, mgr := range first {
		if reflect.TypeOf(mgr) != reflect.TypeOf(want[i]) {
			t.Errorf("manager %d = %T, want %T", i, mgr, want[i])
		}
	}

	first[0] = nil
	second := CreateManagers()
	if first[0] != nil {
		t.Fatal("constructing managers overwrote a previous result")
	}
	if second[0] == nil {
		t.Fatal("constructing managers reused a modified result")
	}
	second[1] = nil
	if first[1] == nil {
		t.Fatal("modifying managers changed a previous result")
	}
}

func TestCreateManagersLifecycleDoesNotBind(t *testing.T) {
	previous := engine.AudioMgr
	t.Cleanup(func() { engine.AudioMgr = previous })
	engine.AudioMgr = nil
	managers := CreateManagers()
	if engine.AudioMgr != nil {
		t.Fatal("constructing managers bound the audio manager")
	}
	for _, mgr := range managers {
		t.Run(reflect.TypeOf(mgr).Elem().Name(), func(t *testing.T) {
			// Lifecycle callbacks must remain safe before native bindings exist.
			mgr.OnStart()
			mgr.OnUpdate(1.0 / 60)
			mgr.OnFixedUpdate(1.0 / 60)
			mgr.OnPause(true)
			mgr.OnPause(false)
			mgr.OnDestroy()
			if engine.AudioMgr != nil {
				t.Fatal("manager lifecycle bound the audio manager")
			}
		})
	}
	unbind := enginewrap.BindManagers(managers[:1])
	t.Cleanup(unbind)
	if engine.AudioMgr == nil || engine.AudioMgr == managers[0].(engine.IAudioMgr) {
		t.Fatal("BindManagers did not wrap the constructed audio manager")
	}
	unbind()
	if engine.AudioMgr != nil {
		t.Fatal("unbinding retained the public audio manager")
	}
}

func TestBoundManagersDispatchBeforeBackendCalls(t *testing.T) {
	previousAudio := engine.AudioMgr
	t.Cleanup(func() {
		engine.AudioMgr = previousAudio
		// This package's tests do not install an engine runtime.
		enginewrap.Init(nil)
	})
	t.Cleanup(enginewrap.BindManagers([]engine.IManager{new(audioMgr)}))
	var managers enginewrap.EngineManagers
	for _, test := range []struct {
		name string
		call func()
	}{
		{"direct setter", func() { engine.AudioMgr.SetVolume(1, 0.75) }},
		{"wrapped setter", func() { managers.AudioMgr.SetVolume(1, 0.75) }},
		{"direct getter", func() { engine.AudioMgr.GetVolume(1) }},
		{"wrapped getter", func() { managers.AudioMgr.GetVolume(1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dispatches := 0
			enginewrap.Init(func(func()) {
				dispatches++
				// Keep unbound backend functions behind the dispatch boundary.
			})
			test.call()
			if dispatches != 1 {
				t.Fatalf("dispatches = %d, want one", dispatches)
			}
		})
	}
}
