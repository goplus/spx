package spx

import (
	"testing"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

func TestGameGoSetBackdropRandomChoosesDifferentBackdrop(t *testing.T) {
	game := &Game{
		baseObj: baseObj{
			costumes: []*costume{
				{name: "backdrop1"},
				{name: "backdrop2"},
				{name: "backdrop3"},
			},
			costumeIndex: 1,
		},
	}

	SetRandomSeed(1)
	defer ResetRandomSeed()
	if ok := game.goSetBackdrop(Random); !ok {
		t.Fatal("goSetBackdrop(Random) = false, want true")
	}

	if got := game.costumeIndex; got < 0 || got >= len(game.costumes) {
		t.Fatalf("costumeIndex = %d, want in [0, %d)", got, len(game.costumes))
	}
	if got := game.costumeIndex; got == 1 {
		t.Fatalf("costumeIndex = %d, want a different backdrop", got)
	}
}

func TestGameGoSetBackdropRandomFloat64ChoosesDifferentBackdrop(t *testing.T) {
	game := &Game{
		baseObj: baseObj{
			costumes: []*costume{
				{name: "backdrop1"},
				{name: "backdrop2"},
			},
			costumeIndex: 0,
		},
	}

	SetRandomSeed(1)
	defer ResetRandomSeed()
	if ok := game.goSetBackdrop(float64(Random)); !ok {
		t.Fatal("goSetBackdrop(float64(Random)) = false, want true")
	}

	if got := game.costumeIndex; got != 1 {
		t.Fatalf("costumeIndex = %d, want 1", got)
	}
}

func TestGameSetRandomBackdropWithNoBackdropsFails(t *testing.T) {
	var game Game
	if ok := game.setRandomBackdrop(); ok {
		t.Fatal("setRandomBackdrop() = true, want false")
	}
}

func TestGameSetRandomBackdropWithSingleBackdropIsNoOp(t *testing.T) {
	game := &Game{
		baseObj: baseObj{
			costumes:     []*costume{{name: "backdrop1"}},
			costumeIndex: 0,
		},
	}

	if ok := game.setRandomBackdrop(); !ok {
		t.Fatal("setRandomBackdrop() = false, want true")
	}
	if got := game.costumeIndex; got != 0 {
		t.Fatalf("costumeIndex = %d, want 0", got)
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
