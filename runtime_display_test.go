package spx

import "testing"

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
