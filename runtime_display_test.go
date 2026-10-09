package spx

import (
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func TestDisplaySizesDefaultToBackdropAndPreserveConfiguredValues(t *testing.T) {
	for _, tt := range []struct {
		name          string
		display       gameDisplayState
		window, world [2]int
	}{
		{
			name: "backdrop defaults", window: [2]int{320, 240}, world: [2]int{320, 240},
		},
		{
			name:    "configured window",
			display: gameDisplayState{WindowWidth: 384, WindowHeight: 216},
			window:  [2]int{384, 216}, world: [2]int{320, 240},
		},
		{
			name:    "configured world",
			display: gameDisplayState{WorldWidth: 1024, WorldHeight: 768},
			window:  [2]int{320, 240}, world: [2]int{1024, 768},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			game := &Game{
				baseObj: baseObj{costumes: []*costume{newCostume(&coreproject.CostumeConfig{
					ImageWidth:       641.5,
					ImageHeight:      481.25,
					BitmapResolution: 2,
				})}},
				displayState: tt.display,
			}

			windowW, windowH := game.windowSize()
			if got := [2]int{windowW, windowH}; got != tt.window {
				t.Fatalf("windowSize() = %v, want %v", got, tt.window)
			}
			worldW, worldH := game.worldSize()
			if got := [2]int{worldW, worldH}; got != tt.world {
				t.Fatalf("worldSize() = %v, want %v", got, tt.world)
			}
		})
	}
}

func TestGameGoSetBackdropRandom(t *testing.T) {
	for _, test := range []struct {
		name     string
		selector any
		costumes []*costume
		initial  int
	}{
		{
			name: "action", selector: Random, initial: 1,
			costumes: []*costume{{name: "backdrop1"}, {name: "backdrop2"}, {name: "backdrop3"}},
		},
		{
			name: "float64", selector: float64(Random),
			costumes: []*costume{{name: "backdrop1"}, {name: "backdrop2"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &Game{baseObj: baseObj{costumes: test.costumes, costumeIndex: test.initial}}
			SetRandomSeed(1)
			t.Cleanup(ResetRandomSeed)

			if !game.goSetBackdrop(test.selector) {
				t.Fatal("goSetBackdrop(Random) = false, want true")
			}
			if got := game.costumeIndex; got < 0 || got >= len(game.costumes) || got == test.initial {
				t.Fatalf("costumeIndex = %d, want in [0, %d) and different from %d", got, len(game.costumes), test.initial)
			}
		})
	}
}

func TestGameSetRandomBackdropWithoutChoice(t *testing.T) {
	for _, test := range []struct {
		name     string
		costumes []*costume
		wantOK   bool
	}{
		{name: "no backdrops"},
		{name: "single backdrop", costumes: []*costume{{name: "backdrop1"}}, wantOK: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := &Game{baseObj: baseObj{costumes: test.costumes}}
			if got := game.setRandomBackdrop(); got != test.wantOK {
				t.Fatalf("setRandomBackdrop() = %v, want %v", got, test.wantOK)
			}
			if got := game.costumeIndex; got != 0 {
				t.Fatalf("costumeIndex = %d, want unchanged index 0", got)
			}
		})
	}
}

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
