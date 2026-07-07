package spx

import (
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestGetCostumeRenderOffsetUsesPivot(t *testing.T) {
	costume := &costume{
		width:            200,
		height:           120,
		bitmapResolution: 2,
		center:           mathf.NewVec2(100, 60),
		pivot:            mathf.NewVec2(10, -5),
	}

	x, y := getCostumeRenderOffset(costume, costume.pivot, 1, 1)
	if x != -10 || y != 5 {
		t.Fatalf("getCostumeRenderOffset = (%v, %v), want (-10, 5)", x, y)
	}
}

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
