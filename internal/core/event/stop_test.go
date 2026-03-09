package event

import "testing"

func TestResolveStop(t *testing.T) {
	isSprite := func(obj any) bool { return obj == "sprite" || obj == "owner" }
	isGame := func(obj any) bool { return obj == "game" }

	filter, abort := ResolveStop(AllSprites, "owner", isSprite, isGame)
	if abort || filter == nil || !filter("sprite", false) || filter("game", false) {
		t.Fatal("AllSprites resolution mismatch")
	}

	filter, abort = ResolveStop(ThisSprite, "owner", isSprite, isGame)
	if abort || filter == nil || !filter("owner", false) || filter("other", false) {
		t.Fatal("ThisSprite resolution mismatch")
	}

	filter, abort = ResolveStop(OtherScriptsInSprite, "owner", isSprite, isGame)
	if abort || filter == nil || !filter("owner", false) || filter("owner", true) {
		t.Fatal("OtherScriptsInSprite resolution mismatch")
	}

	filter, abort = ResolveStop(AllOtherScripts, "owner", isSprite, isGame)
	if abort || filter == nil || !filter("sprite", false) || !filter("game", false) || filter("sprite", true) {
		t.Fatal("AllOtherScripts resolution mismatch")
	}

	filter, abort = ResolveStop(AllStop, "owner", isSprite, isGame)
	if !abort || filter == nil || !filter("sprite", true) || !filter("game", true) || filter("other", false) {
		t.Fatal("AllStop resolution mismatch")
	}

	filter, abort = ResolveStop(ThisScript, "owner", isSprite, isGame)
	if !abort || filter != nil {
		t.Fatal("ThisScript resolution mismatch")
	}
}
