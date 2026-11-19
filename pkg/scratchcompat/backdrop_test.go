/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package scratchcompat_test

import (
	"testing"

	"github.com/goplus/spx/v4"
	"github.com/goplus/spx/v4/pkg/scratchcompat"
)

type customBackdropResolver struct{ backdrop string }

func (r *customBackdropResolver) ResolveBackdropIndex(backdrop string) int {
	if r == nil {
		return -7
	}
	r.backdrop = backdrop
	return 23
}

type overridingBackdropGame struct {
	spx.Game
	backdrop string
}

func (g *overridingBackdropGame) ResolveBackdropIndex(backdrop string) int {
	g.backdrop = backdrop
	return 42
}

func TestResolveBackdropIndexAcceptsGames(t *testing.T) {
	var game spx.Game
	resolver := scratchcompat.BackdropResolver(&game)
	for name, want := range map[string]int{"3": 2, "-2": -3, "missing": -1} {
		if got := scratchcompat.ResolveBackdropIndex(resolver, name); got != want {
			t.Errorf("ResolveBackdropIndex(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestResolveBackdropIndexPreservesMethodDispatch(t *testing.T) {
	custom := &customBackdropResolver{}
	if got := scratchcompat.ResolveBackdropIndex(custom, "custom"); got != 23 || custom.backdrop != "custom" {
		t.Fatalf("custom resolver = (%d, %q), want (23, custom)", got, custom.backdrop)
	}
	override := &overridingBackdropGame{}
	var game scratchcompat.BackdropResolver = override
	if got := scratchcompat.ResolveBackdropIndex(game, "override"); got != 42 || override.backdrop != "override" {
		t.Fatalf("game override = (%d, %q), want (42, override)", got, override.backdrop)
	}
}

func TestResolveBackdropIndexPreservesTypedNilBehavior(t *testing.T) {
	t.Run("custom nil receiver", func(t *testing.T) {
		var resolver *customBackdropResolver
		if got := scratchcompat.ResolveBackdropIndex(resolver, "custom"); got != -7 {
			t.Fatalf("nil custom resolver = %d, want -7", got)
		}
	})
	t.Run("nil Game panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("nil Game did not panic")
			}
		}()
		var game *spx.Game
		scratchcompat.ResolveBackdropIndex(game, "3")
	})
}
