package impl

import (
	"reflect"
	"testing"

	"github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestCreateMgrsReturnsIndependentOrderedSlices(t *testing.T) {
	want := []engine.IManager{
		&audioMgr{}, &cameraMgr{}, &debugMgr{}, &extMgr{}, &inputMgr{},
		&navigationMgr{}, &penMgr{}, &physicsMgr{}, &platformMgr{}, &resMgr{},
		&sceneMgr{}, &spriteMgr{}, &tilemapMgr{}, &tilemapparserMgr{}, &uiMgr{},
	}
	first := CreateMgrs()
	if len(first) != len(want) {
		t.Fatalf("manager count = %d, want %d", len(first), len(want))
	}
	for i, mgr := range first {
		if reflect.TypeOf(mgr) != reflect.TypeOf(want[i]) {
			t.Errorf("manager %d = %T, want %T", i, mgr, want[i])
		}
	}

	first[0] = nil
	second := CreateMgrs()
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

func TestCreateMgrsLifecycleDoesNotBind(t *testing.T) {
	previous := engine.AudioMgr
	t.Cleanup(func() { engine.AudioMgr = previous })
	engine.AudioMgr = nil
	managers := CreateMgrs()
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
	BindMgr(managers[:1])
	if engine.AudioMgr != managers[0].(engine.IAudioMgr) {
		t.Fatal("BindMgr did not bind the constructed audio manager")
	}
}
