/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
