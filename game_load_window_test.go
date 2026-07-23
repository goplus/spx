package spx

import (
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
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
