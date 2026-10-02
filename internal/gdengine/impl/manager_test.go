package impl

import (
	"reflect"
	"testing"

	"github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestCreateMgrsOrder(t *testing.T) {
	want := []engine.IManager{
		&audioMgr{}, &cameraMgr{}, &debugMgr{}, &extMgr{}, &inputMgr{},
		&navigationMgr{}, &penMgr{}, &physicsMgr{}, &platformMgr{}, &resMgr{},
		&sceneMgr{}, &spriteMgr{}, &tilemapMgr{}, &tilemapparserMgr{}, &uiMgr{},
	}
	got := CreateMgrs()
	if len(got) != len(want) {
		t.Fatalf("manager count = %d, want %d", len(got), len(want))
	}
	for i, mgr := range got {
		if reflect.TypeOf(mgr) != reflect.TypeOf(want[i]) {
			t.Errorf("manager %d = %T, want %T", i, mgr, want[i])
		}
	}
}

func TestCreateMgrsOwnsResultSlice(t *testing.T) {
	first := CreateMgrs()
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

func TestCreateMgrsDoesNotBind(t *testing.T) {
	previous := engine.AudioMgr
	t.Cleanup(func() { engine.AudioMgr = previous })
	engine.AudioMgr = nil
	managers := CreateMgrs()
	if engine.AudioMgr != nil {
		t.Fatal("constructing managers bound the audio manager")
	}
	BindMgr(managers[:1])
	if engine.AudioMgr != managers[0].(engine.IAudioMgr) {
		t.Fatal("BindMgr did not bind the constructed audio manager")
	}
}
