package spx

import (
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
)

func TestSetupWorldAndWindowClampsBackdropWindowToWorld(t *testing.T) {
	game := &Game{}
	proj := &coreproject.ProjectConfig{
		Map: coreproject.MapConfig{
			Width:  320,
			Height: 180,
			Mode:   "repeat",
		},
		Backdrops: []*coreproject.BackdropConfig{
			{
				CostumeConfig: coreproject.CostumeConfig{
					Path:        "bg.png",
					ImageWidth:  640,
					ImageHeight: 480,
				},
			},
		},
	}

	game.setupWorldAndWindow(proj)

	if game.displayState.WorldWidth != 320 || game.displayState.WorldHeight != 180 {
		t.Fatalf("world size = %dx%d, want 320x180", game.displayState.WorldWidth, game.displayState.WorldHeight)
	}
	if game.displayState.WindowWidth != 320 || game.displayState.WindowHeight != 180 {
		t.Fatalf("window size = %dx%d, want 320x180", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}
	if game.displayState.MinWorldX != -160 || game.displayState.MinWorldY != -90 {
		t.Fatalf("world min = (%d, %d), want (-160, -90)", game.displayState.MinWorldX, game.displayState.MinWorldY)
	}
	if game.displayState.MapMode != coreproject.MapModeRepeat {
		t.Fatalf("map mode = %d, want %d", game.displayState.MapMode, coreproject.MapModeRepeat)
	}
}

func TestSyncPenCanvasToWorldUsesLogicalStageSize(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{}
	game.displayState.WorldWidth = 640
	game.displayState.WorldHeight = 480
	game.displayState.WindowWidth = 480
	game.displayState.WindowHeight = 360

	game.syncPenCanvasToWorld()

	if spy.canvasCalls != 1 || spy.canvasWidth != 640 || spy.canvasHeight != 480 {
		t.Fatalf("pen canvas calls/size = %d/%dx%d, want 1/640x480", spy.canvasCalls, spy.canvasWidth, spy.canvasHeight)
	}
}

func TestSyncPenCanvasToWorldFlushesPendingCommandsFirst(t *testing.T) {
	spy := setupSpyPenMgr(t)
	game := &Game{penSyncBuffer: internalengine.NewPenSyncBuffer(1)}
	game.displayState.WorldWidth = 640
	game.displayState.WorldHeight = 480
	game.queuePenMove(1, mathf.NewVec2(10, 20))

	game.syncPenCanvasToWorld()

	if want := []string{"batch", "canvas"}; !reflect.DeepEqual(spy.events, want) {
		t.Fatalf("events = %v, want %v", spy.events, want)
	}
}
